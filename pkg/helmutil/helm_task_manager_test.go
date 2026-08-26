package helmutil

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.HelmTask{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
	})
}

func TestCreateTask(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	payload := map[string]interface{}{"chartUrl": "https://example.com/chart.tgz"}
	task, err := mgr.CreateTask("test-cluster", "default", "myapp", "upgrade", 1, payload)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.ID == 0 {
		t.Fatal("expected non-zero task ID")
	}
	if task.Status != model.HelmTaskStatusPending {
		t.Fatalf("status = %q, want %q", task.Status, model.HelmTaskStatusPending)
	}
	if task.ClusterName != "test-cluster" {
		t.Fatalf("clusterName = %q, want %q", task.ClusterName, "test-cluster")
	}
	if task.Namespace != "default" {
		t.Fatalf("namespace = %q, want %q", task.Namespace, "default")
	}
	if task.ReleaseName != "myapp" {
		t.Fatalf("releaseName = %q, want %q", task.ReleaseName, "myapp")
	}
	if task.Type != "upgrade" {
		t.Fatalf("type = %q, want %q", task.Type, "upgrade")
	}
	if task.Payload == "" {
		t.Fatal("expected non-empty payload")
	}
}

func TestCreateTaskNilPayload(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	task, err := mgr.CreateTask("test-cluster", "default", "myapp", "install", 1, nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}
	if task.Payload != "" {
		t.Fatalf("expected empty payload, got %q", task.Payload)
	}
}

func TestStartTaskSucceeded(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	task, err := mgr.CreateTask("test-cluster", "default", "myapp", "upgrade", 1, nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	mgr.Start(task.ID, 5*time.Second, func(ctx context.Context, t model.HelmTask) (string, error) {
		defer wg.Done()
		return "upgrade succeeded", nil
	})
	wg.Wait()

	// Give the manager a moment to finish updating the task
	time.Sleep(100 * time.Millisecond)

	got, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.HelmTaskStatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, model.HelmTaskStatusSucceeded)
	}
	if got.Result != "upgrade succeeded" {
		t.Fatalf("result = %q, want %q", got.Result, "upgrade succeeded")
	}
	if got.StartedAt == nil {
		t.Fatal("expected non-nil StartedAt")
	}
	if got.FinishedAt == nil {
		t.Fatal("expected non-nil FinishedAt")
	}
}

func TestStartTaskFailed(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	task, err := mgr.CreateTask("test-cluster", "default", "myapp", "upgrade", 1, nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	expectedErr := errors.New("upgrade failed")
	var wg sync.WaitGroup
	wg.Add(1)
	mgr.Start(task.ID, 5*time.Second, func(ctx context.Context, t model.HelmTask) (string, error) {
		defer wg.Done()
		return "", expectedErr
	})
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	got, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.HelmTaskStatusFailed {
		t.Fatalf("status = %q, want %q", got.Status, model.HelmTaskStatusFailed)
	}
	if got.Error != expectedErr.Error() {
		t.Fatalf("error = %q, want %q", got.Error, expectedErr.Error())
	}
	if got.StartedAt == nil {
		t.Fatal("expected non-nil StartedAt")
	}
	if got.FinishedAt == nil {
		t.Fatal("expected non-nil FinishedAt")
	}
}

func TestGetTaskNotFound(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	_, err := mgr.GetTask(999999)
	if err == nil {
		t.Fatal("expected error for non-existent task, got nil")
	}
}

func TestStartTaskTransitionsToRunning(t *testing.T) {
	setupTestDB(t)
	mgr := &HelmTaskManager{}

	task, err := mgr.CreateTask("test-cluster", "default", "myapp", "upgrade", 1, nil)
	if err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	started := make(chan struct{})
	done := make(chan struct{})
	mgr.Start(task.ID, 5*time.Second, func(ctx context.Context, t model.HelmTask) (string, error) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		close(done)
		return "ok", nil
	})

	<-started
	got, err := mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.HelmTaskStatusRunning {
		t.Fatalf("status = %q, want %q", got.Status, model.HelmTaskStatusRunning)
	}
	if got.StartedAt == nil {
		t.Fatal("expected non-nil StartedAt while running")
	}

	<-done
	time.Sleep(100 * time.Millisecond)

	got, err = mgr.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Status != model.HelmTaskStatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, model.HelmTaskStatusSucceeded)
	}
}
