package custommodels

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ProhibitedCloudMetadataHosts lists hostnames that must never be dialed.
var ProhibitedCloudMetadataHosts = []string{
	"metadata.google.internal",
	"metadata",
	"instance-data",
	"169.254.169.254",
	"fd00:ec2::254",
	"100.100.100.200",
}

// prohibitedMetadataSubnets holds CIDR blocks for link-local and cloud metadata endpoints.
var prohibitedMetadataSubnets = []*net.IPNet{
	mustParseCIDR("169.254.0.0/16"),     // IPv4 Link-Local & AWS/GCP/Azure IMDS
	mustParseCIDR("fe80::/10"),           // IPv6 Link-Local
	mustParseCIDR("100.100.100.200/32"), // Alibaba Cloud IMDS
}

func mustParseCIDR(s string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("invalid CIDR %s: %v", s, err))
	}
	return ipnet
}

// ValidateEndpointSSRF inspects a target URL and its resolved IP addresses to protect against
// Server-Side Request Forgery (SSRF) and Cloud Metadata Exfiltration (P1-14).
//
// Parameters:
//   - rawURL: The destination HTTP/HTTPS endpoint to validate.
//   - provider: The model provider type (e.g. ProviderLocal, ProviderOpenAI, ProviderAnthropic).
//   - allowLoopback: When true, loopback addresses (127.0.0.1, ::1) are permitted for testing.
//     In production, loopback is strictly permitted only if provider == ProviderLocal (Ollama/LM Studio).
//
// Returns:
//   - *url.URL: The parsed URL.
//   - net.IP: The first verified, pinned IP address.
//   - error: Non-nil if the URL points to forbidden metadata, loopback, or invalid IP targets.
func ValidateEndpointSSRF(rawURL string, provider ProviderType, allowLoopback bool) (*url.URL, net.IP, error) {
	clean := strings.TrimSpace(rawURL)
	if clean == "" {
		return nil, nil, fmt.Errorf("empty endpoint url")
	}

	parsed, err := url.Parse(clean)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid endpoint url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, nil, fmt.Errorf("invalid scheme %q: only http and https are permitted", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, nil, fmt.Errorf("missing host in url")
	}

	hostLower := strings.ToLower(host)

	// Step 1: Reject explicit cloud metadata hostnames
	for _, prohibited := range ProhibitedCloudMetadataHosts {
		if hostLower == prohibited || strings.HasSuffix(hostLower, "."+prohibited) {
			return nil, nil, fmt.Errorf("ssrf blocked: access to cloud metadata host %q is prohibited", host)
		}
	}

	// Step 2: Resolve hostname to candidate IP addresses
	var ips []net.IP
	if parsedIP := net.ParseIP(host); parsedIP != nil {
		ips = []net.IP{parsedIP}
	} else {
		resolved, errLookup := net.LookupIP(host)
		if errLookup != nil {
			return nil, nil, fmt.Errorf("failed to resolve host %q: %w", host, errLookup)
		}
		ips = resolved
	}

	if len(ips) == 0 {
		return nil, nil, fmt.Errorf("no ip addresses resolved for host %q", host)
	}

	var validIP net.IP
	for _, ip := range ips {
		// A. Check for cloud metadata / link-local addresses
		for _, subnet := range prohibitedMetadataSubnets {
			if subnet.Contains(ip) {
				return nil, nil, fmt.Errorf("ssrf blocked: access to cloud metadata ip %s is prohibited", ip)
			}
		}

		// Check AWS IPv6 IMDS [fd00:ec2::254]
		if ip.String() == "fd00:ec2::254" {
			return nil, nil, fmt.Errorf("ssrf blocked: access to cloud metadata ip %s is prohibited", ip)
		}

		// B. Check for loopback addresses (127.0.0.0/8, ::1)
		if ip.IsLoopback() {
			if provider != ProviderLocal && !allowLoopback {
				return nil, nil, fmt.Errorf("ssrf blocked: loopback address %s is only permitted for local provider models", ip)
			}
		}

		// C. Reject unspecified (0.0.0.0, ::) or multicast
		if ip.IsUnspecified() || ip.IsMulticast() {
			return nil, nil, fmt.Errorf("ssrf blocked: invalid or unspecified ip address %s", ip)
		}

		if validIP == nil {
			validIP = ip
		}
	}

	return parsed, validIP, nil
}

// NewSafeTransport constructs an http.Transport with IP-pinning connection dialing.
// It verifies destination IPs before dialing and pins the connection to the validated IP,
// effectively mitigating DNS rebinding attacks.
func NewSafeTransport(provider ProviderType, timeout time.Duration, allowLoopback bool) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address format %q: %w", addr, err)
			}

			// Validate and resolve with SSRF protections
			rawURL := fmt.Sprintf("http://%s", net.JoinHostPort(host, port))
			_, validIP, err := ValidateEndpointSSRF(rawURL, provider, allowLoopback)
			if err != nil {
				return nil, err
			}

			// Pin connection to the resolved, verified IP address
			pinnedAddr := net.JoinHostPort(validIP.String(), port)
			return dialer.DialContext(ctx, network, pinnedAddr)
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: timeout,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewSafeHTTPClient returns an *http.Client utilizing NewSafeTransport.
func NewSafeHTTPClient(provider ProviderType, timeout time.Duration, allowLoopback bool) *http.Client {
	return &http.Client{
		Transport: NewSafeTransport(provider, timeout, allowLoopback),
		Timeout:   timeout,
	}
}
