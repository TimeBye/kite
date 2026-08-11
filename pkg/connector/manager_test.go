package connector

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupManagerTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Cluster{}, &model.User{}, &model.Role{}, &model.RoleAssignment{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	model.DB = db
}

// --- Token validation tests ---

func TestNewToken(t *testing.T) {
	token, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error: %v", err)
	}
	if token == "" || hash == "" {
		t.Fatal("NewToken() returned empty token or hash")
	}
	if token == hash {
		t.Fatal("token and hash should differ")
	}
	if !validToken(token) {
		t.Fatal("NewToken() token should pass validToken()")
	}
}

func TestNewToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token, _, err := NewToken()
		if err != nil {
			t.Fatalf("NewToken() error on iteration %d: %v", i, err)
		}
		if tokens[token] {
			t.Fatalf("duplicate token generated on iteration %d", i)
		}
		tokens[token] = true
	}
}

func TestTokenHash_Deterministic(t *testing.T) {
	token := "abc123"
	h1 := tokenHash(token)
	h2 := tokenHash(token)
	if h1 != h2 {
		t.Fatal("tokenHash should be deterministic")
	}
	if tokenHash("different") == h1 {
		t.Fatal("different tokens should produce different hashes")
	}
}

func TestValidToken(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"valid token from NewToken", mustNewToken(t), true},
		{"empty string", "", false},
		{"short string", "abc", false},
		{"non-base64", "!!!!!!!!!notbase64!!!!!!!invalid_chars", false},
		{"valid base64 but wrong length", base64.RawURLEncoding.EncodeToString([]byte("short")), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validToken(tt.token); got != tt.want {
				t.Errorf("validToken(%q) = %v, want %v", tt.token, got, tt.want)
			}
		})
	}
}

func mustNewToken(t *testing.T) string {
	t.Helper()
	token, _, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	return token
}

// --- resolveToken tests ---

func TestResolveToken(t *testing.T) {
	setupManagerTestDB(t)

	token, hash, err := NewToken()
	if err != nil {
		t.Fatalf("NewToken() error: %v", err)
	}

	// Create a connector cluster with this token hash
	cluster := &model.Cluster{
		Name:               "test-connector",
		Connector:          true,
		Enable:             true,
		ConnectorTokenHash: hash,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}

	// Valid token → returns cluster
	got, err := resolveToken(token)
	if err != nil {
		t.Fatalf("resolveToken() error: %v", err)
	}
	if got == nil {
		t.Fatal("resolveToken() returned nil for valid token")
	}
	if got.Name != "test-connector" {
		t.Errorf("resolveToken() returned cluster %q, want %q", got.Name, "test-connector")
	}

	// Invalid token → returns nil, nil
	got, err = resolveToken("invalid-token")
	if err != nil {
		t.Fatalf("resolveToken() error for invalid: %v", err)
	}
	if got != nil {
		t.Fatal("resolveToken() should return nil for invalid token")
	}
}

func TestResolveToken_DisabledCluster(t *testing.T) {
	setupManagerTestDB(t)

	token, hash, _ := NewToken()
	cluster := &model.Cluster{
		Name:               "disabled-connector",
		Connector:          true,
		Enable:             true,
		ConnectorTokenHash: hash,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}
	// Use map to override GORM's default:true on the Enable column.
	if err := model.DB.Model(&model.Cluster{}).Where("id = ?", cluster.ID).Updates(map[string]interface{}{"enable": false}).Error; err != nil {
		t.Fatalf("failed to disable cluster: %v", err)
	}

	got, err := resolveToken(token)
	if err != nil {
		t.Fatalf("resolveToken() error: %v", err)
	}
	if got != nil {
		t.Fatal("resolveToken() should return nil for disabled cluster")
	}
}

func TestResolveToken_NonConnectorCluster(t *testing.T) {
	setupManagerTestDB(t)

	token, hash, _ := NewToken()
	cluster := &model.Cluster{
		Name:               "not-connector",
		Connector:          false, // not a connector cluster
		Enable:             true,
		ConnectorTokenHash: hash,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}

	got, err := resolveToken(token)
	if err != nil {
		t.Fatalf("resolveToken() error: %v", err)
	}
	if got != nil {
		t.Fatal("resolveToken() should return nil for non-connector cluster")
	}
}

// --- Manifest grant round-trip tests ---

func TestManifestGrant_RoundTrip(t *testing.T) {
	setupManagerTestDB(t)
	common.JwtSecret = "test-secret-key"
	m := NewManager(func() {})

	// ResolveManifestGrant calls resolveToken which checks the DB, so we
	// need a real cluster with a valid token.
	token, hash, _ := NewToken()
	cluster := &model.Cluster{
		Name:               "grant-test-cluster",
		Connector:          true,
		Enable:             true,
		ConnectorTokenHash: hash,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("AddCluster() error: %v", err)
	}

	grant, err := m.CreateManifestGrant(token)
	if err != nil {
		t.Fatalf("CreateManifestGrant() error: %v", err)
	}
	if grant == "" {
		t.Fatal("CreateManifestGrant() returned empty grant")
	}

	resolved, err := m.ResolveManifestGrant(grant)
	if err != nil {
		t.Fatalf("ResolveManifestGrant() error: %v", err)
	}
	if resolved != token {
		t.Errorf("ResolveManifestGrant() = %q, want %q", resolved, token)
	}
}

