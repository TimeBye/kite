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
// It creates a new API key with OwnerUserID set to the current user so that
// permissions are inherited dynamically. Any existing kubeconfig API keys
// (those with the same owner_user_id) are deleted first so that only one
// kubeconfig API key per user is valid at a time.
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

	// Delete previous kubeconfig API keys for this user so only one is valid.
	oldKeys, err := model.ListAPIKeyUsersByOwner(user.ID)
	if err != nil {
		return "", fmt.Errorf("failed to list old API keys: %w", err)
	}
	for _, oldKey := range oldKeys {
		_ = model.DeleteUserByID(oldKey.ID)
	}

	apiKeyName := fmt.Sprintf("kubeconfig-%s-%d", user.Key(), time.Now().UnixNano())
	apiKeyUser, err := model.NewAPIKeyUser(apiKeyName)
	if err != nil {
		return "", fmt.Errorf("failed to create API key: %w", err)
	}

	// Set owner so permissions are inherited dynamically.
	apiKeyUser.OwnerUserID = &user.ID
	if err := model.DB.Model(&model.User{}).Where("id = ?", apiKeyUser.ID).Update("owner_user_id", user.ID).Error; err != nil {
		_ = model.DeleteUserByID(apiKeyUser.ID)
		return "", fmt.Errorf("failed to set API key owner: %w", err)
	}

	cleanupOnError := func() {
		_ = model.DeleteUserByID(apiKeyUser.ID)
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
