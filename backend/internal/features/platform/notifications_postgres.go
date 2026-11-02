package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/moh-sso-dashboard/internal/observability"
)

func (r *PostgresRepository) SubmitNotification(ctx context.Context, p Principal, key string, input NotificationInput) (NotificationStatus, error) {
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return NotificationStatus{}, err
	}
	defer tx.Rollback()
	// A database lock provides a shared quota/idempotency boundary across backend replicas.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "platform-notifications:"+p.ApplicationID); err != nil {
		return NotificationStatus{}, err
	}
	var scopes []string
	err = tx.QueryRowContext(ctx, `SELECT i.scopes FROM ihp_application_identities i JOIN ihp_systems s ON s.client_id=i.application_client_id
 WHERE i.oauth_client_id=$1 AND i.subject=$2 AND i.application_client_id=$3 AND i.enabled AND s.enabled AND s.lifecycle_status='active'
 FOR SHARE OF i,s`, p.ClientID, p.Subject, p.ApplicationID).Scan(pq.Array(&scopes))
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (!contains(scopes, NotificationsWrite) || !p.HasScope(NotificationsWrite))) {
		return NotificationStatus{}, ErrForbidden
	}
	if err != nil {
		return NotificationStatus{}, err
	}
	var id, hash string
	err = tx.QueryRowContext(ctx, `SELECT id::text, request_hash FROM ihp_application_notifications WHERE application_client_id=$1 AND idempotency_key=$2`, p.ApplicationID, key).Scan(&id, &hash)
	if err == nil {
		if hash != notificationHash(input) {
			return NotificationStatus{}, ErrIdempotencyConflict
		}
		result, err := getNotification(ctx, tx, p.ApplicationID, id)
		if err != nil {
			return NotificationStatus{}, err
		}
		result.Replayed = true
		if err := tx.Commit(); err != nil {
			return NotificationStatus{}, err
		}
		return result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return NotificationStatus{}, err
	}
	var recent int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM ihp_application_notifications WHERE application_client_id=$1 AND created_at > now()-interval '1 minute'`, p.ApplicationID).Scan(&recent); err != nil {
		return NotificationStatus{}, err
	}
	if recent >= 60 {
		return NotificationStatus{}, ErrQuota
	}
	id, notificationID, deliveryID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	metadata, _ := json.Marshal(map[string]string{"application_id": p.ApplicationID, "request_id": observability.RequestIDFromContext(ctx), "correlation_id": observability.CorrelationIDFromContext(ctx)})
	_, err = tx.ExecContext(ctx, `INSERT INTO notifications (id,type,title,message,severity,target_role,metadata) VALUES ($1,'platform.application',$2,$3,'info',$4,$5::jsonb)`, notificationID, input.Title, input.Message, "application:"+p.ApplicationID, string(metadata))
	if err != nil {
		return NotificationStatus{}, err
	}
	recipient, _ := json.Marshal(input.Recipient)
	payload, _ := json.Marshal(map[string]string{"subject": input.Title, "text_body": input.Message})
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_deliveries (id,notification_id,channel,recipient,payload,max_attempts) VALUES ($1,$2,'email',$3::jsonb,$4::jsonb,5)`, deliveryID, notificationID, string(recipient), string(payload))
	if err != nil {
		return NotificationStatus{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_application_notifications (id,application_client_id,idempotency_key,request_hash,notification_id,delivery_id,created_by,request_id,correlation_id)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, p.ApplicationID, key, notificationHash(input), notificationID, deliveryID, p.Subject, observability.RequestIDFromContext(ctx), observability.CorrelationIDFromContext(ctx))
	if err != nil {
		return NotificationStatus{}, err
	}
	// Audit identifiers only: never store recipients, message bodies or tokens in operational audit details.
	details, _ := json.Marshal(map[string]string{"requestId": observability.RequestIDFromContext(ctx), "correlationId": observability.CorrelationIDFromContext(ctx), "channel": "email"})
	_, err = tx.ExecContext(ctx, `INSERT INTO ihp_rbac_audit_events (actor_user_id,action,resource_type,resource_id,system_client_id,details)
 VALUES ($1,'application.notification.queued','application_notification',$2,$3,$4::jsonb)`, p.Subject, id, p.ApplicationID, string(details))
	if err != nil {
		return NotificationStatus{}, err
	}
	result, err := getNotification(ctx, tx, p.ApplicationID, id)
	if err != nil {
		return NotificationStatus{}, err
	}
	if err := tx.Commit(); err != nil {
		return NotificationStatus{}, err
	}
	return result, nil
}
func (r *PostgresRepository) GetNotification(ctx context.Context, application, id string) (NotificationStatus, error) {
	return getNotification(ctx, r.DB, application, id)
}

type notificationQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func getNotification(ctx context.Context, db notificationQuerier, application, id string) (NotificationStatus, error) {
	var result NotificationStatus
	err := db.QueryRowContext(ctx, `SELECT n.id::text,n.application_client_id,'email',
 CASE WHEN d.id IS NULL THEN 'EXPIRED' WHEN d.status='SENT' THEN COALESCE(e.status,'EXPIRED') ELSE d.status END,
 COALESCE(d.status,'EXPIRED'),COALESCE(d.attempts,0),COALESCE(e.attempts,0),n.created_at::text
 FROM ihp_application_notifications n LEFT JOIN notification_deliveries d ON d.id=n.delivery_id
 LEFT JOIN LATERAL (SELECT status,attempts FROM email_outbox WHERE metadata ? 'delivery_id' AND metadata->>'delivery_id'=d.id::text ORDER BY created_at DESC LIMIT 1) e ON TRUE
 WHERE n.application_client_id=$1 AND n.id=$2::uuid`, application, id).Scan(&result.ID, &result.ApplicationID, &result.Channel, &result.Status, &result.DispatchStatus, &result.DispatchAttempts, &result.DeliveryAttempts, &result.CreatedAt)
	return result, err
}
