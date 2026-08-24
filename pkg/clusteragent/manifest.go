package clusteragent

import (
	"fmt"
	"strings"
)

// GenerateManifest builds the Kubernetes manifest (Secret, ServiceAccount,
// ClusterRoleBinding, Deployment) used to deploy the Cluster Agent inside a
// target cluster. The image and server URL are injected so the manifest is
// always consistent with the platform configuration.
func GenerateManifest(serverURL, token, publicKey, image string) string {
	serverURL = strings.TrimSpace(serverURL)
	token = strings.TrimSpace(token)
	publicKey = strings.TrimSpace(publicKey)
	image = strings.TrimSpace(image)
	// JSON-encode the string values so the YAML is always valid regardless
	// of special characters.
	tokenJSON := fmt.Sprintf("%q", token)
	publicKeyJSON := fmt.Sprintf("%q", publicKey)
	serverJSON := fmt.Sprintf("%q", serverURL)
	imageJSON := fmt.Sprintf("%q", image)
	tokenHashJSON := fmt.Sprintf("%q", tokenHash(token))
	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: kite-cluster-agent
  namespace: kube-system
type: Opaque
stringData:
  KITE_SERVER: %s
  CLUSTER_AGENT_TOKEN: %s
  CLUSTER_AGENT_PUBLIC_KEY: %s
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kite-cluster-agent
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kite-cluster-agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kite-cluster-agent
    namespace: kube-system
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kite-cluster-agent
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: kite-cluster-agent
  template:
    metadata:
      annotations:
        kite.kubernetes.io/cluster-agent-token-hash: %s
      labels:
        app.kubernetes.io/name: kite-cluster-agent
    spec:
      serviceAccountName: kite-cluster-agent
      containers:
        - name: cluster-agent
          image: %s
          command:
            - /app/kite
          args:
            - cluster-agent
          envFrom:
            - secretRef:
                name: kite-cluster-agent
          ports:
            - name: probe
              containerPort: 8080
          livenessProbe:
            httpGet:
              path: /healthz
              port: probe
            initialDelaySeconds: 10
            periodSeconds: 30
            timeoutSeconds: 5
            failureThreshold: 3
          readinessProbe:
            httpGet:
              path: /readyz
              port: probe
            initialDelaySeconds: 5
            periodSeconds: 10
            timeoutSeconds: 5
            failureThreshold: 6
`, serverJSON, tokenJSON, publicKeyJSON, tokenHashJSON, imageJSON)
}
