package kube

import (
	"fmt"
	"net/http"
	"net/url"

	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	spdy "k8s.io/client-go/transport/spdy"
	httpstream "k8s.io/streaming/pkg/httpstream"
	streamspdy "k8s.io/streaming/pkg/httpstream/spdy"
)

// buildExecutor creates a remotecommand.Executor appropriate for the cluster type.
//
// For connector clusters: uses SPDY with UpgradeTransport injection. This is
// necessary because the standard NewSPDYExecutor/NewWebSocketExecutor create
// their own net.Dialer internally (ignoring config.Transport.DialContext),
// which fails to resolve the virtual "kubernetes-api" hostname. By manually
// creating the SPDY round tripper with UpgradeTransport set, the dialer from
// config.Dial is used via dialerFor() in the spdy roundtripper.
//
// For non-connector clusters: uses the standard WebSocket → SPDY fallback.
func buildExecutor(c *K8sClient, reqURL *url.URL) (remotecommand.Executor, error) {
	if c.IsConnector {
		return buildSPDYExecutorWithDialer(c.Configuration, reqURL)
	}
	return buildFallbackExecutor(c.Configuration, reqURL)
}

// buildFallbackExecutor creates a WebSocket → SPDY fallback executor for
// non-connector clusters (standard client-go behavior).
func buildFallbackExecutor(config *restclient.Config, reqURL *url.URL) (remotecommand.Executor, error) {
	spdyExec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, reqURL)
	if err != nil {
		return nil, fmt.Errorf("create SPDY executor: %w", err)
	}
	wsExec, err := remotecommand.NewWebSocketExecutor(config, http.MethodGet, reqURL.String())
	if err != nil {
		return nil, fmt.Errorf("create WebSocket executor: %w", err)
	}
	exec, err := remotecommand.NewFallbackExecutor(wsExec, spdyExec, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return nil, fmt.Errorf("create fallback executor: %w", err)
	}
	return exec, nil
}

// buildSPDYExecutorWithDialer creates a SPDY executor that uses the custom
// DialContext from the rest.Config. This is needed for connector clusters
// where the SPDY round tripper must route through the remotedialer tunnel
// instead of doing DNS resolution.
//
// The standard remotecommand.NewSPDYExecutor creates a SpdyRoundTripper with
// UpgradeTransport=nil, which falls back to net.Dialer and fails to resolve
// the virtual "kubernetes-api" hostname. By manually creating the round
// tripper with UpgradeTransport set to an *http.Transport that has DialContext,
// the SPDY dialer extracts and uses our custom dialer (see dialerFor() in
// k8s.io/streaming/pkg/httpstream/spdy/roundtripper.go).
func buildSPDYExecutorWithDialer(config *restclient.Config, reqURL *url.URL) (remotecommand.Executor, error) {
	// Build a transport from the rest.Config. rest.TransportFor creates an
	// *http.Transport with the custom DialContext from config.Dial.
	transport, err := restclient.TransportFor(config)
	if err != nil {
		return nil, fmt.Errorf("create transport for SPDY: %w", err)
	}

	// Extract the underlying *http.Transport to pass as UpgradeTransport.
	// The SPDY round tripper will use it via dialerFor() to get DialContext.
	httpTransport, ok := transport.(*http.Transport)
	if !ok {
		httpTransport = extractHTTPTransport(transport)
		if httpTransport == nil {
			return nil, fmt.Errorf("cannot extract *http.Transport for SPDY upgrade")
		}
	}

	// Create streaming SPDY round tripper with UpgradeTransport injected.
	// When UpgradeTransport != nil, TLS config is extracted from the
	// transport's TLSClientConfig (mutually exclusive with TLS field).
	// The SPDY dialer uses dialerFor() to extract transport.DialContext,
	// routing through our remotedialer tunnel.
	spdyRT, err := streamspdy.NewRoundTripperWithConfig(streamspdy.RoundTripperConfig{
		UpgradeTransport: httpTransport,
	})
	if err != nil {
		return nil, fmt.Errorf("create SPDY round tripper: %w", err)
	}

	// Wrap with auth round trippers (bearer token, etc.)
	wrapper, err := restclient.HTTPWrappersForConfig(config, spdyRT)
	if err != nil {
		return nil, fmt.Errorf("wrap SPDY transport with auth: %w", err)
	}

	// Adapt the streaming SPDY round tripper to client-go's Upgrader interface.
	upgrader := spdy.NewUpgraderForStreaming(spdyRT)

	return remotecommand.NewSPDYExecutorForTransports(wrapper, upgrader, http.MethodPost, reqURL)
}

// extractHTTPTransport attempts to find the underlying *http.Transport
// from a potentially wrapped RoundTripper by checking known wrapper types.
func extractHTTPTransport(rt http.RoundTripper) *http.Transport {
	type wrappedRT interface {
		WrappedRoundTripper() http.RoundTripper
	}
	for i := 0; i < 10; i++ {
		if t, ok := rt.(*http.Transport); ok {
			return t
		}
		if w, ok := rt.(wrappedRT); ok {
			rt = w.WrappedRoundTripper()
			continue
		}
		break
	}
	return nil
}
