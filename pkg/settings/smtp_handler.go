package settings

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/email"
	"github.com/zxh326/kite/pkg/i18n"
	"github.com/zxh326/kite/pkg/model"
)

func HandleGetSMTPSetting(c *gin.Context) {
	// If SMTP is managed via env vars, return those values
	if common.SMTPEnvEnabled {
		c.JSON(http.StatusOK, gin.H{
			"enabled":            true,
			"host":               common.SMTPHost,
			"port":               common.SMTPPort,
			"username":           common.SMTPUsername,
			"password":           "",
			"passwordConfigured": common.SMTPPassword != "",
			"fromEmail":          common.SMTPFromEmail,
			"useTLS":             common.SMTPUseTLS,
			"envManaged":         true,
		})
		return
	}

	setting, err := model.GetSMTPSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load SMTP setting: %v", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":            setting.Enabled,
		"host":               setting.Host,
		"port":               setting.Port,
		"username":           setting.Username,
		"password":           "",
		"passwordConfigured": setting.PasswordConfigured(),
		"fromEmail":          setting.FromEmail,
		"useTLS":             setting.UseTLS,
		"envManaged":         false,
	})
}

type UpdateSMTPSettingRequest struct {
	Enabled   *bool   `json:"enabled"`
	Host      *string `json:"host"`
	Port      *int    `json:"port"`
	Username  *string `json:"username"`
	Password  *string `json:"password"`
	FromEmail *string `json:"fromEmail"`
	UseTLS    *bool   `json:"useTLS"`
}

func HandleUpdateSMTPSetting(c *gin.Context) {
	if common.IsSectionManaged("smtp") {
		c.JSON(http.StatusForbidden, gin.H{"error": common.ManagedSectionError})
		return
	}
	if common.SMTPEnvEnabled {
		lang := i18n.FromContext(c)
		c.JSON(http.StatusForbidden, gin.H{"error": i18n.T(lang, "smtp_env_managed")})
		return
	}

	var req UpdateSMTPSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	current, err := model.GetSMTPSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to load SMTP setting: %v", err)})
		return
	}

	setting := *current
	if req.Enabled != nil {
		setting.Enabled = *req.Enabled
	}
	if req.Host != nil {
		setting.Host = *req.Host
	}
	if req.Port != nil {
		setting.Port = *req.Port
	}
	if req.Username != nil {
		setting.Username = *req.Username
	}
	if req.Password != nil && *req.Password != "" {
		setting.Password = model.SecretString(*req.Password)
	}
	if req.FromEmail != nil {
		setting.FromEmail = *req.FromEmail
	}
	if req.UseTLS != nil {
		setting.UseTLS = *req.UseTLS
	}

	if err := setting.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := model.UpdateSMTPSetting(&setting)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update SMTP setting: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"enabled":            updated.Enabled,
		"host":               updated.Host,
		"port":               updated.Port,
		"username":           updated.Username,
		"password":           "",
		"passwordConfigured": updated.PasswordConfigured(),
		"fromEmail":          updated.FromEmail,
		"useTLS":             updated.UseTLS,
		"envManaged":         false,
	})
}

// HandleSendTestEmail sends a test email to verify SMTP configuration.
func HandleSendTestEmail(c *gin.Context) {
	lang := i18n.FromContext(c)
	if !email.IsSMTPEnabled() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": i18n.T(lang, "smtp_not_configured")})
		return
	}

	var req struct {
		To string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate and send a test code
	code, err := email.GetCodeManager().GenerateAndStore(req.To, lang)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	if err := email.SendVerificationCode(req.To, code, lang); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf(i18n.T(lang, "failed_to_send_test_email"), err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
