---
outline: deep
---

# Kubeconfig 下载

Kite 允许你下载一个以 Kite 为代理访问集群的 kubeconfig 文件。这意味着你可以在本地使用 `kubectl`，无需直接暴露 Kubernetes API Server，同时 Kite 的 RBAC 权限控制依然生效。

## 如何使用

1. 点击顶部导航栏的 **下载** 图标。
2. 在弹出的对话框中，选择一个或多个你需要访问的集群。当前集群默认选中。
3. 点击 **下载** — 将生成并下载 kubeconfig 文件。

## 工作原理

```
┌─────────────┐
│   kubectl   │  使用下载的 kubeconfig
└──────┬──────┘
       │ Authorization: Bearer kite-api-key
       ▼
┌─────────────────────────────────────────┐
│         Kite Server（反向代理）          │
│  ┌───────────────────────────────────┐  │
│  │ 1. 认证：验证 API Key             │  │
│  │ 2. 集群级 RBAC：CanAccessCluster  │  │
│  │ 3. 解析 K8s API 路径              │  │
│  │ 4. 资源级 RBAC：CanAccess         │  │
│  │ 5. 注入集群认证信息               │  │
│  │ 6. 代理到 K8s API Server          │  │
│  └───────────────────────────────────┘  │
└──────┬──────────────────────────────────┘
       │
       ├──────────────┬───────────────────────┐
       ▼ 直连         ▼ Connector 隧道         ▼
┌──────────────┐                            ┌──────────────┐
│ K8s API Server│          WSS 隧道          │  Agent       │
│（直连集群）    │◀──────────────────────────│ （TCP        │
└──────────────┘   rest.Config + dialer     │  转发器）     │
                                            └──────┬───────┘
                                                   │ 原始 TCP
                                                   ▼
                                           ┌──────────────┐
                                           │ K8s API Server│
                                           │（内网集群）    │
                                           └──────────────┘
```

当你下载 kubeconfig 时，Kite 会：

1. **删除该用户之前下载 kubeconfig 生成的所有旧 API Key**，确保同一时间只有一个 kubeconfig API Key 有效。
2. 创建一个新的 **API Key**，并将其关联到你的账户作为 **Owner**。该 Key 会**动态继承**你的 RBAC 角色 —— 后续角色变更时权限自动更新，无需重新下载。
3. 生成 kubeconfig YAML 文件，其中：
   - `server` 指向 Kite 的 K8s API 代理端点（`/api/v1/clusters/{uuid}/k8s-proxy`）。
   - `token` 为新创建的 API Key。
   - `insecure-skip-tls-verify` 设置为 `true`（流量通过 Kite 转发）。

## 请求链路：`kubectl exec -it pod-name -- sh` 全流程

`kubectl exec` 是最复杂的场景，因为它使用 SPDY（或 WebSocket）协议升级。以下是一次 exec 请求的完整链路。

### 阶段 1：kubectl 发送 SPDY 升级请求

kubectl 读取下载的 kubeconfig，构造一个带 SPDY 升级头的 HTTP POST 请求：

```
POST /api/v1/clusters/{cluster-uuid}/k8s-proxy/api/v1/namespaces/default/pods/my-pod/exec?command=sh&stdin=true&stdout=true&tty=true HTTP/1.1
Host: kite.example.com
Authorization: Bearer kite<api-key>          ← 来自 kubeconfig 的 token 字段
Connection: Upgrade
Upgrade: SPDY/3.1
Content-Length: 0
```

SPDY 允许在单个 TCP 连接上多路复用双向流。这对于 `exec` 至关重要，因为 stdin/stdout/stderr/error 流需要同时传输。

### 阶段 2：Kite 服务端接收并鉴权

请求到达 Kite 的 `HandleK8sProxy` 处理器，执行六个步骤：

```
┌──────────────────────────────────────────────────┐
│              Kite 服务端                          │
│                                                   │
│  1. 认证：从 Authorization 头剥离 "Bearer "       │
│     前缀，验证 API Key（RequireAuth 中间件）       │
│                                                   │
│  2. 集群级 RBAC：CanAccessCluster(user, cluster)  │
│     — 该用户是否有权访问目标集群？                 │
│                                                   │
│  3. 路径解析：parseK8sAPIPath()                   │
│     /api/v1/namespaces/default/pods/my-pod/exec   │
│     → resource=pods, ns=default, subresource=exec │
│                                                   │
│  4. 资源级 RBAC：子资源 "exec" → verb exec        │
│     CanAccess(user, pods, exec, cluster, default) │
│                                                   │
│  5. 清除传入的认证头：                             │
│     Del Authorization, Cookie, Impersonate-*      │
│                                                   │
│  6. 构建代理 transport（强制 HTTP/1.1，           │
│     因为存在 Upgrade 头）                         │
└──────────────────────────────────────────────────┘
```

