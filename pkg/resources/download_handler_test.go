package resources

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zxh326/kite/pkg/cluster"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// setupDownloadTestRouter creates a Gin router with download routes registered
// for the given resource type, mimicking the production route registration.
func setupDownloadTestRouter(t *testing.T, cs *cluster.ClientSet, resourceType string, clusterScoped bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Inject cluster into context before route handlers
	router.Use(func(c *gin.Context) {
		c.Set("cluster", cs)
		c.Next()
	})

	// Register routes mimicking registerClusterScopeRoutes/registerNamespaceScopeRoutes
	if clusterScoped {
		router.GET("/"+resourceType+"/_all/:name/download", setResourceType(resourceType), DownloadSingle)
		router.POST("/"+resourceType+"/_all/download", setResourceType(resourceType), DownloadBatch)
	} else {
		router.GET("/"+resourceType+"/:namespace/:name/download", setResourceType(resourceType), DownloadSingle)
		router.POST("/"+resourceType+"/download", setResourceType(resourceType), DownloadBatch)
	}

	return router
}

// setupCRDDownloadTestRouter creates a Gin router with CRD-style download routes
// that use :crd path parameter instead of setResourceType middleware.
func setupCRDDownloadTestRouter(t *testing.T, cs *cluster.ClientSet) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("cluster", cs)
		c.Next()
	})
	router.GET("/:crd/:namespace/:name/download", DownloadSingle)
	router.POST("/:crd/download", DownloadBatch)
	router.GET("/:crd/_all/:name/download", DownloadSingle)
	router.POST("/:crd/_all/download", DownloadBatch)
	return router
}

func newDownloadTestClientSet(t *testing.T, objs ...runtime.Object) *cluster.ClientSet {
	t.Helper()
	scheme := kube.GetScheme()
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &cluster.ClientSet{
		Name:      "test",
		K8sClient: &kube.K8sClient{Client: k8sClient},
	}
}

func TestDownloadSingle_NamespaceScopedResource(t *testing.T) {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nginx",
			Namespace:       "default",
			UID:             "pod-uid",
			ResourceVersion: "123",
			Annotations:     map[string]string{"kubectl.kubernetes.io/last-applied-configuration": "{}"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "nginx", Image: "nginx:1.27"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	cs := newDownloadTestClientSet(t, pod)

	// Initialize handlers map so GetResource can find the pod handler
	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "pods", false)

	t.Run("raw mode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/pods/default/nginx/download?neat=false", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "Pod-default-nginx.yaml")

		body := rec.Body.String()
		// Raw mode: managedFields and kubectl annotation removed
		assert.NotContains(t, body, "managedFields")
		assert.NotContains(t, body, "kubectl.kubernetes.io/last-applied-configuration")
		// Raw mode: system fields preserved
		assert.Contains(t, body, "resourceVersion")
		// Raw mode: status preserved
		assert.Contains(t, body, "status:")
	})

	t.Run("neat mode", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/pods/default/nginx/download?neat=true", nil)
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		// Neat mode: system fields removed
		assert.NotContains(t, body, "resourceVersion")
		assert.NotContains(t, body, "creationTimestamp")
		// Neat mode: status removed
		assert.NotContains(t, body, "status:")
		// Non-system fields preserved
		assert.Contains(t, body, "nginx:1.27")
	})
}

func TestDownloadSingle_ClusterScopedResource(t *testing.T) {
	node := &corev1.Node{
		TypeMeta: metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "node-1",
			UID:             "node-uid",
			ResourceVersion: "999",
		},
		Status: corev1.NodeStatus{},
	}

	cs := newDownloadTestClientSet(t, node)

	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "nodes", true)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nodes/_all/node-1/download?neat=true", nil)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "Node-node-1.yaml")

	body := rec.Body.String()
	assert.NotContains(t, body, "status:")
	assert.NotContains(t, body, "resourceVersion")
	assert.Contains(t, body, "name: node-1")
}

