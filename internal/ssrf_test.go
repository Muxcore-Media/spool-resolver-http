package internal

import "testing"

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
	_, err := validateSpoolURL("https://spool.example.com/tags", []string{"spool.example.com"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = validateSpoolURL("http://spool.example.com/tags", []string{"spool.example.com"}, false, false)
	if err == nil {
		t.Fatal("expected http denied")
	}
}
