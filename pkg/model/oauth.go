package model

import (
	"errors"
	"strings"
)

const AuthProviderPassword = "password"
const AuthProviderLDAP = "ldap"

const ReservedOAuthProviderNamePassword = AuthProviderPassword
const ReservedOAuthProviderNameLDAP = AuthProviderLDAP

var ErrReservedOAuthProviderName = errors.New("oauth provider names 'password' and 'ldap' are reserved for built-in credential providers")

func NormalizeOAuthProviderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func IsReservedOAuthProviderName(name string) bool {
	switch NormalizeOAuthProviderName(name) {
	case ReservedOAuthProviderNamePassword, ReservedOAuthProviderNameLDAP:
		return true
	default:
		return false
	}
}

type OAuthProvider struct {
	Model
	Name           LowerCaseString `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	ClientID       string          `json:"clientId" gorm:"type:varchar(255);not null"`
	ClientSecret   SecretString    `json:"clientSecret" gorm:"type:text;not null"`
	AuthURL        string          `json:"authUrl" gorm:"type:varchar(255)"`
	TokenURL       string          `json:"tokenUrl" gorm:"type:varchar(255)"`
	UserInfoURL    string          `json:"userInfoUrl" gorm:"type:varchar(255)"`
	Scopes         string          `json:"scopes" gorm:"type:varchar(255)"`
	Issuer         string          `json:"issuer" gorm:"type:varchar(255)"`
	Enabled        bool            `json:"enabled" gorm:"type:boolean;default:true"`
	UsernameClaim  string          `json:"usernameClaim" gorm:"type:varchar(255)"`
	NameClaim      string          `json:"nameClaim" gorm:"type:varchar(255)"`
	EmailClaim     string          `json:"emailClaim" gorm:"type:varchar(255)"`
	AvatarURLClaim string          `json:"avatarUrlClaim" gorm:"type:varchar(255)"`
	GroupsClaim    string          `json:"groupsClaim" gorm:"type:varchar(255)"`
	AllowedGroups  string          `json:"allowedGroups" gorm:"type:text"`

	RedirectURL string `json:"-" gorm:"-"`
}

func GetAllOAuthProviders() ([]OAuthProvider, error) {
	var providers []OAuthProvider
	err := DB.Find(&providers).Error
	return providers, err
}

func GetOAuthProvidersWithPagination(page, size int) ([]OAuthProvider, int64, error) {
	var providers []OAuthProvider
	var total int64
	if err := DB.Model(&OAuthProvider{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := DB.Offset((page - 1) * size).Limit(size).Find(&providers).Error; err != nil {
		return nil, 0, err
	}
	return providers, total, nil
}

func GetEnabledOAuthProviders() ([]OAuthProvider, error) {
	var providers []OAuthProvider
	err := DB.Where("enabled = ?", true).Find(&providers).Error
	return providers, err
}

func GetOAuthProviderByName(name string) (OAuthProvider, error) {
	var provider OAuthProvider
	name = NormalizeOAuthProviderName(name)
	err := DB.Where("name = ? AND enabled = ?", name, true).First(&provider).Error
	if err != nil {
		return OAuthProvider{}, err
	}
	return provider, nil
}

func CreateOAuthProvider(provider *OAuthProvider) error {
	if IsReservedOAuthProviderName(string(provider.Name)) {
		return ErrReservedOAuthProviderName
	}
	return DB.Create(provider).Error
}

func UpdateOAuthProvider(provider *OAuthProvider, updates map[string]interface{}) error {
	name := string(provider.Name)
	switch value := updates["name"].(type) {
	case string:
		name = value
	case LowerCaseString:
		name = string(value)
	}
	if IsReservedOAuthProviderName(name) {
		return ErrReservedOAuthProviderName
	}
	return DB.Model(provider).Updates(updates).Error
}

func DeleteOAuthProvider(id uint) error {
	return DB.Delete(&OAuthProvider{}, id).Error
}
