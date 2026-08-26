package helm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/model"
	repo "helm.sh/helm/v4/pkg/repo/v1"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupHelmTestDB(t *testing.T) {
	t.Helper()
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := db.AutoMigrate(&model.HelmRepository{}); err != nil {
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

func TestListChartsGracefulDegradation(t *testing.T) {
	setupHelmTestDB(t)

	goodRepo := model.HelmRepository{
		Name: "good-repo",
		URL:  "https://good.example.com/charts",
	}
	badRepo := model.HelmRepository{
		Name: "bad-repo",
		URL:  "https://invalid.example.com/charts",
	}
	if err := model.DB.Create(&goodRepo).Error; err != nil {
		t.Fatalf("creating good repository: %v", err)
	}
	if err := model.DB.Create(&badRepo).Error; err != nil {
		t.Fatalf("creating bad repository: %v", err)
	}

	h := NewHelmChartHandler()

	cachedIndex := repo.NewIndexFile()
	cachedIndex.Entries["working-chart"] = []*repo.ChartVersion{
		newChartVersion("1.0.0"),
	}
	h.indexCache[repositoryIndexCacheKey(goodRepo)] = cachedRepositoryIndex{
		indexFile: cachedIndex,
		expiresAt: time.Now().Add(1 * time.Minute),
	}

	router := gin.New()
	router.GET("/charts", h.ListCharts)

	req := httptest.NewRequest(http.MethodGet, "/charts", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Items []helmChart `json:"items"`
		Total int         `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}

	if resp.Total != 1 {
		t.Fatalf("total = %d, want 1", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(resp.Items))
	}
	if resp.Items[0].Name != "myapp" {
		t.Fatalf("chart name = %q, want %q", resp.Items[0].Name, "myapp")
	}
	if resp.Items[0].RepositoryName != "good-repo" {
		t.Fatalf("repository name = %q, want %q", resp.Items[0].RepositoryName, "good-repo")
	}
}
