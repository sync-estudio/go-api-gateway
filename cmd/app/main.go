package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"sync.gateway/internal/auth"
	"sync.gateway/internal/config"
	"sync.gateway/internal/handler"
	"sync.gateway/internal/middleware"
	"sync.gateway/internal/proxy"
	"sync.gateway/internal/service"
)

func main() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("[ENV] Failed to load .env file: %v", err)
	}

	// Load configuration from YAML file
	cfg, err := config.LoadFromFile("config.yaml")

	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	port := strconv.Itoa(cfg.Proxy.Port)
	if port == "" {
		port = "8080"
	}

	redisOpts, redisSource, err := loadRedisOptions()
	if err != nil {
		log.Fatalf("[RATE-LIMITER] %v", err)
	}

	rdb := redis.NewClient(redisOpts)
	log.Printf("[REDIS] Using Redis configuration from %s", redisSource)

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("[REDIS] Failed to connect to Redis: %v", err)
	}

	registry := service.New()
	registry.Add(cfg.Services...)

	mux := http.NewServeMux()

	// Register proxy handlers for each service
	for _, svc := range cfg.Services {
		p, err := proxy.NewProxy(svc.URL)
		if err != nil {
			log.Fatalf("[PROXY] Failed to create proxy for %s: %v", svc.Alias, err)
		}

		if err := proxy.RegisterHandler(mux, svc.Alias, p); err != nil {
			log.Fatalf("[PROXY] Failed to register handler for %s: %v", svc.Alias, err)
		}

		authStatus := "disabled"
		if svc.Auth.Enabled {
			authStatus = fmt.Sprintf("enabled (provider: %s)", svc.Auth.Provider)
		}
		log.Printf("[PROXY] Registered route: %s -> %s [auth: %s]", svc.Alias, svc.URL, authStatus)
	}

	// Register health check endpoint
	mux.HandleFunc("/health", handler.HealthHandler())
	log.Println("[PROXY] Registered health check: /health")

	// Register root endpoint
	mux.HandleFunc("/", handler.NewRootHandler(registry))
	log.Println("[PROXY] Registered root endpoint: /")

	var httpHandler http.Handler = mux

	// RATE LIMITER
	httpHandler = middleware.NewRateLimiter(rdb, registry)(httpHandler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.HasAuth() {
		provider := cfg.GetDefaultProvider()
		if provider != nil && provider.JWKSURL != "" {
			log.Printf("[AUTH] Initializing JWKS provider: %s", provider.JWKSURL)

			jwksProvider := auth.NewJWKSProvider(provider.JWKSURL, provider.RefreshInterval)

			if err := jwksProvider.Start(ctx); err != nil {
				log.Fatalf("[AUTH] Failed to initialize JWKS provider: %v", err)
			}

			validator := auth.NewValidator(jwksProvider, provider.Issuer)

			httpHandler = middleware.NewAuthMiddleware(validator, registry)(httpHandler)
			log.Println("[AUTH] Auth middleware enabled")
		} else {
			log.Println("[AUTH] Auth configured but no valid provider found, skipping auth middleware")
		}
	} else {
		log.Println("[AUTH] No auth providers configured, auth middleware disabled")
	}

	httpHandler = middleware.NewLoggingMiddleware(registry)(httpHandler)

	// HOT RELOAD
	if cfg.HotReload.Enabled {
		watcher, err := config.WatchConfig("config.yaml", func(newCfg *config.YAMLConfig) {
			log.Println("[HOT-RELOAD] Configuration changed, updating registry...")
			registry.Clear()
			registry.Add(newCfg.Services...)
			log.Printf("[HOT-RELOAD] Registry updated with %d services", len(newCfg.Services))
			log.Printf("[HOT-RELOAD] NOTE: New routes require server restart")
		})
		if err != nil {
			log.Printf("[HOT-RELOAD] Failed to enable hot reload: %v", err)
		} else {
			go func() {
				<-ctx.Done()
				if watcher != nil {
					watcher.Stop()
				}
			}()
			log.Println("[HOT-RELOAD] Hot reload enabled")
		}
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("[PROXY] Received signal %v, shutting down...", sig)
		cancel() // Stop JWKS background refresh
		os.Exit(0)
	}()

	// Start server
	log.Printf("[PROXY] Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, httpHandler); err != nil {
		log.Fatalf("[PROXY] Server failed: %v", err)
	}
}

func loadRedisOptions() (*redis.Options, string, error) {
	for _, envName := range []string{"REDIS_PRIVATE_URL", "REDIS_ADDR"} {
		value := strings.TrimSpace(os.Getenv(envName))

		if value == "" {
			continue
		}

		if strings.HasPrefix(value, "redis://") || strings.HasPrefix(value, "rediss://") {
			parsedOpts, err := redis.ParseURL(value)
			if err != nil {
				return nil, "", fmt.Errorf("invalid %s: %w", envName, err)
			}
			return parsedOpts, envName, nil
		}

		return &redis.Options{
			Addr: value,
			DB:   0,
		}, envName, nil
	}

	host := strings.TrimSpace(os.Getenv("REDISHOST"))
	port := strings.TrimSpace(os.Getenv("REDISPORT"))

	if host != "" && port != "" {
		return &redis.Options{
			Addr:     net.JoinHostPort(host, port),
			Username: strings.TrimSpace(os.Getenv("REDISUSER")),
			Password: os.Getenv("REDISPASSWORD"),
			DB:       0,
		}, "REDISHOST/REDISPORT", nil
	}

	return nil, "", errors.New("missing Redis configuration: set REDIS_PRIVATE_URL (Railway private), REDIS_URL, REDIS_ADDR, or REDISHOST/REDISPORT")
}
