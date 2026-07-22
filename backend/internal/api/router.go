package api

import (
	"net"
	"net/http"

	"github.com/gest-postgres/backend/internal/auth"
	"github.com/gest-postgres/backend/internal/httpx"
	"github.com/gest-postgres/backend/internal/infra"
	"github.com/gest-postgres/backend/internal/server"
)

func NewRouter(serverService *server.Service, infraService *infra.Service, authService *auth.Service, allowedOrigins []string, trustedProxies []*net.IPNet) http.Handler {
	SetTrustedProxies(trustedProxies)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/update-check", CheckUpdate)
	// Ver update_check.go: ApplyUpdate roda git pull + setup.sh no HOST via
	// update-agent — maior blast radius de toda a API, por isso exige sessão
	// elevada (step-up de senha), não só admin (que POST/DELETE já exige
	// globalmente via withAuth). Status é admin-only: expõe log operacional
	// de build/deploy que viewer não precisa ver.
	mux.HandleFunc("GET /api/v1/update/status", requireAdmin(UpdateStatus))
	mux.HandleFunc("POST /api/v1/update/apply", requireElevated(ApplyUpdate))

	authHandler := NewAuthHandler(authService)
	mux.HandleFunc("POST /api/v1/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authHandler.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", authHandler.Me)
	mux.HandleFunc("POST /api/v1/auth/step-up", authHandler.StepUp)

	usersHandler := NewUsersHandler(authService)
	mux.HandleFunc("GET /api/v1/users", requireAdmin(usersHandler.List))
	mux.HandleFunc("POST /api/v1/users", requireAdmin(usersHandler.Create))
	mux.HandleFunc("DELETE /api/v1/users/{userId}", requireAdmin(usersHandler.Delete))
	mux.HandleFunc("POST /api/v1/users/{userId}/reset-password", requireAdmin(usersHandler.ResetPassword))

	discovery := NewDiscoveryHandler(serverService)
	mux.HandleFunc("GET /api/v1/discover", discovery.Discover)
	mux.HandleFunc("POST /api/v1/discover/{containerId}/register", discovery.Register)

	platform := NewPlatformHandler(serverService)
	mux.HandleFunc("GET /api/v1/platform-stats", platform.Stats)
	mux.HandleFunc("GET /api/v1/platform-stats-history", platform.StatsHistory)

	servers := NewServersHandler(serverService)
	mux.HandleFunc("POST /api/v1/servers", servers.Create)
	mux.HandleFunc("GET /api/v1/servers", servers.List)
	mux.HandleFunc("GET /api/v1/servers/{id}", servers.Get)
	mux.HandleFunc("PATCH /api/v1/servers/{id}", servers.Update)
	mux.HandleFunc("POST /api/v1/servers/{id}/start", servers.Start)
	mux.HandleFunc("POST /api/v1/servers/{id}/stop", servers.Stop)
	mux.HandleFunc("POST /api/v1/servers/{id}/restart", servers.Restart)
	mux.HandleFunc("DELETE /api/v1/servers/{id}", servers.Delete)

	detail := NewDetailHandler(serverService)
	mux.HandleFunc("GET /api/v1/servers/{id}/password", requireAdmin(detail.Password))
	mux.HandleFunc("POST /api/v1/servers/{id}/password/rotate", detail.RotateSuperuserPassword)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/rotate-password", detail.RotateRolePassword)
	mux.HandleFunc("GET /api/v1/servers/{id}/databases", detail.Databases)
	mux.HandleFunc("POST /api/v1/servers/{id}/databases", detail.CreateDatabase)
	mux.HandleFunc("POST /api/v1/servers/{id}/databases/test", detail.CreateTestDatabase)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/databases/{name}", detail.DropDatabase)
	mux.HandleFunc("GET /api/v1/servers/{id}/database-sizes", detail.DatabaseSizes)
	mux.HandleFunc("GET /api/v1/servers/{id}/metrics-history", detail.MetricsHistory)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables", detail.Tables)
	mux.HandleFunc("GET /api/v1/servers/{id}/erd", detail.ERD)
	mux.HandleFunc("POST /api/v1/servers/{id}/tables", detail.CreateTable)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/tables/{schema}/{table}", detail.DropTable)
	mux.HandleFunc("GET /api/v1/servers/{id}/tables/{schema}/{table}/rows", detail.TableRows)
	mux.HandleFunc("POST /api/v1/servers/{id}/query", detail.Query)
	mux.HandleFunc("POST /api/v1/servers/{id}/explain", detail.Explain)
	mux.HandleFunc("GET /api/v1/servers/{id}/activity", detail.Activity)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/cancel", detail.CancelBackend)
	mux.HandleFunc("POST /api/v1/servers/{id}/activity/{pid}/terminate", detail.TerminateBackend)
	mux.HandleFunc("GET /api/v1/servers/{id}/logs", detail.Logs)
	mux.HandleFunc("GET /api/v1/servers/{id}/logs-timeline", detail.LogsTimeline)
	mux.HandleFunc("GET /api/v1/servers/{id}/stats", detail.Stats)
	mux.HandleFunc("GET /api/v1/servers/{id}/roles", detail.ListRoles)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles", detail.CreateRole)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/roles/{name}", detail.DropRole)
	mux.HandleFunc("GET /api/v1/servers/{id}/roles/{name}/privileges", detail.RolePrivileges)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/privileges/{schema}/{table}/{privilege}/grant", detail.GrantPrivilege)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/privileges/{schema}/{table}/{privilege}/revoke", detail.RevokePrivilege)
	mux.HandleFunc("POST /api/v1/servers/{id}/roles/{name}/access", detail.SetAccessLevel)
	mux.HandleFunc("GET /api/v1/servers/{id}/triggers", detail.ListTriggers)
	mux.HandleFunc("GET /api/v1/servers/{id}/trigger-functions", detail.ListTriggerFunctions)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers", detail.CreateTrigger)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers/{schema}/{table}/{name}/enable", detail.EnableTrigger)
	mux.HandleFunc("POST /api/v1/servers/{id}/triggers/{schema}/{table}/{name}/disable", detail.DisableTrigger)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/triggers/{schema}/{table}/{name}", detail.DropTrigger)
	mux.HandleFunc("GET /api/v1/servers/{id}/slow-queries", detail.SlowQueries)
	mux.HandleFunc("POST /api/v1/servers/{id}/slow-queries/reset", detail.ResetQueryStats)
	mux.HandleFunc("POST /api/v1/servers/{id}/query-stats/enable", detail.EnableQueryStats)
	mux.HandleFunc("POST /api/v1/servers/{id}/pooling/enable", detail.EnablePooling)
	mux.HandleFunc("POST /api/v1/servers/{id}/pooling/disable", detail.DisablePooling)

	mux.HandleFunc("GET /api/v1/servers/{id}/backups", detail.ListBackups)
	mux.HandleFunc("POST /api/v1/servers/{id}/backups", detail.CreateBackup)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/backups/{backupId}", detail.DeleteBackup)
	// Download é o `pg_dump` completo do banco — banco inteiro num arquivo
	// só, além do que o modelo "viewer lê linha de tabela" já aceita
	// documentado (achado de auditoria: extrapola o que GET=viewer deveria
	// cobrir).
	mux.HandleFunc("GET /api/v1/servers/{id}/backups/{backupId}/download", requireAdmin(detail.DownloadBackup))
	mux.HandleFunc("POST /api/v1/servers/{id}/backups/{backupId}/restore", detail.RestoreBackup)
	mux.HandleFunc("GET /api/v1/servers/{id}/backup-policies", detail.ListBackupPolicies)
	mux.HandleFunc("POST /api/v1/servers/{id}/backup-policies", detail.CreateBackupPolicy)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/backup-policies/{policyId}", detail.DeleteBackupPolicy)
	mux.HandleFunc("POST /api/v1/servers/{id}/backup-policies/{policyId}/enabled", detail.SetBackupPolicyEnabled)
	mux.HandleFunc("POST /api/v1/servers/{id}/backup-policies/{policyId}/run", detail.RunBackupPolicy)

	notificationChannels := NewNotificationChannelsHandler(serverService)
	// webhook_url é segredo-portador pra Slack/Discord/etc (o path da URL É
	// a credencial) — GET sozinho não bastava pra proteger isso de viewer
	// (achado de auditoria).
	mux.HandleFunc("GET /api/v1/notification-channels", requireAdmin(notificationChannels.List))
	mux.HandleFunc("POST /api/v1/notification-channels", notificationChannels.Create)
	mux.HandleFunc("DELETE /api/v1/notification-channels/{channelId}", notificationChannels.Delete)
	mux.HandleFunc("POST /api/v1/notification-channels/{channelId}/test", notificationChannels.Test)

	gdrive := NewGDriveHandler(serverService)
	mux.HandleFunc("GET /api/v1/gdrive/status", gdrive.Status)
	mux.HandleFunc("POST /api/v1/gdrive/config", gdrive.SetConfig)
	// auth-url/callback são GET (o navegador do usuário navega até eles,
	// não dá pra ser POST) mas ESCREVEM estado persistente (oauth_state,
	// depois refresh_token) — sem requireAdmin, um viewer completa o
	// próprio consentimento OAuth e sequestra o destino de todo backup
	// futuro pra conta Google dele (achado de auditoria). O cookie do
	// admin de verdade (SameSite=Lax, navegação GET top-level) continua
	// passando normalmente no callback.
	mux.HandleFunc("GET /api/v1/gdrive/auth-url", requireAdmin(gdrive.AuthURL))
	mux.HandleFunc("GET /api/v1/gdrive/callback", requireAdmin(gdrive.Callback))
	mux.HandleFunc("POST /api/v1/gdrive/disconnect", gdrive.Disconnect)

	infraHandler := NewInfraHandler(infraService)
	terminalHandler := NewTerminalHandler(infraService, allowedOrigins)
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/exec", terminalHandler.Exec)

	// Leitura/escrita de arquivo de container/volume é tratada como
	// privilegiada (requireAdmin) e a escrita/exclusão pede step-up também,
	// paridade com o file manager de host — sem isso um viewer lê
	// /proc/1/environ de qualquer container (inclusive o do próprio backend,
	// que carrega CREDENTIAL_ENCRYPTION_KEY) só com uma sessão de leitura.
	filesHandler := NewFilesHandler(infraService)
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/files", requireAdmin(filesHandler.ListContainerFiles))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/files/stat", requireAdmin(filesHandler.StatContainerFile))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/files/content", requireAdmin(filesHandler.ReadContainerFile))
	mux.HandleFunc("PUT /api/v1/infra/containers/{containerId}/files/content", requireElevated(filesHandler.WriteContainerFile))
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/files/upload", requireElevated(filesHandler.UploadContainerFile))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/files/download", requireAdmin(filesHandler.DownloadContainerFile))
	mux.HandleFunc("DELETE /api/v1/infra/containers/{containerId}/files", requireElevated(filesHandler.DeleteContainerFile))

	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/files", requireAdmin(filesHandler.ListVolumeFiles))
	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/files/stat", requireAdmin(filesHandler.StatVolumeFile))
	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/files/content", requireAdmin(filesHandler.ReadVolumeFile))
	mux.HandleFunc("PUT /api/v1/infra/volumes/{volumeName}/files/content", requireElevated(filesHandler.WriteVolumeFile))
	mux.HandleFunc("POST /api/v1/infra/volumes/{volumeName}/files/upload", requireElevated(filesHandler.UploadVolumeFile))
	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/files/download", requireAdmin(filesHandler.DownloadVolumeFile))
	mux.HandleFunc("DELETE /api/v1/infra/volumes/{volumeName}/files", requireElevated(filesHandler.DeleteVolumeFile))
	mux.HandleFunc("GET /api/v1/infra/containers", infraHandler.ListContainers)
	mux.HandleFunc("POST /api/v1/infra/containers", infraHandler.CreateContainer)
	mux.HandleFunc("POST /api/v1/infra/containers/from-git", infraHandler.CreateContainerFromGit)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/start", infraHandler.StartContainer)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/stop", infraHandler.StopContainer)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/restart", infraHandler.RestartContainer)
	mux.HandleFunc("DELETE /api/v1/infra/containers/{containerId}", infraHandler.RemoveContainer)
	// logs/inspect/stats admin-only: mesmo segredo que o file manager já
	// protege (env do container, que pro backend inclui
	// CREDENTIAL_ENCRYPTION_KEY/ADMIN_PASSWORD) vaza igual por essas rotas
	// irmãs — log pode ecoar senha de bootstrap, inspect devolve Config.Env
	// cru. GET sozinho não era gate suficiente (achado de auditoria).
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/logs", requireAdmin(infraHandler.ContainerLogs))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/inspect", requireAdmin(infraHandler.ContainerDetail))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/stats", requireAdmin(infraHandler.ContainerStats))
	mux.HandleFunc("GET /api/v1/infra/containers/{containerId}/stats-history", infraHandler.ContainerStatsHistory)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/networks", infraHandler.ConnectContainerNetwork)
	mux.HandleFunc("DELETE /api/v1/infra/containers/{containerId}/networks/{networkName}", infraHandler.DisconnectContainerNetwork)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/env", infraHandler.UpdateContainerEnv)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/volumes", infraHandler.AttachContainerVolume)
	mux.HandleFunc("POST /api/v1/infra/containers/{containerId}/resources", infraHandler.UpdateContainerResources)
	mux.HandleFunc("POST /api/v1/infra/system-prune", infraHandler.SystemPrune)
	mux.HandleFunc("GET /api/v1/infra/networks", infraHandler.ListNetworks)
	mux.HandleFunc("POST /api/v1/infra/networks", infraHandler.CreateNetwork)
	mux.HandleFunc("DELETE /api/v1/infra/networks/{networkId}", infraHandler.RemoveNetwork)
	mux.HandleFunc("GET /api/v1/infra/volumes", infraHandler.ListVolumes)
	mux.HandleFunc("POST /api/v1/infra/volumes", infraHandler.CreateVolume)
	mux.HandleFunc("DELETE /api/v1/infra/volumes/{volumeName}", infraHandler.RemoveVolume)

	volumeBackupsHandler := NewVolumeBackupsHandler(infraService)
	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/backups", volumeBackupsHandler.List)
	mux.HandleFunc("POST /api/v1/infra/volumes/{volumeName}/backups", volumeBackupsHandler.Create)
	mux.HandleFunc("GET /api/v1/infra/volumes/{volumeName}/backups/{backupId}/download", volumeBackupsHandler.Download)
	mux.HandleFunc("POST /api/v1/infra/volumes/{volumeName}/backups/{backupId}/restore", volumeBackupsHandler.Restore)
	mux.HandleFunc("DELETE /api/v1/infra/volumes/{volumeName}/backups/{backupId}", volumeBackupsHandler.Delete)
	mux.HandleFunc("GET /api/v1/infra/compose", infraHandler.ListComposeProjects)
	mux.HandleFunc("POST /api/v1/infra/compose", infraHandler.DeployCompose)
	mux.HandleFunc("DELETE /api/v1/infra/compose/{name}", infraHandler.RemoveComposeProject)
	mux.HandleFunc("POST /api/v1/infra/build", infraHandler.BuildFromDockerfile)
	mux.HandleFunc("POST /api/v1/infra/build/upload", infraHandler.BuildFromContext)
	mux.HandleFunc("GET /api/v1/infra/traefik", infraHandler.TraefikStatus)
	mux.HandleFunc("POST /api/v1/infra/traefik/enable", infraHandler.EnableTraefik)
	mux.HandleFunc("POST /api/v1/infra/traefik/disable", infraHandler.DisableTraefik)
	mux.HandleFunc("GET /api/v1/infra/proxy-routes", infraHandler.ListProxyRoutes)
	mux.HandleFunc("POST /api/v1/infra/proxy-routes", infraHandler.CreateProxyRoute)
	mux.HandleFunc("DELETE /api/v1/infra/proxy-routes/{routeId}", infraHandler.DeleteProxyRoute)
	mux.HandleFunc("GET /api/v1/infra/firewall-rules", infraHandler.ListFirewallRules)
	mux.HandleFunc("POST /api/v1/infra/firewall-rules", infraHandler.AddFirewallRule)
	mux.HandleFunc("DELETE /api/v1/infra/firewall-rules/{port}/{proto}", infraHandler.RemoveFirewallRule)
	hostFilesHandler := NewHostFilesHandler(infraService)
	mux.HandleFunc("GET /api/v1/infra/host-files", hostFilesHandler.List)
	mux.HandleFunc("GET /api/v1/infra/host-files/stat", hostFilesHandler.Stat)
	mux.HandleFunc("GET /api/v1/infra/host-files/content", hostFilesHandler.Read)
	mux.HandleFunc("PUT /api/v1/infra/host-files/content", requireElevated(hostFilesHandler.Write))
	mux.HandleFunc("POST /api/v1/infra/host-files/upload", requireElevated(hostFilesHandler.Upload))
	mux.HandleFunc("GET /api/v1/infra/host-files/download", hostFilesHandler.Download)
	mux.HandleFunc("DELETE /api/v1/infra/host-files", requireElevated(hostFilesHandler.Delete))

	cronJobsHandler := NewCronJobsHandler(infraService)
	mux.HandleFunc("GET /api/v1/infra/cron-jobs", cronJobsHandler.List)
	mux.HandleFunc("POST /api/v1/infra/cron-jobs", cronJobsHandler.Create)
	mux.HandleFunc("DELETE /api/v1/infra/cron-jobs/{cronJobId}", cronJobsHandler.Delete)
	mux.HandleFunc("POST /api/v1/infra/cron-jobs/{cronJobId}/enabled", cronJobsHandler.SetEnabled)
	mux.HandleFunc("POST /api/v1/infra/cron-jobs/{cronJobId}/run", cronJobsHandler.RunNow)

	gitDeploymentsHandler := NewGitDeploymentsHandler(infraService)
	mux.HandleFunc("GET /api/v1/infra/git-deployments", gitDeploymentsHandler.List)
	mux.HandleFunc("POST /api/v1/infra/git-deployments", gitDeploymentsHandler.Create)
	mux.HandleFunc("DELETE /api/v1/infra/git-deployments/{deploymentId}", gitDeploymentsHandler.Delete)
	mux.HandleFunc("POST /api/v1/infra/git-deployments/{deploymentId}/redeploy", gitDeploymentsHandler.RedeployNow)
	mux.HandleFunc("POST /api/v1/infra/git-deployments/{deploymentId}/webhook", gitDeploymentsHandler.Webhook)

	mux.HandleFunc("GET /api/v1/infra/git-credentials", infraHandler.ListGitCredentials)
	mux.HandleFunc("POST /api/v1/infra/git-credentials", infraHandler.CreateGitCredential)
	mux.HandleFunc("DELETE /api/v1/infra/git-credentials/{credentialId}", infraHandler.DeleteGitCredential)
	mux.HandleFunc("GET /api/v1/servers/{id}/hba-rules", detail.ListHbaRules)
	mux.HandleFunc("POST /api/v1/servers/{id}/hba-rules", detail.AddHbaRule)
	mux.HandleFunc("POST /api/v1/servers/{id}/hba-rules/delete", detail.DeleteHbaRule)

	mux.HandleFunc("GET /api/v1/servers/{id}/config", detail.GetExpandedConfig)
	mux.HandleFunc("PUT /api/v1/servers/{id}/config", detail.PutExpandedConfig)
	mux.HandleFunc("GET /api/v1/servers/{id}/extensions", detail.Extensions)
	mux.HandleFunc("POST /api/v1/servers/{id}/extensions/{name}/enable", detail.EnableExtension)
	mux.HandleFunc("POST /api/v1/servers/{id}/extensions/{name}/disable", detail.DisableExtension)

	mux.HandleFunc("GET /api/v1/servers/{id}/views", detail.ListViews)
	mux.HandleFunc("POST /api/v1/servers/{id}/views", detail.CreateView)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/views/{schema}/{name}", detail.DropView)

	mux.HandleFunc("GET /api/v1/servers/{id}/materialized-views", detail.ListMaterializedViews)
	mux.HandleFunc("POST /api/v1/servers/{id}/materialized-views", detail.CreateMaterializedView)
	mux.HandleFunc("POST /api/v1/servers/{id}/materialized-views/{schema}/{name}/refresh", detail.RefreshMaterializedView)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/materialized-views/{schema}/{name}", detail.DropMaterializedView)

	mux.HandleFunc("GET /api/v1/servers/{id}/sequences", detail.ListSequences)
	mux.HandleFunc("POST /api/v1/servers/{id}/sequences", detail.CreateSequence)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/sequences/{schema}/{name}", detail.DropSequence)

	mux.HandleFunc("GET /api/v1/servers/{id}/types", detail.ListTypes)
	mux.HandleFunc("POST /api/v1/servers/{id}/types/enum", detail.CreateEnumType)
	mux.HandleFunc("POST /api/v1/servers/{id}/types/domain", detail.CreateDomain)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/types/{schema}/{name}", detail.DropType)

	mux.HandleFunc("GET /api/v1/servers/{id}/functions", detail.ListFunctions)
	mux.HandleFunc("POST /api/v1/servers/{id}/functions", detail.CreateFunction)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/functions/{schema}/{name}", detail.DropFunction)

	mux.HandleFunc("GET /api/v1/servers/{id}/indexes/suggestions", detail.SuggestIndexes)
	mux.HandleFunc("GET /api/v1/servers/{id}/indexes/unused", detail.UnusedIndexes)
	mux.HandleFunc("POST /api/v1/servers/{id}/indexes/{schema}/{name}/reindex-concurrently", detail.ReindexConcurrently)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/indexes/{schema}/{name}", detail.DropIndex)

	mux.HandleFunc("GET /api/v1/servers/{id}/bloat", detail.Bloat)
	mux.HandleFunc("GET /api/v1/servers/{id}/wraparound", detail.Wraparound)
	mux.HandleFunc("GET /api/v1/servers/{id}/health-score", detail.HealthScore)
	mux.HandleFunc("GET /api/v1/servers/{id}/capacity-forecast", detail.CapacityForecast)
	mux.HandleFunc("GET /api/v1/servers/{id}/tuning-suggestions", detail.SuggestTuning)

	mux.HandleFunc("GET /api/v1/servers/{id}/retention-policies", detail.ListRetentionPolicies)
	mux.HandleFunc("POST /api/v1/servers/{id}/retention-policies", detail.CreateRetentionPolicy)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/retention-policies/{policyId}", detail.DeleteRetentionPolicy)
	mux.HandleFunc("POST /api/v1/servers/{id}/retention-policies/{policyId}/enabled", detail.SetRetentionPolicyEnabled)
	mux.HandleFunc("POST /api/v1/servers/{id}/retention-policies/{policyId}/run", detail.RunRetentionPolicy)

	mux.HandleFunc("GET /api/v1/servers/{id}/alert-rules", detail.ListAlertRules)
	mux.HandleFunc("POST /api/v1/servers/{id}/alert-rules", detail.CreateAlertRule)
	mux.HandleFunc("DELETE /api/v1/servers/{id}/alert-rules/{ruleId}", detail.DeleteAlertRule)
	mux.HandleFunc("POST /api/v1/servers/{id}/alert-rules/{ruleId}/enabled", detail.SetAlertRuleEnabled)

	return withCORS(allowedOrigins)(withAuth(authService)(withLogging(mux)))
}
