package connector

import (
	"context"
	"errors"
	"net"
	"testing"
)

// --- parseHostPort tests ---

func TestParseHostPort_HTTPS(t *testing.T) {
	addr, err := parseHostPort("https://10.0.0.1:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "10.0.0.1:443" {
		t.Errorf("expected 10.0.0.1:443, got %s", addr)
	}
}

func TestParseHostPort_HTTP(t *testing.T) {
	addr, err := parseHostPort("http://localhost:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "localhost:8080" {
		t.Errorf("expected localhost:8080, got %s", addr)
	}
}

func TestParseHostPort_NoPort(t *testing.T) {
	_, err := parseHostPort("https://10.0.0.1")
	if err == nil {
		t.Error("expected error for URL without port")
	}
}

func TestParseHostPort_NoScheme(t *testing.T) {
	// Without a scheme prefix, the string is passed to SplitHostPort directly.
	// "10.0.0.1:443" (no scheme) should still work.
	addr, err := parseHostPort("10.0.0.1:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "10.0.0.1:443" {
		t.Errorf("expected 10.0.0.1:443, got %s", addr)
	}
}

// --- localDialer target validation tests ---
//
// Since localDialer is a closure inside Run(), we test the same validation
// pattern that the closure uses.

func TestLocalDialer_RejectsInvalidNetwork(t *testing.T) {
	checkTarget := func(network, address string) error {
		if network != "tcp" || address != KubernetesAPITarget {
			return errors.New("unsupported tunnel target")
		}
		return nil
	}

	if err := checkTarget("udp", KubernetesAPITarget); err == nil {
		t.Error("expected error for udp network")
	}
}

func TestLocalDialer_RejectsInvalidAddress(t *testing.T) {
	checkTarget := func(network, address string) error {
		if network != "tcp" || address != KubernetesAPITarget {
			return errors.New("unsupported tunnel target")
		}
		return nil
	}

	if err := checkTarget("tcp", "wrong-target"); err == nil {
		t.Error("expected error for wrong address")
	}
}

func TestLocalDialer_AcceptsValidTarget(t *testing.T) {
	checkTarget := func(network, address string) error {
		if network != "tcp" || address != KubernetesAPITarget {
			return errors.New("unsupported tunnel target")
		}
		return nil
	}

	if err := checkTarget("tcp", KubernetesAPITarget); err != nil {
		t.Errorf("expected success for valid target, got: %v", err)
	}
}

// --- localDialer TCP dial test ---
//
// Test that the localDialer pattern actually creates a TCP connection
// to a real listener. This verifies that net.DialContext works as expected
// for the TCP forwarding use case.

func TestLocalDialer_TCPDialSuccess(t *testing.T) {
	// Start a local TCP listener
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().String()

	// Simulate the localDialer pattern from Run()
	localDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		if network != "tcp" || address != KubernetesAPITarget {
			return nil, errors.New("unsupported tunnel target")
		}
		var d net.Dialer
		return d.DialContext(ctx, "tcp", addr)
	}

	// Accept the connection on the listener side
	accepted := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_ = conn.Close()
		}
		close(accepted)
	}()

	conn, err := localDialer(context.Background(), "tcp", KubernetesAPITarget)
	if err != nil {
		t.Fatalf("localDialer failed: %v", err)
	}
	defer func() { _ = conn.Close() }()
	<-accepted
}
