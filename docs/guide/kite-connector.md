---
outline: deep
---

# Kite Connector

Kite Connector is used to onboard Kubernetes clusters that Kite Server cannot reach directly. The Connector runs inside the target cluster, initiates the connection to Kite, and forwards Kubernetes API requests issued by Kite over that connection.

## Background

In private networks, edge environments, or firewall-restricted scenarios, the target cluster may be able to reach Kite while Kite Server cannot connect to the cluster's kube-apiserver. The traditional kubeconfig onboarding method requires Kite Server to connect to kube-apiserver, so it cannot cover this one-way network topology.

Kite Connector reverses the connection direction:

- The Connector dials Kite Server from the cluster side; Kite does not need to enter the cluster network.
- During the WebSocket handshake, the Connector sends the Kubernetes API credentials (extracted from its kubeconfig or in-cluster ServiceAccount) to Kite Server. Kite stores them in memory only (never persisted to disk or database) and uses them to authenticate directly against kube-apiserver.
- Kite still uses its existing Kubernetes Client to access the cluster, so resource management, logs, and terminals do not need a separate protocol.

## How It Works

The connection chain looks like this:

```text
Kite Kubernetes Client
        │
        ▼
Kite Server (rest.Config with tunnel dialer + credentials)
        │
        ▼
WebSocket tunnel (established by the Connector)
        │
        ▼
Connector raw TCP forwarder
        │
        ▼
Target cluster kube-apiserver
```

The detailed process:

1. When you create a Connector cluster in the Kite UI, Kite generates a random connection token, stores only its SHA-256 hash, and shows the raw token once after creation.
2. The Connector extracts Kubernetes API credentials (bearer token, CA cert, client cert/key) from its kubeconfig or in-cluster ServiceAccount, then connects to Kite Server's `/api/v1/connector/connect` WebSocket endpoint using the connection token. The credentials are sent via a header during the WebSocket handshake (protected by WSS/TLS). Kite validates the token, stores the credentials in memory, and binds the connection to the corresponding cluster.
3. Kite Server builds a `rest.Config` using the stored credentials and a custom dialer that routes through the remotedialer tunnel. TLS and authentication are handled end-to-end by Kite Server's transport — no local proxy or HTTP listener is needed.
4. When Kite requests a tunnel connection, the Connector creates a raw TCP connection to kube-apiserver using `net.Dial`. The Connector does not parse HTTP, inject headers, or manage transports — it simply forwards bytes.
5. Header cleanup (stripping `Authorization` and `Impersonate-*` headers from the incoming request) is performed by Kite Server's `HandleK8sProxy` handler before the request enters the tunnel.

A single Connector WebSocket can carry multiple Kubernetes API connections. Streaming requests such as logs, watches, and terminals also travel over the same tunnel.

## Usage

### 1. Create a Connector Cluster

Go to **Settings → Cluster Management** in Kite, select **Add Cluster**, and then:

1. Fill in the cluster name and description.
2. Set the cluster type to **Kite Connector**.
3. Create the cluster.

After creation, Kite shows the connection info only once, and you can choose the command-line or Kubernetes YAML option.

Command-line option:

```bash
kite connector --server='https://kite.example.com' --token='<connector-token>'
```

`--server` may include Kite's Base Path. For example:

```bash
kite connector --server='https://kite.example.com/kite' --token='<connector-token>'
```

The Connector automatically appends `/api/v1/connector/connect` to this address; do not add this API path to `--server` manually.

The Kubernetes YAML option creates the following resources:

- A `kite-connector-token` Secret in the `kube-system` namespace to hold the Connector Token.
- A `kite-connector` ServiceAccount in the `kube-system` namespace.
- A ClusterRoleBinding that binds this ServiceAccount to `cluster-admin`.
- A single-replica `kite-connector` Deployment in the `kube-system` namespace.

You can deploy in either of the following ways:

**Option 1: Deploy directly via URL**

After creation, Kite generates a manifest download URL that you can apply directly with `kubectl apply`:

```bash
kubectl apply -f 'https://kite.example.com/api/v1/connector/manifest?grant=<manifest-grant>'
```

The URL contains an encrypted manifest grant that is separate from the Connector Token. The grant expires after 10 minutes and remains valid across Kite restarts while the JWT secret is unchanged. It can be reused until it expires, so apply it promptly or use the YAML shown in the dialog. The returned YAML contains the Connector Token and must still be handled as a secret.

