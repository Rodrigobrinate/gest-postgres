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
	"github.com/gest-postgres/backend/internal/auth"
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

	authService := auth.NewService(pool)
	if err := authService.SeedAdminIfMissing(ctx, cfg.AdminPassword); err != nil {
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
	go serverService.RunMetricRollup(ctx)

	infraService := infra.NewService(dockerClient, pool, cfg.ManagedNetworkName, secretBox)
	go infraService.RunCronSweep(ctx, 1*time.Minute)

	router := api.NewRouter(serverService, infraService, authService, cfg.AllowedOrigins, cfg.TrustedProxies)

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		// ReadTimeout generoso (não curto) — cobre corpo de upload de
		// arquivo grande (file manager, contexto de build via upload) numa
		// conexão lenta; ainda fecha o buraco de slow-loris no corpo (hoje
		// só ReadHeaderTimeout existia, sem limite nenhum pro corpo).
		// Sem WriteTimeout de propósito: bloquearia download de backup/
		// arquivo grande e o WebSocket do terminal, que ficam abertos por
		// tempo indeterminado por design.
		ReadTimeout: 5 * time.Minute,
		IdleTimeout: 120 * time.Second,
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
