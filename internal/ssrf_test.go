package internal

import (
	"context"
	"net/url"
	"testing"
)

func TestHostAllowed(t *testing.T) {
	allow := []string{"spool.example.com", ".github.com", "githubusercontent.com", "cdn.example.com:443"}
	cases := []struct {
		host string
		ok   bool
	}{
		{"spool.example.com", true},
		{"SPOOL.EXAMPLE.COM", true},
		{"api.github.com", true},
		{"raw.githubusercontent.com", true},
		{"evil.com", false},
		{"cdn.example.com:443", true},
		{"cdn.example.com", true},
	}
	for _, tc := range cases {
		if got := hostAllowed(tc.host, allow); got != tc.ok {
			t.Errorf("hostAllowed(%q)=%v want %v", tc.host, got, tc.ok)
		}
	}
}

func TestValidateSpoolURLHTTPS(t *testing.T) {
	ctx := context.Background()
	_, err := validateSpoolURL(ctx, "https://127.0.0.1/tags", []string{"127.0.0.1"}, false, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateSpoolURL(ctx, "http://127.0.0.1/tags", []string{"127.0.0.1"}, false, true)
	if err == nil {
		t.Fatal("expected http denied")
	}
}

func TestValidateSpoolURLUserinfo(t *testing.T) {
	_, err := validateSpoolURL(context.Background(), "https://user:pass@127.0.0.1/tags", []string{"127.0.0.1"}, false, true)
	if err == nil {
		t.Fatal("expected userinfo denial")
	}
}

func TestValidateSpoolURLBlocksLocalhostDNS(t *testing.T) {
	_, err := validateSpoolURL(context.Background(), "https://localhost/tags", []string{"localhost"}, false, false)
	if err == nil {
		t.Fatal("expected localhost DNS resolution denial")
	}
}

func TestValidateSpoolURLAllowsPrivateWhenOptedIn(t *testing.T) {
	_, err := validateSpoolURL(context.Background(), "http://127.0.0.1:8080/tags", []string{"127.0.0.1"}, true, true)
	if err != nil {
		t.Fatalf("expected private host allowed when opted in: %v", err)
	}
}

func TestBlockPrivateHostLiteralIP(t *testing.T) {
	ctx := context.Background()
	if err := blockPrivateHost(ctx, "127.0.0.1", false); err == nil {
		t.Fatal("expected loopback denial")
	}
	if err := blockPrivateHost(ctx, "169.254.169.254", false); err == nil {
		t.Fatal("expected metadata IP denial")
	}
	if err := blockPrivateHost(ctx, "127.0.0.1", true); err != nil {
		t.Fatalf("expected allow_private to permit loopback: %v", err)
	}
}

func TestJoinSpoolURL(t *testing.T) {
	u, err := url.Parse("https://github.com/Muxcore-Media/spool")
	if err != nil {
		t.Fatal(err)
	}
	got := joinSpoolURL(u, "default")
	want := "https://github.com/Muxcore-Media/spool/tags/default.json"
	if got != want {
		t.Fatalf("joinSpoolURL=%q want %q", got, want)
	}
}
