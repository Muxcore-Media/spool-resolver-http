package internal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Muxcore-Media/core/pkg/contracts"
)

func (m *Module) Settings() []contracts.SettingDef {
	return m.settingsDefs()
}

func (m *Module) UpdateSetting(key, value string) error {
	return m.updateSetting(key, value)
}

func (m *Module) policySnapshot() (hosts []string, allowHTTP, allowPrivate bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hosts = append([]string(nil), m.allowedHosts...)
	return hosts, m.allowHTTP, m.allowPrivate
}

func (m *Module) settingsDefs() []contracts.SettingDef {
	hosts, allowHTTP, allowPrivate := m.policySnapshot()
	return []contracts.SettingDef{
		{
			Key:         "allowed_hosts",
			Label:       "Allowed Hosts",
			Type:        contracts.SettingTypeString,
			Value:       strings.Join(hosts, ","),
			Default:     "",
			Description: "Comma-separated host allowlist (SPOOL_RESOLVER_ALLOWED_HOSTS); empty denies all",
			Group:       "SSRF",
		},
		{
			Key:         "allow_http",
			Label:       "Allow HTTP",
			Type:        contracts.SettingTypeBool,
			Value:       strconv.FormatBool(allowHTTP),
			Default:     "false",
			Description: "Permit http:// URLs (SPOOL_RESOLVER_ALLOW_HTTP; for local/dev only)",
			Group:       "SSRF",
		},
		{
			Key:         "allow_private",
			Label:       "Allow Private IPs",
			Type:        contracts.SettingTypeBool,
			Value:       strconv.FormatBool(allowPrivate),
			Default:     "false",
			Description: "Permit private/link-local targets (SPOOL_RESOLVER_ALLOW_PRIVATE)",
			Group:       "SSRF",
		},
	}
}

func parseBoolSetting(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "on":
		return true, nil
	case "false", "0", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", value)
	}
}

func (m *Module) updateSetting(key, value string) error {
	switch key {
	case "allowed_hosts", "SPOOL_RESOLVER_ALLOWED_HOSTS":
		hosts := parseAllowlist(value)
		m.mu.Lock()
		m.allowedHosts = hosts
		m.mu.Unlock()
		return nil
	case "allow_http", "SPOOL_RESOLVER_ALLOW_HTTP":
		b, err := parseBoolSetting(value)
		if err != nil {
			return fmt.Errorf("invalid allow_http: %w", err)
		}
		m.mu.Lock()
		m.allowHTTP = b
		m.mu.Unlock()
		return nil
	case "allow_private", "SPOOL_RESOLVER_ALLOW_PRIVATE":
		b, err := parseBoolSetting(value)
		if err != nil {
			return fmt.Errorf("invalid allow_private: %w", err)
		}
		m.mu.Lock()
		m.allowPrivate = b
		m.mu.Unlock()
		return nil
	default:
		return fmt.Errorf("unknown setting %q", key)
	}
}
