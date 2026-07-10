package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"edugrade-enterprise/services/api-gateway/internal/auth"
	"edugrade-enterprise/services/api-gateway/internal/config"
	"edugrade-enterprise/services/api-gateway/internal/db"
	"edugrade-enterprise/services/api-gateway/internal/logger"
	"edugrade-enterprise/services/api-gateway/internal/server"
)

func main() {
	cfg, err := config.Load(".env")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logg := logger.New(os.Stdout, cfg.Service.LogLevel)
	if len(os.Args) > 1 && os.Args[1] == "bootstrap-admin" {
		if err := runBootstrapAdmin(context.Background(), cfg, logg); err != nil {
			logg.Error(context.Background(), "bootstrap admin failed", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
		return
	}
	srv, cleanup, err := server.New(cfg, logg)
	if err != nil {
		logg.Error(context.Background(), "create server failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	defer cleanup()

	httpServer := &http.Server{
		Addr:              cfg.Service.Addr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    cfg.Security.MaxHeaderBytes,
	}

	errCh := make(chan error, 1)
	go func() {
		logg.Info(context.Background(), "api gateway starting", map[string]any{"addr": cfg.Service.Addr()})
		errCh <- httpServer.ListenAndServe()
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		logg.Info(context.Background(), "shutdown signal received", map[string]any{"signal": sig.String()})
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			logg.Error(context.Background(), "server stopped unexpectedly", map[string]any{"error": err.Error()})
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logg.Error(context.Background(), "graceful shutdown failed", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
	logg.Info(context.Background(), "api gateway stopped", nil)
}

func runBootstrapAdmin(ctx context.Context, cfg config.Config, logg *logger.Logger) error {
	password := os.Getenv("EDUGRADE_BOOTSTRAP_PASSWORD")
	if password == "" {
		return errors.New("EDUGRADE_BOOTSTRAP_PASSWORD is required")
	}
	postgresDB, closePostgres, err := db.OpenPostgres(cfg.Postgres)
	if err != nil {
		return err
	}
	defer closePostgres()
	result, err := auth.BootstrapInitialAdmin(ctx, auth.NewPostgresStore(postgresDB), auth.BootstrapAdminInput{
		TenantCode:  os.Getenv("EDUGRADE_BOOTSTRAP_TENANT_CODE"),
		RoleCode:    os.Getenv("EDUGRADE_BOOTSTRAP_ROLE_CODE"),
		Username:    envOrDefault("EDUGRADE_BOOTSTRAP_USERNAME", "platform_admin"),
		DisplayName: envOrDefault("EDUGRADE_BOOTSTRAP_DISPLAY_NAME", "Platform Admin"),
		Password:    password,
	})
	if err != nil {
		return err
	}
	logg.Info(ctx, "bootstrap admin completed", map[string]any{
		"tenant_code": result.TenantCode,
		"username":    result.Username,
		"role_code":   result.RoleCode,
	})
	return nil
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
