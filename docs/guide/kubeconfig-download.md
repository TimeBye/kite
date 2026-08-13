---
outline: deep
---

# Kubeconfig Download

Kite allows you to download a kubeconfig file that uses Kite as a proxy to access your clusters. This means you can use `kubectl` from your local machine without directly exposing the Kubernetes API Server, while Kite's RBAC permission control is still enforced.

## How to Use

1. Click the **Download** icon in the top navigation bar.
2. In the dialog, select one or more clusters you want to access. The current cluster is selected by default.
3. Click **Download** — a kubeconfig file will be generated and downloaded.

## How It Works

```
┌─────────────┐
│   kubectl   │  uses downloaded kubeconfig
└──────┬──────┘
       │ Authorization: Bearer kite-api-key
       ▼
┌─────────────────────────────────────────┐
│         Kite Server (Reverse Proxy)     │
│  ┌───────────────────────────────────┐  │
│  │ 1. Auth: validate API Key         │  │
│  │ 2. Cluster RBAC: CanAccessCluster │  │
│  │ 3. Parse K8s API path             │  │
│  │ 4. Resource RBAC: CanAccess       │  │
│  │ 5. Inject cluster credentials     │  │
│  │ 6. Proxy to K8s API Server        │  │
│  └───────────────────────────────────┘  │
└──────┬──────────────────────────────────┘
       │
       ├──────────────┬───────────────────────┐
       ▼ Direct       ▼ Connector tunnel       ▼
┌──────────────┐                            ┌──────────────┐
│ K8s API Server│          WSS tunnel        │  Agent       │
│ (direct)      │◀──────────────────────────│  (TCP        │
└──────────────┘   rest.Config + dialer     │   forwarder) │
                                            └──────┬───────┘
                                                   │ raw TCP
                                                   ▼
                                           ┌──────────────┐
                                           │ K8s API Server│
                                           │ (private)     │
                                           └──────────────┘
```

When you download a kubeconfig, Kite:

1. **Deletes any previously downloaded kubeconfig API keys** for the current user, so only one kubeconfig API key is valid at a time.
2. Creates a new **API Key** linked to your account as its **owner**. The key dynamically inherits your RBAC roles — if your roles change later, the key's permissions update automatically without re-downloading.
3. Generates a kubeconfig YAML where:
   - `server` points to Kite's K8s API proxy endpoint (`/api/v1/clusters/{uuid}/k8s-proxy`).
   - `token` is the newly created API Key.
   - `insecure-skip-tls-verify` is set to `true` (traffic goes through Kite).

## Request Flow: `kubectl exec -it pod-name -- sh` End-to-End

`kubectl exec` is the most complex case because it uses SPDY (or WebSocket) protocol upgrade. Here is the full journey of a single exec request.

### Phase 1: kubectl Sends SPDY Upgrade Request

kubectl reads the downloaded kubeconfig and constructs an HTTP POST request with SPDY upgrade headers:

```
POST /api/v1/clusters/{cluster-uuid}/k8s-proxy/api/v1/namespaces/default/pods/my-pod/exec?command=sh&stdin=true&stdout=true&tty=true HTTP/1.1
Host: kite.example.com
Authorization: Bearer kite<api-key>          ← from kubeconfig token field
Connection: Upgrade
Upgrade: SPDY/3.1
Content-Length: 0
```

SPDY allows multiplexed, bidirectional streams over a single TCP connection. This is essential for `exec` where stdin/stdout/stderr/error streams must flow simultaneously.

### Phase 2: Kite Server Receives and Authorizes

The request hits Kite's `HandleK8sProxy` handler, which performs six steps:

```
┌──────────────────────────────────────────────────┐
│              Kite Server                          │
│                                                   │
│  1. Auth: Strip "Bearer " prefix from             │
│     Authorization header, validate API Key        │
│     (RequireAuth middleware)                      │
│                                                   │
│  2. Cluster RBAC: CanAccessCluster(user, cluster) │
│     — Does this user have access to the cluster?  │
│                                                   │
│  3. Path Parse: parseK8sAPIPath()                 │
│     /api/v1/namespaces/default/pods/my-pod/exec   │
│     → resource=pods, ns=default, subresource=exec │
│                                                   │
│  4. Resource RBAC: subresource "exec" → verb exec │
│     CanAccess(user, pods, exec, cluster, default) │
│                                                   │
│  5. Strip incoming auth headers:                  │
│     Del Authorization, Cookie, Impersonate-*      │
│                                                   │
│  6. Build proxy transport (force HTTP/1.1         │
│     because Upgrade header is present)            │
└──────────────────────────────────────────────────┘
```

