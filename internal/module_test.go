package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	spoolresolverv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/spoolresolver/v1"
)

func testModule(srv *httptest.Server) *Module {
	u, _ := url.Parse(srv.URL)
	return NewModule(Config{
		AllowedHosts: []string{u.Host, u.Hostname()},
		AllowHTTP:    true,
		AllowPrivate: true,
	})
}

func TestModuleInfo(t *testing.T) {
	m := NewModule(Config{})
	info := m.Info()
	if info.ID == "" {
		t.Error("module ID must not be empty")
	}
	if info.Version == "" {
		t.Error("module version must not be empty")
	}
	if info.MinCoreVersion == "" {
		t.Error("MinCoreVersion must not be empty")
	}
	if info.HTTPAddr != "" {
		t.Errorf("HTTPAddr should be empty for gRPC-only sidecar, got %q", info.HTTPAddr)
	}
	if len(info.Capabilities) == 0 || info.Capabilities[0] != "spool.resolver" {
		t.Errorf("expected spool.resolver capability, got %v", info.Capabilities)
	}
}

func TestResolveTagSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tags/default.json" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"name":        "default",
			"description": "default tag",
			"version":     "1.0.0",
			"modules": []map[string]any{
				{"repo": "github.com/org/mod", "version": "v1.0.0", "required": true},
			},
		})
	}))
	defer srv.Close()

	m := testModule(srv)
	ctx := context.Background()
	resp, err := m.ResolveTag(ctx, &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: srv.URL,
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if len(resp.TagDefinitionJson) == 0 {
		t.Fatal("expected non-empty tag definition")
	}

	var tag struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(resp.TagDefinitionJson, &tag); err != nil {
		t.Fatal(err)
	}
	if tag.Name != "default" {
		t.Errorf("expected name=default, got %s", tag.Name)
	}
	if tag.Version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %s", tag.Version)
	}
}

func TestResolveTagNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	m := testModule(srv)
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: srv.URL,
		TagName:  "nonexistent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected error for 404")
	}
}

func TestResolveTagServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := testModule(srv)
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: srv.URL,
		TagName:  "error",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected error for 500")
	}
}

func TestResolveTagInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer srv.Close()

	m := testModule(srv)
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: srv.URL,
		TagName:  "bad",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestResolveTagOversizedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"big","description":"` + strings.Repeat("x", maxTagBodyBytes) + `"}`))
	}))
	defer srv.Close()

	m := testModule(srv)
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: srv.URL,
		TagName:  "big",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" || !strings.Contains(resp.Error, "exceeds") {
		t.Fatalf("expected oversized body error, got %q", resp.Error)
	}
}

func TestResolveTagEmptyTagName(t *testing.T) {
	m := NewModule(Config{AllowedHosts: []string{"spool.example.com"}})
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "https://spool.example.com/tags",
		TagName:  "   ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" || !strings.Contains(resp.Error, "tag name required") {
		t.Fatalf("expected empty tag error, got %q", resp.Error)
	}
}

func TestResolveTagSSRFDenied(t *testing.T) {
	m := NewModule(Config{
		AllowedHosts: []string{"spool.example.com"},
		AllowHTTP:    false,
	})
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "http://169.254.169.254/latest/meta-data",
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected SSRF denial")
	}

	resp, err = m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "https://evil.example/tags",
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected allowlist denial")
	}

	resp, err = m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "https://spool.example.com/tags",
		TagName:  "../etc/passwd",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected bad tag denial")
	}
}

func TestResolveTagCheckRedirectRevalidation(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "ok", "version": "1.0.0", "modules": []any{}})
	}))
	defer final.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/tags/ok.json", http.StatusFound)
	}))
	defer redirector.Close()

	u, _ := url.Parse(redirector.URL)
	m := NewModule(Config{
		AllowedHosts: []string{u.Host, u.Hostname()},
		AllowHTTP:    true,
		AllowPrivate: true,
	})
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: redirector.URL,
		TagName:  "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/tags/x.json", http.StatusFound)
	}))
	defer evil.Close()

	eu, _ := url.Parse(evil.URL)
	m2 := NewModule(Config{
		AllowedHosts: []string{eu.Host, eu.Hostname()},
		AllowHTTP:    true,
		AllowPrivate: false,
	})
	resp, err = m2.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: evil.URL,
		TagName:  "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected redirect SSRF denial")
	}
}

func TestResolveTagEmptyAllowlist(t *testing.T) {
	m := NewModule(Config{AllowHTTP: true, AllowPrivate: true})
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "http://127.0.0.1:9/tags",
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected empty allowlist denial")
	}
}

func TestLifecycle(t *testing.T) {
	m := NewModule(Config{GRPCAddr: ":0"})
	ctx := context.Background()
	if err := m.Health(ctx); err == nil {
		t.Fatal("expected health failure before Init")
	}
	if err := m.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Health(ctx); err != nil {
		t.Fatal("expected health to pass while serving")
	}
	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Health(ctx); err == nil {
		t.Fatal("expected health failure after Stop")
	}
}

func TestSettingsAllowlistLive(t *testing.T) {
	m := NewModule(Config{AllowedHosts: []string{"old.example"}, AllowHTTP: false})
	defs := m.Settings()
	if len(defs) != 4 || defs[0].Key != "allowed_hosts" {
		t.Fatalf("Settings=%+v", defs)
	}
	if err := m.UpdateSetting("allowed_hosts", "spool.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := m.UpdateSetting("allow_http", "true"); err != nil {
		t.Fatal(err)
	}
	hosts, allowHTTP, _ := m.policySnapshot()
	if len(hosts) != 1 || hosts[0] != "spool.example.com" || !allowHTTP {
		t.Fatalf("policy hosts=%v allowHTTP=%v", hosts, allowHTTP)
	}
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "https://evil.example/tags",
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected allowlist denial for evil.example")
	}
}

func TestUpdateSettingAllowPrivateDeniesLoopback(t *testing.T) {
	m := NewModule(Config{
		AllowedHosts: []string{"127.0.0.1"},
		AllowHTTP:    true,
		AllowPrivate: true,
	})
	if err := m.UpdateSetting("allow_private", "false"); err != nil {
		t.Fatal(err)
	}
	resp, err := m.ResolveTag(context.Background(), &spoolresolverv1.ResolveTagRequest{
		SpoolUrl: "http://127.0.0.1:65535/tags",
		TagName:  "default",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Fatal("expected loopback denial after allow_private=false")
	}
}

func TestUpdateSettingUnknownKey(t *testing.T) {
	m := NewModule(Config{})
	if err := m.UpdateSetting("not_a_real_setting", "true"); err == nil {
		t.Fatal("expected unknown setting error")
	}
}

func TestUpdateSettingTimeout(t *testing.T) {
	m := NewModule(Config{})
	if err := m.UpdateSetting("timeout", "45s"); err != nil {
		t.Fatal(err)
	}
	m.mu.RLock()
	got := m.timeout
	m.mu.RUnlock()
	if got != 45*time.Second {
		t.Fatalf("timeout=%v", got)
	}
}
