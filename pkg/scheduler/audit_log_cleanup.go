package scheduler

import (
	"context"
	"time"

	"github.com/zxh326/kite/pkg/model"
	"k8s.io/klog/v2"
)

const (
	auditLogCleanupInterval = time.Hour
)

func StartAuditLogCleanup(ctx context.Context) {
	go runAuditLogCleanupLoop(ctx)
}

func runAuditLogCleanupLoop(ctx context.Context) {
	runAuditLogCleanup()
	ticker := time.NewTicker(auditLogCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runAuditLogCleanup()
		}
	}
}

func runAuditLogCleanup() {
	setting, err := model.GetGeneralSetting()
	if err != nil {
		klog.Errorf("Audit log cleanup: failed to load general setting: %v", err)
		return
	}
	days := setting.AuditLogRetentionDays
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	result := model.DB.Where("created_at < ?", cutoff).Delete(&model.ResourceHistory{})
	if result.Error != nil {
		klog.Errorf("Audit log cleanup: failed to delete expired records: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		klog.Infof("Audit log cleanup: deleted %d records older than %d days", result.RowsAffected, days)
	}
}
