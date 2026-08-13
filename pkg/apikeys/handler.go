package apikeys

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/rbac"
)

type CreateAPIKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

func ListAPIKeys(c *gin.Context) {
	page := 1
	size := 20
	if p := strings.TrimSpace(c.Query("page")); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page parameter"})
			return
		}
	}
	if s := strings.TrimSpace(c.Query("size")); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed > 0 {
			size = parsed
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid size parameter"})
			return
		}
	}

	query := model.DB.Model(&model.User{}).Where("provider = ?", common.APIKeyProvider)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count API keys"})
		return
	}

	var apiKeys []model.User
	if err := query.Preload("Owner").Order("id").Offset((page - 1) * size).Limit(size).Find(&apiKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}
	for i := range apiKeys {
		apiKeys[i].Roles = rbac.GetUserRoles(apiKeys[i])
		apiKeys[i].APIKey = model.SecretString(apiKeys[i].GetAPIKey())
		// Don't expose the owner's API key or password
		if apiKeys[i].Owner != nil {
			apiKeys[i].Owner.APIKey = ""
			apiKeys[i].Owner.Password = ""
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  apiKeys,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// ListIndependentAPIKeys returns API keys without an owner (manually created).
// Used by the RBAC assignment dialog to list API keys that can have roles
// assigned directly.
func ListIndependentAPIKeys(c *gin.Context) {
	apiKeys, err := model.ListIndependentAPIKeyUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list API keys"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKeys": apiKeys})
}

func CreateAPIKey(c *gin.Context) {
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	apiKey, err := model.NewAPIKeyUser(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create API key: %v", err)})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"apiKey": apiKey})
}

func DeleteAPIKey(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid ID"})
		return
	}

	if err := model.DeleteUserByID(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete API key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "API key deleted successfully"})
}
