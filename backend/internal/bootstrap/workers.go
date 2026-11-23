package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moh-sso-dashboard/internal/config"
	platformfeature "github.com/moh-sso-dashboard/internal/features/platform"
	logger "github.com/moh-sso-dashboard/internal/log"
	emailRepo "github.com/moh-sso-dashboard/internal/repository/email"
	notificationDeliveryRepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	"github.com/moh-sso-dashboard/internal/service"
	"github.com/moh-sso-dashboard/internal/worker"
	"github.com/mohuganda/moh-sso-dashboard/backend/core"
	reportschedulerfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

type workerDependencies struct {
	Statuses                       map[string]*core.WorkerStatus
	PrimaryDB                      *sql.DB
	Config                         *config.Config
	EmailRepository                emailRepo.EmailRepository
	SMTPService                    worker.EmailSender
	NotificationDeliveryRepository notificationDeliveryRepo.NotificationDeliveryRepository
	EmailService                   service.EmailService
	SMSService                     service.SMSService
	AuditService                   *service.AuditService
	Logger                         *logger.Logger
}

func enabledDeliveryWorkers(cfg *config.Config) []string {
	if cfg.CoreOnly() {
		return []string{}
	}
	names := []string{"email-outbox"}
	if cfg == nil || cfg.NotificationDelivery.WorkerEnabled {
		names = append(names, "notification-email")
	}
	return names
}
func schedulerWorker(cfg *config.Config, services services, log *logger.Logger) *reportschedulerfeature.Worker {
	if !cfg.ModuleEnabled("report-scheduler") {
		return nil
	}
	return reportschedulerfeature.NewWorker(services.ReportScheduler, cfg.ReportSchedulerWorkerInterval, log.Error)
}

func startBackgroundWorkers(ctx context.Context, deps workerDependencies) (core.Stop, error) {
	if deps.Config.CoreOnly() {
		return core.StartWorkerGroup(ctx, nil, nil), nil
	}
	heartbeat := func(name string) func(context.Context) {
		return func(ctx context.Context) {
			if deps.PrimaryDB == nil {
				return
			}
			timeout, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			if err := (&platformfeature.PostgresRepository{DB: deps.PrimaryDB}).RecordWorkerHeartbeat(timeout, name); err != nil {
				deps.Logger.Error("platform worker heartbeat unavailable", "worker", name)
			}
		}
	}
	// Prepare every worker before launching any goroutine so constructor failures
	// cannot leave partially started workers accessing released resources.
	tasks := []core.WorkerTask{}
	email, err := worker.NewEmailWorker(deps.EmailRepository, deps.SMTPService, 3*time.Second, 20, 3, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("email worker: %w", err)
	}
	email.SetStatus(deps.Statuses["email-outbox"])
	email.SetHeartbeat(heartbeat("email-outbox"))
	tasks = append(tasks, core.WorkerTask{Name: "email-outbox", Run: email.Start})
	if deps.Config == nil || deps.Config.NotificationDelivery.WorkerEnabled {
		interval := 3 * time.Second
		batch := int32(20)
		if deps.Config != nil {
			interval = deps.Config.NotificationDelivery.WorkerInterval
			batch = deps.Config.NotificationDelivery.BatchSize
		}
		notification, err := worker.NewNotificationEmailDeliveryWorker(deps.NotificationDeliveryRepository, deps.EmailService, interval, batch, 3, deps.Logger)
		if err != nil {
			return nil, fmt.Errorf("notification email worker: %w", err)
		}
		notification.SetStatus(deps.Statuses["notification-email"])
		notification.SetHeartbeat(heartbeat("notification-email"))
		tasks = append(tasks, core.WorkerTask{Name: "notification-email", Run: notification.Start})
		if deps.SMSService != nil && deps.SMSService.Enabled() {
			sms, err := worker.NewNotificationSMSDeliveryWorker(deps.NotificationDeliveryRepository, deps.SMSService, interval, batch, 3, deps.Logger, deps.AuditService)
			if err != nil {
				return nil, fmt.Errorf("notification SMS worker: %w", err)
			}
			tasks = append(tasks, core.WorkerTask{Name: "notification-sms", Run: sms.Start})
		}
	}
	return core.StartWorkerGroup(ctx, tasks, deps.Logger.Error), nil
}
