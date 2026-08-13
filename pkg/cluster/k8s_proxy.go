package cluster

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zxh326/kite/pkg/common"
	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/rbac"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/proxy"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

// discoveryPaths are K8s API paths that only require cluster-level access.
var discoveryPaths = map[string]bool{
	"/api":     true,
	"/apis":    true,
	"/version": true,
	"/openapi": true,
}

// subResourceVerbs maps sub-resources to their RBAC verb.
var subResourceVerbs = map[string]string{
	"exec":        string(common.VerbExec),
	"attach":      string(common.VerbExec),
	"log":         string(common.VerbLog),
	"portforward": string(common.VerbExec),
}

// HandleK8sProxy proxies K8s API requests through Kite with RBAC enforcement.
// Route: ANY /api/v1/clusters/:clusterUUID/k8s-proxy/*path
func (cm *ClusterManager) HandleK8sProxy(c *gin.Context) {
	user := c.MustGet("user").(model.User)
	clusterUUID := c.Param("clusterUUID")
	proxyPath := c.Param("path")

	// Path traversal protection: decode and check for ".."
	decodedPath, err := url.PathUnescape(proxyPath)
	if err != nil {
		writeK8sStatus(c, http.StatusBadRequest, "invalid proxy path", metav1.StatusReasonBadRequest)
		return
	}
	if strings.Contains(decodedPath, "..") {
		writeK8sStatus(c, http.StatusBadRequest, "invalid proxy path: path traversal detected", metav1.StatusReasonBadRequest)
		return
	}

	cluster, err := model.GetClusterByUUID(clusterUUID)
	if err != nil {
		writeK8sStatus(c, http.StatusNotFound, fmt.Sprintf("cluster %q not found", clusterUUID), metav1.StatusReasonNotFound)
		return
	}

	// Layer 1: cluster-level access control
	if !rbac.CanAccessCluster(user, cluster.Name) {
		writeK8sStatus(c, http.StatusForbidden,
			rbac.NoAccess(user.Key(), "access", "cluster", "", cluster.Name), metav1.StatusReasonForbidden)
		return
	}

	// Layer 2: resource-level RBAC (skip for discovery endpoints)
	if !isDiscoveryPath(decodedPath) {
		resource, namespace, verb, subResource := parseK8sAPIPath(decodedPath, c.Request.Method)
		if resource != "" {
			checkVerb := verb
			if subResource != "" {
				if v, ok := subResourceVerbs[subResource]; ok {
					checkVerb = v
				}
			}
			if namespace == "" {
				namespace = common.AllNamespaces
			}
			if !rbac.CanAccess(user, resource, checkVerb, cluster.Name, namespace) {
				writeK8sStatus(c, http.StatusForbidden,
					rbac.NoAccess(user.Key(), checkVerb, resource, namespace, cluster.Name), metav1.StatusReasonForbidden)
				return
			}
		}
	}

	// Get the rest.Config for the cluster
	restConfig, err := cm.getRestConfig(cluster)
	if err != nil {
		klog.Errorf("failed to get rest config for cluster %s: %v", cluster.Name, err)
		writeK8sStatus(c, http.StatusInternalServerError, "failed to connect to cluster", metav1.StatusReasonInternalError)
		return
	}

	// Build the target URL
	targetURL, err := buildK8sProxyURL(restConfig.Host, decodedPath)
	if err != nil {
		writeK8sStatus(c, http.StatusBadRequest, err.Error(), metav1.StatusReasonBadRequest)
		return
	}
	// Preserve query parameters on the target URL.
	//
	// NewUpgradeAwareHandler only copies req.URL.RawQuery onto the proxy
	// location in its *non-upgrade* ServeHTTP branch. For SPDY/WebSocket
	// upgrades (kubectl exec, port-forward, attach, websocket exec) the
	// tryUpgrade path directly uses h.Location as-is, so query params
	// (stdin=true, stdout=true, tty=true, container=..., command=...,
	// watch=true, timeoutSeconds=..., etc.) would otherwise be lost and
	// the K8s API would reply with errors like "you must specify at least
	// 1 of stdin, stdout, stderr".
	targetURL.RawQuery = c.Request.URL.RawQuery

	// Build transport with cluster credentials. Only force HTTP/1.1 for
	// upgrade requests (kubectl exec, attach, port-forward); regular API
	// calls can use HTTP/2 for better multiplexing performance.
	forceHTTP1 := c.Request.Header.Get("Upgrade") != ""
	transport, err := buildProxyTransport(restConfig, forceHTTP1)
	if err != nil {
		klog.Errorf("failed to build transport for cluster %s: %v", cluster.Name, err)
		writeK8sStatus(c, http.StatusInternalServerError, "failed to create transport", metav1.StatusReasonInternalError)
		return
	}

	// Clean auth and impersonation headers from the incoming request
	c.Request.Header.Del("Authorization")
	c.Request.Header.Del("Cookie")
	for name := range c.Request.Header {
		if strings.HasPrefix(strings.ToLower(name), "impersonate-") {
			c.Request.Header.Del(name)
		}
	}

	// NewUpgradeAwareHandler.tryUpgrade writes the request directly to the
	// backend connection via req.Write(conn), bypassing transport.RoundTrip.
	// This means the auth round trippers inside rest.TransportFor (bearer token,
	// exec provider, auth provider) are NOT applied for SPDY/WebSocket upgrade
	// requests (kubectl exec, attach, port-forward), causing the K8s API server
	// to return 401 "unauthorized".
	//
	// UpgradeTransport solves this: DialForUpgrade calls WrapRequest(req) which
	// runs the auth round trippers against a MirrorRequest round tripper that
	// captures the modified request (with auth headers added) without actually
	// sending it. The modified request is then written to the backend connection.
	responder := &k8sProxyResponder{}
	handler := proxy.NewUpgradeAwareHandler(targetURL, transport, false, false, responder)
	authConfig := rest.CopyConfig(restConfig)
	authConfig.WrapTransport = func(http.RoundTripper) http.RoundTripper {
		return proxy.MirrorRequest
	}
	authTransport, err := rest.TransportFor(authConfig)
	if err != nil {
		klog.Errorf("failed to create auth transport for cluster %s: %v", cluster.Name, err)
		writeK8sStatus(c, http.StatusInternalServerError, "failed to create transport", metav1.StatusReasonInternalError)
		return
	}
	handler.UpgradeTransport = proxy.NewUpgradeRequestRoundTripper(transport, authTransport)
	handler.FlushInterval = 200 * time.Millisecond

	handler.ServeHTTP(c.Writer, c.Request)
}

