package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hacking-robots-and-beer/cisco/api/internal/handler"
	"github.com/hacking-robots-and-beer/cisco/api/internal/migrations"
	"github.com/hacking-robots-and-beer/cisco/api/internal/reconciler"
	"github.com/hacking-robots-and-beer/cisco/api/internal/repository"
	"github.com/hacking-robots-and-beer/cisco/api/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := mustEnv("DATABASE_URL")
	encKeyHex := mustEnv("ENCRYPTION_KEY")

	// Parse encryption key (32 bytes)
	encKey, err := parseHexKey(encKeyHex)
	if err != nil {
		slog.Error("invalid ENCRYPTION_KEY", "error", err)
		os.Exit(1)
	}

	// Database pool
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		slog.Error("failed to create db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Wait for DB to be ready (retry up to 20 seconds)
	for i := 0; i < 10; i++ {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		pingErr := pool.Ping(pingCtx)
		pingCancel()
		if pingErr == nil {
			break
		}
		slog.Info("waiting for database...", "attempt", i+1)
		time.Sleep(2 * time.Second)
	}

	// Run migrations
	if err := repository.RunMigrations(context.Background(), pool, migrations.InitSQL); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")

	// Dependencies
	repo := repository.New(pool, encKey)
	svc := service.New(repo)
	rec := reconciler.New(repo, 30*time.Second)
	h := handler.New(svc, rec)

	// Start reconciler in background
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	go rec.Run(bgCtx)

	// HTTP server
	r := gin.Default()
	r.Use(corsMiddleware())
	h.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("API server started", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down")
	bgCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	slog.Info("server stopped")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		slog.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func parseHexKey(hex string) ([]byte, error) {
	if len(hex) != 64 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 64 hex characters (32 bytes), got %d", len(hex))
	}
	key := make([]byte, 32)
	for i := 0; i < 32; i++ {
		hi := hexVal(hex[i*2])
		lo := hexVal(hex[i*2+1])
		if hi < 0 || lo < 0 {
			return nil, fmt.Errorf("invalid hex character in ENCRYPTION_KEY at position %d", i*2)
		}
		key[i] = byte(hi<<4 | lo)
	}
	return key, nil
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
