package clusteragent

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rancher/remotedialer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

const registrationRefreshInterval = 10 * time.Minute

// tunnelConnected is set to 1 while the WebSocket tunnel to the Kite server is
// active, and 0 when it is disconnected. The readiness probe reads this to
// report whether the agent is ready to serve tunnelled requests.
var tunnelConnected atomic.Bool

func Run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("kite cluster-agent", flag.ContinueOnError)
	klog.InitFlags(flags)
	server := flags.String("server", "", "Kite server URL")
	token := flags.String("token", "", "Cluster Agent token")
	publicKey := flags.String("public-key", "", "Cluster Agent registration public key")
	kubeconfig := flags.String("kubeconfig", "", "Path to kubeconfig file")
	probeAddr := flags.String("probe-addr", ":8080", "HTTP probe listen address for liveness/readiness checks")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *server == "" {
		*server = os.Getenv("KITE_SERVER")
	}
	if *token == "" {
		*token = os.Getenv("CLUSTER_AGENT_TOKEN")
	}
	if *publicKey == "" {
		*publicKey = os.Getenv("CLUSTER_AGENT_PUBLIC_KEY")
	}
	if *server == "" {
		return errors.New("--server is required")
	}
	if *token == "" {
		return errors.New("--token is required")
	}
	if *publicKey == "" {
		return errors.New("--public-key is required")
	}

	serverURL, err := url.Parse(*server)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	if serverURL.Fragment != "" {
		return errors.New("server URL must not contain a fragment")
	}
	if serverURL.Scheme != "http" && serverURL.Scheme != "https" {
		return errors.New("server URL must use http or https")
	}
	if serverURL.Host == "" {
		return errors.New("invalid cluster agent server URL")
	}

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

	registrationURL := *serverURL
	registrationURL.Path = strings.TrimRight(registrationURL.Path, "/") + "/api/v1/cluster-agent/register"
	registrationURL.RawPath = ""
	registrationURL.RawQuery = ""
	connectURL := *serverURL
	if connectURL.Scheme == "http" {
		connectURL.Scheme = "ws"
	} else {
		connectURL.Scheme = "wss"
	}
	connectURL.Path = strings.TrimRight(connectURL.Path, "/") + "/api/v1/cluster-agent/connect"
	connectURL.RawPath = ""
	connectURL.RawQuery = ""

	// Start the HTTP probe server for Kubernetes liveness/readiness probes.
	// The liveness probe always returns 200 while the process is running.
	// The readiness probe returns 200 only when the WebSocket tunnel is
	// connected, so Kubernetes stops routing traffic (and eventually restarts
	// the pod via the liveness probe) if the tunnel is silently dropped.
	go startProbeServer(ctx, *probeAddr)

	client := &http.Client{Timeout: 30 * time.Second}
	go func() {
		ticker := time.NewTicker(registrationRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := registerClusterAgent(ctx, client, registrationURL.String(), *token, *publicKey, config); err != nil {
					klog.Warningf("Failed to refresh cluster agent registration: %v", err)
				}
			}
		}
	}()

	headers := http.Header{"Authorization": []string{"Bearer " + *token}}
	authorizer := func(network, _ string) bool {
		return network == "tcp"
	}

	klog.Info("Cluster Agent started")
	for {
		err := registerClusterAgent(ctx, client, registrationURL.String(), *token, *publicKey, config)
		if err == nil {
			tunnelConnected.Store(true)
			err = remotedialer.ConnectToProxy(ctx, connectURL.String(), headers, authorizer, nil, nil)
			tunnelConnected.Store(false)
		}
		if ctx.Err() != nil {
			return nil //nolint:nilerr // Context cancellation is a clean shutdown.
		}
		klog.Warningf("Cluster Agent unavailable: %v; retrying in 5 seconds", err)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}
}

// startProbeServer runs an HTTP server with /healthz and /readyz endpoints.
// /healthz always returns 200 — the process is alive.
// /readyz returns 200 only when the tunnel is connected — the agent can serve.
func startProbeServer(ctx context.Context, addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if tunnelConnected.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	})
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("Probe server error: %v", err)
	}
}
