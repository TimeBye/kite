# User Management

Kite supports multiple user management methods, combining OAuth with local password users, and working with the RBAC permission system to achieve flexible access control.

## User Types

- **OAuth Users**: Login through third-party identity providers (such as GitHub, OIDC, etc.).

For how to configure OAuth, see [OAuth Configuration Guide](./oauth-setup)

- **Password Users**: Login through username and password. Password users can enable authenticator app MFA and register passkeys for passwordless login.
- **API Keys**: Used for scripts, CI/CD, or external systems calling Kite APIs. See [API Authentication](../api/authentication).

## User Management

Users with the **admin** role can access the settings entry in the upper right corner of the page to enter the user and permission management interface.

In this interface, you can:

- View all current users and their role information
- Add new users (only password users are supported)
- Disable or delete accounts that are no longer needed
- Modify user role assignments to achieve permission adjustments

![User Management](../screenshots/user-m.png)

## Account Security

MFA and passkey login are enabled by default and can be managed by admins in **Settings -> Authentication**.

All users can manage their own security settings from the account settings dialog:

- Update their display nickname
- Enable MFA with a TOTP authenticator app
- Add or delete passkeys
- Use passkeys to sign in when passkey login is enabled

Password users can also change their account password from the account settings dialog. OAuth and LDAP users manage their password through their identity provider.

MFA and passkeys are available for all user types. Password users authenticate security operations with their current password. OAuth and LDAP users authenticate security operations via email verification code (requires SMTP configuration) or MFA code if MFA is already enabled.

## Email Configuration

Users can have an email address associated with their account. For password users, the email can be set in Account Settings (requires current password verification). For OAuth and LDAP users, the email is automatically synced from the identity provider during login (configurable via `emailClaim` for OAuth and `emailAttribute` for LDAP).

Email is required for OAuth/LDAP users to set up MFA or Passkeys, as the email verification code serves as step-up authentication. Configure SMTP settings in Settings → SMTP or via environment variables (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM_EMAIL`, `SMTP_USE_TLS`).

## Best Practices

- Recommend prioritizing OAuth users to achieve unified identity management
- Password users are suitable for special or temporary scenarios
- Enable MFA or passkeys for all users
- Regularly review user lists and role assignments to ensure minimal permissions
- Disable unused accounts to reduce security risks

For permission assignment, refer to [RBAC Permission Management](./rbac-config)