// getRestConfig returns a rest.Config for the given cluster.
func (cm *ClusterManager) getRestConfig(cluster *model.Cluster) (*rest.Config, error) {
	if cluster.InCluster {
		return rest.InClusterConfig()
	}
	if cluster.Connector {
		creds := cm.connectorManager.GetCredentials(cluster.ID)
		if creds == nil {
			return nil, fmt.Errorf("connector cluster %s has no credentials (agent not connected?)", cluster.Name)
		}
		dialer := cm.connectorManager.Dialer(cluster.ID)
		return creds.ToRestConfig(dialer), nil
	}
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(string(cluster.Config)))
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	return config, nil
}

// buildK8sProxyURL constructs the target URL for the K8s API server.
func buildK8sProxyURL(host, proxyPath string) (*url.URL, error) {
	target, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parse cluster host: %w", err)
	}
	target.Path = path.Join(target.Path, "/"+strings.TrimPrefix(proxyPath, "/"))
	target.RawPath = ""
	target.RawQuery = ""
	return target, nil
}

// buildProxyTransport creates an HTTP transport from the rest.Config.
// When forceHTTP1 is true, the transport is restricted to HTTP/1.1 for
// SPDY/WebSocket upgrade support (kubectl exec, attach, port-forward).
func buildProxyTransport(restConfig *rest.Config, forceHTTP1 bool) (http.RoundTripper, error) {
	cfg := rest.CopyConfig(restConfig)
	if forceHTTP1 {
		cfg.NextProtos = []string{"http/1.1"}
	}
	return rest.TransportFor(cfg)
}

// isDiscoveryPath returns true for K8s API discovery endpoints.
func isDiscoveryPath(p string) bool {
	p = strings.TrimSuffix(p, "/")
	if discoveryPaths[p] {
		return true
	}
	// /api/v1, /apis/<group>/<version>, /openapi/v2, /openapi/v3, etc.
	for prefix := range discoveryPaths {
		if strings.HasPrefix(p, prefix+"/") {
			// Exclude resource paths like /api/v1/namespaces/.../pods
			// by checking that the path has at most 3 segments after the prefix
			rest := strings.TrimPrefix(p, prefix+"/")
			segments := strings.Split(rest, "/")
			if prefix == "/api" {
				// /api/v1 is discovery, /api/v1/namespaces is not
				return len(segments) <= 1
			}
			if prefix == "/apis" {
				// /apis/<group>/<version> is discovery, /apis/<group>/<version>/namespaces is not
				return len(segments) <= 2
			}
			// /openapi/v2, /openapi/v3 are discovery
			return len(segments) <= 1
		}
	}
	return false
}

