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

Upstream PR: <https://github.com/dexidp/dex/pull/3712>

### 005-password-policy.patch

This patch implements password strength requirements and rotation rules
for local user accounts. The following features are added:

1. Configurable minimum password strength (using complexity checks)
2. Password expiration and forced rotation
3. Password reuse prevention
4. Account lockout after failed attempts

Password complexity supports both predefined levels (`None`, `Low`, `Fair`,
`Good`, `Excellent`) and a `Custom` level. When `Custom` is selected, the
individual checks (`minLength`, `specialCharacters`, `numbers`, `capitalized`,
`repeatedChars`) are wired through the new `WithCustomComplexity`
`PasswordPolicy` option. The login handler enforces the policy via
`Complexity.Validate(password)` so the same code path works for both predefined
and custom levels.

### 006-fix-render-error.patch

This patch changes the Internal Error message to a human-readable 'Access Denied' when login with a local user is restricted by group or email.

### 007-ipv6-host.patch

In the latest go versions (1.25.2, 1.24.8) the bug was fixed, and without this patch Dex fails with an error

Upstream PR: <https://github.com/dexidp/dex/pull/4363>

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

### 010-provide-custom-CA-to-gitlab-connector.patch

This patch allows Gitlab connector to use custom CA for HTTPS connections.

### 011-forced-password-change.patch

This patch adds a forced password change flag (`requireResetHashOnNextSuccLogin`) for local users.
The flag can be set externally (e.g. by a controller). After a successful login, the user is redirected to the password change page.
The flag is reset on successful password change.

### 012-saml-support.patch

Adds refresh token support to the SAML connector. The SAML connector now implements `RefreshConnector` by caching the user identity in `ConnectorData` during initial authentication and returning it on refresh. Also persists `ConnectorData` in `OfflineSessions` for proper session management. Includes comprehensive tests.

### 013-build-id-cache-invalidation.patch

Added cache get parameter to main CSS file URL that gets opaque dex build identifier assigned to it. This prevents stale caches from breaking the login page.

### 014-fix-cve.patch

This patch fixes:

- CVE-2025-47914
- CVE-2025-58181
- CVE-2026-26958
- CVE-2026-32952
- CVE-2026-33487 
- CVE-2026-34986
- CVE-2026-33186
- CVE-2026-29181

### 015-ratelimit-lock-unlock-users.patch

Adds per-IP rate limiting on Dex password endpoints (`/auth/{conn}/login`, `/token`)
and extends the existing local-user account-lockout to all password connectors
(`local`, `ldap`, `atlassian-crowd`).

Key changes:

- Token-bucket per-IP `IPRateLimiter` middleware, configurable via the new `rateLimit`
  section in Dex config.
- `OfflineSessions` is extended with `Email`, `IncorrectPasswordLoginAttempts`,
  `LockedUntil` (across `storage`, `kubernetes`, `etcd`, `sql` migration, `ent`);
  used as the per-user lockout store for non-local connectors.
- `passwordPolicy.lockout.applyToConnectors` selects which connector types lockout
  applies to.
- LDAP and Atlassian Crowd `Login()` returns a partial `Identity{UserID, Email}` on
  failed auth when the user exists, so the lockout counter can be indexed by a
  stable handle.

  ### 016-fix-error-template-buildid.patch

  Fix error template

### 017-fix-upstream-refresh-token-rotation.patch

This patch fixes broken token refresh with upstream providers that rotate refresh tokens, GitLab
in particular. GitLab invalidates the previous refresh token as soon as it is used, while Dex keeps
the upstream credentials in two places: the offline session shared by all clients of the user, which
is updated on every rotation, and a private copy inside every refresh token, which is a snapshot
taken at login time and never updated. Because the snapshot took precedence, the first refresh of a
client failed as soon as another client of the same user had rotated the shared credential, and it
kept failing forever, since a failed refresh does not clear the snapshot. Users had to re-login
every `idTokenTTL`.

Key changes:

- The offline session is the source of connector data; the refresh token's own copy is only a
  fallback for tokens issued before connector data was moved to offline sessions.
- The connector is called at most once per request, so a storage that retries the refresh token
  updater (the Kubernetes storage does that on resource conflicts) cannot spend an already rotated
  upstream refresh token twice.
- Connector data rotated upstream is persisted even when the request fails afterwards, so such a
  failure stays a single failed request instead of breaking every further refresh of the user.
- `updateOfflineSession` no longer dereferences a missing refresh token reference.
- Regression tests for all three scenarios.

Upstream is affected as well, including `master`: an upstream PR is to be opened on top of this
patch.