package auth

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/zxh326/kite/pkg/model"
)

func TestLDAPUserProfileMapping(t *testing.T) {
	entry := &ldap.Entry{Attributes: []*ldap.EntryAttribute{
		{Name: "uid", Values: []string{"alice"}},
		{Name: "cn", Values: []string{"Alice"}},
		{Name: "mail", Values: []string{"alice@example.com"}},
		{Name: "jpegPhoto", Values: []string{"https://example.com/alice.png"}},
	}}
	cfg := ldapConfig{UsernameAttribute: "uid", DisplayNameAttribute: "cn", EmailAttribute: "mail", AvatarURLAttribute: "jpegPhoto"}
	user := ldapUserFromEntry(entry, cfg, model.SliceString{"developers"})
	if user.Username != "alice" || user.Name != "Alice" || user.Email != "alice@example.com" || user.AvatarURL != "https://example.com/alice.png" {
		t.Fatalf("ldapUserFromEntry() = %#v", user)
	}
	if user.NameSource != model.AuthProviderLDAP || user.EmailSource != model.AuthProviderLDAP || user.AvatarURLSource != model.AuthProviderLDAP {
		t.Fatalf("ldapUserFromEntry() sources = %#v", user)
	}
	if !user.EmailVerified || user.EmailVerifiedAt == nil {
		t.Fatalf("ldapUserFromEntry() email verification = %#v", user)
	}
}

func TestLDAPUserProfileMappingDisabled(t *testing.T) {
	entry := &ldap.Entry{Attributes: []*ldap.EntryAttribute{
		{Name: "uid", Values: []string{"alice"}},
		{Name: "displayName", Values: []string{"Alice"}},
		{Name: "mail", Values: []string{"alice@example.com"}},
		{Name: "avatar", Values: []string{"https://example.com/alice.png"}},
	}}
	user := ldapUserFromEntry(entry, ldapConfig{UsernameAttribute: "uid"}, nil)
	if user.Name != "" || user.Email != "" || user.AvatarURL != "" {
		t.Fatalf("ldapUserFromEntry() synced disabled profile fields: %#v", user)
	}
	if user.NameSource != "" || user.EmailSource != "" || user.AvatarURLSource != "" {
		t.Fatalf("ldapUserFromEntry() sources = %#v", user)
	}
}

func TestLDAPUserWithoutDisplayNameMapping(t *testing.T) {
	entry := &ldap.Entry{Attributes: []*ldap.EntryAttribute{
		{Name: "uid", Values: []string{"alice"}},
	}}
	user := ldapUserFromEntry(entry, ldapConfig{UsernameAttribute: "uid"}, nil)
	if user.Name != "" || user.NameSource != "" {
		t.Fatalf("ldapUserFromEntry() = %#v, want no display name", user)
	}
}

func TestNewLDAPConfig(t *testing.T) {
	t.Run("disabled setting", func(t *testing.T) {
		_, err := newLDAPConfig(&model.LDAPSetting{Enabled: false})
		if !errors.Is(err, ErrLDAPDisabled) {
			t.Fatalf("newLDAPConfig() error = %v, want ErrLDAPDisabled", err)
		}
	})

	t.Run("invalid setting", func(t *testing.T) {
		_, err := newLDAPConfig(&model.LDAPSetting{Enabled: true})
		if !errors.Is(err, ErrLDAPNotConfigured) {
			t.Fatalf("newLDAPConfig() error = %v, want ErrLDAPNotConfigured", err)
		}
	})

	t.Run("valid setting", func(t *testing.T) {
		setting := &model.LDAPSetting{
			Enabled:              true,
			ServerURL:            "ldap://ldap.example.com",
			BindDN:               "cn=admin,dc=example,dc=com",
			BindPassword:         "secret",
			UserBaseDN:           "ou=users,dc=example,dc=com",
			UserFilter:           "(uid=%s)",
			UsernameAttribute:    "uid",
			DisplayNameAttribute: "cn",
			GroupBaseDN:          "ou=groups,dc=example,dc=com",
			GroupFilter:          "(member=%s)",
			GroupNameAttribute:   "cn",
		}

		got, err := newLDAPConfig(setting)
		if err != nil {
			t.Fatalf("newLDAPConfig() error = %v", err)
		}
		if got.ServerURL != setting.ServerURL || got.BindDN != setting.BindDN || got.BindPassword != string(setting.BindPassword) {
			t.Fatalf("newLDAPConfig() returned unexpected config: %#v", got)
		}
	})
}

func TestFormatLDAPFilter(t *testing.T) {
	got, err := formatLDAPFilter("(uid=%s)", "alice")
	if err != nil {
		t.Fatalf("formatLDAPFilter() error = %v", err)
	}
	if got != "(uid=alice)" {
		t.Fatalf("formatLDAPFilter() = %q, want %q", got, "(uid=alice)")
	}

	if _, err := formatLDAPFilter("(uid=%s)(mail=%s)", "alice"); !errors.Is(err, ErrLDAPNotConfigured) {
		t.Fatalf("formatLDAPFilter() error = %v, want ErrLDAPNotConfigured", err)
	}
}
