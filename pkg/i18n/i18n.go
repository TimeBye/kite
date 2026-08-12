package i18n

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const LocaleKey = "locale"

var messages = map[string]map[string]string{
	"en": {
		// Email verification errors
		"email_service_not_configured":      "email service is not configured, please contact the administrator",
		"no_email_associated":               "no email address associated with your account",
		"failed_to_send_verification_email": "failed to send verification email",
		"email_verification_code_invalid":   "email verification code is required and must be valid",
		"email_can_only_set_for_password":   "email can only be set for password users",
		"failed_to_update_email":            "failed to update email",
		"current_password_incorrect":        "current password is incorrect",

		// Code manager errors
		"code_cooldown":          "please wait %d seconds before requesting a new code",
		"code_not_found":         "no verification code found, please request a new one",
		"code_expired":           "verification code has expired, please request a new one",
		"code_too_many_attempts": "too many failed attempts, please request a new code",
		"code_invalid":           "invalid verification code",

		// SMTP errors
		"smtp_not_configured":       "SMTP is not enabled or configured",
		"from_email_not_configured": "from email is not configured",
		"smtp_env_managed":          "SMTP is managed by environment variables and cannot be modified through the UI",
		"smtp_from_email_required":  "fromEmail is required when enabled is true",
		"failed_to_send_test_email": "Failed to send test email: %v",

		// Email content
		"verification_email_subject": "Kite Verification Code",
		"verification_email_body":    "Your verification code is: %s\n\nThis code will expire in 10 minutes.",
	},
	"zh": {
		// Email verification errors
		"email_service_not_configured":      "邮件服务未配置，请联系管理员",
		"no_email_associated":               "您的账户未关联邮箱地址",
		"failed_to_send_verification_email": "发送验证邮件失败",
		"email_verification_code_invalid":   "邮箱验证码无效或已过期",
		"email_can_only_set_for_password":   "仅密码用户可设置邮箱",
		"failed_to_update_email":            "更新邮箱失败",
		"current_password_incorrect":        "当前密码不正确",

		// Code manager errors
		"code_cooldown":          "请在 %d 秒后再请求新的验证码",
		"code_not_found":         "未找到验证码，请重新请求",
		"code_expired":           "验证码已过期，请重新请求",
		"code_too_many_attempts": "尝试次数过多，请重新请求验证码",
		"code_invalid":           "验证码无效",

		// SMTP errors
		"smtp_not_configured":       "SMTP 未启用或未配置",
		"from_email_not_configured": "发件人邮箱未配置",
		"smtp_env_managed":          "SMTP 由环境变量管理，无法通过界面修改",
		"smtp_from_email_required":  "启用时必须填写发件人邮箱",
		"failed_to_send_test_email": "发送测试邮件失败：%v",

		// Email content
		"verification_email_subject": "Kite 验证码",
		"verification_email_body":    "您的验证码是：%s\n\n该验证码将在 10 分钟后过期。",
	},
}

func T(lang, key string, args ...interface{}) string {
	msgs, ok := messages[lang]
	if !ok {
		msgs = messages["en"]
	}
	msg, ok := msgs[key]
	if !ok {
		msg = messages["en"][key]
	}
	if msg == "" {
		return key
	}
	if len(args) > 0 {
		return sprintf(msg, args...)
	}
	return msg
}

// sprintf is a simple fmt.Sprintf wrapper to avoid importing fmt in the map definition.
func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// Normalize extracts a 2-letter language code from the Accept-Language header value.
func Normalize(header string) string {
	if header == "" {
		return "en"
	}
	parts := strings.SplitN(header, ",", 2)
	lang := strings.TrimSpace(parts[0])
	if len(lang) >= 2 {
		lang = lang[:2]
	}
	switch lang {
	case "zh":
		return "zh"
	default:
		return "en"
	}
}

// Middleware extracts the Accept-Language header and stores the locale in the gin context.
func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := Normalize(c.GetHeader("Accept-Language"))
		c.Set(LocaleKey, lang)
		c.Next()
	}
}

// FromContext returns the locale stored in the gin context, defaulting to "en".
func FromContext(c *gin.Context) string {
	lang, ok := c.Get(LocaleKey)
	if !ok {
		return "en"
	}
	return lang.(string)
}

// FromRequest returns the locale from an HTTP request, defaulting to "en".
func FromRequest(r *http.Request) string {
	return Normalize(r.Header.Get("Accept-Language"))
}
