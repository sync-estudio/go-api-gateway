package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/redis/go-redis/v9"
	"sync.gateway/internal/auth"
	"sync.gateway/internal/config"
	"sync.gateway/internal/handler"
	"sync.gateway/internal/middleware"
	"sync.gateway/internal/proxy"
	"sync.gateway/internal/service"
)

type SwappableHandler struct {
	value atomic.Value
}

func NewSwappableHandler() *SwappableHandler {
	s := &SwappableHandler{}
	s.value.Store(http.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gateway not initialized", http.StatusServiceUnavailable)
	})))
	return s
}

func (s *SwappableHandler) Swap(next http.Handler) {
	if next == nil {
		next = http.NotFoundHandler()
	}
	s.value.Store(next)
}

func (s *SwappableHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, _ := s.value.Load().(http.Handler)
	if h == nil {
		http.Error(w, "gateway not initialized", http.StatusServiceUnavailable)
		return
	}
	h.ServeHTTP(w, r)
}

type Manager struct {
	redisClient *redis.Client
	swapper     *SwappableHandler

	mu             sync.RWMutex
	activeConfig   *config.YAMLConfig
	activeJWKSStop context.CancelFunc
}

func NewManager(redisClient *redis.Client) *Manager {
	return &Manager{
		redisClient: redisClient,
		swapper:     NewSwappableHandler(),
	}
}

func (m *Manager) Handler() http.Handler {
	return m.swapper
}

func (m *Manager) Close() {
	m.mu.Lock()
	stop := m.activeJWKSStop
	m.activeJWKSStop = nil
	m.mu.Unlock()

	if stop != nil {
		stop()
	}
}

func (m *Manager) CurrentConfig() (*config.YAMLConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activeConfig == nil {
		return nil, fmt.Errorf("gateway config not initialized")
	}

	copyCfg, err := cloneConfig(m.activeConfig)
	if err != nil {
		return nil, err
	}

	return copyCfg, nil
}

func (m *Manager) Init(cfg *config.YAMLConfig) error {
	return m.apply(cfg, "", false)
}

func (m *Manager) ApplyAndPersist(cfg *config.YAMLConfig, path string) error {
	if path == "" {
		return fmt.Errorf("config path is required")
	}
	return m.apply(cfg, path, true)
}

func (m *Manager) apply(cfg *config.YAMLConfig, path string, persist bool) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	m.mu.RLock()
	active := m.activeConfig
	m.mu.RUnlock()

	if active != nil && active.Proxy.Port != 0 && cfg.Proxy.Port != 0 && active.Proxy.Port != cfg.Proxy.Port {
		return fmt.Errorf("proxy.port change requires restart (current: %d, requested: %d)", active.Proxy.Port, cfg.Proxy.Port)
	}

	normalized := *cfg
	if err := config.NormalizeAndValidate(&normalized); err != nil {
		return err
	}

	built, err := buildGatewayHandler(&normalized, m.redisClient)
	if err != nil {
		return err
	}

	if persist {
		if err := config.Save(path, &normalized); err != nil {
			if built.jwksStop != nil {
				built.jwksStop()
			}
			return err
		}
	}

	m.mu.Lock()
	oldStop := m.activeJWKSStop
	m.activeConfig = &normalized
	m.activeJWKSStop = built.jwksStop
	m.swapper.Swap(built.handler)
	m.mu.Unlock()

	if oldStop != nil {
		oldStop()
	}

	return nil
}

type gatewayBuildResult struct {
	handler  http.Handler
	jwksStop context.CancelFunc
}

func buildGatewayHandler(cfg *config.YAMLConfig, redisClient *redis.Client) (*gatewayBuildResult, error) {
	registry := service.New()
	registry.Add(cfg.Services...)

	mux := http.NewServeMux()

	for _, svc := range cfg.Services {
		p, err := proxy.NewProxy(svc.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to create proxy for %s: %w", svc.Alias, err)
		}

		if err := proxy.RegisterHandler(mux, svc.Alias, p); err != nil {
			return nil, fmt.Errorf("failed to register handler for %s: %w", svc.Alias, err)
		}

		authStatus := "disabled"
		if svc.Auth.Enabled {
			authStatus = fmt.Sprintf("enabled (provider: %s)", svc.Auth.Provider)
		}
		log.Printf("[PROXY] Registered route: %s -> %s [auth: %s]", svc.Alias, svc.URL, authStatus)
	}

	mux.HandleFunc("/health", handler.HealthHandler())
	log.Println("[PROXY] Registered health check: /health")

	mux.HandleFunc("/", handler.NewRootHandler(registry))
	log.Println("[PROXY] Registered root endpoint: /")

	result := &gatewayBuildResult{
		handler: mux,
	}

	wrapped := result.handler

	if cfg.HasAuth() {
		provider := cfg.GetDefaultProvider()
		if provider != nil && provider.JWKSURL != "" {
			jwksCtx, jwksCancel := context.WithCancel(context.Background())
			log.Printf("[AUTH] Initializing JWKS provider: %s", provider.JWKSURL)

			jwksProvider := auth.NewJWKSProvider(provider.JWKSURL, provider.RefreshInterval.Std())
			if err := jwksProvider.Start(jwksCtx); err != nil {
				jwksCancel()
				return nil, fmt.Errorf("failed to initialize JWKS provider: %w", err)
			}

			validator := auth.NewValidator(jwksProvider, provider.Issuer)
			wrapped = middleware.NewAuthMiddleware(validator, registry)(wrapped)
			result.jwksStop = jwksCancel
			log.Println("[AUTH] Auth middleware enabled")
		} else {
			log.Println("[AUTH] Auth configured but no valid provider found, skipping auth middleware")
		}
	} else {
		log.Println("[AUTH] No auth providers configured, auth middleware disabled")
	}

	wrapped = middleware.NewRateLimiter(redisClient, registry)(wrapped)

	if cfg.CORS.Enabled {
		corsMiddleware := middleware.NewCorsMiddleware(&middleware.CORSConfig{
			AllowedOrigins:   cfg.CORS.AllowedOrigins,
			AllowedMethods:   cfg.CORS.AllowedMethods,
			AllowedHeaders:   cfg.CORS.AllowedHeaders,
			ExposedHeaders:   cfg.CORS.ExposedHeaders,
			AllowCredentials: cfg.CORS.AllowCredentials,
			MaxAge:           cfg.CORS.MaxAge,
		})
		wrapped = corsMiddleware(wrapped)
		log.Println("[CORS] CORS middleware enabled")
	}

	wrapped = middleware.NewLoggingMiddleware(registry)(wrapped)
	result.handler = wrapped

	return result, nil
}

func cloneConfig(cfg *config.YAMLConfig) (*config.YAMLConfig, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone config: %w", err)
	}

	copyCfg, err := config.ParseJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to clone config: %w", err)
	}

	return copyCfg, nil
}
