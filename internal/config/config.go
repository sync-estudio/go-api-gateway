package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AuthProviderConfig configures an authentication provider.
type AuthProviderConfig struct {
	Type            string        `yaml:"type"`             // Provider type: "jwks"
	JWKSURL         string        `yaml:"jwks_url"`         // JWKS endpoint URL
	RefreshInterval time.Duration `yaml:"refresh_interval"` // How often to refresh keys (default: 1h)
	Issuer          string        `yaml:"issuer"`           // Expected issuer claim (optional)
}

// ServiceAuthConfig configures authentication for a specific service.
type ServiceAuthConfig struct {
	Enabled  bool   `yaml:"enabled"`  // Whether auth is required for this service
	Provider string `yaml:"provider"` // Provider name (references auth.providers key, uses default if empty)
}

// ServiceRateLimitConfig configures rate limiting for a specific service.
type ServiceRateLimitConfig struct {
	Requests int64         `yaml:"requests"` // Max requests allowed in the window
	Window   time.Duration `yaml:"window"`   // Time window for request counting (e.g. 1m)
}

// ServiceConfig holds a service URL and its route alias.
type ServiceConfig struct {
	URL       string                 `yaml:"url"`
	Alias     string                 `yaml:"alias"`
	Auth      ServiceAuthConfig      `yaml:"auth"`
	RateLimit ServiceRateLimitConfig `yaml:"rate_limit"`
}

// ProxyConfig holds the proxy server configuration.
type ProxyConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// AuthConfig holds all authentication configuration.
type AuthConfig struct {
	DefaultProvider string                        `yaml:"default_provider"` // Default provider for services without explicit provider
	Providers       map[string]AuthProviderConfig `yaml:"providers"`        // Named auth providers
}

// YAMLConfig is the root configuration structure.
type YAMLConfig struct {
	Proxy    ProxyConfig     `yaml:"proxy"`
	Auth     AuthConfig      `yaml:"auth"`
	Services []ServiceConfig `yaml:"services"`
}

// LoadFromFile loads configuration from a YAML file.
func LoadFromFile(path string) (*YAMLConfig, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg YAMLConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	log.Printf("[CONFIG] Loaded config from: %s", path)
	log.Printf("[CONFIG] Proxy: %s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
	log.Printf("[CONFIG] Services: %d configured", len(cfg.Services))
	if cfg.Auth.DefaultProvider != "" {
		log.Printf("[CONFIG] Auth: default provider = %s", cfg.Auth.DefaultProvider)
	}

	return &cfg, nil
}

// applyDefaults sets default values for configuration fields.
func applyDefaults(cfg *YAMLConfig) {
	// Default proxy port
	if cfg.Proxy.Port == 0 {
		cfg.Proxy.Port = 8080
	}

	// Default refresh interval for auth providers
	if cfg.Auth.Providers == nil {
		cfg.Auth.Providers = make(map[string]AuthProviderConfig)
	}
	for name, provider := range cfg.Auth.Providers {
		if provider.RefreshInterval == 0 {
			provider.RefreshInterval = time.Hour
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
			cfg.Services[i].RateLimit.Window = time.Minute
		}
	}
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
