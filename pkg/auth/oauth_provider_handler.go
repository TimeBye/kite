package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
)

func (h *AuthHandler) ListOAuthProviders(c *gin.Context) {
	page := 1
	size := 20
	if p := c.Query("page"); p != "" {
		_, _ = fmt.Sscanf(p, "%d", &page)
		if page <= 0 {
			page = 1
		}
	}
	if s := c.Query("size"); s != "" {
		_, _ = fmt.Sscanf(s, "%d", &size)
		if size <= 0 {
			size = 20
		}
		if size > 200 {
			size = 200
		}
	}

	providers, total, err := model.GetOAuthProvidersWithPagination(page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve OAuth providers",
		})
		return
	}

	for i := range providers {
		providers[i].ClientSecret = "***"
	}

	c.JSON(http.StatusOK, gin.H{
		"data": providers, "total": total, "page": page, "size": size,
	})
}

func (h *AuthHandler) CreateOAuthProvider(c *gin.Context) {
	if common.IsSectionManaged("oauth") {
		c.JSON(http.StatusForbidden, gin.H{"error": common.ManagedSectionError})
		return
	}

	var request updateOAuthProviderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request payload: " + err.Error(),
		})
		return
	}
	provider := request.OAuthProvider
	provider.Name = model.LowerCaseString(model.NormalizeOAuthProviderName(string(provider.Name)))

	if provider.Name == "" || provider.ClientID == "" || string(provider.ClientSecret) == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name, ClientID, and ClientSecret are required",
		})
		return
	}
	if model.IsReservedOAuthProviderName(string(provider.Name)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": model.ErrReservedOAuthProviderName.Error(),
		})
		return
	}

	if err := model.CreateOAuthProvider(&provider); err != nil {
		if errors.Is(err, model.ErrReservedOAuthProviderName) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create OAuth provider: " + err.Error(),
		})
		return
	}

	provider.ClientSecret = "***"
	c.JSON(http.StatusCreated, gin.H{
		"provider": provider,
	})
}

type updateOAuthProviderRequest struct {
	model.OAuthProvider
	UsernameClaimPresent  bool
	NameClaimPresent      bool
	EmailClaimPresent     bool
	AvatarURLClaimPresent bool
	GroupsClaimPresent    bool
}

func (r *updateOAuthProviderRequest) UnmarshalJSON(data []byte) error {
	type alias model.OAuthProvider
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(data, (*alias)(&r.OAuthProvider)); err != nil {
		return err
	}
	_, r.UsernameClaimPresent = raw["usernameClaim"]
	_, r.NameClaimPresent = raw["nameClaim"]
	_, r.EmailClaimPresent = raw["emailClaim"]
	_, r.AvatarURLClaimPresent = raw["avatarUrlClaim"]
	_, r.GroupsClaimPresent = raw["groupsClaim"]
	return nil
}

func (h *AuthHandler) UpdateOAuthProvider(c *gin.Context) {
	if common.IsSectionManaged("oauth") {
		c.JSON(http.StatusForbidden, gin.H{"error": common.ManagedSectionError})
		return
	}

	id := c.Param("id")
	var request updateOAuthProviderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request payload: " + err.Error(),
		})
		return
	}
	provider := request.OAuthProvider

	dbID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid provider ID",
		})
		return
	}
	provider.ID = uint(dbID)
	provider.Name = model.LowerCaseString(model.NormalizeOAuthProviderName(string(provider.Name)))

	if provider.Name == "" || provider.ClientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Name and ClientID are required",
		})
		return
	}
	if model.IsReservedOAuthProviderName(string(provider.Name)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": model.ErrReservedOAuthProviderName.Error(),
		})
		return
	}

	updates := map[string]interface{}{
		"name":           provider.Name,
		"client_id":      provider.ClientID,
		"auth_url":       provider.AuthURL,
		"token_url":      provider.TokenURL,
		"user_info_url":  provider.UserInfoURL,
		"scopes":         provider.Scopes,
		"issuer":         provider.Issuer,
		"allowed_groups": provider.AllowedGroups,
		"enabled":        provider.Enabled,
	}
	if provider.ClientSecret != "" {
		updates["client_secret"] = provider.ClientSecret
	}
	if request.UsernameClaimPresent {
		updates["username_claim"] = provider.UsernameClaim
	}
	if request.NameClaimPresent {
		updates["name_claim"] = provider.NameClaim
	}
	if request.EmailClaimPresent {
		updates["email_claim"] = provider.EmailClaim
	}
	if request.AvatarURLClaimPresent {
		updates["avatar_url_claim"] = provider.AvatarURLClaim
	}
	if request.GroupsClaimPresent {
		updates["groups_claim"] = provider.GroupsClaim
	}

	if err := model.UpdateOAuthProvider(&provider, updates); err != nil {
		if errors.Is(err, model.ErrReservedOAuthProviderName) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update OAuth provider: " + err.Error(),
		})
		return
	}
	provider.ClientSecret = "***"
	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
	})
}

func (h *AuthHandler) DeleteOAuthProvider(c *gin.Context) {
	if common.IsSectionManaged("oauth") {
		c.JSON(http.StatusForbidden, gin.H{"error": common.ManagedSectionError})
		return
	}

	id := c.Param("id")
	dbID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid provider ID",
		})
		return
	}

	if err := model.DeleteOAuthProvider(uint(dbID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete OAuth provider: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OAuth provider deleted successfully",
	})
}

func (h *AuthHandler) GetOAuthProvider(c *gin.Context) {
	id := c.Param("id")
	dbID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid provider ID",
		})
		return
	}

	var provider model.OAuthProvider
	if err := model.DB.First(&provider, uint(dbID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "OAuth provider not found",
		})
		return
	}

	provider.ClientSecret = "***"
	c.JSON(http.StatusOK, gin.H{
		"provider": provider,
	})
}
