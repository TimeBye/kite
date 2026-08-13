package connector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	restclient "k8s.io/client-go/rest"
)

// K8sCredentials holds the Kubernetes API server credentials that the Agent
// extracts from its kubeconfig/InClusterConfig and sends to the Server during
// the WebSocket handshake. The Server uses these to build a rest.Config that
// performs TLS + auth directly, so the Agent can be a pure TCP forwarder.
type K8sCredentials struct {
	Host        string `json:"host"`     // e.g. "https://10.0.0.1:443"
	BearerToken string `json:"token"`    // bearer token (if any)
	CAData      []byte `json:"caData"`   // CA cert PEM (if any)
	CertData    []byte `json:"certData"` // client cert PEM (if any)
	KeyData     []byte `json:"keyData"`  // client key PEM (if any)
	Insecure    bool   `json:"insecure"` // skip TLS verification
}

// credentialHeader is the HTTP header used to transmit credentials
// from Agent to Server during the WebSocket handshake.
const credentialHeader = "X-Kite-K8s-Credentials"

// extractCredentials reads auth-relevant fields from a rest.Config, resolving
// any file-based references (CAFile, CertFile, KeyFile) into in-memory data.
func extractCredentials(config *restclient.Config) (*K8sCredentials, error) {
	tls := config.TLSClientConfig
	creds := &K8sCredentials{
		Host:        config.Host,
		BearerToken: config.BearerToken,
		Insecure:    tls.Insecure,
	}

	if len(tls.CAData) > 0 {
		creds.CAData = tls.CAData
	} else if tls.CAFile != "" {
		data, err := os.ReadFile(tls.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		creds.CAData = data
	}

	if len(tls.CertData) > 0 {
		creds.CertData = tls.CertData
	} else if tls.CertFile != "" {
		data, err := os.ReadFile(tls.CertFile)
		if err != nil {
			return nil, fmt.Errorf("read cert file: %w", err)
		}
		creds.CertData = data
	}

	if len(tls.KeyData) > 0 {
		creds.KeyData = tls.KeyData
	} else if tls.KeyFile != "" {
		data, err := os.ReadFile(tls.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("read key file: %w", err)
		}
		creds.KeyData = data
	}

	return creds, nil
}

// MarshalHeader serializes credentials to a base64-encoded JSON string
// suitable for use as an HTTP header value.
func (c *K8sCredentials) MarshalHeader() (string, error) {
	data, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal credentials: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// UnmarshalHeader decodes credentials from a base64-encoded JSON header value.
func UnmarshalHeader(value string) (*K8sCredentials, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode credentials header: %w", err)
	}
	var creds K8sCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("unmarshal credentials: %w", err)
	}
	return &creds, nil
}

// ToRestConfig creates a rest.Config from credentials with a custom dialer.
// The dialer routes connections through the remotedialer tunnel.
func (c *K8sCredentials) ToRestConfig(dialer func(ctx context.Context, network, addr string) (net.Conn, error)) *restclient.Config {
	return &restclient.Config{
		Host:        c.Host,
		BearerToken: c.BearerToken,
		TLSClientConfig: restclient.TLSClientConfig{
			Insecure: c.Insecure,
			CAData:   c.CAData,
			CertData: c.CertData,
			KeyData:  c.KeyData,
		},
		Dial: dialer,
	}
}

// TLSConfig returns a *tls.Config suitable for SPDY round tripper creation.
func (c *K8sCredentials) TLSConfig() *tls.Config {
	if !c.Insecure && len(c.CAData) == 0 && len(c.CertData) == 0 {
		return nil
	}
	cfg := &tls.Config{
		InsecureSkipVerify: c.Insecure,
	}
	if len(c.CAData) > 0 {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(c.CAData)
		cfg.RootCAs = pool
	}
	if len(c.CertData) > 0 && len(c.KeyData) > 0 {
		cert, err := tls.X509KeyPair(c.CertData, c.KeyData)
		if err == nil {
			cfg.Certificates = []tls.Certificate{cert}
		}
	}
	return cfg
}

// parseHostPort extracts host:port from a Kubernetes API server URL.
// e.g. "https://10.0.0.1:443" → "10.0.0.1:443"
func parseHostPort(apiURL string) (string, error) {
	var host, port string
	// Strip scheme
	for _, prefix := range []string{"https://", "http://"} {
		if len(apiURL) > len(prefix) && apiURL[:len(prefix)] == prefix {
			apiURL = apiURL[len(prefix):]
			break
		}
	}
	host, port, err := net.SplitHostPort(apiURL)
	if err != nil {
		// No port specified, default based on scheme
		return "", fmt.Errorf("parse API server host:port from %q: %w", apiURL, err)
	}
	return net.JoinHostPort(host, port), nil
}

// credentialsHeaderName returns the HTTP header name used for credential transport.
func credentialsHeaderName() string {
	return credentialHeader
}

// SetCredentialHeader sets the credentials header on an HTTP header map.
func SetCredentialHeader(headers http.Header, creds *K8sCredentials) error {
	value, err := creds.MarshalHeader()
	if err != nil {
		return err
	}
	headers.Set(credentialHeader, value)
	return nil
}
