# Authentication

Kite supports interactive login through OAuth, password users, LDAP, MFA, and passkeys, plus programmatic access through API keys. API keys authenticate as special users and follow the same RBAC model as interactive users.

## Interactive login

Password login uses:

```http
POST /api/auth/login/password
Content-Type: application/json
```

Request body:

```json
{
  "username": "alice",
  "password": "change-me",
  "mfa_code": "123456"
}
```

`mfa_code` is required only when MFA is enabled for the password user.

LDAP login uses the same request body at `POST /api/auth/login/ldap`.

## MFA

MFA is available for password users. Users manage MFA from account settings, or through these authenticated endpoints:

```http
POST /api/users/me/mfa/setup
POST /api/users/me/mfa/enable
POST /api/users/me/mfa/disable
```

Setup requires the current password:

```json
{
  "current_password": "change-me"
}
```

The setup response includes `secret`, `otpauth_url`, and `qr_code`. Enable or disable MFA by submitting a TOTP code:

```json
{
  "code": "123456"
}
```

## Passkey login

Passkeys are available for password users. Users can register and delete passkeys from account settings.

Current-user passkey endpoints:

```http
GET /api/users/me/passkeys
POST /api/users/me/passkeys/begin
POST /api/users/me/passkeys/finish
DELETE /api/users/me/passkeys/:id
```

Registration starts with the passkey name and current password. If the user has MFA enabled, `mfa_code` is also required:

```json
{
  "name": "Work laptop",
  "current_password": "change-me",
  "mfa_code": "123456"
}
```

Passkey sign-in uses the WebAuthn begin/finish flow:

```http
POST /api/auth/passkey/login/begin
POST /api/auth/passkey/login/finish
```

## Authentication settings

Admins can enable or disable password login, MFA, and passkey login in **Settings -> Authentication**.

The same settings are available through:

```http
GET /api/v1/admin/general-setting/
PUT /api/v1/admin/general-setting/
```

Relevant request fields:

```json
{
  "passwordLoginDisabled": false,
  "enableMFA": true,
  "enablePasskeyLogin": true
}
```

## Login attempt blocking

Credential-based logins are temporarily blocked by client IP after repeated invalid password or MFA attempts. Kite allows up to 10 failures within 1 minute; the next failure blocks further credential login attempts from that client IP for 5 minutes.

When Kite runs behind an ingress or load balancer, the client IP used by this limiter depends on `TRUSTED_PROXIES`. See [Environment Variables](/config/env#environment-variables) for how to configure trusted proxy ranges.

## API key format

The full API key format is:

```text
kite<ID>-<SECRET>
```

Use the full value directly in the `Authorization` header. Do not prepend `Bearer`.

```http
Authorization: kite12-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## Where to configure it

Users with the `admin` role can create API keys in **Settings -> API Keys**.

After creating a key, copy the full value and use it as the `Authorization` header for API requests.

## Permissions

API keys use the same RBAC model as regular users. There are two types of API keys:

- **Standalone API Keys** (created manually in **Settings -> API Keys**): These have no owner and require roles to be assigned directly via RBAC role assignment. Use `subjectType: "apikey"` when assigning roles through the API, or select "API Key" in the RBAC assignment dialog.
- **Kubeconfig API Keys** (auto-generated when downloading a kubeconfig): These are linked to the downloading user as their **owner** and dynamically inherit the owner's current RBAC roles. When the owner's roles change, the API key's permissions update automatically. Only one kubeconfig API key per user is valid at a time — downloading a new kubeconfig revokes the previous one.

- Creating a standalone API key does not automatically grant any resource permissions.
- Resource access under `/api/v1/...` is still checked by RBAC.
- Admin APIs under `/api/v1/admin/...` require the caller to have the `admin` role.
- Cluster resource APIs usually also require `x-cluster-name`.

## Authenticate requests

Example:

```bash
curl \
  -H "Authorization: kite12-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" \
  -H "x-cluster-name: demo-cluster" \
  https://kite.example.com/api/v1/pods/default
```

Notes:

- Resource endpoints under `/api/v1/...` usually also require `x-cluster-name`.