// parseK8sAPIPath extracts resource, namespace, and verb from a K8s API path.
// Examples:
//
//	/api/v1/namespaces/default/pods → resource=pods, ns=default, verb=get
//	/apis/apps/v1/namespaces/default/deployments → resource=deployments, ns=default, verb=get
//	/api/v1/namespaces/default/pods/my-pod/exec → resource=pods, ns=default, verb=exec, subResource=exec
func parseK8sAPIPath(apiPath, method string) (resource, namespace, verb, subResource string) {
	verb = methodToVerb(method)
	path := strings.TrimPrefix(apiPath, "/")
	segments := strings.Split(path, "/")
	if len(segments) < 3 {
		return "", "", verb, ""
	}

	var idx int
	switch {
	case segments[0] == "api" && segments[1] == "v1":
		idx = 2
	case segments[0] == "apis" && len(segments) >= 3:
		idx = 3
	default:
		return "", "", verb, ""
	}

	// At this point segments[idx] could be "namespaces" (namespace-scoped) or a resource (cluster-scoped)
	if idx >= len(segments) {
		return "", "", verb, ""
	}

	if segments[idx] == "namespaces" {
		// /api/v1/namespaces/:ns/:resource[/:name[/:subresource]]
		if idx+2 >= len(segments) {
			// /api/v1/namespaces/:ns — the resource is "namespaces" itself
			return "namespaces", "", verb, ""
		}
		namespace = segments[idx+1]
		resource = segments[idx+2]
		if idx+4 < len(segments) {
			subResource = segments[idx+4]
		}
	} else {
		// Cluster-scoped resource: /api/v1/:resource[/:name[/:subresource]]
		resource = segments[idx]
		namespace = ""
		if idx+2 < len(segments) {
			subResource = segments[idx+2]
		}
	}

	// Normalize: if subResource is actually the resource name (no name segment),
	// we may have miscounted. But for RBAC purposes, the resource and verb are what matter.
	// Reset subResource if it looks like a resource name (we only care about known sub-resources)
	if subResource != "" {
		if _, ok := subResourceVerbs[subResource]; !ok {
			subResource = ""
		}
	}

	return resource, namespace, verb, subResource
}

func methodToVerb(method string) string {
	switch method {
	case http.MethodPost:
		return string(common.VerbCreate)
	case http.MethodPut, http.MethodPatch:
		return string(common.VerbUpdate)
	case http.MethodDelete:
		return string(common.VerbDelete)
	default:
		return string(common.VerbGet)
	}
}

// k8sProxyResponder implements proxy.ErrorResponder.
type k8sProxyResponder struct{}

func (r *k8sProxyResponder) Error(w http.ResponseWriter, req *http.Request, err error) {
	klog.Errorf("k8s proxy responder error: path=%s, method=%s, err=%v", req.URL.Path, req.Method, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadGateway)
	if err := json.NewEncoder(w).Encode(metav1.Status{
		TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
		Status:   metav1.StatusFailure,
		Message:  fmt.Sprintf("proxy error: %s", err.Error()),
		Reason:   metav1.StatusReasonServiceUnavailable,
		Code:     http.StatusBadGateway,
	}); err != nil {
		klog.Errorf("failed to encode proxy error response: %v", err)
	}
}

// writeK8sStatus writes a Kubernetes-compatible metav1.Status JSON response.
// client-go's transformResponse always tries to decode 4xx/5xx bodies as
// Status objects (because newUnstructuredResponseError sets
// isUnexpectedResponse=true), so returning a proper Status lets kubectl
// display our message instead of "unknown (get pods)".
func writeK8sStatus(c *gin.Context, code int, message string, reason metav1.StatusReason) {
	c.JSON(code, metav1.Status{
		TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
		Status:   metav1.StatusFailure,
		Message:  message,
		Reason:   reason,
		Code:     int32(code),
	})
}
