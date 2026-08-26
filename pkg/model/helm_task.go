package model

import "time"

const (
	HelmTaskStatusPending   = "pending"
	HelmTaskStatusRunning   = "running"
	HelmTaskStatusSucceeded = "succeeded"
	HelmTaskStatusFailed    = "failed"

	HelmTaskTypeInstall   = "install"
	HelmTaskTypeUpgrade   = "upgrade"
	HelmTaskTypeRollback  = "rollback"
	HelmTaskTypeUninstall = "uninstall"
)

type HelmTask struct {
	Model
	ClusterName string     `json:"clusterName" gorm:"type:varchar(100);not null;index"`
	Namespace   string     `json:"namespace" gorm:"type:varchar(255);not null;index"`
	ReleaseName string     `json:"releaseName" gorm:"type:varchar(255);not null"`
	Type        string     `json:"type" gorm:"type:varchar(20);not null;index"`
	Status      string     `json:"status" gorm:"type:varchar(20);not null;default:pending;index"`
	CreatorID   uint       `json:"creatorId" gorm:"index"`
	Payload     string     `json:"payload,omitempty" gorm:"type:text"`
	Result      string     `json:"result,omitempty" gorm:"type:text"`
	Error       string     `json:"error,omitempty" gorm:"type:text"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}

func (HelmTask) TableName() string {
	return "helm_tasks"
}
