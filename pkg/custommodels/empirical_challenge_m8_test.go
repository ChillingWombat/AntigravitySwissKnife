package custommodels

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestChallenge_IPv6LinkLocalAndMappedIPv6(t *testing.T) {
	// Test IPv6 link-local addresses
	linkLocalTargets := []string{
		"http://[fe80::1]/v1",
		"http://[fe80::dead:beef]/v1",
		"http://[fe80::a00:27ff:fe8e:e9c7]:8080/v1",
		"http://[FE80::1]/v1",
	}

	for _, target := range linkLocalTargets {
		for _, provider := range []ProviderType{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom, ProviderLocal} {
			_, _, err := ValidateEndpointSSRF(target, provider, false)
			if err == nil {
				t.Errorf("FAIL: Expected SSRF block for IPv6 link-local %q under %s, but got nil error", target, provider)
			} else if !strings.Contains(err.Error(), "ssrf blocked") && !strings.Contains(err.Error(), "cloud metadata") {
				t.Errorf("Expected 'ssrf blocked' or 'cloud metadata' error for %q, got: %v", target, err)
			}
		}
	}

	// Test IPv4-mapped IPv6 addresses for Cloud Metadata (169.254.169.254)
	mappedMetadataTargets := []string{
		"http://[::ffff:169.254.169.254]/v1",
		"http://[::ffff:169.254.169.254]:80/v1",
		"http://[::ffff:a9fe:a9fe]/v1", // 169.254.169.254 in hex
		"http://[::ffff:100.100.100.200]/v1",
	}

	for _, target := range mappedMetadataTargets {
		for _, provider := range []ProviderType{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom, ProviderLocal} {
			_, _, err := ValidateEndpointSSRF(target, provider, false)
			if err == nil {
				t.Errorf("FAIL: Expected SSRF block for mapped metadata %q under %s, but got nil error", target, provider)
			} else if !strings.Contains(err.Error(), "ssrf blocked") && !strings.Contains(err.Error(), "cloud metadata") {
				t.Logf("Blocked with other error: %v for %q", err, target)
			}
		}
	}

	// Test IPv4-mapped IPv6 addresses for Loopback (127.0.0.1) under external providers
	mappedLoopbackTargets := []string{
		"http://[::ffff:127.0.0.1]/v1",
		"http://[::ffff:127.0.0.1]:8765/v1",
		"http://[::ffff:7f00:1]/v1", // 127.0.0.1 in hex
	}

	for _, target := range mappedLoopbackTargets {
		for _, provider := range []ProviderType{ProviderOpenAI, ProviderAnthropic, ProviderGemini, ProviderCustom} {
			_, _, err := ValidateEndpointSSRF(target, provider, false)
			if err == nil {
				t.Errorf("FAIL: Expected SSRF block for mapped loopback %q under external provider %s, but got nil error", target, provider)
			}
		}
	}
}

func TestChallenge_SafeTransport_IPv6DialAddress(t *testing.T) {
	transport := NewSafeTransport(ProviderLocal, 2*time.Second, true)
	if transport.DialContext == nil {
		t.Fatal("Transport.DialContext is nil")
	}

	// Let's test dialing an IPv6 loopback [::1]:port
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Pick an unused local port
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 not available on this host")
	}
	defer listener.Close()

	addr := listener.Addr().String() // e.g. "[::1]:12345"
	t.Logf("Testing DialContext on IPv6 listener: %s", addr)

	// Step A: Check ValidateEndpointSSRF directly on properly bracketed URL:
	bracketedURL := fmt.Sprintf("http://%s", addr)
	_, validIP, vErr := ValidateEndpointSSRF(bracketedURL, ProviderLocal, true)
	if vErr != nil {
		t.Fatalf("ValidateEndpointSSRF failed on bracketed IPv6 URL %s: %v", bracketedURL, vErr)
	}
	t.Logf("ValidateEndpointSSRF succeeded for bracketed URL %s: validIP=%s", bracketedURL, validIP)

	// Step B: Now test DialContext (which fails because NewSafeTransport creates unbracketed rawURL)
	conn, dialErr := transport.DialContext(ctx, "tcp", addr)
	if dialErr != nil {
		t.Errorf("BUG CONFIRMED: DialContext failed for IPv6 loopback %s: %v", addr, dialErr)
		return
	}
	defer conn.Close()
	t.Logf("Successfully dialed %s through NewSafeTransport", addr)
}

func TestChallenge_SafeTransport_IPPinningProof(t *testing.T) {
	// Test that NewSafeTransport pins the connection to the IP returned by ValidateEndpointSSRF
	transport := NewSafeTransport(ProviderLocal, 2*time.Second, true)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().String() // e.g. "127.0.0.1:54321"

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	conn, dialErr := transport.DialContext(ctx, "tcp", addr)
	if dialErr != nil {
		t.Fatalf("DialContext failed for %s: %v", addr, dialErr)
	}
	defer conn.Close()

	remoteAddr := conn.RemoteAddr().String()
	t.Logf("Connected to remoteAddr: %s (expected %s)", remoteAddr, addr)
	if remoteAddr != addr {
		t.Errorf("RemoteAddr mismatch: expected %s, got %s", addr, remoteAddr)
	}
}
