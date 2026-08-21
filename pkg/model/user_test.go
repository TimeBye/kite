package model

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUserKey(t *testing.T) {
	tests := []struct {
		name     string
		user     User
		expected string
	}{
		{"username", User{Model: Model{ID: 1}, Username: "alice", Name: "Alice", Sub: "sub"}, "alice"},
		{"name", User{Model: Model{ID: 2}, Name: "Alice", Sub: "sub"}, "Alice"},
		{"sub", User{Model: Model{ID: 3}, Sub: "sub"}, "sub"},
		{"id", User{Model: Model{ID: 4}}, "4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.Key(); got != tt.expected {
				t.Fatalf("Key() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUserGetAPIKey(t *testing.T) {
	user := User{
		Model:  Model{ID: 42},
		APIKey: SecretString("secret"),
	}

	if got, want := user.GetAPIKey(), "kite42-secret"; got != want {
		t.Fatalf("GetAPIKey() = %q, want %q", got, want)
	}
}

func TestCheckPassword(t *testing.T) {
	hash, err := utils.HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if !CheckPassword(hash, "secret") {
		t.Fatal("CheckPassword() returned false for matching password")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("CheckPassword() returned true for non-matching password")
	}
}

func TestUpsertLDAPUserClearsDisabledProfileMappingSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrating database: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	existing := &User{
		Username:        "alice",
		Name:            "Alice",
		NameSource:      AuthProviderLDAP,
		Email:           "alice@example.com",
		EmailSource:     AuthProviderLDAP,
		EmailVerified:   true,
		AvatarURL:       "https://example.com/alice.png",
		AvatarURLSource: AuthProviderLDAP,
		Provider:        AuthProviderLDAP,
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("creating user: %v", err)
	}

	updated, err := UpsertLDAPUser(&User{Username: "alice"})
	if err != nil {
		t.Fatalf("UpsertLDAPUser() error = %v", err)
	}
	if updated.Name != existing.Name || updated.Email != existing.Email || updated.AvatarURL != existing.AvatarURL {
		t.Fatalf("UpsertLDAPUser() = %#v, want preserved profile values", updated)
	}
	if updated.NameSource != "" || updated.EmailSource != "" || updated.AvatarURLSource != "" {
		t.Fatalf("UpsertLDAPUser() = %#v, want cleared profile sources", updated)
	}
	var stored User
	if err := db.First(&stored, existing.ID).Error; err != nil {
		t.Fatalf("loading user: %v", err)
	}
	if stored.NameSource != "" || stored.EmailSource != "" || stored.AvatarURLSource != "" {
		t.Fatalf("stored user = %#v, want cleared profile sources", stored)
	}
}

func TestEmailOTPRestrictions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if err := db.AutoMigrate(&EmailOTP{}); err != nil {
		t.Fatalf("migrating database: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	now := time.Now()
	code, err := CreateEmailOTP(1, "alice@example.com", EmailOTPSetupMFA, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("creating OTP: %v", err)
	}
	if _, err := CreateEmailOTP(1, "alice@example.com", EmailOTPSetupMFA, now.Add(10*time.Minute)); !errors.Is(err, ErrEmailOTPTooFrequent) {
		t.Fatalf("second OTP error = %v, want %v", err, ErrEmailOTPTooFrequent)
	}
	if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPEnableMFA, code, now); err != nil || verified {
		t.Fatalf("cross-purpose OTP verification = %v, %v", verified, err)
	}
	if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPSetupMFA, code, now); err != nil || !verified {
		t.Fatalf("OTP verification = %v, %v", verified, err)
	}
	if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPSetupMFA, code, now); err != nil || verified {
		t.Fatalf("consumed OTP verification = %v, %v", verified, err)
	}

	attemptCode, err := CreateEmailOTP(1, "alice@example.com", EmailOTPDisableMFA, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("creating attempt OTP: %v", err)
	}
	for range 5 {
		if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPDisableMFA, "000000", now); err != nil || verified {
			t.Fatalf("invalid OTP verification = %v, %v", verified, err)
		}
	}
	if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPDisableMFA, attemptCode, now); err != nil || verified {
		t.Fatalf("locked OTP verification = %v, %v", verified, err)
	}

	expiredCode, err := CreateEmailOTP(1, "alice@example.com", EmailOTPAddPasskey, now.Add(-time.Minute))
	if err != nil {
		t.Fatalf("creating expired OTP: %v", err)
	}
	if verified, err := ConsumeEmailOTP(1, "alice@example.com", EmailOTPAddPasskey, expiredCode, now); err != nil || verified {
		t.Fatalf("expired OTP verification = %v, %v", verified, err)
	}
}

func TestUpdateUserEmailMarksEmailVerified(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrating database: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	user := &User{Username: "alice"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("creating user: %v", err)
	}
	if err := UpdateUserEmail(user.ID, "alice@example.com"); err != nil {
		t.Fatalf("updating user email: %v", err)
	}
	var stored User
	if err := db.First(&stored, user.ID).Error; err != nil {
		t.Fatalf("loading user: %v", err)
	}
	if stored.Email != "alice@example.com" || !stored.EmailVerified || stored.EmailVerifiedAt == nil {
		t.Fatalf("stored email verification = %#v", stored)
	}
}

func TestFindWithSubOrUpsertUserClearsDisabledProfileMappingSources(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrating database: %v", err)
	}
	oldDB := DB
	DB = db
	t.Cleanup(func() { DB = oldDB })

	existing := &User{
		Username:        "alice",
		Sub:             "subject-1",
		Provider:        "oidc",
		Name:            "Local Name",
		Email:           "local@example.com",
		AvatarURL:       "https://example.com/local.png",
		NameSource:      "oidc",
		EmailSource:     "oidc",
		AvatarURLSource: "oidc",
	}
	if err := db.Create(existing).Error; err != nil {
		t.Fatalf("creating user: %v", err)
	}

	synced := &User{Username: "alice", Sub: "subject-1", Provider: "oidc"}
	if err := FindWithSubOrUpsertUser(synced); err != nil {
		t.Fatalf("syncing user: %v", err)
	}
	if synced.Name != existing.Name || synced.Email != existing.Email || synced.AvatarURL != existing.AvatarURL {
		t.Fatalf("disabled profile mapping values were not preserved: %#v", synced)
	}
	if synced.NameSource != "" || synced.EmailSource != "" || synced.AvatarURLSource != "" {
		t.Fatalf("disabled profile mapping sources were not cleared: %#v", synced)
	}
	var stored User
	if err := db.First(&stored, existing.ID).Error; err != nil {
		t.Fatalf("loading user: %v", err)
	}
	if stored.NameSource != "" || stored.EmailSource != "" || stored.AvatarURLSource != "" {
		t.Fatalf("stored user = %#v, want cleared profile sources", stored)
	}

	synced.Name, synced.NameSource = "External Name", "oidc"
	if err := FindWithSubOrUpsertUser(synced); err != nil {
		t.Fatalf("syncing mapped user: %v", err)
	}
	if synced.Name != "External Name" || synced.NameSource != "oidc" || synced.Email != existing.Email || synced.AvatarURL != existing.AvatarURL {
		t.Fatalf("mapped profile sync mismatch: %#v", synced)
	}
}
