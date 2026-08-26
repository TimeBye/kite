package helmutil

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/zxh326/kite/pkg/model"
	"k8s.io/klog/v2"
)

// HelmTaskManager runs Helm operations in background goroutines and tracks
// their status in the helm_tasks table. It is intentionally lightweight — no
// persistent worker loop or scheduler scan is needed because each task is
// launched directly by the HTTP handler.
type HelmTaskManager struct{}

// HelmTaskFunc is the function executed inside a task goroutine. It receives
// a context (cancelled when the server shuts down) and the task record.
type HelmTaskFunc func(ctx context.Context, task model.HelmTask) (string, error)

var (
	taskManager     *HelmTaskManager
	taskManagerOnce sync.Once
)

// InitHelmTaskManager initialises the global task manager. It is called once
// during application startup.
func InitHelmTaskManager() {
	taskManagerOnce.Do(func() {
		taskManager = &HelmTaskManager{}
	})
}

// GetHelmTaskManager returns the global task manager. It panics if
// InitHelmTaskManager has not been called.
func GetHelmTaskManager() *HelmTaskManager {
	if taskManager == nil {
		panic("helm task manager not initialised")
	}
	return taskManager
}

// CreateTask persists a new HelmTask in pending status and returns it.
func (m *HelmTaskManager) CreateTask(clusterName, namespace, releaseName, taskType string, creatorID uint, payload interface{}) (*model.HelmTask, error) {
	payloadData := ""
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		payloadData = string(data)
	}
	task := model.HelmTask{
		ClusterName: clusterName,
		Namespace:   namespace,
		ReleaseName: releaseName,
		Type:        taskType,
		Status:      model.HelmTaskStatusPending,
		CreatorID:   creatorID,
		Payload:     payloadData,
	}
	if err := model.DB.Create(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// Start launches a goroutine that executes fn and updates the task status.
// The goroutine runs with a timeout derived from timeoutDuration. If
// timeoutDuration is 0, helmActionDefaultTimeout is used.
func (m *HelmTaskManager) Start(taskID uint, timeoutDuration time.Duration, fn HelmTaskFunc) {
	if timeoutDuration <= 0 {
		timeoutDuration = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	go m.run(ctx, taskID, cancel, fn)
}

func (m *HelmTaskManager) run(ctx context.Context, taskID uint, cancel context.CancelFunc, fn HelmTaskFunc) {
	defer cancel()
	now := time.Now()
	model.DB.Model(&model.HelmTask{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"status":     model.HelmTaskStatusRunning,
		"started_at": now,
	})

	var task model.HelmTask
	if err := model.DB.First(&task, taskID).Error; err != nil {
		klog.Errorf("Failed to load helm task %d: %v", taskID, err)
		return
	}

	result, err := fn(ctx, task)
	finishedAt := time.Now()
	updates := map[string]interface{}{
		"finished_at": finishedAt,
	}
	if err != nil {
		updates["status"] = model.HelmTaskStatusFailed
		updates["error"] = err.Error()
		klog.Errorf("Helm task %d failed: %v", taskID, err)
	} else {
		updates["status"] = model.HelmTaskStatusSucceeded
		if result != "" {
			updates["result"] = result
		}
		klog.Infof("Helm task %d succeeded", taskID)
	}
	model.DB.Model(&model.HelmTask{}).Where("id = ?", taskID).Updates(updates)
}

// GetTask retrieves a task by ID.
func (m *HelmTaskManager) GetTask(taskID uint) (*model.HelmTask, error) {
	var task model.HelmTask
	if err := model.DB.First(&task, taskID).Error; err != nil {
		return nil, err
	}
	return &task, nil
}