如果任何一步检查失败，Kite 返回 Kubernetes 标准的 `metav1.Status` JSON 响应，使 kubectl 能显示有意义的错误信息（如 `Error from server (Forbidden): user admin does not have permission to get pods in namespace kube-system on cluster my-cluster (get pods)`），而不是 `unknown (get pods)`。

### 阶段 3：Kite 服务端代理到 K8s API

Kite 连接 Kubernetes API Server 的方式取决于集群类型：

#### 直连集群（InCluster / Kubeconfig）

Kite 的 `UpgradeAwareHandler` 升级 TCP 连接，直接转发 SPDY 帧：

```
┌──────────────┐    SPDY 升级     ┌────────────────┐
│  Kite 服务端  │ ──────────────→ │  K8s API Server │
│  (Upgrade    │   HTTP/1.1 + TLS │  (验证            │
│  AwareHandler)│  Bearer Token    │   集群 token)    │
└──────────────┘                  └────────────────┘
```

`UpgradeTransport` 机制通过 `MirrorRequest` 注入集群自身的凭证（kubeconfig 中的 Bearer Token）到升级请求中。`MirrorRequest` 是一个特殊的 RoundTripper，它捕获认证头而不实际发送请求。

#### Connector 集群（远程，防火墙后）

对于通过 Connector 连接的集群，请求经过 Connector 隧道转发。Agent 是纯 TCP 转发器——不解析 HTTP、不管理 transport。TLS 和认证由 Kite Server 的 transport 端到端完成，使用 Agent 在 WebSocket 握手时发送的凭证：

```
┌──────────┐               ┌─────────────┐    WSS 隧道     ┌──────────┐  原始 TCP  ┌────────────┐
│   Kite   │ TLS + 认证    │ Kite Server │ ──────────────→ │  Agent   │ ────────→ │ K8s API    │
│  服务端   │ ───────────→ │ rest.Config │  remotedialer   │ (TCP     │ net.Dial  │ Server     │
│          │ (存储的凭证   │ + dialer    │  WebSocket      │ 转发器)   │           │            │
│          │  + dialer)    │             │  (加密)         │          │           │            │
└──────────┘               └─────────────┘                 └──────────┘           └────────────┘
```

**Connector 集群分步流程：**

1. **Agent 发送凭证**：WebSocket 握手时，Connector 从 kubeconfig 或集群内 ServiceAccount 提取 Kubernetes API 凭证（bearer token、CA 证书、客户端证书/密钥），通过 `X-Kite-K8s-Credentials` 头发送给 Kite Server（受 WSS/TLS 加密保护）。
2. **Kite 构建 `rest.Config`**，使用 `creds.ToRestConfig(dialer)`，其中 `dialer` 通过 remotedialer 隧道路由。config 包含真实的 kube-apiserver 地址、TLS 设置和 bearer token——全部来自存储的凭证。
3. **`Dialer`** 通过 `remotedialer` 向 Agent 发起隧道连接请求，目标地址为 `kubernetes-api`。
4. **Agent 端的 `localDialer`** 收到隧道请求后，使用 `net.Dial` 创建到 kube-apiserver 的原始 TCP 连接。不解析 HTTP、不注入请求头、不管理 transport——纯字节转发。
5. **TLS 握手**在 Kite Server 的 transport 和 kube-apiserver 之间端到端完成，透明地穿过隧道和 Agent 的 TCP 连接。

### 阶段 4：K8s API Server 响应

K8s API Server 验证集群凭证后，打开 SPDY 连接，建立多个流：

| 流 | 方向 | 内容 |
|----|------|------|
| Stream 0 | Server → Client | 错误信息（如有） |
| Stream 1 | Client → Server | stdin（你的键盘输入） |
| Stream 2 | Server → Client | stdout（命令输出） |
| Stream 3 | Server → Client | stderr（错误输出） |

这些 SPDY 帧沿原路返回：`K8s API → Agent TCP 转发器 → 隧道 → Kite UpgradeAwareHandler → kubectl`。

### 阶段 5：交互式会话

SPDY 连接建立后，kubectl 和容器运行时双向交换数据。此时 Kite 的代理是透明的——它只是在劫持的 TCP 连接上转发字节流。

## Transport 策略

K8s 客户端组件对 `config.Transport` 的使用方式不同。Kite 通过两条独立的 `rest.Config` 路径处理：

| 路径 | 组件 | Transport 使用方式 | 配置 |
|------|------|-------------------|------|
| kubectl/ktctl proxy | `UpgradeAwareHandler` | `utilnet.DialerFor(transport)` 提取 `DialContext` | `Transport` + 隧道 dialer（直连集群）或 `Dial` + 凭证（connector 集群） |
| terminal/files | `SPDY/WebSocket Executor` | 忽略 `config.Transport`，自建 transport | SPDY + `UpgradeTransport` 注入（connector）或 WebSocket → SPDY 回退（直连） |

