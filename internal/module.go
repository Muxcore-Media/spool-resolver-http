package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"

	"github.com/Muxcore-Media/core/pkg/contracts"
	spoolresolverv1 "github.com/Muxcore-Media/core/proto/gen/muxcore/spoolresolver/v1"
	modulesdk "github.com/Muxcore-Media/core/sdk/go/module"
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
		cfg.GRPCAddr = ":9675"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if v := os.Getenv("SPOOL_RESOLVER_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
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
	}
	m.client = &http.Client{
		Timeout: cfg.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			hosts, allowHTTP, allowPrivate := m.policySnapshot()
			if _, err := validateSpoolURL(req.URL.String(), hosts, allowHTTP, allowPrivate); err != nil {
				return err
			}
			return nil
		},
	}
	return m
}

func (m *Module) Info() contracts.ModuleInfo {
	return contracts.ModuleInfo{
		ID:           m.id,
		Name:         "Spool Resolver HTTP",
		Version:      "0.1.4",
		Roles:        []string{"infrastructure"},
		Description:  "HTTPS JSON spool tag resolver with host allowlist (SSRF-hardened)",
		Author:       "MuxCore",
		Capabilities: []string{contracts.CapabilitySpoolResolver, "settings"},
		Contracts: []contracts.ContractDeclaration{
			{
				Repo:      "github.com/Muxcore-Media/core/pkg/contracts",
				Interface: "SpoolResolver",
				Version:   "v0.4.0",
			},
		},
		MinCoreVersion: "0.4.0",
		HTTPAddr:       m.grpcAddr,
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
	)
	return nil
}

func (m *Module) Start(ctx context.Context) error {
	m.grpcSrv = grpc.NewServer()
	spoolresolverv1.RegisterSpoolResolverServiceServer(m.grpcSrv, m)
	modulesdk.RegisterSettings(m.grpcSrv, m.id, m)
	go func() {
		slog.Info("spool-resolver-http gRPC service started", "addr", m.grpcAddr)
		if err := m.grpcSrv.Serve(m.lis); err != nil {
			slog.Error("spool-resolver-http gRPC serve error", "error", err)
		}
	}()
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	if m.grpcSrv != nil {
		m.grpcSrv.GracefulStop()
	}
	m.client.CloseIdleConnections()
	slog.Info("spool-resolver-http stopped")
	return nil
}

func (m *Module) Health(ctx context.Context) error {
	return nil
}

func (m *Module) ResolveTag(ctx context.Context, req *spoolresolverv1.ResolveTagRequest) (*spoolresolverv1.ResolveTagResponse, error) {
	if err := safeTagName(req.GetTagName()); err != nil {
		return &spoolresolverv1.ResolveTagResponse{Error: err.Error()}, nil
	}
	hosts, allowHTTP, allowPrivate := m.policySnapshot()
	base, err := validateSpoolURL(req.GetSpoolUrl(), hosts, allowHTTP, allowPrivate)
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

	resp, err := m.client.Do(httpReq)
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

	var tag contracts.TagDefinition
	if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
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

var _ contracts.Module = (*Module)(nil)
