package cluster

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupProxyTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Cluster{}, &model.User{}, &model.Role{}, &model.RoleAssignment{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	model.DB = db
}

func TestParseK8sAPIPath(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		method      string
		resource    string
		namespace   string
		verb        string
		subResource string
	}{
		{
			name:      "core list pods",
			path:      "/api/v1/namespaces/default/pods",
			method:    "GET",
			resource:  "pods",
			namespace: "default",
			verb:      "get",
		},
		{
			name:      "core get single pod",
			path:      "/api/v1/namespaces/default/pods/my-pod",
			method:    "GET",
			resource:  "pods",
			namespace: "default",
			verb:      "get",
		},
		{
			name:        "core exec pod",
			path:        "/api/v1/namespaces/default/pods/my-pod/exec",
			method:      "POST",
			resource:    "pods",
			namespace:   "default",
			verb:        "create",
			subResource: "exec",
		},
		{
			name:        "core attach pod",
			path:        "/api/v1/namespaces/default/pods/my-pod/attach",
			method:      "POST",
			resource:    "pods",
			namespace:   "default",
			verb:        "create",
			subResource: "attach",
		},
		{
			name:        "core log pod",
			path:        "/api/v1/namespaces/default/pods/my-pod/log",
			method:      "GET",
			resource:    "pods",
			namespace:   "default",
			verb:        "get",
			subResource: "log",
		},
		{
			name:        "core portforward pod",
			path:        "/api/v1/namespaces/default/pods/my-pod/portforward",
			method:      "POST",
			resource:    "pods",
			namespace:   "default",
			verb:        "create",
			subResource: "portforward",
		},
		{
			name:      "apps list deployments",
			path:      "/apis/apps/v1/namespaces/default/deployments",
			method:    "GET",
			resource:  "deployments",
			namespace: "default",
			verb:      "get",
		},
		{
			name:      "apps create deployment",
			path:      "/apis/apps/v1/namespaces/default/deployments",
			method:    "POST",
			resource:  "deployments",
			namespace: "default",
			verb:      "create",
		},
		{
			name:      "apps update deployment",
			path:      "/apis/apps/v1/namespaces/default/deployments/my-deploy",
			method:    "PUT",
			resource:  "deployments",
			namespace: "default",
			verb:      "update",
		},
		{
			name:      "apps patch deployment",
			path:      "/apis/apps/v1/namespaces/default/deployments/my-deploy",
			method:    "PATCH",
			resource:  "deployments",
			namespace: "default",
			verb:      "update",
		},
		{
			name:      "apps delete deployment",
			path:      "/apis/apps/v1/namespaces/default/deployments/my-deploy",
			method:    "DELETE",
			resource:  "deployments",
			namespace: "default",
			verb:      "delete",
		},
		{
			name:      "cluster-scoped nodes",
			path:      "/api/v1/nodes",
			method:    "GET",
			resource:  "nodes",
			namespace: "",
			verb:      "get",
		},
		{
			name:      "cluster-scoped single node",
			path:      "/api/v1/nodes/my-node",
			method:    "GET",
			resource:  "nodes",
			namespace: "",
			verb:      "get",
		},
		{
			name:      "cluster-scoped namespaces list",
			path:      "/api/v1/namespaces",
			method:    "GET",
			resource:  "namespaces",
			namespace: "",
			verb:      "get",
		},
		{
			name:   "discovery api v1",
			path:   "/api/v1",
			method: "GET",
			verb:   "get",
		},
		{
			name:   "discovery apis group",
			path:   "/apis/apps/v1",
			method: "GET",
			verb:   "get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource, namespace, verb, subResource := parseK8sAPIPath(tt.path, tt.method)
			if resource != tt.resource {
				t.Errorf("resource = %q, want %q", resource, tt.resource)
			}
			if namespace != tt.namespace {
				t.Errorf("namespace = %q, want %q", namespace, tt.namespace)
			}
			if verb != tt.verb {
				t.Errorf("verb = %q, want %q", verb, tt.verb)
			}
			if subResource != tt.subResource {
				t.Errorf("subResource = %q, want %q", subResource, tt.subResource)
			}
		})
	}
}

func TestIsDiscoveryPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api", true},
		{"/api/", true},
		{"/api/v1", true},
		{"/apis", true},
		{"/apis/apps/v1", true},
		{"/version", true},
		{"/openapi/v2", true},
		{"/openapi", true},
		{"/api/v1/namespaces/default/pods", false},
		{"/apis/apps/v1/namespaces/default/deployments", false},
		{"/api/v1/nodes", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isDiscoveryPath(tt.path); got != tt.want {
				t.Errorf("isDiscoveryPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestSanitizeClusterName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with space", "with-space"},
		{"with/slash", "with-slash"},
		{"with:colon", "with-colon"},
		{"中文集群", "中文集群"},
		{`a\b:c*d?e"f<g>h|i`, "a-b-c-d-e-f-g-h-i"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitizeClusterName(tt.input); got != tt.want {
				t.Errorf("sanitizeClusterName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMethodToVerb(t *testing.T) {
	tests := []struct {
		method string
		want   string
	}{
		{"GET", "get"},
		{"POST", "create"},
		{"PUT", "update"},
		{"PATCH", "update"},
		{"DELETE", "delete"},
		{"HEAD", "get"},
		{"OPTIONS", "get"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := methodToVerb(tt.method); got != tt.want {
				t.Errorf("methodToVerb(%q) = %q, want %q", tt.method, got, tt.want)
			}
		})
	}
}

func TestGenerateKubeconfig(t *testing.T) {
	setupProxyTestDB(t)

	role := &model.Role{
		Name:       "test-role",
		Clusters:   model.SliceString{"*"},
		Resources:  model.SliceString{"*"},
		Namespaces: model.SliceString{"*"},
		Verbs:      model.SliceString{"*"},
	}
	if err := model.DB.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}

	if err := model.AddRoleAssignment("test-role", model.SubjectTypeUser, "testuser"); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	cluster := &model.Cluster{
		Name:   "test-cluster",
		Config: "fake-config",
		Enable: true,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	user := model.User{
		Model:    model.Model{ID: 1},
		Username: "testuser",
		Roles: []common.Role{{
			Name:     "test-role",
			Clusters: []string{"*"},
		}},
	}

	yaml, err := GenerateKubeconfig(user, []string{cluster.UUID}, "https://kite.example.com")
	if err != nil {
		t.Fatalf("GenerateKubeconfig failed: %v", err)
	}

	if yaml == "" {
		t.Fatal("expected non-empty kubeconfig")
	}

	// The proxy URL should point to Kite's k8s-proxy endpoint.
	wantURL := "https://kite.example.com/api/v1/clusters/" + cluster.UUID + "/k8s-proxy"
	if !strings.Contains(yaml, wantURL) {
		t.Errorf("kubeconfig does not contain proxy URL %q", wantURL)
	}

	// insecure-skip-tls-verify must be set.
	if !strings.Contains(yaml, "insecure-skip-tls-verify: true") {
		t.Error("kubeconfig does not contain insecure-skip-tls-verify")
	}

	// The token should be a kite API key (starts with "kite").
	if !strings.Contains(yaml, "token: kite") {
		t.Error("kubeconfig does not contain a kite API key token")
	}

	// An API key user should have been created.
	apiKeys, err := model.ListAPIKeyUsers()
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(apiKeys) != 1 {
		t.Fatalf("expected 1 API key user, got %d", len(apiKeys))
	}
}

func TestGenerateKubeconfigMultiCluster(t *testing.T) {
	setupProxyTestDB(t)

	role := &model.Role{
		Name:       "test-role",
		Clusters:   model.SliceString{"*"},
		Resources:  model.SliceString{"*"},
		Namespaces: model.SliceString{"*"},
		Verbs:      model.SliceString{"*"},
	}
	if err := model.DB.Create(role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := model.AddRoleAssignment("test-role", model.SubjectTypeUser, "testuser"); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	cluster1 := &model.Cluster{Name: "cluster-1", Config: "cfg1", Enable: true}
	cluster2 := &model.Cluster{Name: "cluster-2", Config: "cfg2", Enable: true}
	if err := model.AddCluster(cluster1); err != nil {
		t.Fatalf("create cluster1: %v", err)
	}
	if err := model.AddCluster(cluster2); err != nil {
		t.Fatalf("create cluster2: %v", err)
	}

	user := model.User{
		Model:    model.Model{ID: 1},
		Username: "testuser",
		Roles: []common.Role{{
			Name:     "test-role",
			Clusters: []string{"*"},
		}},
	}

	yaml, err := GenerateKubeconfig(user, []string{cluster1.UUID, cluster2.UUID}, "https://kite.example.com")
	if err != nil {
		t.Fatalf("GenerateKubeconfig failed: %v", err)
	}

	// Both clusters should appear in the kubeconfig.
	if !strings.Contains(yaml, "cluster-1") {
		t.Error("kubeconfig does not contain cluster-1")
	}
	if !strings.Contains(yaml, "cluster-2") {
		t.Error("kubeconfig does not contain cluster-2")
	}

	// Current context should be the first cluster.
	if !strings.Contains(yaml, "current-context: cluster-1") {
		t.Error("current-context should be cluster-1")
	}
}

func TestGenerateKubeconfigAccessDenied(t *testing.T) {
	setupProxyTestDB(t)

	cluster := &model.Cluster{
		Name:   "denied-cluster",
		Config: "fake-config",
		Enable: true,
	}
	if err := model.AddCluster(cluster); err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	user := model.User{
		Model:    model.Model{ID: 1},
		Username: "testuser",
		// No roles — should be denied
	}

	_, err := GenerateKubeconfig(user, []string{cluster.UUID}, "https://kite.example.com")
	if err == nil {
		t.Fatal("expected error for access denied, got nil")
	}
}

func TestGenerateKubeconfigClusterNotFound(t *testing.T) {
	setupProxyTestDB(t)

	user := model.User{
		Model:    model.Model{ID: 1},
		Username: "testuser",
		Roles: []common.Role{{
			Name:     "test-role",
			Clusters: []string{"*"},
		}},
	}

	_, err := GenerateKubeconfig(user, []string{"nonexistent-uuid"}, "https://kite.example.com")
	if err == nil {
		t.Fatal("expected error for nonexistent cluster, got nil")
	}
}

func TestBuildK8sProxyURL(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		path     string
		wantPath string
	}{
		{
			name:     "standard https",
			host:     "https://k8s.example.com",
			path:     "/api/v1/namespaces/default/pods",
			wantPath: "/api/v1/namespaces/default/pods",
		},
		{
			name:     "with port",
			host:     "https://k8s.example.com:6443",
			path:     "/apis/apps/v1/deployments",
			wantPath: "/apis/apps/v1/deployments",
		},
		{
			name:     "connector http",
			host:     "http://127.0.0.1:12345",
			path:     "/api/v1",
			wantPath: "/api/v1",
		},
		{
			name:     "root path",
			host:     "https://k8s.example.com",
			path:     "/version",
			wantPath: "/version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := buildK8sProxyURL(tt.host, tt.path)
			if err != nil {
				t.Fatalf("buildK8sProxyURL error: %v", err)
			}
			if u.Path != tt.wantPath {
				t.Errorf("path = %q, want %q", u.Path, tt.wantPath)
			}
		})
	}
}
