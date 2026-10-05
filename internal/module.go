package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	spoolresolverv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/spoolresolver/v1"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
	manifest "github.com/Muxcore-Media/spool-resolver-http"
)

const (
	defaultGRPCAddr = "127.0.0.1:9675"
	defaultTimeout  = 30 * time.Second
	maxTagBodyBytes = 1 << 20
	envTimeout      = "SPOOL_RESOLVER_TIMEOUT"
)

type Module struct {
	spoolresolverv1.UnimplementedSpoolResolverServiceServer

	mu           sync.RWMutex
	client       *http.Client
	id           string
	grpcAddr     string
	grpcSrv      *grpc.Server
	lis          net.Listener
	allowedHosts []string
	allowHTTP    bool
	allowPrivate bool
	timeout      time.Duration
}

type Config struct {
	ID           string
	GRPCAddr     string
	Timeout      time.Duration
	AllowedHosts []string
	AllowHTTP    bool
	AllowPrivate bool
}

func NewModule(cfg Config) *Module {
	if cfg.ID == "" {
		cfg.ID = "spool-resolver-http"
	}
	if cfg.GRPCAddr == "" {
		cfg.GRPCAddr = defaultGRPCAddr
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if v := strings.TrimSpace(os.Getenv("SPOOL_RESOLVER_GRPC_ADDR")); v != "" {
		cfg.GRPCAddr = v
	}
	if v := strings.TrimSpace(os.Getenv(envTimeout)); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			slog.Warn("invalid SPOOL_RESOLVER_TIMEOUT; using default", "value", v, "default", defaultTimeout)
		} else {
			cfg.Timeout = d
		}
	}
	if len(cfg.AllowedHosts) == 0 {
		cfg.AllowedHosts = parseAllowlist(os.Getenv("SPOOL_RESOLVER_ALLOWED_HOSTS"))
	}
	if !cfg.AllowHTTP {
		cfg.AllowHTTP = envBool("SPOOL_RESOLVER_ALLOW_HTTP")
	}
	if !cfg.AllowPrivate {
		cfg.AllowPrivate = envBool("SPOOL_RESOLVER_ALLOW_PRIVATE")
	}

	m := &Module{
		id:           cfg.ID,
		grpcAddr:     cfg.GRPCAddr,
		allowedHosts: cfg.AllowedHosts,
		allowHTTP:    cfg.AllowHTTP,
		allowPrivate: cfg.AllowPrivate,
		timeout:      cfg.Timeout,
	}
	m.client = m.newHTTPClient(cfg.Timeout)
	return m
}

func (m *Module) newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: m.dialContext,
		},
		CheckRedirect: m.checkRedirect,
	}
}

func (m *Module) rebuildHTTPClient() {
	m.mu.RLock()
	timeout := m.timeout
	m.mu.RUnlock()
	client := m.newHTTPClient(timeout)
	m.mu.Lock()
	m.client = client
	m.mu.Unlock()
}

func (m *Module) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	_, _, allowPrivate := m.policySnapshot()
	if err := blockPrivateHost(ctx, host, allowPrivate); err != nil {
		return nil, err
	}
	m.mu.RLock()
	timeout := m.timeout
	m.mu.RUnlock()
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, network, addr)
}

func (m *Module) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 {
		return fmt.Errorf("too many redirects")
	}
	hosts, allowHTTP, allowPrivate := m.policySnapshot()
	if _, err := validateSpoolURL(req.Context(), req.URL.String(), hosts, allowHTTP, allowPrivate); err != nil {
		return err
	}
	return nil
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Spool Resolver HTTP",
		Version:      modulesdk.ManifestVersion(manifest.ManifestJSON),
		Roles:        []string{"infrastructure"},
		Description:  "HTTPS JSON spool tag resolver with host allowlist (SSRF-hardened)",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilitySpoolResolver, "settings"},
		Contracts: []contracts.ContractDeclaration{
			{
				Repo:      "github.com/Muxcore-Media/core/pkg/contracts",
				Interface: "SpoolResolver",
				Version:   "v0.5.8",
			},
		},
		MinCoreVersion: "0.5.8",
	}
}

func (m *Module) Init(ctx context.Context) error {
	lis, err := net.Listen("tcp", m.grpcAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", m.grpcAddr, err)
	}
	m.lis = lis
	slog.Info("spool-resolver-http initialized",
		"addr", m.grpcAddr,
		"allowed_hosts", len(m.allowedHosts),
		"allow_http", m.allowHTTP,
		"timeout", m.timeout,
	)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	srv := grpc.NewServer()
	lis := m.lis
	m.grpcSrv = srv
	spoolresolverv1.RegisterSpoolResolverServiceServer(srv, m)
	modulesdk.RegisterSettings(srv, m.id, m)
	go func() {
		slog.Info("spool-resolver-http gRPC service started", "addr", m.grpcAddr)
		if err := srv.Serve(lis); err != nil {
			slog.Error("spool-resolver-http gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
		m.grpcSrv = nil
	}
	if m.lis != nil {
		_ = m.lis.Close()
		m.lis = nil
	}
	m.client.CloseIdleConnections()
	slog.Info("spool-resolver-http stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	if m.lis == nil {
		return errors.New("listener not initialized")
	}
	if m.grpcSrv == nil {
		return errors.New("gRPC server not serving")
	}
	return nil
}

func (m *Module) ResolveTag(ctx context.Context, req *spoolresolverv1.ResolveTagRequest) (*spoolresolverv1.ResolveTagResponse, error) {
	if err := safeTagName(req.GetTagName()); err != nil {
		return &spoolresolverv1.ResolveTagResponse{Error: err.Error()}, nil
	}
	hosts, allowHTTP, allowPrivate := m.policySnapshot()
	base, err := validateSpoolURL(ctx, req.GetSpoolUrl(), hosts, allowHTTP, allowPrivate)
	if err != nil {
		return &spoolresolverv1.ResolveTagResponse{Error: err.Error()}, nil
	}
	fetchURL := joinSpoolURL(base, strings.TrimSpace(req.GetTagName()))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, fetchURL, nil)
	if err != nil {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("create request: %v", err),
		}, nil
	}

	m.mu.RLock()
	client := m.client
	m.mu.RUnlock()

	resp, err := client.Do(httpReq)
	if err != nil {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("fetch tag: %v", err),
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("spool returned %s", resp.Status),
		}, nil
	}

	limited := io.LimitReader(resp.Body, maxTagBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("read tag body: %v", err),
		}, nil
	}
	if len(body) > maxTagBodyBytes {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("tag body exceeds %d bytes", maxTagBodyBytes),
		}, nil
	}

	var tag contracts.TagDefinition
	if err := json.Unmarshal(body, &tag); err != nil {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("decode tag: %v", err),
		}, nil
	}

	raw, err := json.Marshal(tag)
	if err != nil {
		return &spoolresolverv1.ResolveTagResponse{
			Error: fmt.Sprintf("marshal tag: %v", err),
		}, nil
	}

	return &spoolresolverv1.ResolveTagResponse{TagDefinitionJson: raw}, nil
}

func parseTimeoutSetting(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("timeout required")
	}
	if sec, err := strconv.Atoi(value); err == nil && sec > 0 {
		return time.Duration(sec) * time.Second, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q", value)
	}
	return d, nil
}

var _ contracts.Module = (*Module)(nil)
