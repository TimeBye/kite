package model

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const (
	EmailOTPEmailChange    = "change_email"
	EmailOTPSetupMFA       = "setup_mfa"
	EmailOTPEnableMFA      = "enable_mfa"
	EmailOTPDisableMFA     = "disable_mfa"
	EmailOTPAddPasskey     = "add_passkey"
	EmailOTPDeletePasskey  = "delete_passkey"
	emailOTPResendInterval = time.Minute
	emailOTPMaxAttempts    = 5
)

var ErrEmailOTPTooFrequent = errors.New("email verification code was sent too recently")

type EmailOTP struct {
	Model
	UserID    uint      `gorm:"index;not null"`
	Email     string    `gorm:"type:varchar(255);index;not null"`
	Purpose   string    `gorm:"type:varchar(50);index;not null"`
	CodeHash  string    `gorm:"type:char(64);not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	Attempts  int       `gorm:"not null;default:0"`
}

func CreateEmailOTP(userID uint, email, purpose string, expiresAt time.Time) (string, error) {
	var previous EmailOTP
	err := DB.Where("user_id = ? AND email = ? AND purpose = ?", userID, email, purpose).Order("created_at DESC").First(&previous).Error
	if err == nil && time.Since(previous.CreatedAt) < emailOTPResendInterval {
		return "", ErrEmailOTPTooFrequent
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	code, err := newEmailOTPCode()
	if err != nil {
		return "", err
	}
	if err := DB.Where("user_id = ? AND email = ? AND purpose = ?", userID, email, purpose).Delete(&EmailOTP{}).Error; err != nil {
		return "", err
	}
	otp := EmailOTP{UserID: userID, Email: email, Purpose: purpose, CodeHash: emailOTPHash(code), ExpiresAt: expiresAt}
	if err := DB.Create(&otp).Error; err != nil {
		return "", err
	}
	return code, nil
}

func ConsumeEmailOTP(userID uint, email, purpose, code string, now time.Time) (bool, error) {
	var otp EmailOTP
	err := DB.Where("user_id = ? AND email = ? AND purpose = ? AND expires_at > ?", userID, email, purpose, now).Order("created_at DESC").First(&otp).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if otp.CodeHash == emailOTPHash(code) {
		result := DB.Delete(&otp)
		return result.RowsAffected == 1, result.Error
	}
	if otp.Attempts+1 >= emailOTPMaxAttempts {
		return false, DB.Delete(&otp).Error
	}
	return false, DB.Model(&otp).Update("attempts", gorm.Expr("attempts + ?", 1)).Error
}

func DeleteEmailOTP(userID uint, email, purpose string) error {
	return DB.Where("user_id = ? AND email = ? AND purpose = ?", userID, email, purpose).Delete(&EmailOTP{}).Error
}

func UpdateUserEmail(id uint, email string) error {
	now := time.Now()
	err := DB.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
		"email":             email,
		"email_verified":    true,
		"email_verified_at": now,
	}).Error
	InvalidateUserCache(uint64(id))
	return err
}

func newEmailOTPCode() (string, error) {
	var data [4]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	value := uint32(data[0])<<24 | uint32(data[1])<<16 | uint32(data[2])<<8 | uint32(data[3])
	return fmt.Sprintf("%06d", value%1000000), nil
}

func emailOTPHash(code string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(code)))
}
