package manager

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Regression test for the PATCH(kite) fix (upstream #3629 / #3636):
// Manager.Start must return after cancellation arrives before cache readiness.
func TestStartReturnsAfterCancelBeforeCacheReady(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)

	mgr, err := New(&rest.Config{Host: "http://127.0.0.1:1"}, Options{
		Scheme:         scheme,
		LeaderElection: false,
		BaseContext:    func() context.Context { return ctx },
		Logger:         logr.Discard(),
		Metrics:        metricsserver.Options{BindAddress: "0"},
		MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
			return mapper, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Start an informer against an unreachable API server; it can never sync.
	if _, err := mgr.GetCache().GetInformer(ctx, &corev1.ConfigMap{}, cache.BlockUntilSynced(false)); err != nil {
		t.Fatal(err)
	}

	cancel()

	done := make(chan error, 1)
	go func() { done <- mgr.Start(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Manager.Start did not return after cancellation")
	}
}
