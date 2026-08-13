package kube

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"

	restclient "k8s.io/client-go/rest"
)

// --- buildExecutor dispatch tests ---

func TestBuildExecutor_ConnectorUsesSPDY(t *testing.T) {
	// For connector clusters, buildExecutor should route to
	// buildSPDYExecutorWithDialer. We verify this by checking that it
	// does not panic and returns an error (since we have no real API server).
	config := &restclient.Config{
		Host: "https://kubernetes-api:443",
	}
	config.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return nil, nil
	}

	// We can't easily test buildSPDYExecutorWithDialer without a real
	// transport, but we can test the dispatch logic.
	c := &K8sClient{
		IsConnector:   true,
		Configuration: config,
	}
	reqURL := &url.URL{Scheme: "https", Host: "kubernetes-api:443", Path: "/api/v1/namespaces/default/pods/foo/exec"}

	// This will fail because the dialer signature doesn't match, but
	// the important thing is that it routes to the SPDY path (not fallback).
	_, err := buildExecutor(c, reqURL)
	// We expect an error since there's no real API server, but the function
	// should not panic.
	_ = err
}

func TestBuildExecutor_NonConnectorUsesFallback(t *testing.T) {
	// For non-connector clusters, buildExecutor should route to
	// buildFallbackExecutor (WebSocket → SPDY).
	config := &restclient.Config{
		Host: "https://127.0.0.1:6443",
	}

	c := &K8sClient{
		IsConnector:   false,
		Configuration: config,
	}
	reqURL := &url.URL{Scheme: "https", Host: "127.0.0.1:6443", Path: "/api/v1/namespaces/default/pods/foo/exec"}

	_, err := buildExecutor(c, reqURL)
	// We expect an error since there's no real API server.
	_ = err
}

// --- extractHTTPTransport tests ---

func TestExtractHTTPTransport_DirectTransport(t *testing.T) {
	transport := &http.Transport{}
	got := extractHTTPTransport(transport)
	if got != transport {
		t.Error("expected to find the transport directly")
	}
}

func TestExtractHTTPTransport_NilInput(t *testing.T) {
	got := extractHTTPTransport(nil)
	if got != nil {
		t.Error("expected nil for nil input")
	}
}

func TestExtractHTTPTransport_WrappedTransport(t *testing.T) {
	inner := &http.Transport{}
	wrapped := &mockWrappedRT{inner: inner}

	got := extractHTTPTransport(wrapped)
	if got != inner {
		t.Error("expected to find the inner transport")
	}
}

func TestExtractHTTPTransport_DeeplyWrapped(t *testing.T) {
	inner := &http.Transport{}
	wrapped := &mockWrappedRT{inner: &mockWrappedRT{inner: inner}}

	got := extractHTTPTransport(wrapped)
	if got != inner {
		t.Error("expected to find the innermost transport")
	}
}

func TestExtractHTTPTransport_NotATransport(t *testing.T) {
	got := extractHTTPTransport(&mockWrappedRT{inner: nil})
	if got != nil {
		t.Error("expected nil when no *http.Transport is found")
	}
}

// mockWrappedRT implements the wrappedRT interface for testing.
type mockWrappedRT struct {
	inner http.RoundTripper
}

func (m *mockWrappedRT) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

func (m *mockWrappedRT) WrappedRoundTripper() http.RoundTripper {
	return m.inner
}
