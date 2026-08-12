package email

import (
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zxh326/kite/pkg/i18n"
)

const (
	codeLength    = 6
	codeTTL       = 10 * time.Minute
	sendCooldown  = 60 * time.Second // min interval between sends to same email
	maxAttempts   = 5
	attemptWindow = 10 * time.Minute
)

type codeEntry struct {
	code      string
	expiresAt time.Time
	sentAt    time.Time
	attempts  []time.Time
}

type CodeManager struct {
	mu    sync.Mutex
	codes map[string]*codeEntry // key: email
}

var defaultCodeManager = &CodeManager{
	codes: make(map[string]*codeEntry),
}

// GetCodeManager returns the singleton CodeManager.
func GetCodeManager() *CodeManager {
	return defaultCodeManager
}

// GenerateAndStore generates a new verification code for the email,
// stores it, and returns the code. It enforces a send cooldown.
func (m *CodeManager) GenerateAndStore(email, lang string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Check cooldown
	if entry, ok := m.codes[email]; ok {
		if now.Sub(entry.sentAt) < sendCooldown {
			seconds := int(sendCooldown.Seconds()) - int(now.Sub(entry.sentAt).Seconds())
			return "", fmt.Errorf(i18n.T(lang, "code_cooldown"), seconds)
		}
	}

	code, err := generateCode()
	if err != nil {
		return "", err
	}

	m.codes[email] = &codeEntry{
		code:      code,
		expiresAt: now.Add(codeTTL),
		sentAt:    now,
		attempts:  nil,
	}

	return code, nil
}

// Verify checks the code for the email. On success, the code is consumed.
// On failure, an attempt is recorded; after maxAttempts the code is invalidated.
func (m *CodeManager) Verify(email, code, lang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.codes[email]
	if !ok {
		return errors.New(i18n.T(lang, "code_not_found"))
	}

	now := time.Now()
	if now.After(entry.expiresAt) {
		delete(m.codes, email)
		return errors.New(i18n.T(lang, "code_expired"))
	}

	// Clean old attempts outside the window
	validAttempts := entry.attempts[:0]
	for _, t := range entry.attempts {
		if now.Sub(t) < attemptWindow {
			validAttempts = append(validAttempts, t)
		}
	}
	entry.attempts = validAttempts

	if len(entry.attempts) >= maxAttempts {
		delete(m.codes, email)
		return errors.New(i18n.T(lang, "code_too_many_attempts"))
	}

	if entry.code != code {
		entry.attempts = append(entry.attempts, now)
		return errors.New(i18n.T(lang, "code_invalid"))
	}

	// Success — consume the code
	delete(m.codes, email)
	return nil
}

// cleanupExpired removes expired entries. Called periodically.
func (m *CodeManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for email, entry := range m.codes {
		if now.After(entry.expiresAt) {
			delete(m.codes, email)
		}
	}
}

func generateCode() (string, error) {
	buf := make([]byte, codeLength)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	code := make([]byte, codeLength)
	for i := 0; i < codeLength; i++ {
		code[i] = '0' + (buf[i] % 10)
	}
	return string(code), nil
}

// StartCleanup starts a background goroutine that periodically cleans expired codes.
func StartCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			defaultCodeManager.cleanupExpired()
		}
	}()
}
