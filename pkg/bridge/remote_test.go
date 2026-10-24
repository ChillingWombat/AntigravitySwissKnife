package bridge

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteTransport_LoopbackVsVerified(t *testing.T) {
	loopbacks := []string{
		"127.0.0.1",
		"127.0.0.1:8765",
		"localhost",
		"localhost:3000",
		"::1",
		"[::1]",
		"[::1]:8080",
		"http://127.0.0.1:9999",
	}

	for _, host := range loopbacks {
		tr := getRemoteTransport(host)
		if tr != remoteLoopbackTransport {
			t.Errorf("host %q expected remoteLoopbackTransport, got %v", host, tr)
		}
		if tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
			t.Errorf("host %q expected InsecureSkipVerify: true on loopback transport", host)
		}

		cl := getCSRFClient(host)
		if cl != remoteLoopbackCSRFClient {
			t.Errorf("host %q expected remoteLoopbackCSRFClient, got %v", host, cl)
		}
		if cl.Timeout != 2*time.Second {
			t.Errorf("expected 2s timeout on CSRF client, got %v", cl.Timeout)
		}
	}

	externals := []string{
		"example.com",
		"example.com:443",
		"192.168.1.100",
		"192.168.1.100:8443",
		"10.0.0.1",
		"https://sub.domain.com",
	}

	for _, host := range externals {
		tr := getRemoteTransport(host)
		if tr != remoteVerifiedTransport {
			t.Errorf("host %q expected remoteVerifiedTransport, got %v", host, tr)
		}
		if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
			t.Errorf("host %q must NOT have InsecureSkipVerify on verified transport", host)
		}

		cl := getCSRFClient(host)
		if cl != remoteVerifiedCSRFClient {
			t.Errorf("host %q expected remoteVerifiedCSRFClient, got %v", host, cl)
		}
	}
}

func TestRemoteProxy_RewriteHeaders(t *testing.T) {
	targetURL, _ := url.Parse("http://127.0.0.1:45678")
	upstreamCSRF := "test-csrf-token-123"

	var capturedOut *http.Request
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(targetURL)
			pr.Out.Host = targetURL.Host
			pr.Out.Header.Set("Origin", targetURL.Scheme+"://"+targetURL.Host)
			pr.Out.Header.Set("Referer", targetURL.Scheme+"://"+targetURL.Host+"/")
			if upstreamCSRF != "" {
				pr.Out.Header.Set("x-codeium-csrf-token", upstreamCSRF)
			}
			if pr.Out.Method == http.MethodGet && (pr.Out.URL.Path == "/" || pr.Out.URL.Path == "/index.html" || strings.HasPrefix(pr.Out.URL.Path, "/c/")) {
				pr.Out.Header.Del("Accept-Encoding")
			}
			capturedOut = pr.Out
		},
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}, nil
		}),
	}

	// 1. GET /index.html with Accept-Encoding: gzip -> Accept-Encoding removed
	req := httptest.NewRequest("GET", "/index.html", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Host = "public.tunnel.dev"
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if capturedOut == nil {
		t.Fatal("proxy rewrite did not run")
	}
	if capturedOut.Host != "127.0.0.1:45678" {
		t.Errorf("expected Out.Host '127.0.0.1:45678', got %q", capturedOut.Host)
	}
	if capturedOut.Header.Get("Origin") != "http://127.0.0.1:45678" {
		t.Errorf("expected Origin 'http://127.0.0.1:45678', got %q", capturedOut.Header.Get("Origin"))
	}
	if capturedOut.Header.Get("Referer") != "http://127.0.0.1:45678/" {
		t.Errorf("expected Referer 'http://127.0.0.1:45678/', got %q", capturedOut.Header.Get("Referer"))
	}
	if capturedOut.Header.Get("x-codeium-csrf-token") != upstreamCSRF {
		t.Errorf("expected CSRF token %q, got %q", upstreamCSRF, capturedOut.Header.Get("x-codeium-csrf-token"))
	}
	if ae := capturedOut.Header.Get("Accept-Encoding"); ae != "" {
		t.Errorf("expected Accept-Encoding to be deleted for /index.html, got %q", ae)
	}

	// 2. POST /api/chat -> Accept-Encoding preserved
	reqPost := httptest.NewRequest("POST", "/api/chat", strings.NewReader(`{}`))
	reqPost.Header.Set("Accept-Encoding", "gzip")
	wPost := httptest.NewRecorder()
	proxy.ServeHTTP(wPost, reqPost)

	if ae := capturedOut.Header.Get("Accept-Encoding"); ae != "gzip" {
		t.Errorf("expected Accept-Encoding 'gzip' preserved for POST, got %q", ae)
	}
}

