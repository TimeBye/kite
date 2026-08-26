package resources

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zxh326/kite/pkg/cluster"
	"github.com/zxh326/kite/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestEnsureHelmReleaseAutoUpgradeTargetDisabled(t *testing.T) {
	h := &HelmReleaseHandler{}
	code, err := h.ensureHelmReleaseAutoUpgradeTarget(nil, "default", "my-release", helmReleaseAutoUpgradeRequest{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != http.StatusOK {
		t.Fatalf("code = %d, want %d", code, http.StatusOK)
	}
}

func TestEnsureHelmReleaseAutoUpgradeTargetNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/version":
			_, _ = w.Write([]byte(`{"major":"1","minor":"35","gitVersion":"v1.35.0"}`))
		case strings.HasSuffix(r.URL.Path, "/secrets"):
			_ = json.NewEncoder(w).Encode(corev1.SecretList{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "SecretList"},
				Items:    []corev1.Secret{},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	cs := &cluster.ClientSet{
		Name: "test-cluster",
		K8sClient: &kube.K8sClient{
			Configuration: &rest.Config{Host: server.URL},
		},
	}

	h := &HelmReleaseHandler{}
	code, err := h.ensureHelmReleaseAutoUpgradeTarget(cs, "default", "missing-release", helmReleaseAutoUpgradeRequest{Enabled: true})
	if err == nil {
		t.Fatal("expected error for missing release, got nil")
	}
	if code != http.StatusNotFound {
		t.Fatalf("code = %d, want %d; err=%v", code, http.StatusNotFound, err)
	}
}
