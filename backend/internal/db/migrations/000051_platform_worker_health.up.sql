CREATE TABLE ihp_platform_worker_heartbeats (
    worker_name TEXT PRIMARY KEY,
    last_seen TIMESTAMPTZ NOT NULL DEFAULT now()
);
