package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/moh-sso-dashboard/core"
	"github.com/moh-sso-dashboard/internal/config"
	dataqualityfeature "github.com/moh-sso-dashboard/internal/features/data_quality"
	platformfeature "github.com/moh-sso-dashboard/internal/features/platform"
	logger "github.com/moh-sso-dashboard/internal/log"
	emailRepo "github.com/moh-sso-dashboard/internal/repository/email"
	notificationDeliveryRepo "github.com/moh-sso-dashboard/internal/repository/notification_delivery"
	processRepo "github.com/moh-sso-dashboard/internal/repository/processes"
	"github.com/moh-sso-dashboard/internal/service"
	importSvc "github.com/moh-sso-dashboard/internal/service/import"
	"github.com/moh-sso-dashboard/internal/storage"
	"github.com/moh-sso-dashboard/internal/worker"
	reportschedulerfeature "github.com/mohuganda/moh-sso-dashboard/backend/modules/report-scheduler"
)

type workerDependencies struct {
	Statuses                       map[string]*core.WorkerStatus
	PrimaryDB                      *sql.DB
	Config                         *config.Config
	ProcessRepository              processRepo.ProcessRepository
	ImportService                  *importSvc.Service
	FileStorage                    storage.Storage
	EmailRepository                emailRepo.EmailRepository
	SMTPService                    worker.EmailSender
	NotificationDeliveryRepository notificationDeliveryRepo.NotificationDeliveryRepository
	EmailService                   service.EmailService
	SMSService                     service.SMSService
	AuditService                   *service.AuditService
	Logger                         *logger.Logger
	DQADB                          *sql.DB
	DWHDB                          *sql.DB
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
	if deps.DQADB != nil && deps.DWHDB != nil {
		dqa := dataqualityfeature.NewScheduleWorker(dataqualityfeature.NewDQAStore(deps.DQADB), deps.DWHDB, time.Minute, deps.Logger.Error)
		tasks = append(tasks, core.WorkerTask{Name: "DQA scheduled run", Run: dqa.Start})
	}
	document, err := worker.NewDocumentWorker(deps.ProcessRepository, deps.ImportService, 3*time.Second, deps.FileStorage, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("document worker: %w", err)
	}
	tasks = append(tasks, core.WorkerTask{Name: "document", Run: document.Start})
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
