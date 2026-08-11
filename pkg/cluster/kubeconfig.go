package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/zxh326/kite/pkg/model"
	"github.com/zxh326/kite/pkg/rbac"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// sanitizeClusterName removes characters that are invalid in kubeconfig
// context/cluster names and file systems, while preserving Unicode letters.
func sanitizeClusterName(name string) string {
	replacer := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "-", "\"", "-", "<", "-", ">", "-", "|", "-",
		" ", "-",
	)
	return replacer.Replace(name)
}

// GenerateKubeconfig creates a kubeconfig YAML that uses Kite as a proxy.
// It creates a new API key that inherits the user's current roles.
func GenerateKubeconfig(user model.User, clusterUUIDs []string, serverURL string) (string, error) {
	clusters := make([]*model.Cluster, 0, len(clusterUUIDs))
	for _, uuidStr := range clusterUUIDs {
		cluster, err := model.GetClusterByUUID(uuidStr)
		if err != nil {
			return "", fmt.Errorf("cluster not found: %s", uuidStr)
		}
		if !rbac.CanAccessCluster(user, cluster.Name) {
			return "", fmt.Errorf("access denied to cluster: %s", cluster.Name)
		}
		clusters = append(clusters, cluster)
	}

	if len(clusters) == 0 {
		return "", fmt.Errorf("no clusters selected")
	}

	apiKeyName := fmt.Sprintf("kubeconfig-%s-%d", user.Key(), time.Now().UnixNano())
	apiKeyUser, err := model.NewAPIKeyUser(apiKeyName)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	// If anything fails after API key creation, clean up the key so we don't
	// leave orphaned credentials.
	cleanupOnError := func() {
		_ = model.DeleteUserByID(apiKeyUser.ID)
	}

	// Inherit the user's current roles.
	for _, role := range rbac.GetUserRoles(user) {
		if err := model.AddRoleAssignment(role.Name, model.SubjectTypeUser, apiKeyUser.Username); err != nil {
			cleanupOnError()
			return "", fmt.Errorf("failed to assign role %s: %w", role.Name, err)
		}
	}

	token := apiKeyUser.GetAPIKey()
	username := sanitizeClusterName(user.Key())

	config := clientcmdapi.NewConfig()
	for _, cluster := range clusters {
		clusterName := sanitizeClusterName(cluster.Name)
		proxyURL := fmt.Sprintf("%s/api/v1/clusters/%s/k8s-proxy", strings.TrimRight(serverURL, "/"), cluster.UUID)

		config.Clusters[clusterName] = &clientcmdapi.Cluster{
			Server:                proxyURL,
			InsecureSkipTLSVerify: true,
		}
		config.Contexts[clusterName] = &clientcmdapi.Context{
			Cluster:   clusterName,
			AuthInfo:  username,
			Namespace: "default",
		}
	}
	config.AuthInfos[username] = &clientcmdapi.AuthInfo{
		Token: token,
	}

	// Set current context to the first selected cluster.
	config.CurrentContext = sanitizeClusterName(clusters[0].Name)

	yaml, err := clientcmd.Write(*config)
	if err != nil {
		cleanupOnError()
		return "", fmt.Errorf("failed to marshal kubeconfig: %w", err)
	}

	return string(yaml), nil
}
