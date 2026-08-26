# Helm 管理

Kite 在 Dashboard 中提供基础 Helm 管理能力，包括 Chart 发现、Release 安装、升级、回滚和卸载。

## App Catalog

从侧边栏打开 **App Catalog** 可以浏览 Helm Charts。

Kite 支持两类 Chart 来源：

- **Artifact Hub**：搜索公开 Helm Charts。
- **Repositories**：浏览在 Kite 中托管的 Helm Repositories。

::: tip
使用 Artifact Hub 来源时，Kite 可能会请求 Artifact Hub 来获取 Chart 列表和详情。
:::

::: warning
Kite 只是展示 Chart 信息，不对其中的内容负责。安装或升级前，请仔细审查 Chart 详情、templates 和 values。
:::

拥有 **admin** 角色的用户可以添加或删除 Helm Repository。删除 Repository 只会从 Kite 移除这个来源，不会卸载已有 Release。如果有定时自动升级任务引用了该 Repository，Kite 会自动禁用这些任务，避免它们尝试基于不存在的 Repository 进行升级。任务会被禁用而非删除，以保留其历史记录和配置。

进入 Chart 详情后，可以查看 README、values、templates 和版本。如果 Chart package 可用，可以直接从 Kite 安装。

### 搜索与分页

两类 Chart 来源均使用服务端分页和搜索。搜索框会将查询词发送到后端，后端按名称、描述、关键字等元数据过滤后再返回结果。每页可选 10、20 或 50 条。

搜索输入有 400ms 防抖延迟后才触发 API 请求。这对 Artifact Hub 来源尤为重要——每次搜索都会查询外部 API，防抖可以避免打字过程中发送过多请求。

### 缓存与刷新

Kite 会缓存仓库索引（`index.yaml`）5 分钟，以及 Chart 内容（README、values、templates）10 分钟，以加快浏览速度。

当仓库推送了新的 Chart 版本时，由于缓存的存在，新版本可能不会立即出现。要强制 Kite 拉取最新数据：

- 点击 App Catalog 页面的**刷新**按钮。这会发送 `refresh=true` 查询参数，跳过缓存重新下载索引和 Chart 内容。
- 打开升级弹窗时也会自动触发刷新，确保升级时能看到最新的可用版本。

### 容错处理

从多个 Repository 列出 Charts 时，如果某个 Repository 不可达（网络错误、索引无效等），Kite 会记录一条警告并跳过该 Repository，而不是让整个 Chart 列表返回错误。其他正常 Repository 中的 Charts 仍会正常展示。

## Helm Releases

从侧边栏打开 **Helm Release** 可以查看已安装的 Releases。

Release 详情页会展示状态、Chart 版本、values、资源、历史记录、日志和渲染后的 manifests。

安装和升级前支持 dry-run 预览。你可以在详情页升级 Release，在历史记录中回滚，也可以删除 Release 来从集群中卸载。

### Values 合并预览

安装和升级弹窗均展示三栏：

- **Default Values**：Chart 内置的 `values.yaml`（只读）。
- **Custom Values**：你的自定义覆盖值，可编辑 YAML。
- **Merged Values Preview**：Chart 默认值与自定义值、`--set` 覆盖合并后的结果，使用 Helm 的 coalesce 逻辑计算，展示最终将应用到 Release 的值。

合并预览会在你编辑自定义值或 `--set` 参数时自动更新（有短暂防抖延迟），方便在执行 dry-run 或安装前验证覆盖效果。

### 安装弹窗中的版本选择

安装弹窗中包含版本选择器，列出所有可用的 Chart 版本。默认选中最新版本，你可以从下拉列表中选择其他版本——Kite 会获取所选版本的 Chart 详情和默认值，合并预览也会相应更新。

版本下拉列表中每一项显示版本号、AppVersion 和发布日期，当前（默认）版本会标记"Current"标签。

### 安装弹窗中的 Namespace 选择

安装弹窗使用统一的 Namespace 选择器，将浏览已有 Namespace 和创建新 Namespace 合并为一个控件。你可以按名称搜索已有的 Namespace，也可以输入新名称并选择出现的"Create"选项。当选中的 Namespace 不存在时，下方会内联显示"创建 Namespace"复选框，允许 Helm 在安装时自动创建该 Namespace。选择已有 Namespace 时复选框会自动隐藏。

### Set Values 和高级选项

安装和升级弹窗均支持 Helm `--set` 参数，用于快速覆盖值而无需编辑 YAML。点击弹窗中的**高级设置**展开该区域：

- **Set values (--set)**：每行一个 `key=value`，使用 Helm `--set` 语法（如 `image.tag=v2.0.0`、`replicas=3`、`servers[0].port=8080`）。这些值会合并到 YAML values 之上。
- **force-conflicts**：强制覆盖冲突字段（服务端 Apply）。
- **wait**：等待所有 Kubernetes 资源就绪后再完成操作。
- **失败时回滚**：操作失败时自动回滚 Release。

::: tip
Set values 的优先级高于 YAML values。适合用于快速覆盖少量值，无需修改整个 values 文件。
:::

### 异步操作

安装、升级、回滚和卸载操作以异步方式执行，以避免长时间运行的操作导致 HTTP 超时：

1. API 返回 `202 Accepted`，包含任务 ID 和初始状态（`pending`）。
2. 操作在后台 goroutine 中执行，任务状态依次经过 `pending` → `running` → `succeeded` 或 `failed`。
3. 通过 `GET /api/v1/helmrelease/tasks/:taskID` 轮询任务状态，直到状态变为 `succeeded` 或 `failed`。
4. 前端每 2 秒自动轮询一次，任务完成后更新界面。

Dry-run 操作保持同步执行，因为它们速度快，可以立即返回预览结果。

::: tip
如果任务失败，错误信息会保存在任务记录中并显示在界面上。任务记录会持久化到数据库中，便于审计。
:::

### 自动升级错误处理

为 Release 配置自动升级计划时，如果目标 Release 在集群中不存在，API 会返回 `404 Not Found` 而不是 `500 Internal Server Error`。这样可以明确表示 Release 不存在，而不是指示服务器内部问题。

## 权限

Repository 管理需要 **admin** 角色。Release 操作通过 Kite RBAC 的 `helmrelease` 资源控制（`get`、`create`、`update`、`delete`）。

::: warning
请谨慎授予 `helmrelease` 权限。Helm 操作会使用 Kite 中配置的集群凭据执行，因此拥有 `helmrelease` 的 `create`、`update` 或 `delete` 权限的用户，可能可以创建、更新或删除 Chart 渲染出的资源，即使该用户自己的 Kubernetes RBAC 权限不允许直接执行这些操作。

Kite 中配置的集群凭据也需要具备操作 Chart 渲染资源的 Kubernetes 权限。
:::
