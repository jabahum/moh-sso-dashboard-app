-- Some installations applied migration 42 before its status constraint included
-- polling. Reassert the current contract in a new migration for those databases.
BEGIN;

ALTER TABLE report_executions
    DROP CONSTRAINT IF EXISTS report_executions_status_check;

ALTER TABLE report_executions
    ADD CONSTRAINT report_executions_status_check
    CHECK (status IN ('queued','generating','polling','generated','delivering','completed','failed','cancelled','retrying'));

COMMIT;
