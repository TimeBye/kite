# 认证

Kite 支持通过 OAuth、密码用户、LDAP、MFA 和 Passkey 进行页面登录，也支持通过 API 密钥进行程序化访问。API 密钥会以一个特殊用户身份完成认证，并沿用与页面登录用户相同的 RBAC 模型。

## 页面登录

密码登录接口：

```http
POST /api/auth/login/password
Content-Type: application/json
```

请求体：

```json
{
  "username": "alice",
  "password": "change-me",
  "mfa_code": "123456"
}
```

只有当该密码用户已启用 MFA 时，才需要传 `mfa_code`。

LDAP 登录使用相同请求体，接口为 `POST /api/auth/login/ldap`。

## MFA

MFA 适用于密码用户。用户可以在账号设置中管理 MFA，也可以使用以下已认证接口：

```http
POST /api/users/me/mfa/setup
POST /api/users/me/mfa/enable
POST /api/users/me/mfa/disable
```

初始化 MFA 需要当前密码：

```json
{
  "current_password": "change-me"
}
```

初始化响应包含 `secret`、`otpauth_url` 和 `qr_code`。启用或停用 MFA 时提交 TOTP 验证码：

```json
{
  "code": "123456"
}
```

## Passkey 登录

Passkey 适用于密码用户。用户可以在账号设置中注册和删除 Passkey。

当前用户 Passkey 接口：

```http
GET /api/users/me/passkeys
POST /api/users/me/passkeys/begin
POST /api/users/me/passkeys/finish
DELETE /api/users/me/passkeys/:id
```

注册 Passkey 时需要传名称和当前密码。如果该用户已启用 MFA，还需要传 `mfa_code`：

```json
{
  "name": "Work laptop",
  "current_password": "change-me",
  "mfa_code": "123456"
}
```

Passkey 登录使用 WebAuthn begin/finish 流程：

```http
POST /api/auth/passkey/login/begin
POST /api/auth/passkey/login/finish
```

## 认证设置

管理员可以在 **设置 -> 认证** 中启用或停用密码登录、MFA 和 Passkey 登录。

同样的设置也可以通过以下接口管理：

```http
GET /api/v1/admin/general-setting/
PUT /api/v1/admin/general-setting/
```

相关请求字段：

```json
{
  "passwordLoginDisabled": false,
  "enableMFA": true,
  "enablePasskeyLogin": true
}
```

## 登录尝试封禁

基于账号密码的登录会在连续失败时按客户端 IP 临时封禁。Kite 允许 1 分钟内最多 10 次错误密码或 MFA 验证失败；第 11 次失败会封禁该客户端 IP 的账号密码登录 5 分钟。

当 Kite 运行在 Ingress 或负载均衡器后面时，封禁使用的客户端 IP 取决于 `TRUSTED_PROXIES` 配置。详见[环境变量](/zh/config/env)。

## API 密钥格式

完整 API 密钥格式如下：

```text
kite<ID>-<SECRET>
```

使用时直接把完整值放进 `Authorization` 请求头，不要加 `Bearer`。

```http
Authorization: kite12-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## 在哪里配置

拥有 `admin` 角色的用户可以在 **设置 -> API 密钥** 中创建 API 密钥。

创建后，复制完整密钥值，并把它作为 API 请求里的 `Authorization` 请求头使用。

## 权限说明

API 密钥与普通用户共用同一套 RBAC 权限模型。API 密钥分为两种类型：

- **独立 API 密钥**（在 **设置 -> API 密钥** 中手动创建）：没有 Owner，需要通过 RBAC 角色分配直接为其分配角色。通过 API 分配角色时使用 `subjectType: "apikey"`，或在 RBAC 角色分配对话框中选择"API Key"。
- **Kubeconfig API 密钥**（下载 kubeconfig 时自动生成）：关联到下载用户作为其 **Owner**，动态继承 Owner 当前的 RBAC 角色。Owner 角色变更后权限自动更新，无需重新下载。同一时间每个用户只有一个 kubeconfig API 密钥有效 —— 下载新 kubeconfig 会自动作废旧的。

- 创建独立 API 密钥本身不会自动获得任何资源权限。
- `/api/v1/...` 下的资源访问仍然会经过 RBAC 校验。
- `/api/v1/admin/...` 下的管理接口要求调用方拥有 `admin` 角色。
- 集群资源接口通常还需要传 `x-cluster-name`。

## 认证请求

示例：

```bash
curl \
  -H "Authorization: kite12-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \
  -H "x-cluster-name: demo-cluster" \
  https://kite.example.com/api/v1/pods/default
```

说明：

- `/api/v1/...` 下的资源接口通常还需要传 `x-cluster-name`。
