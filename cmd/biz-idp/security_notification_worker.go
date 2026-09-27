package main

import (
	"context"
	"fmt"
	"os"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	"gorm.io/gorm"
)

type idpSecurityNotificationRunner struct {
	worker notificationapp.SecurityDeliveryWorker
}

func newIDPSecurityNotificationRunner(
	database *gorm.DB,
	protection *accesspersistence.VerificationProtection,
	sender accessports.SecurityNotificationSender,
	autoMigrate bool,
) (*idpSecurityNotificationRunner, error) {
	if database == nil || protection == nil || sender == nil {
		return nil, accessdomain.ErrNotificationUnavailable
	}
	repository, err := accesspersistence.NewVerificationRepository(database, protection)
	if err != nil {
		return nil, err
	}
	if autoMigrate {
		if err := repository.EnsureReliableSecurityNotificationSchema(context.Background()); err != nil {
			return nil, fmt.Errorf("migrate reliable security notification schema: %w", err)
		}
	}
	worker := notificationapp.SecurityDeliveryWorker{
		Repository: repository,
		Sender:     sender,
		Policy:     accessdomain.EnterpriseNotificationRetryPolicy(),
		WorkerID:   fmt.Sprintf("biz-idp-%d", os.Getpid()),
	}
	if err := worker.Validate(); err != nil {
		return nil, err
	}
	return &idpSecurityNotificationRunner{worker: worker}, nil
}

func (runner *idpSecurityNotificationRunner) Run(ctx context.Context) error {
	if runner == nil {
		return accessdomain.ErrNotificationUnavailable
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		result, err := runner.worker.RunOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		if result.EventID != "" {
			timer.Reset(time.Millisecond)
			continue
		}
		timer.Reset(100 * time.Millisecond)
	}
}
