package bootstrap

import (
	"context"
	"database/sql"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/moh-sso-dashboard/internal/cache"
	"github.com/moh-sso-dashboard/internal/config"
	"github.com/moh-sso-dashboard/internal/geography"
	kcClientPkg "github.com/moh-sso-dashboard/internal/keycloak"
	db "github.com/moh-sso-dashboard/internal/migrate"
	"github.com/moh-sso-dashboard/internal/ratelimit"
)

type databases struct {
	Primary     *sql.DB
	Remote      *sql.DB
	DWH         *sql.DB
	DQA         *sql.DB
	Observatory *sql.DB
	observatory *observatoryMonitor
	registry    *geography.Replicator
}

func initDatabases(ctx context.Context, cfg *config.Config) (databases, error) {
	if cfg.CoreOnly() {
		primary, err := db.InitDB(ctx, db.DBConfig{Driver: cfg.DBDriver, DSN: cfg.DbSource(), MaxOpenConns: 25, MaxIdleConns: 10, ConnMaxLifetime: 30 * time.Minute, WaitTimeout: 30 * time.Second})
		if err != nil {
			return databases{}, err
		}
		dbs := databases{Primary: primary}
		if err := db.MigrateDB(primary, "file://internal/db/migrations"); err != nil {
			dbs.Close()
			return databases{}, err
		}
		return dbs, nil
	}

	observatoryDSN, err := cfg.ObservatoryDBSource()
	if err != nil {
		return databases{}, err
	}
	dbConfig := func(dsn string) db.DBConfig {
		return db.DBConfig{
			Driver:          cfg.DBDriver,
			DSN:             dsn,
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
			WaitTimeout:     30 * time.Second,
		}
	}

	primaryDB, err := db.InitDB(ctx, dbConfig(cfg.DbSource()))
	if err != nil {
		return databases{}, err
	}

	remoteDB, err := db.InitDB(ctx, dbConfig(cfg.RemoteDbSource()))
	if err != nil {
		primaryDB.Close()
		return databases{}, err
	}

	dwhCfg := dbConfig(cfg.DwhDbSource())
	if cfg.IsDevelopment() {
		dwhCfg.WaitTimeout = 6 * time.Second
	}
	dwhDB, err := db.InitDB(ctx, dwhCfg)
	if err != nil {
		if !cfg.IsDevelopment() {
			primaryDB.Close()
			remoteDB.Close()
			return databases{}, err
		}
		log.Warn().
			Err(err).
			Str("dwh_host", cfg.DWHHost).
			Str("dwh_db", cfg.DWHDBName).
			Msg("DWH database unavailable in development; continuing without DWH connection")
		dwhDB = nil
	}

	var dqaDB *sql.DB
	if dwhDB != nil {
		dqaCfg := dbConfig(cfg.DqaDbSource())
		if cfg.IsDevelopment() {
			dqaCfg.WaitTimeout = 6 * time.Second
		}
		dqaDB, err = db.InitDB(ctx, dqaCfg)
		if err != nil {
			if !cfg.IsDevelopment() {
				primaryDB.Close()
				remoteDB.Close()
				dwhDB.Close()
				return databases{}, err
			}
			log.Warn().
				Err(err).
				Str("dwh_host", cfg.DWHHost).
				Msg("DQA database connection failed in development; falling back to DWH connection")
			dqaDB = dwhDB
		}
	}

	dbs := databases{Primary: primaryDB, Remote: remoteDB, DWH: dwhDB, DQA: dqaDB}
	if err := db.MigrateDB(primaryDB, "file://internal/db/migrations"); err != nil {
		dbs.Close()
		return databases{}, err
	}
	dbs.Observatory, err = db.OpenDB(dbConfig(observatoryDSN))
	if err != nil {
		dbs.Close()
		return databases{}, err
	}
	registry := geography.NewReplicator(dbs.Primary, dbs.Observatory)
	dbs.registry = registry
	dbs.observatory = startObservatoryMonitor(ctx, 15*time.Second, dbs.Observatory.PingContext,
		func(ctx context.Context) error {
			if err := db.MigrateDBContext(ctx, dbs.Observatory, "file://internal/db/surveillance_migrations"); err != nil {
				return err
			}
			return registry.Sync(ctx)
		}, registry.Refresh)
	return dbs, nil
}

func (dbs databases) Close() {
	if dbs.observatory != nil {
		dbs.observatory.Close()
	}
	if dbs.Observatory != nil {
		dbs.Observatory.Close()
	}
	if dbs.Primary != nil {
		dbs.Primary.Close()
	}
	if dbs.Remote != nil {
		dbs.Remote.Close()
	}
	if dbs.DWH != nil {
		dbs.DWH.Close()
	}
	if dbs.DQA != nil && dbs.DQA != dbs.DWH {
		dbs.DQA.Close()
	}
}

type cacheRuntime struct {
	Redis       *redis.Client
	Cache       *cache.RedisCache
	RateLimiter *ratelimit.Limiter
}

func initCache(ctx context.Context, cfg *config.Config) cacheRuntime {
	rdb := cache.NewRedisClient(cache.RedisConfig{
		Host:         cfg.RedisHost,
		Port:         cfg.RedisPort,
		Password:     cfg.RedisPassword,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	cache.MustPing(ctx, rdb)

	return cacheRuntime{
		Redis:       rdb,
		Cache:       cache.NewRedisCache(rdb),
		RateLimiter: ratelimit.New(rdb),
	}
}

type keycloakClients struct {
	Admin *kcClientPkg.KeyAdminClient
	Web   *kcClientPkg.Client
}

func initKeycloak(cfg *config.Config, cacheAdapter *cache.RedisCache) (keycloakClients, error) {
	adminKC := kcClientPkg.NewAdminClient(
		cfg.KeycloakBaseURL,
		cfg.KeycloakRealm,
		cfg.KeycloakAdminClientID,
		cfg.KeycloakAdminClientSecret,
	)

	if err := adminKC.Authenticate(); err != nil {
		return keycloakClients{}, err
	}

	webKC := kcClientPkg.NewWebClient(
		cfg.KeycloakBaseURL,
		cfg.KeycloakRealm,
		cfg.KeycloakWebClientID,
		cfg.KeycloakWebClientSecret,
		cacheAdapter,
		cfg,
	)

	return keycloakClients{
		Admin: adminKC,
		Web:   webKC,
	}, nil
}
