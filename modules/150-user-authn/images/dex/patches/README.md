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

#### Fix CVEs

- CVE-2025-47914
- CVE-2025-58181
- CVE-2026-25680
- CVE-2026-25681
- CVE-2026-26958
- CVE-2026-27136
- CVE-2026-29181
- CVE-2026-32952
- CVE-2026-33186
- CVE-2026-33487
- CVE-2026-33814
- CVE-2026-34986
- CVE-2026-39821
- CVE-2026-39824
- CVE-2026-42502
- CVE-2026-42506
- CVE-2026-46600
- CVE-2026-56852

#### GHSA

- GHSA-hrxh-6v49-42gf

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
in particular. Such a provider invalidates the presented refresh token as soon as it is used, so a
credential has to have exactly one holder that may spend it. Dex broke that: a login wrote the
credential it obtained into two long-lived objects at once, the offline session shared by all clients
of the user and a private copy inside the issued refresh token, and read the private copy first.
As soon as any other client of the same user refreshed, it spent the shared credential and the
private copy became dead. The refresh of the affected client then failed forever, because a failed
refresh does not replace the copy it presented, and users had to log in again every `idTokenTTL`.

The fix makes every upstream credential belong to exactly one storage object, and writes the rotated
successor back to the object the spent one was read from. A login keeps its credential in the refresh
token it issues and no longer overwrites the credential of the offline session, so the clients of a
user stop sharing one credential. The offline session keeps serving the clients that have none of
their own, which is every token issued before this patch and every provider that issues a single
credential per user.

No storage schema change is involved, and that is deliberate: which object owns a credential is
derived from comparing the copy in the refresh token with the one in the offline session, so old and
new replicas can serve requests side by side during a rolling update, and a rollback keeps working.

Since a login no longer overwrites the credential of the offline session, that credential would be
written once, when the session is created, and afterwards only advanced by the refreshes that spend
it — so a credential that died for good would leave every client relying on it to be authenticated
separately, where a single login used to repair all of them at once. To keep that property, the patch
drops a shared credential once it is established to be dead, after which the existing path that seeds
an empty one applies again and one interactive login of any client restores the rest.

Establishing that requires telling apart the two ways a refresh can fail, which is why the patch also
makes the GitLab, OIDC and Google connectors wrap the error of the upstream token request instead of
flattening it into a string. A credential is treated as dead only when the provider itself answers
that it is invalid, expired or revoked (RFC 6749 `invalid_grant`) and nothing rotated it meanwhile.
A provider that fails to answer — unreachable, timing out, returning a 5xx — says nothing about the
credential, so such a failure costs a single failed request and touches neither the stored credential
nor anything else. An outage therefore cannot turn into a cluster of users logging in again, and a
connector that does not pass the error through simply never has its credentials dropped.

Key changes:

- Credentials are offered to the provider in order, the token's own one first and the shared one as a
  fallback, so a token whose private credential turned out to be spent rejoins the shared credential
  instead of failing forever. This is what heals sessions broken by the old behaviour, without a
  migration.
- A rotated credential is written back to its own object only, and a shared one only if the session
  still holds the credential the request spent, so a request that lost a race cannot overwrite a
  newer credential another client has published.
- A refresh that lost a race for the shared credential continues from the credential the winner
  published instead of failing. The failure path is bounded tightly, because it is walked by every
  client of a user whose credential died: a refresh that nobody overtook costs one extra read of the
  offline session and one short wait, further attempts are only made while the shared credential is
  actually advancing, and a provider that failed to answer costs nothing at all.
- A shared credential the provider itself refused, and that nothing rotated meanwhile, is dropped, so
  that the next interactive login restores it for every client of the user at once.
- The GitLab, OIDC and Google connectors wrap the error of the upstream token request with `%w`, which
  is what makes a refused credential distinguishable from a provider that failed to answer.
- The connector is called at most once per request, so a storage that retries the refresh token
  updater (the Kubernetes storage does that on resource conflicts) cannot spend an already rotated
  upstream refresh token twice.
- A credential rotated upstream is persisted even when the request fails afterwards, so such a
  failure stays a single failed request instead of breaking every further refresh.
- Refreshes served from the reuse interval no longer clear the stored credential, which would have
  thrown away the only copy of a credential nothing had rotated.
- An upstream rejection is no longer reported as a failure to update the refresh token.
- Upstream credentials are no longer written to the logs.
- `updateOfflineSession` no longer dereferences a missing refresh token reference.
- Regression tests for every scenario above; all of them fail without the patch.

Upstream is affected as well, including `master`: an upstream PR is to be opened on top of this
patch.