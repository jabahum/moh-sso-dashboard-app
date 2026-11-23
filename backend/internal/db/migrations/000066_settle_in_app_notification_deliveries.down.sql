-- Data-only migration: the rows it settled cannot be told apart from in-app
-- deliveries created SENT afterwards, and putting them back in a queue state
-- no worker drains would only restore the bug. Intentionally a no-op.
SELECT 1;