**Option 2: Copy the YAML and deploy manually**

Copy the YAML from the connection info dialog, save it to a file, and then deploy:

```bash
kubectl apply -f kite-connector.yaml
```

The image used by the Deployment is configured via **Connector Image** in the platform settings, defaulting to `ghcr.io/kite-org/kite:latest`. The token is injected into the Connector process via a Secret.

### 2. Start It in the Target Cluster

When starting manually via the command line, run the generated command inside a Pod that can reach kube-apiserver. When `--kubeconfig` is not specified, the Connector uses `rest.InClusterConfig()` to read the Pod's ServiceAccount credentials.

Once the Connector connects successfully, the cluster management page shows it as "Connected". If the connection drops, the Connector retries every 5 seconds.

### 3. Test with a Local kubeconfig

For local development and testing, you can specify a kubeconfig explicitly:

```bash
kite connector \
  --server='http://localhost:8080' \
  --token='<connector-token>' \
  --kubeconfig='/path/to/kubeconfig'
```

The Connector then uses the API server and credentials from the kubeconfig's current context. `http://` is recommended only for local testing; production should use `https://`.

## Things to Note

### Token Security

- The Connector Token currently has no expiry. As long as the corresponding cluster exists and is enabled, it can be used to reconnect. Disabling a cluster stops Kite from using it and rejects subsequent new or reconnecting Connector handshakes, but does not actively close already-established Connector sessions; re-enabling the cluster keeps the original token valid.
- The Kite database stores only the token hash. The raw token is shown only once after cluster creation, so save it somewhere safe immediately.
- There is no dedicated token rotation endpoint yet. If a token leaks, delete and recreate the Connector cluster to generate a new token.
- `--token` appears in process arguments and shell history. Restrict login and process-viewing permissions on the Connector host, and never paste the real token into logs, tickets, or chats.

### Server Identity Verification

- Production must use `https://` with a Kite Server certificate issued by a CA trusted by the Connector's runtime environment. The Connector validates the certificate domain and trust chain by standard TLS rules, preventing connections to a forged Kite Server.
- With `http://` there is no server identity verification, and neither the token nor the tunnel data is protected by TLS. Use it only for controlled local testing.
- The connection currently uses a Bearer Token; mTLS, client certificate issuance, and certificate rotation are not yet implemented.
- During the WebSocket handshake, the Connector sends Kubernetes API credentials (bearer token, CA cert, client cert/key) to Kite Server via the `X-Kite-K8s-Credentials` header. This transmission is protected by WSS/TLS encryption. The credentials are stored in Kite Server memory only (never persisted to disk or database) and are automatically cleared when the connection drops. When the ServiceAccount token rotates, the Connector automatically updates the credentials on reconnect.

### Kubernetes Permissions and Auditing

- The permissions of the Connector's ServiceAccount or kubeconfig determine what Kite can do in the cluster. If you need features like logs or terminals, grant the corresponding Kubernetes RBAC permissions as well.
- The generated YAML binds the Connector ServiceAccount to `cluster-admin`, granting administrative privileges over the entire cluster. Confirm this meets your security requirements before deploying.
- Kite users are still governed by Kite's own RBAC, but the identity kube-apiserver sees is the ServiceAccount or kubeconfig user used by the Connector, not the actual logged-in Kite user.
- Header cleanup (stripping `Authorization` and `Impersonate-*` headers) is performed by Kite Server before requests enter the tunnel. The Connector itself is a pure TCP forwarder and does not process HTTP headers.

### Network and Availability

- The Connector only needs outbound access to Kite Server and the target cluster's kube-apiserver; it does not expose any new listening port to the outside.
- Kite Server does not open any local listening port for connector clusters. It uses the stored credentials and the remotedialer tunnel dialer directly in its `rest.Config`. The Connector only makes outbound TCP connections to kube-apiserver.
- The Ingress or reverse proxy in front of Kite must support WebSocket Upgrade and long-lived connections.
- Connector session routing across multiple Kite Server replicas is not yet implemented. When using Connector clusters, run a single Kite Server replica first.
- A tunnel disconnect aborts any running logs, watch, or terminal connections. After the Connector reconnects, clients must re-initiate these streaming requests.
- For connector clusters, terminals and command execution use the SPDY protocol (with the tunnel dialer injected via `UpgradeTransport`). Non-connector clusters use the standard WebSocket → SPDY fallback.
