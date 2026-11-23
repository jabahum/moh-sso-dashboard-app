-- =====================================================
-- Notification Deliveries
-- =====================================================

-- name: CreateNotificationDelivery :one
INSERT INTO notification_deliveries (
  notification_id,
  channel,
  status,
  recipient,
  template_name,
  template_data,
  payload,
  scheduled_at,
  sent_at,
  attempts,
  max_attempts
)
VALUES (
  $1,
  $2,
  COALESCE(sqlc.narg('delivery_status')::text, 'PENDING'),
  $3,
  $4,
  $5,
  $6,
  $7,
  -- A delivery created already SENT (in-app: the stored notification is the
  -- delivery) records that single completed attempt.
  CASE WHEN sqlc.narg('delivery_status')::text = 'SENT' THEN now() END,
  CASE WHEN sqlc.narg('delivery_status')::text = 'SENT' THEN 1 ELSE 0 END,
  COALESCE(sqlc.narg('delivery_max_attempts'), 5)
)
RETURNING *;


-- name: ListNotificationDeliveriesByNotificationID :many
SELECT *
FROM notification_deliveries
WHERE notification_id = $1
ORDER BY created_at ASC;


-- name: GetNotificationDeliveryByID :one
SELECT *
FROM notification_deliveries
WHERE id = $1;


-- name: ClaimPendingNotificationDeliveries :many
-- Claims due PENDING/RETRY deliveries plus PROCESSING deliveries whose lease
-- expired (the claiming worker crashed or was stopped mid-batch) and still have
-- attempts left. Claiming starts an attempt, so `attempts` is incremented here
-- and nowhere else: it counts delivery attempts actually started, including
-- one interrupted by a crash, and never manual retries.
WITH next_deliveries AS (
  SELECT nd.id
  FROM notification_deliveries nd
  WHERE
    nd.channel = sqlc.arg(channel)
    AND (
      (
        nd.status IN ('PENDING', 'RETRY')
        AND (nd.scheduled_at IS NULL OR nd.scheduled_at <= now())
      )
      OR (
        nd.status = 'PROCESSING'
        AND COALESCE(nd.locked_at, nd.updated_at) < now() - make_interval(secs => sqlc.arg(lease_seconds)::double precision)
        AND nd.attempts < nd.max_attempts
      )
    )
  ORDER BY nd.scheduled_at NULLS FIRST, nd.created_at ASC
  LIMIT sqlc.arg(batch_limit)
  FOR UPDATE SKIP LOCKED
)
UPDATE notification_deliveries nd
SET
  status = 'PROCESSING',
  attempts = nd.attempts + 1,
  locked_at = now(),
  updated_at = now()
FROM next_deliveries next
WHERE nd.id = next.id
RETURNING nd.*;


-- name: FailExhaustedStaleNotificationDeliveries :execrows
-- A PROCESSING delivery whose lease expired on its final attempt is not
-- reclaimed (that would exceed max_attempts); it is failed instead so it does
-- not stay PROCESSING forever.
UPDATE notification_deliveries
SET
  status = 'FAILED',
  locked_at = NULL,
  last_error = 'Processing lease expired during the final attempt',
  updated_at = now()
WHERE
  channel = sqlc.arg(channel)
  AND status = 'PROCESSING'
  AND COALESCE(locked_at, updated_at) < now() - make_interval(secs => sqlc.arg(lease_seconds)::double precision)
  AND attempts >= max_attempts;


-- name: MarkNotificationDeliverySent :exec
UPDATE notification_deliveries
SET
  status = 'SENT',
  sent_at = now(),
  locked_at = NULL,
  last_error = NULL,
  updated_at = now()
WHERE id = $1;


-- name: MarkNotificationDeliveryRetry :exec
UPDATE notification_deliveries
SET
  status = 'RETRY',
  locked_at = NULL,
  last_error = $2,
  scheduled_at = now() + ($3::text)::interval,
  updated_at = now()
WHERE id = $1;


-- name: MarkNotificationDeliveryFailed :exec
UPDATE notification_deliveries
SET
  status = 'FAILED',
  locked_at = NULL,
  last_error = $2,
  updated_at = now()
WHERE id = $1;


-- name: CancelNotificationDelivery :exec
UPDATE notification_deliveries
SET
  status = 'CANCELLED',
  locked_at = NULL,
  updated_at = now()
WHERE id = $1;


-- name: CountPendingNotificationDeliveriesByChannel :one
SELECT COUNT(*)::bigint
FROM notification_deliveries
WHERE
  channel = $1
  AND status IN ('PENDING', 'RETRY')
  AND (scheduled_at IS NULL OR scheduled_at <= now());


-- name: ListNotificationDeliveries :many
SELECT nd.*
FROM notification_deliveries nd
WHERE
  (
    sqlc.narg('filter_channel')::text IS NULL
    OR nd.channel = sqlc.narg('filter_channel')::text
  )
  AND (
    sqlc.narg('filter_status')::text IS NULL
    OR nd.status = sqlc.narg('filter_status')::text
  )
  AND (
    sqlc.narg('filter_search')::text IS NULL
    OR nd.id::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR nd.notification_id::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR nd.recipient::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR COALESCE(nd.provider_message_id, '') ILIKE '%' || sqlc.narg('filter_search')::text || '%'
  )
ORDER BY nd.created_at DESC
LIMIT $1 OFFSET $2;


-- name: CountNotificationDeliveries :one
SELECT COUNT(*)::bigint
FROM notification_deliveries nd
WHERE
  (
    sqlc.narg('filter_channel')::text IS NULL
    OR nd.channel = sqlc.narg('filter_channel')::text
  )
  AND (
    sqlc.narg('filter_status')::text IS NULL
    OR nd.status = sqlc.narg('filter_status')::text
  )
  AND (
    sqlc.narg('filter_search')::text IS NULL
    OR nd.id::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR nd.notification_id::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR nd.recipient::text ILIKE '%' || sqlc.narg('filter_search')::text || '%'
    OR COALESCE(nd.provider_message_id, '') ILIKE '%' || sqlc.narg('filter_search')::text || '%'
  );


-- name: ListNotificationDeliveryMetrics :many
SELECT
  nd.channel,
  COALESCE(nd.provider, '')::text AS provider,
  COUNT(*)::bigint AS total,
  COUNT(*) FILTER (WHERE nd.status = 'PENDING')::bigint AS pending,
  COUNT(*) FILTER (WHERE nd.status = 'PROCESSING')::bigint AS processing,
  COUNT(*) FILTER (WHERE nd.status = 'SENT')::bigint AS sent,
  COUNT(*) FILTER (WHERE nd.status = 'FAILED')::bigint AS failed,
  COUNT(*) FILTER (WHERE nd.status = 'RETRY')::bigint AS retry,
  COUNT(*) FILTER (WHERE nd.status = 'CANCELLED')::bigint AS cancelled,
  COALESCE(
    AVG(EXTRACT(EPOCH FROM (nd.sent_at - nd.created_at)))
      FILTER (WHERE nd.sent_at IS NOT NULL),
    0
  )::double precision AS avg_processing_seconds
FROM notification_deliveries nd
GROUP BY nd.channel, COALESCE(nd.provider, '')
ORDER BY nd.channel ASC, provider ASC;
