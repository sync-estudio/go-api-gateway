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
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"sync.gateway/internal/admin"
	"sync.gateway/internal/config"
	"sync.gateway/internal/runtime"
)

func main() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		log.Printf("[ENV] Failed to load .env file: %v", err)
	}

	configPath := strings.TrimSpace(os.Getenv("CONFIG_PATH"))
	if configPath == "" {
		configPath = "config.json"
	}

	cfg, loadedFrom, err := config.LoadWithYAMLFallback(configPath, "config.yaml")

	if err != nil {
		log.Fatalf("[CONFIG] Failed to load config: %v", err)
	}

	if loadedFrom != configPath {
		log.Printf("[CONFIG] Loaded from fallback file: %s", loadedFrom)
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = strconv.Itoa(cfg.Proxy.Port)
		if port == "" {
			port = "8080"
		}
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

	manager := runtime.NewManager(rdb)
	if err := manager.Init(cfg); err != nil {
		log.Fatalf("[CONFIG] Failed to initialize gateway runtime: %v", err)
	}

	adminCreds := admin.LoadCredentialsFromEnv()
	if adminCreds.Enabled {
		log.Printf("[ADMIN] Admin UI enabled at /admin for %s", adminCreds.Email)
	} else {
		log.Println("[ADMIN] Admin UI login disabled; set ADMIN_EMAIL, ADMIN_PASSWORD, ADMIN_SESSION_SECRET")
	}

	rootMux := http.NewServeMux()
	adminHandler := admin.NewHandler(manager, configPath, adminCreds)
	adminHandler.Register(rootMux)
	rootMux.Handle("/", manager.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: rootMux,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		log.Printf("[PROXY] Received signal %v, shutting down...", sig)

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("[PROXY] Shutdown error: %v", err)
		}

		manager.Close()
	}()

	// Start server
	log.Printf("[PROXY] Starting server on port %s", port)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[PROXY] Server failed: %v", err)
	}
}

func loadRedisOptions() (*redis.Options, string, error) {
	for _, envName := range []string{"REDIS_PRIVATE_URL", "REDIS_URL", "REDIS_ADDR"} {
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
