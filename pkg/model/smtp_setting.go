package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type SMTPSetting struct {
	Model
	Enabled   bool         `json:"enabled" gorm:"column:enabled;type:boolean;not null;default:false"`
	Host      string       `json:"host" gorm:"column:host;type:varchar(255)"`
	Port      int          `json:"port" gorm:"column:port;type:integer;default:587"`
	Username  string       `json:"username" gorm:"column:username;type:varchar(255)"`
	Password  SecretString `json:"-" gorm:"column:password;type:text"`
	FromEmail string       `json:"fromEmail" gorm:"column:from_email;type:varchar(255)"`
	UseTLS    bool         `json:"useTLS" gorm:"column:use_tls;type:boolean;not null;default:true"`
}

func DefaultSMTPSetting() SMTPSetting {
	return SMTPSetting{
		Model:   Model{ID: 1},
		Enabled: false,
		Port:    587,
		UseTLS:  true,
	}
}

func (s SMTPSetting) Normalized() SMTPSetting {
	normalized := DefaultSMTPSetting()
	normalized.Model = s.Model
	if normalized.ID == 0 {
		normalized.ID = 1
	}
	normalized.Enabled = s.Enabled
	normalized.Host = strings.TrimSpace(s.Host)
	normalized.Port = s.Port
	if normalized.Port == 0 {
		normalized.Port = 587
	}
	normalized.Username = strings.TrimSpace(s.Username)
	normalized.Password = s.Password
	normalized.FromEmail = strings.TrimSpace(s.FromEmail)
	normalized.UseTLS = s.UseTLS
	return normalized
}

func (s SMTPSetting) Validate() error {
	normalized := s.Normalized()
	if !normalized.Enabled {
		return nil
	}
	if normalized.Host == "" {
		return errors.New("host is required when enabled is true")
	}
	if normalized.Port <= 0 || normalized.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if normalized.FromEmail == "" {
		return errors.New("fromEmail is required when enabled is true")
	}
	return nil
}

func GetSMTPSetting() (*SMTPSetting, error) {
	setting, err := getOrCreateSMTPSetting()
	if err != nil {
		return nil, err
	}
	normalized := setting.Normalized()
	return &normalized, nil
}

func UpdateSMTPSetting(setting *SMTPSetting) (*SMTPSetting, error) {
	if setting == nil {
		return nil, errors.New("smtp setting is nil")
	}
	current, err := getOrCreateSMTPSetting()
	if err != nil {
		return nil, err
	}
	normalized := setting.Normalized()
	current.Enabled = normalized.Enabled
	current.Host = normalized.Host
	current.Port = normalized.Port
	current.Username = normalized.Username
	current.FromEmail = normalized.FromEmail
	current.UseTLS = normalized.UseTLS
	if string(normalized.Password) != "" {
		current.Password = normalized.Password
	}
	if err := DB.Save(current).Error; err != nil {
		return nil, err
	}
	updated := current.Normalized()
	return &updated, nil
}

func (s *SMTPSetting) PasswordConfigured() bool {
	if s == nil {
		return false
	}
	return string(s.Password) != ""
}

func getOrCreateSMTPSetting() (*SMTPSetting, error) {
	var setting SMTPSetting
	err := DB.First(&setting, 1).Error
	if err == nil {
		return &setting, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	setting = DefaultSMTPSetting()
	if err := DB.Create(&setting).Error; err != nil {
		return nil, err
	}
	return &setting, nil
}
