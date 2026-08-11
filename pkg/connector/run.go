package connector

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rancher/remotedialer"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func Run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("kite connector", flag.ContinueOnError)
	klog.InitFlags(flags)
	server := flags.String("server", "", "Kite server URL")
	token := flags.String("token", "", "Kite connector token")
	kubeconfig := flags.String("kubeconfig", "", "Path to kubeconfig file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		return errors.New("--server is required")
	}
	if *token == "" {
		return errors.New("--token is required")
	}

	serverURL, err := url.Parse(*server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if serverURL.Fragment != "" {
		return errors.New("server URL must not contain a fragment")
	}
	switch serverURL.Scheme {
	case "http":
		serverURL.Scheme = "ws"
	case "https":
		serverURL.Scheme = "wss"
	default:
		return errors.New("server URL must use http or https")
	}
	if serverURL.Host == "" {
		return errors.New("invalid connector server URL")
	}
	serverURL.Path = strings.TrimRight(serverURL.Path, "/") + "/api/v1/connector/connect"
	serverURL.RawPath = ""

	var config *rest.Config
	if *kubeconfig == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("load in-cluster Kubernetes configuration: %w", err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			return fmt.Errorf("load kubeconfig: %w", err)
		}
	}
	target, err := url.Parse(config.Host)
	if err != nil {
		return fmt.Errorf("parse Kubernetes API URL: %w", err)
	}
	target.Path = "/"

	// Regular transport for non-upgrade requests. Allows HTTP/2 for better
	// multiplexing on regular API calls (List, Get, Watch, etc.).
	regularTransport, err := rest.TransportFor(config)
	if err != nil {
		return fmt.Errorf("create transport: %w", err)
	}

	// HTTP/1.1-only transport for upgrade requests. HTTP/2 does not allow
	// Upgrade headers, so SPDY/WebSocket upgrade requests (kubectl exec,
	// attach, port-forward) must use HTTP/1.1.
	upgradeConfig := rest.CopyConfig(config)
	upgradeConfig.NextProtos = []string{"http/1.1"}
	upgradeTransport, err := rest.TransportFor(upgradeConfig)
	if err != nil {
		return fmt.Errorf("create upgrade transport: %w", err)
	}

	// authTransport captures auth headers (bearer token, exec provider, etc.)
	// from the kubeconfig via MirrorRequest without actually sending a
	// request. The captured headers are injected into upgrade requests by
	// UpgradeAwareHandler.WrapRequest.
	authConfig := rest.CopyConfig(config)
	authConfig.WrapTransport = func(http.RoundTripper) http.RoundTripper {
		return proxy.MirrorRequest
	}
	authTransport, err := rest.TransportFor(authConfig)
	if err != nil {
		return fmt.Errorf("create auth transport: %w", err)
	}

	// Create a single UpgradeAwareHandler that all tunnel connections share.
	// The regular transport handles non-upgrade requests (HTTP/2 allowed),
	// while the upgrade transport handles SPDY/WebSocket upgrades (HTTP/1.1).
	handler := proxy.NewUpgradeAwareHandler(target, regularTransport, false, false, &agentResponder{})
	handler.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(upgradeTransport, authTransport)
	handler.UseRequestLocation = true
	handler.FlushInterval = 200 * time.Millisecond

	// localDialer is called by remotedialer when the server requests a new
	// tunnel connection to the K8s API. It creates a net.Pipe and serves
	// HTTP on one end through the UpgradeAwareHandler. This properly handles
	// SPDY/WebSocket upgrades (including Content-Length: 0 on POST) by using
	// Go's standard HTTP server and UpgradeAwareHandler, instead of raw byte
	// forwarding which could break SPDY upgrade requests.
	localDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != KubernetesAPITarget {
			return nil, fmt.Errorf("unsupported tunnel target %s/%s", network, address)
		}
		serverConn, clientConn := net.Pipe()
		go func() {
			listener := &singleConnListener{conn: serverConn}
			server := &http.Server{
				Handler:           handler,
				ReadHeaderTimeout: 30 * time.Second,
			}
			_ = server.Serve(listener)
		}()
		return clientConn, nil
	}

	authorizer := func(network, address string) bool {
		return network == "tcp" && address == KubernetesAPITarget
	}
	headers := http.Header{"Authorization": []string{"Bearer " + *token}}

	klog.Info("Kite connector started")
	retryCount := 0
	for {
		err := remotedialer.ConnectToProxyWithDialer(ctx, serverURL.String(), headers, authorizer, nil, localDialer, nil)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // Context cancellation is a clean shutdown.
		}
		retryCount++
		klog.Warningf("Kite connector connection lost (retry %d): %v", retryCount, err)
		if retryCount >= 10 {
			return fmt.Errorf("kite connector failed to connect after %d retries: %w", retryCount, err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

// singleConnListener is a net.Listener that returns one connection on the
// first Accept call and an error on subsequent calls. This allows serving
// a single HTTP connection via http.Server.Serve for each tunnel connection.
type singleConnListener struct {
	conn net.Conn
	once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	accepted := false
	l.once.Do(func() {
		accepted = true
	})
	if !accepted {
		return nil, errors.New("single-connection listener exhausted")
	}
	return l.conn, nil
}

func (l *singleConnListener) Close() error { return nil }

func (l *singleConnListener) Addr() net.Addr { return l.conn.LocalAddr() }

// agentResponder implements proxy.ErrorResponder for the agent-side
// UpgradeAwareHandler.
type agentResponder struct{}

func (r *agentResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	klog.Errorf("agent proxy error: method=%s, path=%s, err=%v", req.Method, req.URL.Path, err)
	http.Error(w, fmt.Sprintf("proxy error: %s", err), http.StatusBadGateway)
}
