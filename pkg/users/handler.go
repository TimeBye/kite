package users

import (
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/mfa"
	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/passkey"
	"github.com/zxh326/kite/pkg/rbac"
	"github.com/zxh326/kite/pkg/settings"
	"k8s.io/klog/v2"
)

type createPasswordUser struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name"`
}

func CreatePasswordUser(c *gin.Context) {
	var userreq createPasswordUser
	if err := c.ShouldBindJSON(&userreq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// check only admin or users count is zero
	user := &model.User{
		Username: userreq.Username,
		Password: userreq.Password,
		Name:     userreq.Name,
		Provider: "password",
	}

	_, err := model.GetUserByUsername(user.Username)
	if err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user already exists"})
		return
	}

	if err := model.AddUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

func ListUsers(c *gin.Context) {
	page := 1
	size := 20
	search := strings.TrimSpace(c.Query("search"))
	role := strings.TrimSpace(c.Query("role"))
	sortBy := strings.TrimSpace(c.Query("sortBy"))
	sortOrder := strings.ToLower(strings.TrimSpace(c.Query("sortOrder")))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
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
	offset := (page - 1) * size

	users, total, err := model.ListUsers(
		size,
		offset,
		search,
		sortBy,
		sortOrder,
		role,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users"})
		return
	}
	for i := range users {
		users[i].Roles = rbac.GetUserRoles(users[i])
	}
	c.JSON(http.StatusOK, gin.H{"users": users, "total": total, "page": page, "size": size})
}

func UpdateUser(c *gin.Context) {
	var id uint64
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := model.GetUserByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if req.Name != "" {
		user.Name = req.Name
	}
	if req.AvatarURL != "" {
		user.AvatarURL = req.AvatarURL
	}

	if err := model.UpdateUser(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func UpdateCurrentUser(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	var req struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
		EmailOTP  string `json:"email_otp"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	avatarURL := strings.TrimSpace(req.AvatarURL)
	if (user.NameSource != "" && name != user.Name) || (user.EmailSource != "" && email != user.Email) || (user.AvatarURLSource != "" && avatarURL != user.AvatarURL) {
		c.JSON(http.StatusForbidden, gin.H{"error": "profile field is managed by the authentication provider"})
		return
	}
	if email != user.Email {
		verified, err := model.ConsumeEmailOTP(user.ID, email, model.EmailOTPEmailChange, strings.TrimSpace(req.EmailOTP), time.Now())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify email verification code"})
			return
		}
		if !verified {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired email verification code"})
			return
		}
		if err := model.UpdateUserEmail(user.ID, email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update email"})
			return
		}
		user.Email = email
	}
	if err := model.UpdateUserProfile(user.ID, name, user.Email, avatarURL); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user"})
		return
	}
	user.Name = name
	user.AvatarURL = avatarURL
	c.JSON(http.StatusOK, user)
}

func RequestCurrentUserEmailUpdate(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.EmailSource != "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "email is managed by the authentication provider"})
		return
	}
	var req struct {
		Email           string `json:"email" binding:"required"`
		CurrentPassword string `json:"current_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	email := strings.TrimSpace(req.Email)
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email must be a valid email address"})
		return
	}
	if user.Provider == "" || user.Provider == model.AuthProviderPassword {
		if !model.CheckPassword(user.Password, req.CurrentPassword) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
			return
		}
	}
	code, err := model.CreateEmailOTP(user.ID, email, model.EmailOTPEmailChange, time.Now().Add(10*time.Minute))
	if errors.Is(err, model.ErrEmailOTPTooFrequent) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "please wait before requesting another email verification code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create email verification code"})
		return
	}
	if err := settings.SendEmailOTP(c.Request.Context(), email, code); err != nil {
		_ = model.DeleteEmailOTP(user.ID, email, model.EmailOTPEmailChange)
		if errors.Is(err, settings.ErrSMTPNotEnabled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP is not configured. Please contact the administrator."})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to send email verification code"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ChangeCurrentUserPassword(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if user.Provider != "" && user.Provider != model.AuthProviderPassword {
		c.JSON(http.StatusForbidden, gin.H{"error": "password can only be changed for password users"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password" binding:"required"`
		NewPassword     string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !model.CheckPassword(user.Password, req.CurrentPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "current password is incorrect"})
		return
	}
	if err := model.ResetPasswordByID(user.ID, req.NewPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to change password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func RequestCurrentUserSecurityOTP(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if strings.TrimSpace(user.Email) == "" || !user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a verified email is required for security changes"})
		return
	}
	purpose := strings.TrimSpace(c.Query("purpose"))
	if !isSecurityOTPPurpose(purpose) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email verification purpose"})
		return
	}
	code, err := model.CreateEmailOTP(user.ID, user.Email, purpose, time.Now().Add(10*time.Minute))
	if errors.Is(err, model.ErrEmailOTPTooFrequent) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "please wait before requesting another email verification code"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create email verification code"})
		return
	}
	if err := settings.SendEmailOTP(c.Request.Context(), user.Email, code); err != nil {
		_ = model.DeleteEmailOTP(user.ID, user.Email, purpose)
		if errors.Is(err, settings.ErrSMTPNotEnabled) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "SMTP is not configured. Please contact the administrator."})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to send email verification code"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func consumeCurrentUserSecurityOTP(c *gin.Context, user model.User, purpose, code string) bool {
	if strings.TrimSpace(user.Email) == "" || !user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a verified email is required for security changes"})
		return false
	}
	verified, err := model.ConsumeEmailOTP(user.ID, user.Email, purpose, strings.TrimSpace(code), time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify email verification code"})
		return false
	}
	if !verified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired email verification code"})
		return false
	}
	return true
}

