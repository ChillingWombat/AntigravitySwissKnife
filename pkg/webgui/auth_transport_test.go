package webgui

import (
	"net/http"
	"runtime"
	"strings"
	"sync"
)

var (
	testServersMu sync.RWMutex
	testServers   = make(map[string]string) // addr -> token
)

func init() {
	onServerStartHook = func(addr, token string) {
		testServersMu.Lock()
		testServers[addr] = token
		if strings.HasPrefix(addr, "127.0.0.1:") {
			testServers["localhost:"+strings.TrimPrefix(addr, "127.0.0.1:")] = token
		}
		testServersMu.Unlock()
	}

	onServerStopHook = func(addr string) {
		testServersMu.Lock()
		delete(testServers, addr)
		if strings.HasPrefix(addr, "127.0.0.1:") {
			delete(testServers, "localhost:"+strings.TrimPrefix(addr, "127.0.0.1:"))
		}
		testServersMu.Unlock()
	}

	origTransport := http.DefaultTransport
	if origTransport == nil {
		origTransport = &http.Transport{}
	}

	wrapped := &testAuthRoundTripper{
		orig: origTransport,
	}

	http.DefaultTransport = wrapped
	http.DefaultClient.Transport = wrapped
}

type testAuthRoundTripper struct {
	orig http.RoundTripper
}

func (t *testAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// If the request already has an auth header or query parameter, pass through unmodified.
	if req.Header.Get("X-Session-Token") != "" ||
		req.Header.Get("Authorization") != "" ||
		req.Header.Get("X-App-Password") != "" ||
		(req.URL != nil && (req.URL.Query().Get("token") != "" || req.URL.Query().Get("password") != "")) {
		return t.orig.RoundTrip(req)
	}

	// Check if this request is destined for one of our registered test servers.
	host := ""
	if req.URL != nil {
		host = req.URL.Host
	}
	if host == "" {
		host = req.Host
	}

	testServersMu.RLock()
	token, ok := testServers[host]
	testServersMu.RUnlock()

	if !ok || token == "" {
		return t.orig.RoundTrip(req)
	}

	// Check if caller stack is intentionally testing unauthenticated access.
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, "TestChallenger_") ||
			strings.Contains(frame.Function, "TestTOTPAuthProtection") {
			return t.orig.RoundTrip(req)
		}
		if !more {
			break
		}
	}

	// Clone request and inject X-Session-Token.
	reqClone := req.Clone(req.Context())
	reqClone.Header.Set("X-Session-Token", token)
	return t.orig.RoundTrip(reqClone)
}