func TestRemoteCookie_SecureFlag(t *testing.T) {
	rm := NewRemoteManager(filepath.Join(t.TempDir(), "remote_control.json"))
	cfg := rm.GetConfig()
	cfg.SecurityPIN = "12345678"
	if err := rm.UpdateConfig(cfg); err != nil {
		t.Fatalf("UpdateConfig: %v", err)
	}
	body, _ := json.Marshal(map[string]string{"pin": "12345678"})

	// 1. Plain HTTP without HTTPS header -> Secure: false
	reqHTTP := httptest.NewRequest("POST", "/remote/api/auth", strings.NewReader(string(body)))
	reqHTTP.Header.Set("Content-Type", "application/json")
	wHTTP := httptest.NewRecorder()
	rm.handleAuth(wHTTP, reqHTTP)

	var foundSecureHTTP bool
	for _, c := range wHTTP.Result().Cookies() {
		if c.Name == "swiss_remote_pin" {
			foundSecureHTTP = c.Secure
		}
	}
	if foundSecureHTTP {
		t.Errorf("expected Secure: false over plain HTTP")
	}

	// 2. HTTPS request via X-Forwarded-Proto: https -> Secure: true
	reqXFP := httptest.NewRequest("POST", "/remote/api/auth", strings.NewReader(string(body)))
	reqXFP.Header.Set("Content-Type", "application/json")
	reqXFP.Header.Set("X-Forwarded-Proto", "https")
	wXFP := httptest.NewRecorder()
	rm.handleAuth(wXFP, reqXFP)

	var foundSecureXFP bool
	for _, c := range wXFP.Result().Cookies() {
		if c.Name == "swiss_remote_pin" {
			foundSecureXFP = c.Secure
		}
	}
	if !foundSecureXFP {
		t.Errorf("expected Secure: true with X-Forwarded-Proto: https")
	}

	// 3. HTTPS request via r.TLS != nil -> Secure: true
	reqTLS := httptest.NewRequest("POST", "/remote/api/auth", strings.NewReader(string(body)))
	reqTLS.Header.Set("Content-Type", "application/json")
	reqTLS.TLS = &tls.ConnectionState{}
	wTLS := httptest.NewRecorder()
	rm.handleAuth(wTLS, reqTLS)

	var foundSecureTLS bool
	for _, c := range wTLS.Result().Cookies() {
		if c.Name == "swiss_remote_pin" {
			foundSecureTLS = c.Secure
		}
	}
	if !foundSecureTLS {
		t.Errorf("expected Secure: true with r.TLS != nil")
	}

	// 4. Logout cookie over HTTPS -> Secure: true
	reqLogout := httptest.NewRequest("POST", "/remote/api/logout", nil)
	reqLogout.Header.Set("X-Forwarded-Proto", "HTTPS")
	wLogout := httptest.NewRecorder()
	rm.handleLogout(wLogout, reqLogout)

	var foundSecureLogout bool
	for _, c := range wLogout.Result().Cookies() {
		if c.Name == "swiss_remote_pin" {
			foundSecureLogout = c.Secure
		}
	}
	if !foundSecureLogout {
		t.Errorf("expected Secure: true on logout with X-Forwarded-Proto: HTTPS")
	}
}
