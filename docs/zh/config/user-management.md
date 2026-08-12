# 用户管理

Kite 支持多种用户管理方式，结合 OAuth 与本地密码用户，配合 RBAC 权限系统，实现灵活的访问控制。

## 用户类型

- **OAuth 用户**：通过第三方身份提供商（如 GitHub、OIDC 等）登录。

如何配置 OAuth 见 [OAuth 配置指南](./oauth-setup)

- **密码用户**： 通过用户名和密码登录。密码用户可以启用认证器应用 MFA，也可以注册 Passkey 用于免密码登录。
- **API 密钥**：用于脚本、CI/CD 或外部系统调用 Kite API。参见 [API 认证](../api/authentication)。

## 用户管理

拥有 **admin** 角色的用户，可在页面右上角进入设置入口，进入用户与权限管理界面。

在该界面，您可以：

- 查看当前所有用户及其角色信息
- 新增用户（仅支持密码用户）
- 禁用或删除不再需要的账号
- 修改用户的角色分配，实现权限调整

![User Management](../../screenshots/user-m.png)

## 账号安全

MFA 和 Passkey 登录默认启用，管理员可以在 **设置 -> 认证** 中管理。

所有用户都可以在账号设置弹窗中管理自己的安全设置：

- 修改显示昵称
- 使用 TOTP 认证器应用启用 MFA
- 添加或删除 Passkey
- 在启用 Passkey 登录后使用 Passkey 登录

密码用户还可以在账号设置弹窗中修改账号密码。OAuth 和 LDAP 用户通过其身份提供商管理密码。

MFA 和 Passkey 已对所有用户类型开放。密码用户通过当前密码验证安全操作。OAuth 和 LDAP 用户通过邮箱验证码（需配置 SMTP）或已启用的 MFA 验证码进行安全操作验证。

## 邮箱配置

用户可以关联邮箱地址。密码用户可以在账号设置中设置邮箱（需要验证当前密码）。OAuth 和 LDAP 用户的邮箱在登录时从身份提供商自动同步（OAuth 通过 `emailClaim` 配置，LDAP 通过 `emailAttribute` 配置）。

OAuth/LDAP 用户设置 MFA 或 Passkey 时需要邮箱，因为邮箱验证码用作安全操作的 step-up 认证。在设置 → SMTP 中配置 SMTP，或通过环境变量配置（`SMTP_HOST`、`SMTP_PORT`、`SMTP_USERNAME`、`SMTP_PASSWORD`、`SMTP_FROM_EMAIL`、`SMTP_USE_TLS`）。

## 最佳实践

- 推荐优先使用 OAuth 用户，实现统一身份管理
- 密码用户适用于特殊或临时场景
- 为所有用户启用 MFA 或 Passkey
- 定期审查用户列表和角色分配，确保权限最小化
- 禁用未使用账号，降低安全风险

权限分配可参考 [RBAC 权限管理](./rbac-config)
