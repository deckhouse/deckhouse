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

### 008-fix-cve.patch
Fix CVEs:
- CVE-2025-47914
- CVE-2025-58181
- CVE-2026-33186
- CVE-2026-26958 
- GHSA-479m-364c-43vc
- CVE-2026-34986

### 009-ratelimit-lock-unlock-users.patch

This patch adds two security features to Dex on top of the existing local-user
password policy.

1. **Per-IP rate limiter** in front of the password endpoints (`POST /token`
   and `POST /auth/{connector}/login`). Implemented as an in-memory
   token-bucket per source IP with periodic eviction of stale buckets.
   Configurable via the `rateLimit` section of the dex config (forwarded from
   the module's `userAuthn.rateLimit`); over-the-limit requests get HTTP 429
   plus a `Retry-After` header. GETs and the discovery endpoints are not
   rate-limited.

2. **Account lockout for non-local connectors** (LDAP, Atlassian Crowd, ...),
   reusing the existing `passwordPolicy.lockout` knobs. The opt-in is the new
   `applyToConnectors` field; the failed-attempt counter and the `lockedUntil`
   timestamp are stored in the per-user `OfflineSessions` resource (already
   created by Dex on every login) under new fields `email`,
   `incorrectPasswordLoginAttempts`, `lockedUntil`. The lockout is enforced as
   a pre-check in `handlePasswordLogin` and applied by
   `recordOfflineSessionFailedAttempt`; a successful login resets the state.
   The `connector/ldap` and `connector/atlassiancrowd` `Login` implementations
   were tweaked to return a partial `Identity` (UserID + Email) on a wrong
   password when the user record was found, which lets the caller index the
   counter even on a failed login.

Manual unlock is performed by patching the user's `OfflineSessions` resource
(clearing `lockedUntil` and `incorrectPasswordLoginAttempts`).