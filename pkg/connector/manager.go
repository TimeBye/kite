package connector

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rancher/remotedialer"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
	"golang.org/x/crypto/hkdf"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

const (
	KubernetesAPITarget  = "kubernetes-api"
	connectorTokenSize   = 32
	manifestGrantTimeout = 10 * time.Minute
	manifestGrantAAD     = "kite:connector-manifest-grant:v1"
	// connectorVersionHeader is the HTTP header sent by the Agent during
	// WebSocket handshake to report its build version.
	connectorVersionHeader = "X-Kite-Connector-Version"
)

var ErrInvalidManifestGrant = errors.New("invalid manifest grant")

type requestStateKey struct{}

type requestState struct {
	authenticated bool
}

type manifestGrant struct {
	ConnectorToken string `json:"token"`
	ExpiresAt      int64  `json:"exp"`
}

type Manager struct {
	server    *remotedialer.Server
	onChange  func()
	jwtSecret string
	creds     map[uint]*K8sCredentials
	versions  map[uint]string
	mu        sync.RWMutex
}

func NewManager(onChange func()) *Manager {
	m := &Manager{
		onChange:  onChange,
		jwtSecret: common.JwtSecret,
		creds:     make(map[uint]*K8sCredentials),
		versions:  make(map[uint]string),
	}
	m.server = remotedialer.New(m.authorize, remotedialer.DefaultErrorWriter)
	return m
}

func NewToken() (string, string, error) {
	value := make([]byte, connectorTokenSize)
	if _, err := rand.Read(value); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(value)
	return token, tokenHash(token), nil
}

func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func validToken(token string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(token)
	return err == nil && len(decoded) == connectorTokenSize
}

func resolveToken(token string) (*model.Cluster, error) {
	if !validToken(token) {
		return nil, nil
	}
	cluster, err := model.GetClusterByConnectorTokenHash(tokenHash(token))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !cluster.Connector || !cluster.Enable {
		return nil, nil
	}
	return cluster, nil
}

func manifestGrantCipher(jwtSecret string) (cipher.AEAD, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(jwtSecret), nil, []byte(manifestGrantAAD)), key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (m *Manager) CreateManifestGrant(connectorToken string) (string, error) {
	payload, err := json.Marshal(manifestGrant{
		ConnectorToken: connectorToken,
		ExpiresAt:      time.Now().Add(manifestGrantTimeout).Unix(),
	})
	if err != nil {
		return "", err
	}
	aead, err := manifestGrantCipher(m.jwtSecret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nil, nonce, payload, []byte(manifestGrantAAD))
	encrypted := make([]byte, len(nonce)+len(ciphertext))
	copy(encrypted, nonce)
	copy(encrypted[len(nonce):], ciphertext)
	return base64.RawURLEncoding.EncodeToString(encrypted), nil
}

func (m *Manager) ResolveManifestGrant(grant string) (string, error) {
	encrypted, err := base64.RawURLEncoding.Strict().DecodeString(grant)
	if err != nil {
		return "", ErrInvalidManifestGrant
	}
	aead, err := manifestGrantCipher(m.jwtSecret)
	if err != nil {
		return "", err
	}
	if len(encrypted) < aead.NonceSize() {
		return "", nil
	}
	nonce, ciphertext := encrypted[:aead.NonceSize()], encrypted[aead.NonceSize():]
	payload, err := aead.Open(nil, nonce, ciphertext, []byte(manifestGrantAAD))
	if err != nil {
		return "", ErrInvalidManifestGrant
	}
	var stored manifestGrant
	if err := json.Unmarshal(payload, &stored); err != nil {
		return "", ErrInvalidManifestGrant
	}
	if time.Now().Unix() >= stored.ExpiresAt {
		return "", ErrInvalidManifestGrant
	}
	cluster, err := resolveToken(stored.ConnectorToken)
	if err != nil {
		return "", err
	}
	if cluster == nil {
		return "", nil
	}
	return stored.ConnectorToken, nil
}

func (m *Manager) authorize(req *http.Request) (string, bool, error) {
	authorization := req.Header.Get("Authorization")
	token, found := strings.CutPrefix(authorization, "Bearer ")
	if !found || token == "" {
		return "", false, nil
	}
	cluster, err := resolveToken(token)
	if err != nil {
		klog.Errorf("Failed to validate connector token: %v", err)
		return "", false, errors.New("failed to validate connector token")
	}
	if cluster == nil {
		return "", false, nil
	}

	// Extract Kubernetes API credentials sent by the Agent. These are used
	// by the Server to perform TLS + auth directly (TCP forwarding mode).
	if credHeader := req.Header.Get(credentialsHeaderName()); credHeader != "" {
		creds, err := UnmarshalHeader(credHeader)
		if err != nil {
			klog.Errorf("Failed to parse credentials from connector: %v", err)
			return "", false, errors.New("invalid credentials header")
		}
		m.mu.Lock()
		m.creds[cluster.ID] = creds
		m.mu.Unlock()
	}

	// Store the connector version for display in the cluster list.
	if ver := req.Header.Get(connectorVersionHeader); ver != "" {
		m.mu.Lock()
		m.versions[cluster.ID] = ver
		m.mu.Unlock()
	}

	if state, ok := req.Context().Value(requestStateKey{}).(*requestState); ok {
		state.authenticated = true
	}
	clientKey := strconv.FormatUint(uint64(cluster.ID), 10)
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			if m.server.HasSession(clientKey) {
				m.onChange()
				return
			}
			select {
			case <-req.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return clientKey, true, nil
}

func (m *Manager) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	state := &requestState{}
	req = req.WithContext(context.WithValue(req.Context(), requestStateKey{}, state))
	m.server.ServeHTTP(rw, req)
	if state.authenticated {
		m.onChange()
	}
}

func (m *Manager) Connected(clusterID uint) bool {
	return m.server.HasSession(strconv.FormatUint(uint64(clusterID), 10))
}

func (m *Manager) Dialer(clusterID uint) func(context.Context, string, string) (net.Conn, error) {
	clientKey := strconv.FormatUint(uint64(clusterID), 10)
	dialer := m.server.Dialer(clientKey)
	return func(ctx context.Context, network, _ string) (net.Conn, error) {
		return dialer(ctx, network, KubernetesAPITarget)
	}
}

// GetCredentials returns the Kubernetes API credentials for a connector cluster.
func (m *Manager) GetCredentials(clusterID uint) *K8sCredentials {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.creds[clusterID]
}

// GetVersion returns the connector agent version for a connector cluster.
func (m *Manager) GetVersion(clusterID uint) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.versions[clusterID]
}

// Remove cleans up credentials and version for a connector cluster.
// Called when a connector cluster is disabled or deleted.
func (m *Manager) Remove(clusterID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.creds, clusterID)
	delete(m.versions, clusterID)
}