- **`getRestConfig`**（用于 `HandleK8sProxy`）：直连集群使用 `Transport` + `MirrorRequest` 认证。Connector 集群使用 `creds.ToRestConfig(dialer)`，设置 `Host`、`TLSClientConfig`、`BearerToken` 和 `Dial`——TLS 和认证由 transport 端到端完成。
- **`buildClientSet`**（用于终端/文件）：Connector 集群的 `K8sClient` 标记 `IsConnector = true`。创建 executor 时，`buildExecutor()` 分发到 `buildSPDYExecutorWithDialer()`，手动创建 SPDY round tripper 并设置 `UpgradeTransport` 为包含隧道 dialer 的 `*http.Transport`。这有效，因为 SPDY 的 `dialerFor()` 会从 `UpgradeTransport` 中提取 `DialContext`。

此外，每个 transport 按 HTTP 版本分离：

- **普通请求**（Get/List/Watch/Create）：允许 HTTP/2，享受多路复用性能
- **升级请求**（exec/attach/port-forward）：强制 HTTP/1.1，因为 HTTP/2 规范禁止 `Upgrade` 头

## RBAC 权限映射

Kite 的反向代理执行两层访问控制：

| 层级 | 检查 | 说明 |
|------|------|------|
| 集群级 | `rbac.CanAccessCluster()` | 验证用户是否有权访问目标集群 |
| 资源级 | `rbac.CanAccess()` | 解析 K8s API 路径并检查资源级权限 |

### HTTP Method 到 RBAC Verb 的映射

Kite 的 RBAC 系统只有 6 个 verb：`get`、`create`、`update`、`delete`、`log`、`exec`。没有单独的 `list` 或 `watch` verb —— K8s 的 `list` 和 `watch` 都是 HTTP GET 请求，因此都会映射到 Kite 的 `get` verb。这意味着在 Kite RBAC 中配置 `verbs: ["get"]` 就可以涵盖 `kubectl get pods`（单个资源）、`kubectl get pods`（列表）和 `kubectl get pods -w`（watch）。

| HTTP Method | K8s Verbs | Kite RBAC Verb |
|-------------|-----------|----------------|
| `GET` | `get`、`list`、`watch` | `get` |
| `POST` | `create` | `create` |
| `PUT` / `PATCH` | `update`、`patch` | `update` |
| `DELETE` | `delete` | `delete` |

### 子资源映射

| 子资源 | RBAC Verb |
|--------|-----------|
| `/pods/{name}/exec` | `exec` |
| `/pods/{name}/attach` | `exec` |
| `/pods/{name}/log` | `log` |
| `/pods/{name}/portforward` | `exec` |

Discovery 端点（`/api`、`/apis`、`/version`、`/openapi`）仅需集群级权限，跳过资源级 RBAC。

## 错误响应

错误响应使用 Kubernetes 标准的 `metav1.Status` JSON 格式，确保 client-go 能正确解析：

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

这很重要，因为 client-go 的 `transformResponse` 对所有 4xx/5xx 响应都会尝试解码 body 为 `metav1.Status` 对象。如果 body 是有效的 Status，其 `message` 字段会替换默认的 "unknown" 文本，kubectl 就能显示实际错误信息，而不是 `Error from server (Forbidden): unknown (get pods)`。

## 支持的 kubectl 命令

所有流式协议均通过代理支持：

| 命令 | 协议 | 支持 |
|------|------|------|
| `kubectl get pods` | HTTP | 是 |
| `kubectl apply -f` | HTTP | 是 |
| `kubectl logs -f` | HTTP chunked 流 | 是 |
| `kubectl get po -w` | HTTP chunked 流 | 是 |
| `kubectl exec -it` | SPDY / WebSocket 升级 | 是 |
| `kubectl attach` | SPDY / WebSocket 升级 | 是 |
| `kubectl port-forward` | SPDY 升级 | 是 |

## 注意事项

- 每次下载都会创建一个 **新的 API Key**，并自动作废上一个 kubeconfig API Key。同一时间每个用户只有一个 kubeconfig API Key 有效。
- kubeconfig API Key **动态继承**下载用户的当前 RBAC 角色。用户角色变更后，API Key 的权限会自动更新，无需重新下载。
- kubeconfig 使用 `insecure-skip-tls-verify: true`，因为 TLS 终止在 Kite 服务端，而非 Kubernetes API Server。
- 对于通过 Connector 连接的集群，代理会自动通过 connector 隧道路由请求。
- 查询参数（如 `?watch=true`、`?container=...`、`?command=...`）会被代理透明转发。
- 代理会清除传入请求中的 `Authorization` 和 `Impersonate-*` 头，并注入集群自身的认证信息再转发到 K8s API Server。
