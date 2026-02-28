## Patches

### 001-client-filters.patch

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on username and user's groups.
We use it in Dex authenticators to make `allowedUsers` and `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### 002-gitlab-refresh-context.patch

Refresh can be called only one. By propagating a context of the user request, refresh can accidentally canceled.

To avoid this, this patch makes refresh requests to declare and utilize their own contexts.

### 003-static-user-groups.patch

Adding group entity to kubernetes authentication.

### 004-2fa.patch

This patch adds support for two-factor authentication (2FA) in Dex.
It allows users to enable 2FA for their accounts, enhancing security by requiring a second form of verification during the login process.

Upstream PR: https://github.com/dexidp/dex/pull/3712

### 005-password-policy.patch

This patch implements password strength requirements and rotation rules
for local user accounts. The following features are added:

1. Configurable minimum password strength (using complexity checks)
2. Password expiration and forced rotation
3. Password reuse prevention
4. Account lockout after failed attempts

### 006-fix-render-error.patch

This patch changes the Internal Error message to a human-readable 'Access Denied' when login with a local user is restricted by group or email.

### 007-ipv6-host.patch

In the latest go versions (1.25.2, 1.24.8) the bug was fixed, and without this patch Dex fails with an error

Upstream PR: https://github.com/dexidp/dex/pull/4363

### 008-hide-internal-500-error-details.patch

This patch prevents internal server error details from being exposed to end users in HTTP responses.
It replaces detailed error messages (including stack traces, database errors, and internal implementation details)
with safe, user-friendly messages while ensuring all error details are properly logged server-side.

Key changes:
- Centralized safe error messages in `server/errors.go`
- Replaced `err.Error()` calls in HTTP responses with generic messages
- Added proper logging for all internal errors
- Added comprehensive tests to prevent future regressions
- Maintained OAuth2/OIDC protocol compliance

### 009-kerberos-ldap-spnego.patch

Adds optional Kerberos (SPNEGO) SSO to the LDAP connector with an opt-in SPNEGOAware hook in the password handler. Server-side validation uses `gokrb5` and a keytab only (no `krb5.conf` required). Includes principal mapping strategies and preserves the existing LDAP identity building and groups logic. Backward compatible when disabled.

### 010-fix-cves.patch

This patch fixes:

- CVE-2025-47914
- CVE-2025-58181

### 011-provide-custom-CA-to-gitlab-connector.patch

This patch allows Gitlab connector to use custom CA for HTTPS connections.

### 012-forced-password-change.patch

This patch adds a forced password change flag (`requireResetHashOnNextSuccLogin`) for local users.
The flag can be set externally (e.g. by a controller). After a successful login, the user is redirected to the password change page.
The flag is reset on successful password change.

### 013-saml-support.patch

Adds refresh token support and simplified Single Logout (SLO) to the SAML connector. The SAML connector now implements `RefreshConnector` by caching the user identity in `ConnectorData` during initial authentication and returning it on refresh. A new `SAMLSLOConnector` interface and `/saml/slo/{connector}` endpoint allow IdPs to invalidate user sessions by sending a SAML `LogoutRequest`. Includes comprehensive tests for both features.
