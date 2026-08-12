package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/i18n"
	"github.com/zxh326/kite/pkg/model"
)

// GetEffectiveSMTPSetting returns the SMTP settings from env vars if
// SMTP_HOST is set, otherwise from the database.
func GetEffectiveSMTPSetting() (*model.SMTPSetting, error) {
	if common.SMTPEnvEnabled {
		return &model.SMTPSetting{
			Enabled:   true,
			Host:      common.SMTPHost,
			Port:      common.SMTPPort,
			Username:  common.SMTPUsername,
			Password:  model.SecretString(common.SMTPPassword),
			FromEmail: common.SMTPFromEmail,
			UseTLS:    common.SMTPUseTLS,
		}, nil
	}
	return model.GetSMTPSetting()
}

// IsSMTPEnabled returns true if SMTP is configured and enabled.
func IsSMTPEnabled() bool {
	setting, err := GetEffectiveSMTPSetting()
	if err != nil {
		return false
	}
	return setting != nil && setting.Enabled && setting.Host != ""
}

// SendVerificationCode sends a 6-digit verification code to the recipient.
// The lang parameter controls the language of the email content.
func SendVerificationCode(to, code, lang string) error {
	setting, err := GetEffectiveSMTPSetting()
	if err != nil {
		return fmt.Errorf("failed to get SMTP settings: %w", err)
	}
	if !setting.Enabled || setting.Host == "" {
		return errors.New(i18n.T(lang, "smtp_not_configured"))
	}

	subject := i18n.T(lang, "verification_email_subject")
	body := i18n.T(lang, "verification_email_body", code)

	return sendMail(setting, to, subject, body)
}

func sendMail(setting *model.SMTPSetting, to, subject, body string) error {
	from := setting.FromEmail
	if from == "" {
		return errors.New(i18n.T("en", "from_email_not_configured"))
	}

	headers := map[string]string{
		"From":         from,
		"To":           to,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var msg strings.Builder
	for k, v := range headers {
		fmt.Fprintf(&msg, "%s: %s\r\n", k, v)
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	addr := net.JoinHostPort(setting.Host, fmt.Sprintf("%d", setting.Port))
	auth := smtp.PlainAuth("", setting.Username, string(setting.Password), setting.Host)

	if setting.UseTLS {
		return sendMailTLS(addr, auth, from, []string{to}, []byte(msg.String()))
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg.String()))
}

func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer func() {
		_ = client.Quit()
	}()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO failed: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}
