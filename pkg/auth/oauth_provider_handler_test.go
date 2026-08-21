package auth

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestOAuthProviderClaimStates(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	if err := db.AutoMigrate(&model.OAuthProvider{}); err != nil {
		t.Fatalf("migrating test database: %v", err)
	}
	model.DB = db
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
	})

	gin.SetMode(gin.TestMode)
	handler := NewAuthHandler()
	router := gin.New()
	router.POST("/oauth-providers", handler.CreateOAuthProvider)
	router.PUT("/oauth-providers/:id", handler.UpdateOAuthProvider)

	response := performAuthRequest(router, http.MethodPost, "/oauth-providers", `{"name":"custom","clientId":"client-id","clientSecret":"client-secret","usernameClaim":"preferred_username"}`, nil)
	if response.Code != http.StatusCreated {
		t.Fatalf("create provider status = %d, want %d", response.Code, http.StatusCreated)
	}
	var stored model.OAuthProvider
	if err := db.Where("name = ?", "custom").First(&stored).Error; err != nil {
		t.Fatalf("loading created provider: %v", err)
	}
	if stored.UsernameClaim != "preferred_username" {
		t.Fatalf("username claim = %q, want %q", stored.UsernameClaim, "preferred_username")
	}

	providerPath := "/oauth-providers/" + strconv.FormatUint(uint64(stored.ID), 10)
	response = performAuthRequest(router, http.MethodPut, providerPath, `{"name":"custom","clientId":"client-id","usernameClaim":""}`, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("empty claim update status = %d, want %d", response.Code, http.StatusOK)
	}
	if err := db.First(&stored, stored.ID).Error; err != nil {
		t.Fatalf("loading updated provider: %v", err)
	}
	if stored.UsernameClaim != "" {
		t.Fatalf("empty username claim = %q, want empty", stored.UsernameClaim)
	}
}
