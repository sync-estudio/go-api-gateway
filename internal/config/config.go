package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration stores durations in configuration and supports YAML/JSON string values like "1m".
type Duration time.Duration

func (d Duration) Std() time.Duration {
	return time.Duration(d)
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*d = 0
		return nil
	}

	if data[0] == '"' {
		var raw string
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}

		parsed, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}

		*d = Duration(parsed)
		return nil
	}

	var rawNanos int64
	if err := json.Unmarshal(data, &rawNanos); err != nil {
		return err
	}

	*d = Duration(time.Duration(rawNanos))
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	trimmed := strings.TrimSpace(value.Value)
	if trimmed == "" {
		*d = 0
		return nil
	}

	parsed, err := time.ParseDuration(trimmed)
	if err == nil {
		*d = Duration(parsed)
		return nil
	}

	nanos, convErr := strconv.ParseInt(trimmed, 10, 64)
	if convErr != nil {
		return fmt.Errorf("invalid duration %q", trimmed)
	}

	*d = Duration(time.Duration(nanos))
	return nil
}

// AuthProviderConfig configures an authentication provider.
type AuthProviderConfig struct {
	Type            string   `yaml:"type" json:"type"`                         // Provider type: "jwks"
	JWKSURL         string   `yaml:"jwks_url" json:"jwks_url"`                 // JWKS endpoint URL
	RefreshInterval Duration `yaml:"refresh_interval" json:"refresh_interval"` // How often to refresh keys (default: 1h)
	Issuer          string   `yaml:"issuer" json:"issuer"`                     // Expected issuer claim (optional)
}

