package cluster

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
)

func TestExtractResourceName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// Core API (api/v1/...)
		{"/api/v1/namespaces/default/pods/my-pod", "my-pod"},
		{"/api/v1/namespaces/default/pods", ""},
		{"/api/v1/namespaces/default/services/my-svc", "my-svc"},
		{"/api/v1/namespaces/kube-system/configmaps/my-cm", "my-cm"},
		{"/api/v1/pods", ""},
		{"/api/v1/namespaces", ""},

		// Named groups (apis/{group}/{version}/...)
		{"/apis/apps/v1/namespaces/default/deployments/my-deploy", "my-deploy"},
		{"/apis/apps/v1/namespaces/default/deployments", ""},
		{"/apis/rbac.authorization.k8s.io/v1/clusterroles/my-cr", "my-cr"},
		{"/apis/rbac.authorization.k8s.io/v1/clusterroles", ""},

		// With subresources — name is still the resource instance name
		{"/api/v1/namespaces/default/pods/my-pod/exec", "my-pod"},
		{"/api/v1/namespaces/default/pods/my-pod/attach", "my-pod"},
		{"/api/v1/namespaces/default/pods/my-pod/log", "my-pod"},
		{"/api/v1/namespaces/default/pods/my-pod/portforward", "my-pod"},

		// Trailing slash
		{"/api/v1/namespaces/default/pods/my-pod/", "my-pod"},

		// Invalid paths
		{"", ""},
		{"/healthz", ""},
		{"/api", ""},
		{"/apis", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := extractResourceName(tt.path)
			if got != tt.want {
				t.Errorf("extractResourceName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCollectProxyAuditInfo(t *testing.T) {
	user := model.User{Model: model.Model{ID: 1}, Username: "alice"}

	tests := []struct {
		name       string
		method     string
		path       string
		wantNil    bool
		wantVerb   string
		wantRes    string
		wantNs     string
		wantName   string
	}{
		{
			name:     "POST create pod",
			method:   http.MethodPost,
			path:     "/api/v1/namespaces/default/pods",
			wantNil:  false,
			wantVerb: string(common.VerbCreate),
			wantRes:  "pods",
			wantNs:   "default",
			wantName: "",
		},
		{
			name:     "DELETE pod by name",
			method:   http.MethodDelete,
			path:     "/api/v1/namespaces/default/pods/my-pod",
			wantNil:  false,
			wantVerb: string(common.VerbDelete),
			wantRes:  "pods",
			wantNs:   "default",
			wantName: "my-pod",
		},
		{
			name:     "PUT update deployment",
			method:   http.MethodPut,
			path:     "/apis/apps/v1/namespaces/default/deployments/my-deploy",
			wantNil:  false,
			wantVerb: string(common.VerbUpdate),
			wantRes:  "deployments",
			wantNs:   "default",
			wantName: "my-deploy",
		},
		{
			name:     "PATCH pod",
			method:   http.MethodPatch,
			path:     "/api/v1/namespaces/default/pods/my-pod",
			wantNil:  false,
			wantVerb: string(common.VerbUpdate),
			wantRes:  "pods",
			wantNs:   "default",
			wantName: "my-pod",
		},
		{
			name:    "GET pod — not audited",
			method:  http.MethodGet,
			path:    "/api/v1/namespaces/default/pods/my-pod",
			wantNil: true,
		},
		{
			name:    "discovery path — not audited",
			method:  http.MethodGet,
			path:    "/api/v1",
			wantNil: true,
		},
		{
			name:    "GET discovery apis — not audited",
			method:  http.MethodGet,
			path:    "/apis/apps/v1",
			wantNil: true,
		},
		{
			name:     "POST exec is mapped to exec verb",
			method:   http.MethodPost,
			path:     "/api/v1/namespaces/default/pods/my-pod/exec",
			wantNil:  false,
			wantVerb: string(common.VerbExec),
			wantRes:  "pods",
			wantNs:   "default",
			wantName: "my-pod",
		},
		{
			name:     "POST cluster-scoped resource (clusterroles)",
			method:   http.MethodPost,
			path:     "/apis/rbac.authorization.k8s.io/v1/clusterroles",
			wantNil:  false,
			wantVerb: string(common.VerbCreate),
			wantRes:  "clusterroles",
			wantNs:   common.AllNamespaces,
			wantName: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tt.method, tt.path, nil)

			cm := &ClusterManager{}
			info := cm.collectProxyAuditInfo(c, user, "test-cluster", tt.path)

			if tt.wantNil {
				if info != nil {
					t.Fatalf("expected nil, got %+v", info)
				}
				return
			}
			if info == nil {
				t.Fatalf("expected non-nil audit info")
			}
			if info.verb != tt.wantVerb {
				t.Errorf("verb = %q, want %q", info.verb, tt.wantVerb)
			}
			if info.resource != tt.wantRes {
				t.Errorf("resource = %q, want %q", info.resource, tt.wantRes)
			}
			if info.namespace != tt.wantNs {
				t.Errorf("namespace = %q, want %q", info.namespace, tt.wantNs)
			}
			if info.name != tt.wantName {
				t.Errorf("name = %q, want %q", info.name, tt.wantName)
			}
			if info.user.ID != user.ID {
				t.Errorf("user.ID = %d, want %d", info.user.ID, user.ID)
			}
			if info.clusterName != "test-cluster" {
				t.Errorf("clusterName = %q, want %q", info.clusterName, "test-cluster")
			}
		})
	}
}