func TestDownloadSingle_NotFound(t *testing.T) {
	cs := newDownloadTestClientSet(t)

	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "pods", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pods/default/missing/download?neat=false", nil)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDownloadSingle_ResourceTypeNotRegistered(t *testing.T) {
	cs := newDownloadTestClientSet(t)

	// Clear handlers so nothing is registered
	oldHandlers := handlers
	handlers = map[string]resourceHandler{}
	t.Cleanup(func() { handlers = oldHandlers })

	// Use CRD-style router so the route matches any resource type
	router := setupCRDDownloadTestRouter(t, cs)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nonexistent/default/test/download?neat=false", nil)
	router.ServeHTTP(rec, req)

	// Should return 404 because no handler and no CRD found
	// (GetResource returns "resource handler for ... not found" which contains "not found")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestDownloadSingle_CRDResource(t *testing.T) {
	// Create a fake CRD and a custom resource as unstructured objects
	crdGVK := schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}
	crGVK := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "MyApp"}

	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(crdGVK)
	crd.SetName("myapps.example.com")
	crd.Object["spec"] = map[string]interface{}{
		"group": "example.com",
		"scope": "Namespaced",
		"versions": []interface{}{
			map[string]interface{}{
				"name":   "v1",
				"served": true,
			},
		},
		"names": map[string]interface{}{
			"kind":   "MyApp",
			"plural": "myapps",
		},
	}

	cr := &unstructured.Unstructured{}
	cr.SetGroupVersionKind(crGVK)
	cr.SetName("my-instance")
	cr.SetNamespace("default")
	cr.Object["spec"] = map[string]interface{}{"message": "hello"}
	cr.Object["status"] = map[string]interface{}{"ready": true}

	cs := newDownloadTestClientSet(t, crd, cr)

	// Clear handlers so GetResource falls through to CRD unstructured path
	oldHandlers := handlers
	handlers = map[string]resourceHandler{}
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupCRDDownloadTestRouter(t, cs)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/myapps.example.com/default/my-instance/download?neat=true", nil)
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	// Neat mode: status removed
	assert.NotContains(t, body, "status:")
	// Spec preserved
	assert.Contains(t, body, "message: hello")
	// Kind from the CRD object
	assert.Contains(t, body, "kind: MyApp")
}

func TestDownloadBatch_NamespaceScopedResources(t *testing.T) {
	pod1 := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
	}
	pod2 := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "pod-2", Namespace: "default"},
	}

	cs := newDownloadTestClientSet(t, pod1, pod2)

	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "pods", false)

	items := []downloadItem{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2", Namespace: "default"},
	}
	body, err := json.Marshal(items)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pods/download?neat=false", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/zip", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".zip")

	// Verify zip contents
	zipReader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)

	fileNames := map[string]bool{}
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}
	assert.True(t, fileNames["Pod-default-pod-1.yaml"], "expected pod-1 yaml in zip")
	assert.True(t, fileNames["Pod-default-pod-2.yaml"], "expected pod-2 yaml in zip")
}

func TestDownloadBatch_ClusterScopedResources(t *testing.T) {
	node1 := &corev1.Node{
		TypeMeta:   metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	node2 := &corev1.Node{
		TypeMeta:   metav1.TypeMeta{Kind: "Node", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
	}

	cs := newDownloadTestClientSet(t, node1, node2)

	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "nodes", true)

	items := []downloadItem{
		{Name: "node-1"},
		{Name: "node-2"},
	}
	body, err := json.Marshal(items)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/nodes/_all/download?neat=true", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "nodes-")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".zip")

	zipReader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)

	fileNames := map[string]bool{}
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}
	assert.True(t, fileNames["Node-node-1.yaml"])
	assert.True(t, fileNames["Node-node-2.yaml"])
}