// ServiceAuthConfig configures authentication for a specific service.
type ServiceAuthConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`   // Whether auth is required for this service
	Provider string `yaml:"provider" json:"provider"` // Provider name (references auth.providers key, uses default if empty)
}

// ServiceRateLimitConfig configures rate limiting for a specific service.
type ServiceRateLimitConfig struct {
	Requests int64    `yaml:"requests" json:"requests"` // Max requests allowed in the window
	Window   Duration `yaml:"window" json:"window"`     // Time window for request counting (e.g. 1m)
}

// ServiceConfig holds a service URL and its route alias.
type ServiceConfig struct {
	URL       string                 `yaml:"url" json:"url"`
	Alias     string                 `yaml:"alias" json:"alias"`
	Auth      ServiceAuthConfig      `yaml:"auth" json:"auth"`
	RateLimit ServiceRateLimitConfig `yaml:"rate_limit" json:"rate_limit"`
}

// CORSConfig holds CORS middleware configuration.
type CORSConfig struct {
	AllowedOrigins   []string `yaml:"allowed_origins" json:"allowed_origins"`
	AllowedMethods   []string `yaml:"allowed_methods" json:"allowed_methods"`
	AllowedHeaders   []string `yaml:"allowed_headers" json:"allowed_headers"`
	ExposedHeaders   []string `yaml:"exposed_headers" json:"exposed_headers"`
	AllowCredentials bool     `yaml:"allow_credentials" json:"allow_credentials"`
	MaxAge           int      `yaml:"max_age" json:"max_age"`
	Enabled          bool     `yaml:"enabled" json:"enabled"`
}

// ProxyConfig holds the proxy server configuration.
type ProxyConfig struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// AuthConfig holds all authentication configuration.
type AuthConfig struct {
	DefaultProvider string                        `yaml:"default_provider" json:"default_provider"` // Default provider for services without explicit provider
	Providers       map[string]AuthProviderConfig `yaml:"providers" json:"providers"`               // Named auth providers
}

// YAMLConfig is the root configuration structure.
type YAMLConfig struct {
	Proxy    ProxyConfig     `yaml:"proxy" json:"proxy"`
	Auth     AuthConfig      `yaml:"auth" json:"auth"`
	CORS     CORSConfig      `yaml:"cors" json:"cors"`
	Services []ServiceConfig `yaml:"services" json:"services"`
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*YAMLConfig, error) {
	return Load(path)
}

// Load loads configuration from a JSON or YAML file.
func Load(path string) (*YAMLConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg YAMLConfig

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(file, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
	default:
		if err := json.Unmarshal(file, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", path, err)
		}
	}

	if err := NormalizeAndValidate(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", path, err)
	}

	log.Printf("[CONFIG] Loaded config from: %s", path)
	log.Printf("[CONFIG] Proxy: %s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
	log.Printf("[CONFIG] Services: %d configured", len(cfg.Services))
	if cfg.Auth.DefaultProvider != "" {
		log.Printf("[CONFIG] Auth: default provider = %s", cfg.Auth.DefaultProvider)
	}

	return &cfg, nil
}

// ParseJSON parses JSON config content and applies defaults/validation.
func ParseJSON(payload []byte) (*YAMLConfig, error) {
	var cfg YAMLConfig
	if err := json.Unmarshal(payload, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if err := NormalizeAndValidate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes configuration to file based on extension (.json, .yaml, .yml).
func Save(path string, cfg *YAMLConfig) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	clone := *cfg
	if err := NormalizeAndValidate(&clone); err != nil {
		return err
	}

	var data []byte
	var err error

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		data, err = yaml.Marshal(&clone)
	case ".json", "":
		data, err = json.MarshalIndent(&clone, "", "  ")
	default:
		return fmt.Errorf("unsupported config file extension %q", ext)
	}
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if !strings.HasSuffix(string(data), "\n") {
		data = append(data, '\n')
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

// LoadWithYAMLFallback loads primary config; if missing, optionally falls back to YAML.
// When fallback is used and primary has a JSON extension, the config is persisted to primary.
func LoadWithYAMLFallback(primaryPath, fallbackYAMLPath string) (*YAMLConfig, string, error) {
	if fileExists(primaryPath) {
		cfg, err := Load(primaryPath)
		if err != nil {
			return nil, "", err
		}
		return cfg, primaryPath, nil
	}

	if fallbackYAMLPath != "" && fileExists(fallbackYAMLPath) {
		cfg, err := Load(fallbackYAMLPath)
		if err != nil {
			return nil, "", err
		}

		ext := strings.ToLower(filepath.Ext(primaryPath))
		if ext == ".json" || ext == "" {
			if saveErr := Save(primaryPath, cfg); saveErr != nil {
				return nil, "", saveErr
			}
			log.Printf("[CONFIG] Migrated YAML config to JSON: %s", primaryPath)
			return cfg, primaryPath, nil
		}

		return cfg, fallbackYAMLPath, nil
	}

	return nil, "", fmt.Errorf("config file not found: %s", primaryPath)
}

// NormalizeAndValidate applies defaults and validates the config.
func NormalizeAndValidate(cfg *YAMLConfig) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	applyDefaults(cfg)
	if err := cfg.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate checks configuration combinations that are unsafe or invalid.
func (c *YAMLConfig) Validate() error {
	seenAliases := make(map[string]struct{}, len(c.Services))

	for i, svc := range c.Services {
		alias := strings.TrimSpace(svc.Alias)
		if alias == "" {
			return fmt.Errorf("services[%d].alias is required", i)
		}
		if !strings.HasPrefix(alias, "/") {
			return fmt.Errorf("services[%d].alias must start with '/'", i)
		}
		if alias == "/" {
			return fmt.Errorf("services[%d].alias cannot be '/'", i)
		}

		normalizedAlias := strings.TrimSuffix(alias, "/")
		switch normalizedAlias {
		case "/admin", "/health":
			return fmt.Errorf("services[%d].alias %q is reserved", i, alias)
		}

		if _, exists := seenAliases[normalizedAlias]; exists {
			return fmt.Errorf("duplicate service alias: %q", alias)
		}
		seenAliases[normalizedAlias] = struct{}{}

		if strings.TrimSpace(svc.URL) == "" {
			return fmt.Errorf("services[%d].url is required", i)
		}

		if _, err := url.ParseRequestURI(svc.URL); err != nil {
			return fmt.Errorf("services[%d].url is invalid: %w", i, err)
		}

		if svc.Auth.Enabled {
			providerName := strings.TrimSpace(svc.Auth.Provider)
			if providerName == "" {
				providerName = strings.TrimSpace(c.Auth.DefaultProvider)
			}
			if providerName == "" {
				return fmt.Errorf("services[%d].auth.enabled is true but no auth provider is configured", i)
			}
			if _, ok := c.Auth.Providers[providerName]; !ok {
				return fmt.Errorf("services[%d].auth.provider references unknown provider %q", i, providerName)
			}
		}
	}

	if !c.CORS.Enabled {
		return nil
	}

	allowedOrigins := c.CORS.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	if c.CORS.AllowCredentials {
		for _, origin := range allowedOrigins {
			if strings.TrimSpace(origin) == "*" {
				return fmt.Errorf("cors.allow_credentials cannot be true when cors.allowed_origins contains '*'")
			}
		}
	}

	return nil
}

// applyDefaults sets default values for configuration fields.
func applyDefaults(cfg *YAMLConfig) {
	// Default proxy port
	if cfg.Proxy.Port == 0 {
		cfg.Proxy.Port = 8080
	}

	// Default CORS settings
	if cfg.CORS.MaxAge == 0 {
		cfg.CORS.MaxAge = 86400
	}

	// Default refresh interval for auth providers
	if cfg.Auth.Providers == nil {
		cfg.Auth.Providers = make(map[string]AuthProviderConfig)
	}
	for name, provider := range cfg.Auth.Providers {
		if provider.RefreshInterval == 0 {
			provider.RefreshInterval = Duration(time.Hour)
			cfg.Auth.Providers[name] = provider
		}
	}

	// Resolve service auth providers to default if not specified
	for i := range cfg.Services {
		if cfg.Services[i].Auth.Enabled && cfg.Services[i].Auth.Provider == "" {
			cfg.Services[i].Auth.Provider = cfg.Auth.DefaultProvider
		}

		if cfg.Services[i].RateLimit.Requests == 0 {
			cfg.Services[i].RateLimit.Requests = 200
		}
		if cfg.Services[i].RateLimit.Window == 0 {
			cfg.Services[i].RateLimit.Window = Duration(time.Minute)
		}
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// GetAuthProvider returns the auth provider config for a service.
// Returns nil if auth is not enabled or provider not found.
func (c *YAMLConfig) GetAuthProvider(svc *ServiceConfig) *AuthProviderConfig {
	if svc == nil || !svc.Auth.Enabled {
		return nil
	}

	providerName := svc.Auth.Provider
	if providerName == "" {
		providerName = c.Auth.DefaultProvider
	}

	if providerName == "" {
		return nil
	}

	if provider, ok := c.Auth.Providers[providerName]; ok {
		return &provider
	}

	return nil
}

// HasAuth returns true if any auth providers are configured.
func (c *YAMLConfig) HasAuth() bool {
	return len(c.Auth.Providers) > 0
}

// GetDefaultProvider returns the default auth provider config.
// Returns nil if no default provider is configured.
func (c *YAMLConfig) GetDefaultProvider() *AuthProviderConfig {
	if c.Auth.DefaultProvider == "" {
		return nil
	}
	if provider, ok := c.Auth.Providers[c.Auth.DefaultProvider]; ok {
		return &provider
	}
	return nil
}
