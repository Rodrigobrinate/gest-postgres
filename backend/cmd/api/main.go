// Comando api sobe o backend da plataforma: conecta no banco de metadados,
// roda migrations, conecta no Docker (via docker-socket-proxy) e serve a API HTTP.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gest-postgres/backend/internal/api"
	"github.com/gest-postgres/backend/internal/config"
	"github.com/gest-postgres/backend/internal/crypto"
	"github.com/gest-postgres/backend/internal/db"
	"github.com/gest-postgres/backend/internal/docker"
	"github.com/gest-postgres/backend/internal/infra"
	"github.com/gest-postgres/backend/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("falha ao iniciar backend", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	pool, err := db.Connect(ctx, cfg.MetadataDatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}

	dockerClient, err := docker.NewClient(cfg.DockerHost)
	if err != nil {
		return err
	}
	defer dockerClient.Close()

	secretBox, err := crypto.NewSecretBox(cfg.CredentialEncryptionKey)
	if err != nil {
		return err
	}

	repo := server.NewRepo(pool)
	serverService := server.NewService(
		repo,
		dockerClient,
		secretBox,
		cfg.ManagedNetworkName,
		cfg.ManagedPortRangeStart,
		cfg.ManagedPortRangeEnd,
	)

	go serverService.RunMetricsCollector(ctx, 15*time.Second)
	go serverService.RunRetentionSweep(ctx, 1*time.Hour)
	go serverService.RunAlertSweep(ctx, 1*time.Minute)
	go serverService.RunPlatformHistoryCollector(ctx, 15*time.Second)
	go serverService.RunBackupSweep(ctx, 1*time.Minute)

	infraService := infra.NewService(dockerClient, pool, cfg.ManagedNetworkName)

	router := api.NewRouter(serverService, infraService)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("erro no shutdown do servidor HTTP", "error", err)
		}
	}()

	slog.Info("backend escutando", "addr", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