If any check fails, Kite returns a Kubernetes-compatible `metav1.Status` JSON response so that kubectl can display a meaningful error message (e.g. `Error from server (Forbidden): user admin does not have permission to get pods in namespace kube-system on cluster my-cluster (get pods)`) instead of `unknown (get pods)`.

### Phase 3: Kite Server Proxies to K8s API

How Kite connects to the Kubernetes API server depends on the cluster type:

#### Direct Clusters (InCluster / Kubeconfig)

Kite's `UpgradeAwareHandler` upgrades the TCP connection and forwards the raw SPDY frames:

```
┌──────────────┐    SPDY Upgrade    ┌────────────────┐
│  Kite Server │ ─────────────────→ │  K8s API Server │
│  (Upgrade    │   HTTP/1.1 + TLS   │  (validates      │
│  AwareHandler)│  Bearer Token      │   cluster token) │
└──────────────┘                    └────────────────┘
```

The `UpgradeTransport` mechanism injects the cluster's own credentials (bearer token from kubeconfig) into the upgrade request via `MirrorRequest` — a round tripper that captures auth headers without actually sending a request.

#### Connector Clusters (Remote, behind firewall)

For connector-based clusters, the request travels through the connector tunnel. The Agent is a pure TCP forwarder — it does not parse HTTP or manage transports. TLS and authentication are handled end-to-end by Kite Server's transport, using credentials that the Agent sent during the WebSocket handshake:

```
┌──────────┐               ┌─────────────┐    WSS tunnel    ┌──────────┐   raw TCP  ┌────────────┐
│   Kite   │ TLS + auth    │ Kite Server │ ──────────────→ │  Agent   │ ────────→ │ K8s API    │
│  Server  │ ───────────→  │ rest.Config │  remotedialer   │ (TCP     │ net.Dial  │ Server     │
│          │ (stored creds │ + dialer    │  WebSocket      │ forwarder)│           │            │
│          │  + dialer)    │             │  (encrypted)    │          │           │            │
└──────────┘               └─────────────┘                 └──────────┘           └────────────┘
```

**Step-by-step for connector clusters:**

1. **Agent sends credentials**: During the WebSocket handshake, the Connector extracts Kubernetes API credentials (bearer token, CA cert, client cert/key) from its kubeconfig or in-cluster ServiceAccount and sends them to Kite Server via the `X-Kite-K8s-Credentials` header (protected by WSS/TLS).
2. **Kite builds a `rest.Config`** using `creds.ToRestConfig(dialer)`, where `dialer` routes through the remotedialer tunnel. The config includes the real kube-apiserver host, TLS settings, and bearer token — all from the stored credentials.
3. **The `Dialer`** asks `remotedialer` to open a new tunnel connection to the agent, targeting `kubernetes-api`.
4. **On the agent side**, `localDialer` receives the tunnel request and creates a raw TCP connection to kube-apiserver using `net.Dial`. No HTTP parsing, no header injection, no transport management — pure byte forwarding.
5. **TLS handshake** happens end-to-end between Kite Server's transport and kube-apiserver, transparently through the tunnel and the Agent's TCP connection.

### Phase 4: K8s API Server Responds

The K8s API server authenticates the cluster credentials and opens a SPDY connection with multiple streams:

| Stream | Direction | Content |
|--------|-----------|---------|
| Stream 0 | Server → Client | Error messages (if any) |
| Stream 1 | Client → Server | stdin (your keyboard input) |
| Stream 2 | Server → Client | stdout (command output) |
| Stream 3 | Server → Client | stderr (error output) |

These SPDY frames flow back through the same path: `K8s API → Agent TCP forwarder → tunnel → Kite UpgradeAwareHandler → kubectl`.

### Phase 5: Interactive Session

Once the SPDY connection is established, kubectl and the kubelet container runtime exchange data bidirectionally. Kite's proxy is transparent at this point — it just forwards bytes over the hijacked TCP connection.

## Transport Strategy

K8s client components use `config.Transport` differently. Kite handles this with two separate `rest.Config` paths:

| Path | Component | Transport usage | Config |
|------|-----------|----------------|--------|
| kubectl/ktctl proxy | `UpgradeAwareHandler` | `utilnet.DialerFor(transport)` extracts `DialContext` | `Transport` + tunnel dialer (direct clusters) or `Dial` + credentials (connector clusters) |
| terminal/files | `SPDY/WebSocket Executor` | Ignores `config.Transport`, creates own transport | SPDY with `UpgradeTransport` injection (connector) or WebSocket → SPDY fallback (direct) |

