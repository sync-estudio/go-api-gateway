package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"sync.gateway/internal/auth"
	"sync.gateway/internal/config"
	"sync.gateway/internal/handler"
	"sync.gateway/internal/middleware"
	"sync.gateway/internal/proxy"
	"sync.gateway/internal/service"
)

func main() {
	// Load configuration from YAML file
	cfg, err := config.LoadFromFile("config.yaml")
	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	port := strconv.Itoa(cfg.Proxy.Port)
	if port == "" {
		port = "8080"
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
