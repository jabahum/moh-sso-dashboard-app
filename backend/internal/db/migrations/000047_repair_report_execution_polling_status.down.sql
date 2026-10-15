-- This repairs drift from the schema already specified by migration 42.
-- Keep polling valid on rollback: removing it could fail on in-flight jobs and
-- would restore a constraint incompatible with the execution engine.
SELECT 1;
