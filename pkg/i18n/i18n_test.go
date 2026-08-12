package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTReturnsEnglish(t *testing.T) {
	msg := T("en", "email_service_not_configured")
	if msg != "email service is not configured, please contact the administrator" {
		t.Fatalf("expected English message, got: %s", msg)
	}
}

func TestTReturnsChinese(t *testing.T) {
	msg := T("zh", "email_service_not_configured")
	if msg != "邮件服务未配置，请联系管理员" {
		t.Fatalf("expected Chinese message, got: %s", msg)
	}
}

func TestTFallbackToEnglish(t *testing.T) {
	msg := T("fr", "email_service_not_configured")
	if msg != "email service is not configured, please contact the administrator" {
		t.Fatalf("expected English fallback, got: %s", msg)
	}
}

func TestTFallbackToKey(t *testing.T) {
	msg := T("en", "nonexistent_key")
	if msg != "nonexistent_key" {
		t.Fatalf("expected key as fallback, got: %s", msg)
	}
}

func TestTWithFormatArgs(t *testing.T) {
	msg := T("en", "code_cooldown", 30)
	if msg != "please wait 30 seconds before requesting a new code" {
		t.Fatalf("expected formatted message, got: %s", msg)
	}
}

func TestTWithFormatArgsChinese(t *testing.T) {
	msg := T("zh", "code_cooldown", 30)
	if msg != "请在 30 秒后再请求新的验证码" {
		t.Fatalf("expected formatted Chinese message, got: %s", msg)
	}
}

func TestTVerificationEmailSubject(t *testing.T) {
	en := T("en", "verification_email_subject")
	zh := T("zh", "verification_email_subject")
	if en != "Kite Verification Code" {
		t.Fatalf("unexpected English subject: %s", en)
	}
	if zh != "Kite 验证码" {
		t.Fatalf("unexpected Chinese subject: %s", zh)
	}
}

func TestTVerificationEmailBody(t *testing.T) {
	en := T("en", "verification_email_body", "123456")
	zh := T("zh", "verification_email_body", "123456")
	if en != "Your verification code is: 123456\n\nThis code will expire in 10 minutes." {
		t.Fatalf("unexpected English body: %s", en)
	}
	if zh != "您的验证码是：123456\n\n该验证码将在 10 分钟后过期。" {
		t.Fatalf("unexpected Chinese body: %s", zh)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		header   string
		expected string
	}{
		{"zh-CN", "zh"},
		{"zh", "zh"},
		{"en-US", "en"},
		{"en", "en"},
		{"", "en"},
		{"fr", "en"},
		{"de-DE,de;q=0.9,en;q=0.8", "en"},
		{"zh-TW,zh;q=0.9,en;q=0.8", "zh"},
		{"ja", "en"},
	}
	for _, tt := range tests {
		got := Normalize(tt.header)
		if got != tt.expected {
			t.Errorf("Normalize(%q) = %q, want %q", tt.header, got, tt.expected)
		}
	}
}

func TestFromContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(LocaleKey, "zh")
	lang := FromContext(c)
	if lang != "zh" {
		t.Fatalf("expected zh, got %s", lang)
	}
}

func TestFromContextDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	lang := FromContext(c)
	if lang != "en" {
		t.Fatalf("expected en default, got %s", lang)
	}
}

func TestFromRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	lang := FromRequest(req)
	if lang != "zh" {
		t.Fatalf("expected zh, got %s", lang)
	}
}

func TestFromRequestDefault(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	lang := FromRequest(req)
	if lang != "en" {
		t.Fatalf("expected en default, got %s", lang)
	}
}

func TestMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Middleware())
	router.GET("/lang", func(c *gin.Context) {
		lang := c.GetString(LocaleKey)
		c.String(http.StatusOK, lang)
	})

	req := httptest.NewRequest("GET", "/lang", nil)
	req.Header.Set("Accept-Language", "zh-CN")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Body.String() != "zh" {
		t.Fatalf("expected zh, got %s", recorder.Body.String())
	}
}

func TestMiddlewareDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(Middleware())
	router.GET("/lang", func(c *gin.Context) {
		lang := c.GetString(LocaleKey)
		c.String(http.StatusOK, lang)
	})

	req := httptest.NewRequest("GET", "/lang", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Body.String() != "en" {
		t.Fatalf("expected en, got %s", recorder.Body.String())
	}
}

func TestAllErrorKeysExist(t *testing.T) {
	keys := []string{
		"email_service_not_configured",
		"no_email_associated",
		"failed_to_send_verification_email",
		"email_verification_code_invalid",
		"email_can_only_set_for_password",
		"failed_to_update_email",
		"code_cooldown",
		"code_not_found",
		"code_expired",
		"code_too_many_attempts",
		"code_invalid",
		"smtp_not_configured",
		"from_email_not_configured",
		"smtp_env_managed",
		"smtp_from_email_required",
		"failed_to_send_test_email",
		"verification_email_subject",
		"verification_email_body",
	}
	for _, key := range keys {
		enMsg := T("en", key)
		if enMsg == key {
			t.Errorf("missing English translation for key: %s", key)
		}
		zhMsg := T("zh", key)
		if zhMsg == key {
			t.Errorf("missing Chinese translation for key: %s", key)
		}
	}
}