- **`getRestConfig`** (for `HandleK8sProxy`): For direct clusters, uses `Transport` + `MirrorRequest` for auth. For connector clusters, uses `creds.ToRestConfig(dialer)` which sets `Host`, `TLSClientConfig`, `BearerToken`, and `Dial` — TLS and auth are handled end-to-end by the transport.
- **`buildClientSet`** (for terminal/files): For connector clusters, the `K8sClient` is marked with `IsConnector = true`. When creating executors, `buildExecutor()` dispatches to `buildSPDYExecutorWithDialer()`, which manually creates a SPDY round tripper with `UpgradeTransport` set to an `*http.Transport` containing the tunnel dialer. This works because SPDY's `dialerFor()` extracts `DialContext` from the `UpgradeTransport`.

Additionally, each transport is split by HTTP version:

- **Regular requests** (Get/List/Watch/Create): HTTP/2 allowed for multiplexing performance
- **Upgrade requests** (exec/attach/port-forward): HTTP/1.1 forced, because HTTP/2 prohibits `Upgrade` headers

## RBAC Permission Mapping

Kite's reverse proxy enforces two layers of access control:

| Layer | Check | Description |
|-------|-------|-------------|
| Cluster-level | `rbac.CanAccessCluster()` | Verifies the user has access to the target cluster |
| Resource-level | `rbac.CanAccess()` | Parses the K8s API path and checks resource-level permissions |

### HTTP Method to RBAC Verb Mapping

Kite's RBAC system has only 6 verbs: `get`, `create`, `update`, `delete`, `log`, `exec`. There is no separate `list` or `watch` verb — both K8s `list` and `watch` are HTTP GET requests, so they are mapped to Kite's `get` verb. This means granting `verbs: ["get"]` in Kite RBAC covers `kubectl get pods` (single resource), `kubectl get pods` (list), and `kubectl get pods -w` (watch).

| HTTP Method | K8s Verbs | Kite RBAC Verb |
|-------------|-----------|----------------|
| `GET` | `get`, `list`, `watch` | `get` |
| `POST` | `create` | `create` |
| `PUT` / `PATCH` | `update`, `patch` | `update` |
| `DELETE` | `delete` | `delete` |

### Sub-resource Mapping

| Sub-resource | RBAC Verb |
|-------------|-----------|
| `/pods/{name}/exec` | `exec` |
| `/pods/{name}/attach` | `exec` |
| `/pods/{name}/log` | `log` |
| `/pods/{name}/portforward` | `exec` |

Discovery endpoints (`/api`, `/apis`, `/version`, `/openapi`) only require cluster-level access and skip resource-level RBAC.

## Error Responses

Error responses use the Kubernetes standard `metav1.Status` JSON format, ensuring client-go can parse and display them properly:

```json
{
  "kind": "Status",
  "apiVersion": "v1",
  "status": "Failure",
  "message": "user admin does not have permission to get pods in namespace kube-system on cluster my-cluster",
  "reason": "Forbidden",
  "code": 403
}
```

This is important because client-go's `transformResponse` always tries to decode 4xx/5xx response bodies as `metav1.Status` objects. If the body is a valid Status, its `message` field replaces the default "unknown" text, so kubectl displays the actual error instead of `Error from server (Forbidden): unknown (get pods)`.

## Supported kubectl Commands

All streaming protocols are supported through the proxy:

| Command | Protocol | Supported |
|---------|----------|-----------|
| `kubectl get pods` | HTTP | Yes |
| `kubectl apply -f` | HTTP | Yes |
| `kubectl logs -f` | HTTP chunked stream | Yes |
| `kubectl get po -w` | HTTP chunked stream | Yes |
| `kubectl exec -it` | SPDY / WebSocket upgrade | Yes |
| `kubectl attach` | SPDY / WebSocket upgrade | Yes |
| `kubectl port-forward` | SPDY upgrade | Yes |

## Notes

- Each download creates a **new API Key** and automatically revokes the previous kubeconfig API key. Only one kubeconfig API key per user is valid at a time.
- Kubeconfig API Keys **dynamically inherit** the downloading user's current RBAC roles. When the user's roles change, the API Key's permissions update automatically — no need to re-download.
- The kubeconfig uses `insecure-skip-tls-verify: true` because the TLS termination happens at the Kite server, not at the Kubernetes API Server.
- For connector-based clusters, the proxy automatically routes through the connector tunnel.
- Query parameters (e.g. `?watch=true`, `?container=...`, `?command=...`) are forwarded transparently by the proxy.
- The proxy cleans `Authorization` and `Impersonate-*` headers from incoming requests and injects the cluster's own credentials when forwarding to the K8s API Server.
