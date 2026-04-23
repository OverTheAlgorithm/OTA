package collector

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// privateRanges lists CIDR blocks that must never be contacted via outbound HTTP.
var privateRanges []*net.IPNet

func init() {
	cidrs := []string{
		"127.0.0.0/8",    // loopback
		"10.0.0.0/8",     // RFC-1918
		"172.16.0.0/12",  // RFC-1918
		"192.168.0.0/16", // RFC-1918
		"169.254.0.0/16", // link-local / cloud metadata
		"0.0.0.0/8",      // "this" network
		"::1/128",         // IPv6 loopback
		"fc00::/7",        // IPv6 unique-local
		"fe80::/10",       // IPv6 link-local
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Sprintf("safe_transport: bad CIDR %s: %v", cidr, err))
		}
		privateRanges = append(privateRanges, network)
	}
}

// isPrivateIP reports whether ip falls within any blocked range.
func isPrivateIP(ip net.IP) bool {
	// Explicit block for cloud metadata endpoint.
	if ip.Equal(net.ParseIP("169.254.169.254")) {
		return true
	}
	for _, network := range privateRanges {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// safeDialContext resolves addr, checks all IPs, and rejects private addresses.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("safe dial: invalid address %q: %w", addr, err)
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("safe dial: DNS lookup %q failed: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("safe dial: no addresses resolved for %q", host)
	}

	for _, a := range addrs {
		if isPrivateIP(a.IP) {
			return nil, fmt.Errorf("safe dial: %q resolves to private IP %s — request blocked", host, a.IP)
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addrs[0].IP.String(), port))
}

// newSafeTransport returns an *http.Transport whose DialContext blocks requests
// to private/internal IP ranges to prevent SSRF attacks.
func newSafeTransport() *http.Transport {
	return &http.Transport{
		DialContext:           safeDialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
	}
}

// safeRedirectPolicy validates that each redirect target uses http or https only.
func safeRedirectPolicy(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("too many redirects")
	}
	scheme := req.URL.Scheme
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("redirect to disallowed scheme %q blocked", scheme)
	}
	return nil
}