func isSecurityOTPPurpose(purpose string) bool {
	switch purpose {
	case model.EmailOTPSetupMFA, model.EmailOTPEnableMFA, model.EmailOTPDisableMFA, model.EmailOTPAddPasskey, model.EmailOTPDeletePasskey:
		return true
	default:
		return false
	}
}

func SetupCurrentUserMFA(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensureMFAEnabled(c) {
		return
	}
	var req struct {
		EmailOTP string `json:"email_otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if user.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mfa is already enabled"})
		return
	}
	if !consumeCurrentUserSecurityOTP(c, user, model.EmailOTPSetupMFA, req.EmailOTP) {
		return
	}

	secret, err := mfa.GenerateSecret()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate mfa secret"})
		return
	}
	if err := model.StoreMFASecret(user.ID, secret); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save mfa secret"})
		return
	}

	otpURL := mfa.URL(user.Username, secret)
	qrCode, err := mfa.QRCodeDataURL(otpURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate mfa qr code"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      secret,
		"otpauth_url": otpURL,
		"qr_code":     qrCode,
	})
}

func EnableCurrentUserMFA(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensureMFAEnabled(c) {
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		EmailOTP string `json:"email_otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(string(user.MFASecret)) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mfa setup is not started"})
		return
	}
	if !consumeCurrentUserSecurityOTP(c, user, model.EmailOTPEnableMFA, req.EmailOTP) {
		return
	}
	if !mfa.Verify(string(user.MFASecret), req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mfa code"})
		return
	}

	if err := model.EnableUserMFA(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to enable mfa"})
		return
	}
	user.MFAEnabled = true
	c.JSON(http.StatusOK, user)
}

