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
	"time"

	"github.com/rancher/remotedialer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	"github.com/zxh326/kite/pkg/version"
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

	// Extract credentials from kubeconfig to send to the Server. The Server
	// needs these to perform TLS + auth directly, since the Agent is now a
	// pure TCP forwarder (no HTTP proxy layer).
	creds, err := extractCredentials(config)
	if err != nil {
		return fmt.Errorf("extract Kubernetes credentials: %w", err)
	}

	// Parse the real kube-apiserver address for TCP dialing.
	apiServerAddr, err := parseHostPort(config.Host)
	if err != nil {
		return fmt.Errorf("parse kube-apiserver address: %w", err)
	}

	// localDialer is called by remotedialer when the Server requests a new
	// tunnel connection to the Kubernetes API. It creates a raw TCP connection
	// to kube-apiserver — no HTTP parsing, no transport management, no auth
	// injection. TLS and auth are handled end-to-end by the Server's transport.
	localDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != KubernetesAPITarget {
			return nil, fmt.Errorf("unsupported tunnel target %s/%s", network, address)
		}
		var d net.Dialer
		return d.DialContext(ctx, "tcp", apiServerAddr)
	}

	authorizer := func(network, address string) bool {
		return network == "tcp" && address == KubernetesAPITarget
	}
	headers := http.Header{
		"Authorization":        []string{"Bearer " + *token},
		connectorVersionHeader: []string{version.Version},
	}
	if err := SetCredentialHeader(headers, creds); err != nil {
		return fmt.Errorf("set credentials header: %w", err)
	}

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
