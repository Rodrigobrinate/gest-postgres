// Comando api sobe o backend da plataforma: conecta no banco de metadados,
// roda migrations, conecta no Docker (via docker-socket-proxy) e serve a API HTTP.
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
	go authService.RunSessionRetentionSweep(ctx, 1*time.Hour)

	repo := server.NewRepo(pool)
	serverService := server.NewService(
		repo,
		dockerClient,
		secretBox,
		cfg.ManagedNetworkName,
		cfg.ManagedNetworkSubnet,
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
	go infraService.RunContainerMetricsCollector(ctx, 15*time.Second)

	if err := seedCloudConnect(ctx, authService, infraService, cfg); err != nil {
		slog.Error("configurando modo cloud no boot", "error", err)
	}

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

// seedCloudConnect aplica o modo cloud (setup.sh --cloud-token/
// --integration-key/--cloud-disconnect) uma vez no boot — sobe/derruba o
// cloudflared e semeia/preserva a chave de integração a partir do que o
// .env diz agora, sempre idempotente (rodar de novo sem mudar o .env não
// faz nada). Reusa o mesmo canal de confiança que ADMIN_PASSWORD já usa
// (env var lida uma vez no boot), sem precisar o setup.sh chamar a API.
func seedCloudConnect(ctx context.Context, authService *auth.Service, infraService *infra.Service, cfg *config.Config) error {
	if cfg.CloudflareTunnelToken != "" {
		status, err := infraService.CloudflareTunnelStatus(ctx)
		if err != nil {
			return fmt.Errorf("checando status do túnel cloudflare: %w", err)
		}
		if !status.Enabled || !status.Running {
			if status.Enabled && !status.Running {
				// Container de uma tentativa anterior morreu/sumiu — limpa
				// antes de recriar (nome fixo, criar em cima de um restante
				// falharia por conflito).
				_ = infraService.DisableCloudflareTunnel(ctx)
			}
			if _, err := infraService.EnableCloudflareTunnel(ctx, cfg.CloudflareTunnelToken); err != nil {
				return fmt.Errorf("subindo cloudflared: %w", err)
			}
			slog.Info("cloudflared ativo (modo cloud)")
		}
	} else {
		// CLOUD_MODE desligado (default, ou depois de --cloud-disconnect) —
		// se sobrou um túnel de uma migração anterior, derruba sozinho.
		if status, err := infraService.CloudflareTunnelStatus(ctx); err == nil && status.Enabled {
			if err := infraService.DisableCloudflareTunnel(ctx); err != nil {
				return fmt.Errorf("desligando cloudflared: %w", err)
			}
			slog.Info("cloudflared desligado (modo cloud off)")
		}
	}

	if err := authService.SeedIntegrationKeyIfProvided(ctx, cfg.IntegrationKeySeed); err != nil {
		return fmt.Errorf("semeando chave de integração: %w", err)
	}
	return nil
}