func TestDownloadBatch_WithFailures(t *testing.T) {
	pod1 := &corev1.Pod{
		TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"},
	}
	// Only pod-1 exists, pod-2 will fail

	cs := newDownloadTestClientSet(t, pod1)

	oldHandlers := handlers
	handlers = newResourceHandlers()
	t.Cleanup(func() { handlers = oldHandlers })

	router := setupDownloadTestRouter(t, cs, "pods", false)

	items := []downloadItem{
		{Name: "pod-1", Namespace: "default"},
		{Name: "pod-2", Namespace: "default"},
	}
	body, err := json.Marshal(items)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pods/download?neat=false", bytes.NewReader(body))
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	zipReader, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	require.NoError(t, err)

	fileNames := map[string]bool{}
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}
	// pod-1 should be in the zip
	assert.True(t, fileNames["Pod-default-pod-1.yaml"])
	// Error summary should be present
	assert.True(t, fileNames["_download_errors.txt"])

	// Verify error summary content
	for _, f := range zipReader.File {
		if f.Name == "_download_errors.txt" {
			rc, err := f.Open()
			require.NoError(t, err)
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			assert.Contains(t, string(content), "pod-2")
		}
	}
}

func TestDownloadBatch_EmptyItems(t *testing.T) {
	cs := newDownloadTestClientSet(t)

	router := setupDownloadTestRouter(t, cs, "pods", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pods/download?neat=false", bytes.NewReader([]byte("[]")))
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDownloadBatch_InvalidBody(t *testing.T) {
	cs := newDownloadTestClientSet(t)

	router := setupDownloadTestRouter(t, cs, "pods", false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pods/download?neat=false", bytes.NewReader([]byte("invalid json")))
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDownloadSingle_EmptyName(t *testing.T) {
	cs := newDownloadTestClientSet(t)

	router := setupDownloadTestRouter(t, cs, "pods", false)

	// With empty name, the handler returns 400 because name is required
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/pods/default//download?neat=false", nil)
	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestBuildYAMLFileName(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		namespace string
		resName   string
		want      string
	}{
		{
			name:      "namespaced resource",
			kind:      "Pod",
			namespace: "default",
			resName:   "nginx",
			want:      "Pod-default-nginx.yaml",
		},
		{
			name:      "cluster-scoped resource",
			kind:      "Node",
			namespace: "",
			resName:   "node-1",
			want:      "Node-node-1.yaml",
		},
		{
			name:      "all namespaces value treated as cluster-scoped",
			kind:      "Pod",
			namespace: common.AllNamespaces,
			resName:   "nginx",
			want:      "Pod-nginx.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildYAMLFileName(tt.kind, tt.namespace, tt.resName)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSetResourceType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("sets resource in context", func(t *testing.T) {
		middleware := setResourceType("pods")
		called := false
		router := gin.New()
		router.GET("/test", middleware, func(c *gin.Context) {
			assert.Equal(t, "pods", c.GetString("resource"))
			called = true
			c.Status(http.StatusOK)
		})

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(rec, req)

		assert.True(t, called)
	})

	t.Run("different resource types", func(t *testing.T) {
		for _, rt := range []string{"pods", "deployments", "nodes", "helmrelease"} {
			t.Run(rt, func(t *testing.T) {
				middleware := setResourceType(rt)
				called := false
				router := gin.New()
				router.GET("/test", middleware, func(c *gin.Context) {
					assert.Equal(t, rt, c.GetString("resource"))
					called = true
					c.Status(http.StatusOK)
				})

				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				router.ServeHTTP(rec, req)

				assert.True(t, called)
			})
		}
	})
}

func TestGetKindFromObject(t *testing.T) {
	t.Run("from typed object", func(t *testing.T) {
		pod := &corev1.Pod{
			TypeMeta: metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
		}
		// Pod implements runtime.Object
		kind := getKindFromObject(pod)
		assert.Equal(t, "Pod", kind)
	})

	t.Run("from map", func(t *testing.T) {
		m := map[string]interface{}{"kind": "Deployment"}
		kind := getKindFromObject(m)
		assert.Equal(t, "Deployment", kind)
	})

	t.Run("from nil", func(t *testing.T) {
		kind := getKindFromObject(nil)
		assert.Equal(t, "", kind)
	})

	t.Run("from unstructured", func(t *testing.T) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		kind := getKindFromObject(u)
		assert.Equal(t, "Deployment", kind)
	})
}
