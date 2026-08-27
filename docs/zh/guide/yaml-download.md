# YAML 下载

Kite 支持将 Kubernetes 资源定义下载为 YAML 文件，支持单选和多选下载。你可以选择原始 YAML（集群返回的完整内容）或精简 YAML（移除 Kubernetes 自动填充的字段和默认值）。

## 功能概览

- **单选下载**：在任意资源详情页，点击头部的 **下载** 按钮，即可将该资源下载为 `.yaml` 文件。
- **多选下载**：在任意资源列表页，通过复选框选中一个或多个资源，然后点击批量操作栏中的 **下载** 按钮。多个资源会被打包成 `.zip` 压缩包，每个资源一个独立的 `.yaml` 文件。
- **两种下载模式**：
  - **原始 YAML**：保留 Kubernetes API 返回的所有字段（`managedFields` 和 `kubectl.kubernetes.io/last-applied-configuration` 注解始终会被移除）。
  - **精简 YAML**：额外移除 Kubernetes 管理的字段、默认值和其他无关注释，生成干净、可移植的清单文件。

## 精简模式

精简模式会移除由 Kubernetes 自动填充的字段，这些字段在导出资源定义时没有实际用途。这类似于 [kubectl-neat](https://github.com/itaysk/kubectl-neat) 工具。

### 精简模式移除的字段

| 字段 | 说明 |
|------|------|
| `metadata.managedFields` | 服务端字段追踪（始终移除） |
| `metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]` | 上次应用的配置注解（始终移除） |
| `metadata.creationTimestamp` | 资源创建时间 |
| `metadata.generation` | 资源版本计数器 |
| `metadata.resourceVersion` | 服务端版本追踪 |
| `metadata.uid` | 集群分配的唯一标识 |
| `metadata.selfLink` | 资源的 API 路径 |
| `metadata.ownerReferences` | 注释保留（以 YAML 注释形式保留，不删除） |
| `status` | 整个状态部分 |
| 空的 `annotations` / `labels` | 清理后为空则移除 |
| OpenAPI 默认值 | 值与 Kubernetes OpenAPI schema 默认值匹配的字段 |

### OpenAPI Schema 缓存

精简模式使用集群的 OpenAPI v2 schema 来识别并移除默认值（例如 Pod 的 `spec.restartPolicy: Always`）。schema 在集群连接建立时获取并缓存，后续所有精简下载都会复用该缓存。当集群重新连接时，缓存会自动刷新。

如果 schema 获取失败（例如网络问题），精简模式将回退为仅进行字段级清理，不移除默认值。

### 示例

使用以下 YAML 创建的 Pod：

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
spec:
  containers:
  - name: nginx
    image: nginx:1.27
```

应用到集群后，Kubernetes 会添加许多字段（`status`、`metadata.uid`、`metadata.resourceVersion`、默认的 `spec.restartPolicy` 等）。下载为**原始 YAML** 会包含所有这些字段。下载为**精简 YAML** 会将它们移除，生成接近原始定义的干净清单。
