## Patches

### 001-go-mod.patch

Changing Golang version from 1.24 to version 1.23

### 002-client-filters.patch

With this patch, Dex can authorize requests for specific `OAuth2Client`s based on username and user's groups.
We use it in Dex authenticators to make `allowedUsers` and `allowedGroups` option to work.

This problem is not solved in upstream, and our patch will not be accepted.

### 003-gitlab-refresh-context.patch

Refresh can be called only one. By propagating a context of the user request, refresh can accidentally canceled.

To avoid this, this patch makes refresh requests to declare and utilize their own contexts.

### 004-static-user-groups.patch

Adding group entity to kubernetes authentication.

### 005-2fa.patch

This patch adds support for two-factor authentication (2FA) in Dex.
It allows users to enable 2FA for their accounts, enhancing security by requiring a second form of verification during the login process.

Upstream PR: https://github.com/dexidp/dex/pull/3712

### 006-oidc-httpclient-to-context.patch

This patch fixes the issue with the `insecureSkipVerify` and `rootCAs` options which do not work in OIDC connector.

Upstream PR: https://github.com/dexidp/dex/pull/4223

### 007-password-policy.patch

This patch implements password strength requirements and rotation rules
for local user accounts. The following features are added:

1. Configurable minimum password strength (using complexity checks)
2. Password expiration and forced rotation
3. Password reuse prevention
4. Account lockout after failed attempts
