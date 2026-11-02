package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moh-sso-dashboard/internal/config"
	systemrbac "github.com/moh-sso-dashboard/internal/features/system_rbac"
	kcClientPkg "github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
)

func runStartupRBACSync(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	services services,
	adminKC *kcClientPkg.KeyAdminClient,
	appLogger *logger.Logger,
) error {
	if cfg == nil || !cfg.RBACStartupSyncEnabled {
		if appLogger != nil {
			appLogger.Info("RBAC startup sync disabled")
		}
		return nil
	}

	if services.RBAC == nil {
		return nil
	}

	if db == nil {
		return errors.New("primary database is nil; cannot run RBAC startup sync")
	}

	if appLogger != nil {
		appLogger.Info(
			"starting RBAC startup sync",
			"seed", cfg.RBACStartupSeedEnabled,
			"seed_path", cfg.RBACStartupSeedPath,
			"realm_export", cfg.RBACStartupSyncRealmExport,
			"live_keycloak", cfg.RBACStartupSyncLiveKeycloak,
			"push_to_keycloak", cfg.RBACStartupSyncPushToKeycloak,
			"sync_users", cfg.RBACStartupSyncUsers,
		)
	}

	ctx = systemrbac.WithStartupDiscovery(ctx)

	// Establish canonical defaults before discovering previously unknown clients.
	if cfg.RBACStartupSeedEnabled && db != nil {
		seed, source, err := loadStartupRBACSeed(cfg.RBACStartupSeedPath)
		if err != nil {
			return err
		}
		if err := systemrbac.ApplySeed(ctx, db, seed); err != nil {
			return err
		}
		if appLogger != nil {
			appLogger.Info("RBAC startup seed applied", "source", source)
		}
	}

	if cfg.RBACStartupSyncRealmExport {
		response, err := services.RBAC.ApplyRealmExportFileSync(ctx, cfg.RBACStartupSyncRealmExportPath)
		if err != nil {
			return err
		}
		if appLogger != nil {
			appLogger.Info(
				"RBAC realm export startup sync complete",
				"source", response.Preview.Report.Source,
				"systems_synced", response.SystemsSynced,
				"roles_synced", response.RolesSynced,
				"access_roles_synced", response.AccessRoles,
				"warnings", response.Preview.Report.Warnings,
			)
		}
	}

	if cfg.RBACStartupSyncLiveKeycloak && adminKC != nil {
		response, err := services.RBAC.ApplyLiveKeycloakSync(ctx, adminKC)
		if err != nil {
			return err
		}
		if appLogger != nil {
			appLogger.Info(
				"RBAC live Keycloak startup sync complete",
				"systems_synced", response.SystemsSynced,
				"roles_synced", response.RolesSynced,
				"access_roles_synced", response.AccessRoles,
				"warnings", response.Preview.Report.Warnings,
			)
		}
	}

	if cfg.RBACStartupSyncUsers && services.Users != nil {
		count, err := services.Users.SyncUsersFromKeycloak(ctx)
		if err != nil {
			return err
		}
		if appLogger != nil {
			appLogger.Info("Keycloak users startup sync complete", "users_synced", count)
		}
	}

	if cfg.RBACStartupSyncPushToKeycloak && adminKC != nil {
		result, err := services.RBAC.PushMissingRBACRolesToKeycloak(ctx, adminKC)
		if err != nil {
			return err
		}
		if appLogger != nil {
			appLogger.Info(
				"RBAC Keycloak push startup sync complete",
				"clients_created", result.ClientsCreated,
				"realm_roles_created", result.RealmRolesCreated,
				"client_roles_created", result.ClientRolesCreated,
				"warnings", result.Warnings,
			)
		}
	}

	return nil
}

func loadStartupRBACSeed(path string) (systemrbac.SeedFile, string, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		seed, err := systemrbac.LoadSeedFile(path)
		if err != nil {
			return systemrbac.SeedFile{}, path, fmt.Errorf("load RBAC startup seed %q: %w", path, err)
		}
		return seed, path, nil
	}

	return systemrbac.DefaultSeed(), "compiled-default", nil
}
