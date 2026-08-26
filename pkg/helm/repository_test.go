package helm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/scheduler"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupHelmRepositoryTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := db.AutoMigrate(&model.HelmRepository{}, &model.ScheduledTask{}); err != nil {
		t.Fatalf("migrating test database: %v", err)
	}
	model.DB = db
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
	})
}

func TestDeleteRepositoryDisablesAutoUpgradeTasks(t *testing.T) {
	setupHelmRepositoryTestDB(t)

	repository := model.HelmRepository{
		Name: "test-repo",
		URL:  "https://invalid.example.com/charts",
	}
	if err := model.DB.Create(&repository).Error; err != nil {
		t.Fatalf("creating repository: %v", err)
	}

	matchingPayload, err := json.Marshal(scheduler.HelmReleaseAutoUpgradePayload{
		Namespace:         "default",
		ResourceType:      "helmreleases",
		ResourceName:      "my-release",
		Source:            "repository",
		RepositoryName:    "test-repo",
		ChartName:         "myapp",
		TimeoutMinutes:    5,
		RollbackOnFailure: true,
	})
	if err != nil {
		t.Fatalf("marshalling matching payload: %v", err)
	}

	otherPayload, err := json.Marshal(scheduler.HelmReleaseAutoUpgradePayload{
		Namespace:         "default",
		ResourceType:      "helmreleases",
		ResourceName:      "other-release",
		Source:            "repository",
		RepositoryName:    "other-repo",
		ChartName:         "otherapp",
		TimeoutMinutes:    5,
		RollbackOnFailure: true,
	})
	if err != nil {
		t.Fatalf("marshalling other payload: %v", err)
	}

	matchingTask := model.ScheduledTask{
		ClusterName:     "cluster-a",
		Type:            scheduler.HelmReleaseAutoUpgradeTaskType,
		Key:             "default/my-release",
		Name:            "Helm release auto upgrade default/my-release",
		CreatorID:       1,
		Enabled:         true,
		ScheduleType:    "interval",
		IntervalMinutes: 60,
		ScheduleTime:    "03:00",
		Payload:         string(matchingPayload),
	}
	otherTask := model.ScheduledTask{
		ClusterName:     "cluster-a",
		Type:            scheduler.HelmReleaseAutoUpgradeTaskType,
		Key:             "default/other-release",
		Name:            "Helm release auto upgrade default/other-release",
		CreatorID:       1,
		Enabled:         true,
		ScheduleType:    "interval",
		IntervalMinutes: 60,
		ScheduleTime:    "03:00",
		Payload:         string(otherPayload),
	}
	if err := model.DB.Create(&matchingTask).Error; err != nil {
		t.Fatalf("creating matching task: %v", err)
	}
	if err := model.DB.Create(&otherTask).Error; err != nil {
		t.Fatalf("creating other task: %v", err)
	}

	h := NewHelmChartHandler()
	router := gin.New()
	router.DELETE("/repositories/:id", h.DeleteRepository)

	req := httptest.NewRequest(http.MethodDelete, "/repositories/"+strconv.FormatUint(uint64(repository.ID), 10), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var updatedMatching model.ScheduledTask
	if err := model.DB.First(&updatedMatching, matchingTask.ID).Error; err != nil {
		t.Fatalf("querying matching task: %v", err)
	}
	if updatedMatching.Enabled {
		t.Fatal("matching task should be disabled after repository deletion")
	}

	var updatedOther model.ScheduledTask
	if err := model.DB.First(&updatedOther, otherTask.ID).Error; err != nil {
		t.Fatalf("querying other task: %v", err)
	}
	if !updatedOther.Enabled {
		t.Fatal("non-matching task should remain enabled")
	}
}
