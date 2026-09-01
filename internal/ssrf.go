package internal

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

func parseAllowlist(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envBool(key string) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func safeTagName(tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("tag name required")
	}
	if strings.Contains(tag, "/") || strings.Contains(tag, `\`) || strings.Contains(tag, "..") {
		return fmt.Errorf("tag name must be a single path segment")
	}
	for _, r := range tag {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '_' || r == '-'
		if !ok {
			return fmt.Errorf("tag name has invalid characters")
		}
	}
	return nil
}

func hostAllowed(host string, allowlist []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || len(allowlist) == 0 {
		return false
	}
	h, port, err := net.SplitHostPort(host)
	if err != nil {
		h = host
		port = ""
	}
	for _, a := range allowlist {
		aHost, aPort, aErr := net.SplitHostPort(a)
		if aErr != nil {
			aHost = a
			aPort = ""
		}
		if aPort != "" && port != "" && aPort != port {
			continue
		}
		if a == host || aHost == h || a == h {
			return true
		}
		if port != "" && a == net.JoinHostPort(h, port) {
			return true
		}
		// suffix match: ".github.com" or "github.com"
		if strings.HasPrefix(aHost, ".") && strings.HasSuffix(h, aHost) {
			return true
		}
		if !strings.HasPrefix(aHost, ".") && (h == aHost || strings.HasSuffix(h, "."+aHost)) {
			return true
		}
	}
	return false
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	// AWS/GCP metadata
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}

func stripPort(hostport string) string {
	h, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return h
}

func blockPrivateHost(ctx context.Context, host string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	hostOnly := stripPort(host)
	if ip := net.ParseIP(hostOnly); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("spool host IP %s is not allowed (private/loopback)", ip)
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupHost(ctx, hostOnly)
	if err != nil {
		return fmt.Errorf("spool host lookup failed for %q: %w", hostOnly, err)
	}
	for _, a := range addrs {
		if ip := net.ParseIP(a); ip != nil && isPrivateOrLocalIP(ip) {
			return fmt.Errorf("spool host %q resolves to private IP %q — blocked", hostOnly, a)
		}
	}
	return nil
}

func validateSpoolURL(ctx context.Context, raw string, allowlist []string, allowHTTP, allowPrivate bool) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("spool URL required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse spool URL: %w", err)
	}
	if u.User != nil {
		return nil, fmt.Errorf("spool URL must not include userinfo")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		// ok
	case "http":
		if !allowHTTP {
			return nil, fmt.Errorf("spool URL must use https (set SPOOL_RESOLVER_ALLOW_HTTP=true only for local/dev)")
		}
	default:
		return nil, fmt.Errorf("spool URL scheme %q not allowed", u.Scheme)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("spool URL host required")
	}
	if len(allowlist) == 0 {
		return nil, fmt.Errorf("SPOOL_RESOLVER_ALLOWED_HOSTS is empty; refusing fetch")
	}
	if !hostAllowed(u.Host, allowlist) {
		return nil, fmt.Errorf("spool host %q not in allowlist", u.Hostname())
	}
	if err := blockPrivateHost(ctx, u.Host, allowPrivate); err != nil {
		return nil, err
	}
	return u, nil
}

func joinSpoolURL(base *url.URL, tag string) string {
	return base.JoinPath("tags", tag+".json").String()
}