func DisableCurrentUserMFA(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensureMFAEnabled(c) {
		return
	}

	var req struct {
		Code     string `json:"code" binding:"required"`
		EmailOTP string `json:"email_otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !user.MFAEnabled {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mfa is not enabled"})
		return
	}
	if !consumeCurrentUserSecurityOTP(c, user, model.EmailOTPDisableMFA, req.EmailOTP) {
		return
	}
	if !mfa.Verify(string(user.MFASecret), req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mfa code"})
		return
	}

	if err := model.DisableUserMFA(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to disable mfa"})
		return
	}
	user.MFAEnabled = false
	user.MFASecret = ""
	c.JSON(http.StatusOK, user)
}

func ListCurrentUserPasskeys(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensurePasskeyLoginEnabled(c) {
		return
	}
	credentials, err := passkey.CredentialsForUser(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list passkeys"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"passkeys": credentials})
}

func BeginCurrentUserPasskeyRegistration(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensurePasskeyLoginEnabled(c) {
		return
	}

	var req struct {
		Name     string `json:"name"`
		EmailOTP string `json:"email_otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !consumeCurrentUserSecurityOTP(c, user, model.EmailOTPAddPasskey, req.EmailOTP) {
		return
	}

	creation, err := passkey.BeginRegistration(c, user, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start passkey registration"})
		return
	}
	c.JSON(http.StatusOK, creation)
}

func FinishCurrentUserPasskeyRegistration(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensurePasskeyLoginEnabled(c) {
		return
	}

	credential, err := passkey.FinishRegistration(c, user)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to finish passkey registration"})
		return
	}
	c.JSON(http.StatusOK, credential)
}

func DeleteCurrentUserPasskey(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	if !ensurePasskeyLoginEnabled(c) {
		return
	}

	var req struct {
		EmailOTP string `json:"email_otp" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if !consumeCurrentUserSecurityOTP(c, user, model.EmailOTPDeletePasskey, req.EmailOTP) {
		return
	}

	if err := passkey.DeleteCredential(user.ID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete passkey"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ensurePasskeyLoginEnabled(c *gin.Context) bool {
	enabled, err := passkey.Enabled()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load general setting"})
		return false
	}
	if !enabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "passkey login is disabled"})
		return false
	}
	return true
}

func ensureMFAEnabled(c *gin.Context) bool {
	setting, err := model.GetGeneralSetting()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load general setting"})
		return false
	}
	if !setting.EnableMFA {
		c.JSON(http.StatusForbidden, gin.H{"error": "mfa is disabled"})
		return false
	}
	return true
}

func DeleteUser(c *gin.Context) {
	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := model.DeleteUserByID(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ResetPassword(c *gin.Context) {
	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := model.ResetPasswordByID(id, req.Password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset password"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func SetUserEnabled(c *gin.Context) {
	var id uint
	if _, err := fmt.Sscanf(c.Param("id"), "%d", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := model.SetUserEnabled(id, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set enabled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateSidebarPreference(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	isAdmin := rbac.UserHasRole(user, model.DefaultAdminRole.Name)
	if !isAdmin {
		setting, err := model.GetGeneralSetting()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load general setting"})
			return
		}
		if strings.TrimSpace(setting.GlobalSidebarPreference) != "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "sidebar customization is disabled by global sidebar"})
			return
		}
	}
	var req struct {
		SidebarPreference string `json:"sidebar_preference" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user.SidebarPreference = req.SidebarPreference
	if err := model.UpdateUser(&user); err != nil {
		klog.Errorf("failed to update sidebar preference for user %s: %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sidebar preference"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UpdateGlobalSidebarPreference(c *gin.Context) {
	var req struct {
		SidebarPreference string `json:"sidebar_preference" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if _, err := model.UpdateGeneralSetting(map[string]interface{}{
		"global_sidebar_preference": req.SidebarPreference,
	}); err != nil {
		klog.Errorf("failed to update global sidebar preference: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update global sidebar preference"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func ClearGlobalSidebarPreference(c *gin.Context) {
	if _, err := model.UpdateGeneralSetting(map[string]interface{}{
		"global_sidebar_preference": "",
	}); err != nil {
		klog.Errorf("failed to clear global sidebar preference: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to clear global sidebar preference"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
