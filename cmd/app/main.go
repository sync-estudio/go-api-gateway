package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

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
		fmt.Printf("[CONFIG] %s", err)
	}

	port := strconv.Itoa(cfg.Proxy.Port) // On cfg its requested as int

	if err != nil {
		log.Fatal(err)
	}

	if port == "" {
		port = "8080"
	}

	// Create service registry
	registry := service.New()
	registry.Add(cfg.Services...)

	// Setup HTTP router
	mux := http.NewServeMux()

	// Register proxy handlers for each service
	for _, svc := range cfg.Services {
		p, err := proxy.NewProxy(svc.URL)
		if err != nil {
			log.Fatal(err)
		}

		if err := proxy.RegisterHandler(mux, svc.Alias, p); err != nil {
			log.Fatal(err)
		}

		log.Printf("[PROXY] Registered route: %s -> %s", svc.Alias, svc.URL)
	}

	// Register health check endpoint
	mux.HandleFunc("/health", handler.HealthHandler())
	log.Println("[PROXY] Registered health check: /health")

	// Register root endpoint
	mux.HandleFunc("/", handler.NewRootHandler(registry))
	log.Println("[PROXY] Registered root endpoint: /")

	// Start server with logging middleware
	fmt.Printf("[Proxy] Started on %s\n", port)
	loggingMiddleware := middleware.NewLoggingMiddleware(registry)
	if err := http.ListenAndServe(":"+port, loggingMiddleware(mux)); err != nil {
		log.Fatal("Failed to start server: " + err.Error())
	}
}