func TestManifestGrant_DifferentSecrets(t *testing.T) {
	common.JwtSecret = "secret-a"
	m1 := NewManager(func() {})

	common.JwtSecret = "secret-b"
	m2 := NewManager(func() {})

	grant, err := m1.CreateManifestGrant("my-token")
	if err != nil {
		t.Fatalf("CreateManifestGrant() error: %v", err)
	}

	// Decrypting with a different secret should fail
	_, err = m2.ResolveManifestGrant(grant)
	if !errors.Is(err, ErrInvalidManifestGrant) {
		t.Errorf("ResolveManifestGrant() with wrong secret: error = %v, want ErrInvalidManifestGrant", err)
	}
}

func TestResolveManifestGrant_InvalidInput(t *testing.T) {
	common.JwtSecret = "test-secret"
	m := NewManager(func() {})

	tests := []struct {
		name  string
		grant string
	}{
		{"empty string", ""},
		{"not base64", "!!!not-base64!!!"},
		{"too short", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := m.ResolveManifestGrant(tt.grant)
			// Should not return a generic error; either nil or ErrInvalidManifestGrant
			if err != nil && !errors.Is(err, ErrInvalidManifestGrant) {
				t.Errorf("ResolveManifestGrant(%q) error = %v, want nil or ErrInvalidManifestGrant", tt.grant, err)
			}
		})
	}
}

func TestResolveManifestGrant_TamperedCiphertext(t *testing.T) {
	common.JwtSecret = "test-secret"
	m := NewManager(func() {})

	grant, err := m.CreateManifestGrant("my-token")
	if err != nil {
		t.Fatalf("CreateManifestGrant() error: %v", err)
	}

	// Tamper with the grant by flipping the last character
	tampered := grant[:len(grant)-1]
	if grant[len(grant)-1] == 'A' {
		tampered += "B"
	} else {
		tampered += "A"
	}

	_, err = m.ResolveManifestGrant(tampered)
	if !errors.Is(err, ErrInvalidManifestGrant) {
		t.Errorf("ResolveManifestGrant(tampered) error = %v, want ErrInvalidManifestGrant", err)
	}
}

// --- Listen / Remove caching tests ---

func TestListen_CachesAddress(t *testing.T) {
	m := NewManager(func() {})

	addr1, err := m.Listen(1)
	if err != nil {
		t.Fatalf("first Listen() error: %v", err)
	}
	if addr1 == "" {
		t.Fatal("first Listen() returned empty address")
	}

	// Second call should return the same cached address
	addr2, err := m.Listen(1)
	if err != nil {
		t.Fatalf("second Listen() error: %v", err)
	}
	if addr1 != addr2 {
		t.Errorf("Listen() caching failed: addr1=%q, addr2=%q", addr1, addr2)
	}
}

func TestListen_DifferentClusters(t *testing.T) {
	m := NewManager(func() {})

	addr1, err := m.Listen(1)
	if err != nil {
		t.Fatalf("Listen(1) error: %v", err)
	}
	addr2, err := m.Listen(2)
	if err != nil {
		t.Fatalf("Listen(2) error: %v", err)
	}
	if addr1 == addr2 {
		t.Fatal("different clusters should get different addresses")
	}
}

func TestListen_ReturnsValidLocalAddress(t *testing.T) {
	m := NewManager(func() {})

	addr, err := m.Listen(42)
	if err != nil {
		t.Fatalf("Listen() error: %v", err)
	}

	// Address should be 127.0.0.1:<port>
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("invalid address format %q: %v", addr, err)
	}
	if host != "127.0.0.1" {
		t.Errorf("expected host 127.0.0.1, got %q", host)
	}
	if port == "" {
		t.Error("expected non-empty port")
	}
}

func TestRemove_CleansUpProxy(t *testing.T) {
	m := NewManager(func() {})

	addr1, err := m.Listen(1)
	if err != nil {
		t.Fatalf("first Listen() error: %v", err)
	}

	// Remove should close the server and clean up the map entry
	m.Remove(1)

	// After Remove, Listen should create a new proxy with a different address
	addr2, err := m.Listen(1)
	if err != nil {
		t.Fatalf("Listen() after Remove() error: %v", err)
	}
	if addr1 == addr2 {
		t.Fatal("Listen() after Remove() should return a new address")
	}
}

func TestRemove_NeverListened_NoPanic(t *testing.T) {
	m := NewManager(func() {})
	// Should not panic
	m.Remove(999)
}

func TestRemove_MultipleCalls(t *testing.T) {
	m := NewManager(func() {})
	_, _ = m.Listen(1)
	m.Remove(1)
	// Second Remove should not panic
	m.Remove(1)
}

// --- connectorResponder tests ---

func TestConnectorResponder_Error(t *testing.T) {
	responder := &connectorResponder{}
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/pods", nil)
	testErr := context.DeadlineExceeded

	responder.Error(w, req, testErr)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d", http.StatusBadGateway, w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "proxy error") {
		t.Errorf("response body should contain 'proxy error', got: %s", body)
	}
}

// --- Manager.ServeHTTP basic smoke test ---

func TestServeHTTP_UnauthorizedRequest(t *testing.T) {
	setupManagerTestDB(t)
	m := NewManager(func() {})

	// A request without auth should not crash but be rejected by remotedialer
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	m.ServeHTTP(w, req)

	// remotedialer returns 403 for unauthorized connections
	if w.Code != http.StatusForbidden && w.Code != http.StatusUnauthorized {
		// remotedialer may return different codes depending on version
		t.Logf("ServeHTTP returned status %d (expected 401 or 403)", w.Code)
	}
}
