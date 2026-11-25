package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/identityreconcile"
	"github.com/moh-sso-dashboard/internal/keycloak"
	logger "github.com/moh-sso-dashboard/internal/log"
)

// autoLinkLockKey is the Postgres advisory lock that keeps startup auto-link to
// one backend replica at a time.
const autoLinkLockKey = "moh-sso-dashboard:identity-sync-auto-link"

var (
	// autoLinkAttempts and autoLinkRetryDelay bound how long startup keeps
	// retrying an application that is not reachable yet (it may start after
	// the dashboard). Tests shorten them.
	autoLinkAttempts   = 20
	autoLinkRetryDelay = 30 * time.Second
	autoLinkPassLimit  = 30 * time.Minute
)

// startIdentitySyncAutoLink links the accounts of every application listed in
// IDENTITY_SYNC_AUTO_LINK_APPS in the background after startup: reuse rows get
// linked to their Keycloak user and create rows get a new, invited Keycloak
// user, without a person reviewing them (see identityreconcile.AutoLinker).
// It does nothing when the setting is empty or identity sync is not configured.
func startIdentitySyncAutoLink(ctx context.Context, cfg *config.Config, db *sql.DB, admin *keycloak.KeyAdminClient, log *logger.Logger) {
	apps := cfg.IdentitySyncAutoLinkApplications()
	if len(apps) == 0 {
		return
	}
	ops := newIdentitySyncOperations(cfg, db, admin)
	if ops == nil {
		log.Warn("identity sync auto-link: skipped, identity sync is not configured (Keycloak admin client, realm, HTTPS issuer)")
		return
	}
	linker := identityreconcile.AutoLinker{Batch: ops.Batch, Target: ops.Target}
	log.Warn("identity sync auto-link: enabled for ", apps, "; reuse and create rows are approved without review as ", identityreconcile.AutoLinkActor)
	go runIdentitySyncAutoLink(ctx, db, linker, apps, log)
}

func runIdentitySyncAutoLink(ctx context.Context, db *sql.DB, linker identityreconcile.AutoLinker, apps []string, log *logger.Logger) {
	conn, err := db.Conn(ctx)
	if err != nil {
		log.Error("identity sync auto-link: no database connection: ", err)
		return
	}
	defer conn.Close()
	var locked bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(hashtext($1))`, autoLinkLockKey).Scan(&locked); err != nil {
		log.Error("identity sync auto-link: lock failed: ", err)
		return
	}
	if !locked {
		log.Info("identity sync auto-link: another replica is running it")
		return
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(hashtext($1))`, autoLinkLockKey)
	}()

	for _, app := range apps {
		for attempt := 1; attempt <= autoLinkAttempts; attempt++ {
			passCtx, cancel := context.WithTimeout(ctx, autoLinkPassLimit)
			result, err := linker.Run(passCtx, app)
			cancel()
			summary, _ := json.Marshal(result)
			if err == nil {
				log.Info("identity sync auto-link: ", app, " done: ", string(summary))
				break
			}
			if ctx.Err() != nil {
				return
			}
			log.Warn("identity sync auto-link: ", app, " attempt ", attempt, "/", autoLinkAttempts, " failed: ", err, " ", string(summary))
			if attempt == autoLinkAttempts {
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(autoLinkRetryDelay):
			}
		}
	}
}
