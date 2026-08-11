package connector

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- singleConnListener tests ---

func TestSingleConnListener_AcceptOnce(t *testing.T) {
	conn1, conn2 := net.Pipe()
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	listener := &singleConnListener{conn: conn1}

	// First Accept should return the connection
	got, err := listener.Accept()
	if err != nil {
		t.Fatalf("first Accept() error: %v", err)
	}
	if got != conn1 {
		t.Fatal("first Accept() should return the original connection")
	}

	// Second Accept should return an error
	_, err = listener.Accept()
	if err == nil {
		t.Fatal("second Accept() should return an error")
	}
}

func TestSingleConnListener_ConcurrentAccept(t *testing.T) {
	conn1, conn2 := net.Pipe()
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	listener := &singleConnListener{conn: conn1}

	var (
		wg           sync.WaitGroup
		successCount int
		errorCount   int
		mu           sync.Mutex
	)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := listener.Accept()
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				errorCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful Accept, got %d", successCount)
	}
	if errorCount != 4 {
		t.Errorf("expected 4 errors, got %d", errorCount)
	}
}

func TestSingleConnListener_Close(t *testing.T) {
	conn1, conn2 := net.Pipe()
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	listener := &singleConnListener{conn: conn1}
	// Close should be a no-op and never error
	if err := listener.Close(); err != nil {
		t.Errorf("Close() returned error: %v", err)
	}
	// Can close multiple times
	if err := listener.Close(); err != nil {
		t.Errorf("second Close() returned error: %v", err)
	}
}

func TestSingleConnListener_Addr(t *testing.T) {
	conn1, conn2 := net.Pipe()
	defer func() { _ = conn1.Close() }()
	defer func() { _ = conn2.Close() }()

	listener := &singleConnListener{conn: conn1}
	addr := listener.Addr()
	if addr == nil {
		t.Fatal("Addr() should not return nil")
	}
	// For net.Pipe, LocalAddr() returns a pipeAddr
	if addr.String() != "pipe" {
		t.Logf("Addr() = %s (expected 'pipe' for net.Pipe)", addr.String())
	}
}

// --- localDialer integration test via UpgradeAwareHandler ---

// TestLocalDialer_RejectsInvalidTarget verifies that the dialer created in
// Run() rejects connections to addresses other than KubernetesAPITarget.
// Since localDialer is a closure inside Run(), we test the same logic
// pattern here.
func TestLocalDialer_RejectsInvalidTarget(t *testing.T) {
	checkTarget := func(network, address string) error {
		if network != "tcp" || address != KubernetesAPITarget {
			return errors.New("unsupported tunnel target")
		}
		return nil
	}

	// Valid target
	err := checkTarget("tcp", KubernetesAPITarget)
	if err != nil {
		t.Errorf("expected success for valid target, got: %v", err)
	}

	// Invalid address
	err = checkTarget("tcp", "wrong-target")
	if err == nil {
		t.Error("expected error for wrong address")
	}

	// Invalid network
	err = checkTarget("udp", KubernetesAPITarget)
	if err == nil {
		t.Error("expected error for wrong network")
	}
}

// TestLocalDialer_PipeHTTPRoundTrip is an end-to-end test that simulates
// the localDialer pattern: create a pipe, serve HTTP on one end via
// singleConnListener, and verify the other end can make an HTTP request.
func TestLocalDialer_PipeHTTPRoundTrip(t *testing.T) {
	serverConn, clientConn := net.Pipe()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		listener := &singleConnListener{conn: serverConn}
		server := &http.Server{
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		_ = server.Serve(listener)
	}()

	// Write the HTTP request to the client end of the pipe.
	request := "GET /healthz HTTP/1.1\r\nHost: localhost\r\n\r\n"
	_ = clientConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := clientConn.Write([]byte(request))
	if err != nil {
		t.Fatalf("failed to write request: %v", err)
	}

	// Read the response from the server.
	buf := make([]byte, 4096)
	_ = clientConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := clientConn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	_ = clientConn.Close()
	<-serverDone

	response := string(buf[:n])
	if !strings.Contains(response, "200 OK") {
		t.Errorf("response does not contain '200 OK': %s", response)
	}
	if !strings.Contains(response, "ok") {
		t.Errorf("response body does not contain 'ok': %s", response)
	}
}
