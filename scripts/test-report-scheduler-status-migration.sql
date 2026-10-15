-- Run with psql -X -f scripts/test-report-scheduler-status-migration.sql.
-- The temporary table shadows the real table and disappears on disconnect.
\set ON_ERROR_STOP on

CREATE TEMPORARY TABLE report_executions (
    status TEXT NOT NULL CONSTRAINT report_executions_status_check
        CHECK (status IN ('queued','generating','generated','delivering','completed','failed','cancelled','retrying'))
);
INSERT INTO report_executions(status)
VALUES ('queued'),('generating'),('generated'),('delivering'),('completed'),('failed'),('cancelled'),('retrying');

-- Reproduce the worker's failure against the previously applied constraint.
DO $$
BEGIN
    BEGIN
        UPDATE report_executions SET status='polling' WHERE status='generating';
        RAISE EXCEPTION 'old constraint unexpectedly accepted polling';
    EXCEPTION WHEN check_violation THEN
        NULL;
    END;
END $$;

\ir ../backend/internal/db/migrations/000047_repair_report_execution_polling_status.up.sql

UPDATE report_executions SET status='polling' WHERE status='generating';
INSERT INTO report_executions(status) VALUES ('generating');

-- Reapplication and rollback retain the repaired status contract and all rows.
\ir ../backend/internal/db/migrations/000047_repair_report_execution_polling_status.up.sql
\ir ../backend/internal/db/migrations/000047_repair_report_execution_polling_status.down.sql

DO $$
BEGIN
    IF (SELECT count(*) FROM report_executions) <> 9
        OR (SELECT count(DISTINCT status) FROM report_executions) <> 9 THEN
        RAISE EXCEPTION 'repair failed to preserve every execution status';
    END IF;
    BEGIN
        INSERT INTO report_executions(status) VALUES ('invalid-status');
        RAISE EXCEPTION 'repaired constraint accepted an invalid status';
    EXCEPTION WHEN check_violation THEN
        NULL;
    END;
END $$;
