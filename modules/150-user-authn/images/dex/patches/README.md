## Patches

Dex v2.46.0 (the `dex` version in `oss.yaml`) plus one patch per zone of
responsibility, applied in file name order (`git apply /patches/*.patch`).
Numbering: 1xx storage, 2xx login, 3xx sessions, 998 dependency bumps, 999
fuzz tests.
A patch changes lines of an earlier patch only when it is a feature on top of
it; a fix goes into the patch it fixes. Each patch is the plain diff of one
commit on top of the previous patches (`git diff <commit>^ <commit>`): it
starts with `diff --git` and has no commit message, sign-off or diffstat.

Each entry names the zone, what the patch changes, its consumer in Deckhouse,
its upstream status and the patches it depends on.

The patches change the Dex CRD manifests; `crds/external/` is copied from the
patched source tree (`crds/pull_dex_crds.sh`), and the image build checks it.

### 100-storage-schema.patch

**Zone:** storage schema. The only patch that adds fields to Dex storage, for
the kubernetes and memory storages only: Deckhouse runs Dex on the kubernetes
storage and the Dex tests run on the memory one. SQL, ent and etcd ignore these
fields.

- `Password`: `previousHashes`, `hashUpdatedAt`,
  `requireResetHashOnNextSuccLogin`, `incorrectPasswordLoginAttempts` (password
  policy of local users), with the JSON names the user-authn controller already
  uses;
- `UserIdentity`: `incorrectPasswordLoginAttempts`, the failed-attempt counter
  of users of external password connectors (LDAP, Atlassian Crowd);
  `claimsRefreshedAt`, when the stored claims were last read from the
  connector (310); `AuthSession`: `mfaAuthenticators`, the authenticators of
  the second factor passed on the browser session, and `rememberMe`, the
  remember-me choice its cookie is issued with again (300);
- the devices of the user (320): `OfflineSessions.grants`, every refresh token
  of the pair by token ID (`refresh` keeps naming the last one of each client);
  `ipAddress`, `userAgent` and `deviceHash` (the hash of the device mark) of
  the sign-in on `AuthRequest`, `AuthCode`, `RefreshToken`, the refresh token
  references and `AuthSession`; `UserIdentity.knownDevices`, the browsers that
  completed a login of the pair (`hash`, `firstSeen`, `lastSeen`);
- `Client`: `allowedEmails`, `allowedGroups`; `Connector`: `allowedEmails`,
  `allowedEmailDomains`, `allowedGroups` (allow-lists of 200), with the JSON
  names the user-authn templates already render;
- `AuthRequest`: `passwordChangeBinding`, the SHA-256 hash of the cookie that
  binds the password change page of 220 to the browser of the login, and
  `pendingPasswordChange`, the change page a login goes to after its second
  factor (220);
- the generation of a (user, connector) pair (290): `UserIdentity.Generation`
  is read from the annotation `user-authn.deckhouse.io/generation`, which the
  user-authn controller writes. Dex never writes it: a new identity is created
  without it, and an update keeps the metadata of the object it read. A value
  that does not parse fails the read instead of reading as 0. `AuthRequest`,
  `AuthCode`, `RefreshToken` and `AuthSession` keep the generation of the
  pair at the login (`generation`);
- `Revocation` (`revocations.dex.coreos.com`): what was revoked (`type`:
  `pair`, `session` or `grant`), the pair (`connectorID`, `userID`, the
  `subject` of its tokens), `generation`, `sessionID`, `grantID`, `clientID`,
  `clients`, `reason`, `source`, `createdAt`. The optional interface
  `RevocationCreator` is implemented by the kubernetes storage, which names
  the object after what it revokes (`pair-<pair>-g<generation>`,
  `session-<sid>`, `grant-<refresh token ID>`, so a repeated write gets
  AlreadyExists) and labels it with `user-authn.deckhouse.io/revocation-type`
  and `user-authn.deckhouse.io/pair` (the name of the pair's `UserIdentity`),
  and by the memory storage; SQL, ent and etcd keep no revocations;
- `scripts/manifests/crds/useridentities.yaml` (printer columns: user,
  connector, email, attempts, blocked until, last login), `authsessions.yaml`
  and `revocations.yaml` (printer columns: type, reason, generation): upstream
  registers the first two from code only and ships no manifest.

The kubernetes storage tries a write that conflicts again six times, 5–10,
10–20, 20–40, 50–100, 100–200 and 200–400 ms apart (upstream: four times within
about 2 ms, shorter than one request to the Kubernetes API). Several logins of
one user at once (a browser restoring its tabs, parallel password grants) write
the same `OfflineSessions` object, and with upstream pauses some of them ended
in a 500.

The CRDs keep `x-kubernetes-preserve-unknown-fields`, so the new fields of
existing kinds change no manifest. Tests read the objects in the form the
user-authn controller and templates write them, round-trip every type through
the kubernetes storage, and check that an update never writes the generation.

**Consumer in Deckhouse:** user-authn-controller (`User` writes the `Password`
fields, `UserAccount` shows the counters, `UserOperation` raises the
generation and writes pair revocations, the revocation feed reads
`Revocation`, `DexClient` writes the allow-lists of its `OAuth2Client`); the
user-authz webhook and basic-auth-proxy (read `Revocation`);
`templates/dex-authenticator/oauth2client.yaml` and `templates/dex/config.yaml`
(allow-lists); `crds/external/` and `crds/pull_dex_crds.sh`.

**Upstream status:** Deckhouse only; the password policy and the allow-lists
are not in upstream.

**Depends on:** nothing.

### 150-transitional-offline-sessions.patch

**Zone:** transitional. Removed in Deckhouse 1.79 together with the controller
migration; nothing depends on it, and the set applies, builds and passes the
tests without it.

Keeps the storage of the Dex v2.44-based Deckhouse releases usable while the
user-authn controller moves the second factor and the lock to `UserIdentity`,
kubernetes storage only:

- the old fields `OfflineSessions.{email, totp, totpConfirmed,
  incorrectPasswordLoginAttempts, lockedUntil}` and `Password.lockedUntil` are
  carried through every update instead of being erased by the full PUT;
- a `UserIdentity` without the annotation `user-authn.deckhouse.io/migrated`
  is read with the old state filled in: the TOTP secret of the pair as
  `mfaSecrets["totp"]` when that is empty (confirmed or not, as stored), and a
  lock from `OfflineSessions.lockedUntil` (`Password.lockedUntil` for local
  users, matched by the email claim and user ID) that is still in force and
  ends later than `blockedUntil`. Any write of the identity stores them,
  including the first create on login. A storage error while reading the old
  state fails the read;
- a pair without a `UserIdentity` whose old fields hold a TOTP secret or a
  lock still in force reads as an identity with them (user ID, connector, the
  secret, the lock), so a check before the first login sees them: the lock
  check of 230 refuses the user before the password. That identity is not
  stored: an update still gets "not found" and the first create on login
  stores the values. It is at generation 0, as a pair without an identity is
  (290). A local user's `Password` is found by user ID, since the pair has no
  email claim yet, in an index of all `Password` objects that Dex lists at
  most once per 30 seconds per replica: the read comes before the password is
  checked, so a list per sign-in attempt let anyone make Dex list them all.
  Parallel reads wait for one list; a failed list is not kept. A lock written
  only to the old `Password` field reaches Dex within those 30 seconds; the
  controller writes `UserIdentity` as well, so this matters only for objects
  its copy has not reached yet.

**Consumer in Deckhouse:** user-authn-controller (package
`identitymigration`):

- it sets the annotation on every `UserIdentity` once the pair is reconciled,
  including the identities Dex creates for pairs with nothing to copy and the
  ones a `UserOperation` writes, which copies the pair in the same write; until
  then each read of such an identity costs Dex one or two extra GETs
  (`OfflineSessions`, `Password`), and each read of a pair without an identity
  one GET of `OfflineSessions`, plus, for a local user, the index above (at
  most one LIST of `Password` objects per 30 seconds);
- `Reset2FA` and `Unlock` clear the old place before `UserIdentity`, so the
  fallback has nothing to bring back;
- the TOTP authenticator ID in the Dex config is `totp`; secrets copied by Dex
  have `createdAt` = creation time of the `OfflineSessions` object, as the
  controller writes.

**Upstream status:** Deckhouse only, temporary.

**Depends on:** 100 (`toStorageUserIdentity`, which returns an error for a
generation that does not parse; context lines in `storage/kubernetes/types.go`
and `storage.go`).

### 200-client-connector-filters.patch

**Zone:** allow-lists of clients and providers: who may sign in to an
application, and which identities a provider may assert. New package
`server/allowlist` (`Apply`, `NotAllowedError` with the refusing `Rule`).

- Client lists: an email outside `allowedEmails` (exact match) is refused;
  groups are narrowed to `allowedGroups`, and an empty intersection is refused.
- Connector lists, for every connector type: the email must be in
  `allowedEmails`, or its domain (the part after the last `@`, exact, so a
  subdomain is another domain) in `allowedEmailDomains`, both case-insensitive;
  groups are narrowed to `allowedGroups`, and an empty intersection is refused.
  The client lists apply first, the connector's to what they left. A connector
  missing from storage refuses the user (fail closed). The config file takes
  the three lists on each connector.
- The login form, SPNEGO, the connector callback and the password change page
  of 220 refuse in `finalizeLogin` before `useridentity.Record`, so a refused
  login records nothing: HTTP 401 page "Access denied. Please contact your
  administrator.", which does not say which list refused. The request carries
  the narrowed groups; the user identity keeps the connector's, since every
  client shares it.
- The password, refresh-token and token-exchange grants refuse with
  `access_denied`, HTTP 401, "User is not allowed.". Refresh checks the stored
  claims before the rotation and the connector's answer after it; a refused
  user loses the client's refresh token, with its offline-session reference.
- `authflow`'s `checkAllowLists` and `renderNotAllowed` are what a login from a
  browser session is to call; that login does not apply the allow-lists yet,
  and Deckhouse keeps sessions off.

Refresh-token introspection reports the groups stored on the token, which the
rotation overwrites with the connector's groups before narrowing, as in v2.44.

**Consumer in Deckhouse:** DexClient and DexAuthenticator `allowedEmails` and
`allowedGroups`, DexProvider `allowedIdentities` (rendered by
`templates/dex/config.yaml`, `dex_connector_identity_limits`); the
identity-assign webhook of user-authz relies on the provider limits;
basic-auth-proxy (password grant) gets the 401.

**Upstream status:** Deckhouse only. v2.46.0 has `Client.allowedConnectors`
(not used by Deckhouse) and the OIDC and SAML option `allowedGroups` (groups
only, per connector type, rendered separately); neither limits emails, domains
or groups per client.

**Depends on:** 100 (fields).

### 210-user-identity-always.patch

**Zone:** user identity always. Every login records the `UserIdentity` of its
(user, connector) pair, and every login and the password, refresh-token and
token-exchange grants enforce its lock, whether `DEX_SESSIONS_ENABLED` is set
or not. The code and device-code exchanges do not check the lock again: they
finish a login that was checked; revoking what a login issued is the job of
the pair generation (290), not of a lock re-check. New package
`server/useridentity`.

- the login form, connector callbacks and SPNEGO (`finalizeLogin`) create or
  refresh the identity and refuse a blocked pair before the request is marked
  logged in; the lock is checked on what was stored, so a lock carried over by
  150 applies to a first login; a refusal there renders upstream's 500
  "Login error.", which 230 replaces with its lock page (403 or 429);
- `LastLogin` is set once every factor is done: by `finalizeLogin` when no MFA
  is configured, otherwise by the `/auth` dispatcher after the MFA step; it is
  the auth time of the login and never moves back (the password grant keeps
  it to the minute, below);
- a login from a browser session (sessions on) of a locked pair is not served
  from the session: the user goes on to the login, which refuses him (with
  230, the lock page), and the session serves again once the lock ends;
- MFA configuration no longer requires `DEX_SESSIONS_ENABLED`;
- the password grant records the identity too; the password, refresh-token and
  token-exchange grants refuse a blocked pair with `access_denied` and HTTP 429
  and keep the refresh token. A pair without an identity is not locked (with
  150, a pair whose old storage holds a lock in force reads as an identity
  with it);
- the allow-lists of 200 come first, so the identity records the connector's
  claims and a user both locked and not allowed gets the 401 of 200;
- a login writes the identity only for new claims or a `LastLogin` to move.
  The password grant keeps `LastLogin` to the minute (a stored one younger
  than a minute stays): a client may send many grants a minute for one user
  (basic-auth-proxy), and a write per grant made parallel ones conflict on the
  identity and fail with 500; `UserAccount` shows the last login to the
  minute. A browser login keeps it exact, as sessions report it as the auth
  time;
- a write of the identity that fails is tried once more 10 to 20 ms later,
  for a failure other than a conflict: the kubernetes storage retries a
  conflict itself (100). An existing identity that still cannot be
  written is read instead: the login goes on unless the pair is blocked,
  without the new claims or `LastLogin`, and Dex logs WARN `failed to record
  the user identity of a login, going on with the stored one`. Only a failed
  read fails the login.

Known limit: a password grant with `offline_access` also rewrites the
`OfflineSessions` of the user (upstream) on every grant. The retries of the
kubernetes storage (100) absorb a burst of such grants for one user; a longer
one can still fail with 500. basic-auth-proxy asks for it
(`offline_access` in its default LDAP scopes) but never uses the refresh
token.

With `skipApprovalScreen: false`, a request with no scope beyond `openid` skips
the approval page once the identity exists, as upstream does with sessions.
Deckhouse runs with `skipApprovalScreen: true`.

**Consumer in Deckhouse:** `UserAccount` (last login, second factor, lock for
every provider), `UserOperation` `Lock` and `Unlock` (write `blockedUntil`),
`staticUsers2FA` (rendered as `mfa.authenticators` without the sessions
switch), basic-auth-proxy (password grant).

**Upstream status:** Deckhouse only: upstream keeps identities, MFA and the
lock behind the experimental sessions flag.

**Depends on:** 200 (context lines and test helpers). Works with 150 when
present.

### 220-password-policy.patch

**Zone:** password policy of local users (users of the built-in password
database).

- package `server/passwordpolicy`: complexity levels `none`, `low`, `fair`,
  `good`, `excellent`, `custom` (the rules and messages of the v2.44 set),
  history of previous hashes (`previousHashes`, trimmed to
  `passwordHistoryLimit`), rotation by `hashUpdatedAt`, and the change
  `requireResetHashOnNextSuccLogin` forces after a password reset.
  `Policy.ChangeReason` is the one decision a login asks; `CheckNew` and
  `Changed` serve the page.
- the password form of the local connector sends a login whose password must
  change to `/password/change` instead of completing it.
  `passwordChangeRedirect` (`server/authflow/passwordchange.go`) is the one
  call a login path makes.
- the page says why the change is due (`forced`: the password was reset,
  `complexity`, `rotation`), asks for the new password twice (the second entry
  is compared in the browser), checks it (not empty at any level and without a
  policy, no spaces, complexity, differs from the current password, not in the
  history), stores it with `hashUpdatedAt = now`, clears
  `requireResetHashOnNextSuccLogin`, and then completes the login: it records
  the identity like the password form, creates the browser session with the
  form's "remember me" when sessions are on, and continues the auth request
  through the `/auth` dispatcher, so the second factor, the consent screen and
  the redirect to the client follow. An allow-list of 200 that refuses the
  user answers 401 as on the login form. A write of the new password that
  conflicts with another writer of the `Password` object (a failed login of
  the user counted on another replica, 230; the user-authn controller) is
  tried again, five times 5 to 20 ms apart, instead of ending in a 500 page.
- config `passwordPolicy` with the keys Deckhouse renders: `id` (accepted,
  unused), `complexityLevel` (any case; unset = `fair`, empty = `none`),
  `custom` (replaces the level's rules), `passwordHistoryLimit`,
  `rotation.interval` (`10s`, `5m`, `1h`, `3d`, `1d12h`).

The page belongs to the login that sent the user there and to the browser of
that login:

- its URL carries the auth request and the user under an HMAC of the request's
  key (step `step:password-change`);
- the login sets the cookie `dex_password_change`: a random value, HttpOnly,
  SameSite=Lax, Secure for an HTTPS issuer, path limited to the page, at most
  10 minutes and never longer than the auth request; the auth request keeps
  the SHA-256 of the value (`passwordChangeBinding`, 100);
- GET and POST open only when both hold; the cookie is compared in constant
  time, and a request without it, with another one, with a tampered URL or for
  another user gets the same 401 "Unauthorized request.";
- a successful change clears the hash and the cookie, so the cookie serves
  once; the end of the auth request ends the binding with it.

When the login has a second factor to pass, the page waits for it: the login
form finalizes the login without creating the browser session and keeps the
page on the auth request (`pendingPasswordChange`, 100); the `/auth`
dispatcher sends the browser to the second factor, then to the page, and the
page creates the session after the change. Before the second factor is done
the page answers 401, so someone who knows only the password cannot set a new
one and keep the owner out. Without a second factor (2FA off, or a client
without an MFA chain) the page opens right after the login form. A storage that
does not keep the field gets the page at once rather than a login that skips
it.

The URL alone, which passes through the browser history and the access log of
ingress-nginx, opens nothing, so the page does not ask for the current
password again: the login form has just checked it. The page refuses a login
that is already complete and continues only its own auth request, whose
redirect URI was checked when the request was made; there is no `back`
parameter. The v2.44 page redirected to any `back` address and served any user
named in the query outside a login.

Not applied (as in v2.44): the password grant (Deckhouse configures it for
LDAP only), refresh, static passwords of the config file (cannot be written).

Behaviour changes against v2.44:

- the change page asks only for the new password; after the change the login
  goes on to the application, where v2.44 sent the user back to the login form
  to type the new password once more;
- an empty new password is refused; v2.44 stored it when the level had no
  minimum length or no policy was set, and the login form then accepted an
  empty password;
- the page states the reason of the change instead of an initiator: a forced
  change follows a password reset, which the user can start too
  (`initiatorType: self`);
- the page opens only inside a login, in the browser of that login;
- answers: 401 not bound to the login, 400 new password refused, 429 locked
  (230), 303 done.

**Consumer in Deckhouse:** ModuleConfig `passwordPolicy.{complexityLevel,
custom, passwordHistoryLimit, rotation}` (`templates/dex/config.yaml`);
`UserOperation ResetPassword` (the controller sets
`requireResetHashOnNextSuccLogin`); `images/dex/src/web/templates/password_change.html`,
data contract: `.Username`, `.Reason` (`forced`, `complexity`, `rotation`, or
empty),
`.Error.Exists`, `.Error.List.{PasswordTooWeak.{Exists,Description},
OldAndNewPassAreEqual, ReusedPassword}`, `.PasswordPolicy.ComplexityRequirements`,
`.ReqPath` (also `.PostURL`, the request URI with its query),
`.BuildIdentifier` (270). The form posts `newPassword` (and `confirmPassword`,
which only the browser compares).

**Upstream status:** Deckhouse only; dexidp/dex#4270 (password policy) closed
unmerged.

**Depends on:** 100 (`Password.previousHashes`, `hashUpdatedAt`,
`requireResetHashOnNextSuccLogin`, `AuthRequest.passwordChangeBinding`), 200
(`renderNotAllowed`), 210 (`finalizeLogin` records the identity).

### 230-account-lockout.patch

**Zone:** account lockout. Locks set by an administrator and by the automatic
lockout live in `UserIdentity.blockedUntil` (upstream field);
`useridentity.CheckBlocked` (210) stays the one lock decision.

- package `server/lockout`: `Guard.Login` checks every password Dex takes: the
  login form and the password grant (basic-auth-proxy).
- a locked (user, connector) pair is refused before its password is judged, so
  the answer does not depend on whether it is right: a local user before the
  hash is compared; an LDAP or Crowd user after the connector has looked him up
  and before the result is used. Answers to an automatic lock: 429 "Account
  temporarily locked" (form), `access_denied` with 429 "Account locked"
  (password, refresh-token and token-exchange grants), as in v2.44. Answers to
  the lock of an administrator (`blockedUntil` equals `AdminLockedUntil` of
  100): 403 "Account locked by an administrator. Contact your administrator."
  (form), `access_denied` with 403 "Account locked by an administrator"
  (grants); v2.44 answered it as an automatic lock. With 150, a pair locked
  only in the storage of the v2.44-based Dex and without a `UserIdentity` yet
  is refused here like any other, with the answer to an automatic lock.
- a lock that `finalizeLogin` (210) finds gets the same page, 403 or 429,
  instead of upstream's 500 "Login error.": the only lock
  check of a connector callback and of SPNEGO, and on the login form and the
  password change page a lock set after the check before the password. The
  refusal is logged once, by `finalizeLogin`. A login from a browser session
  (sessions on) of a locked pair goes on to the login (210), which answers
  with this page.
- the password change page of 220 refuses a pair locked since the login form
  sent the user there with the same page and keeps the password.
- automatic lockout: consecutive failures are counted, for local users on
  `Password.incorrectPasswordLoginAttempts` (kept even without `lockout` in
  the config, for `UserAccount`), for users of the external connectors listed
  in `applyToConnectors` on `UserIdentity.incorrectPasswordLoginAttempts`. The
  attempt that reaches `maxAttempts` (0: the first failure) sets
  `blockedUntil` and resets the count; it creates the identity of a pair that
  never logged in. A right password resets the count.
- never shorten: the new end is the later of the lock in force and
  `now + lockDuration`. An administrator lock without an end (`9999-12-31`,
  annotation `user-authn.deckhouse.io/admin-lock`) stays as it is. When an
  automatic lock outlasts a shorter administrator lock, `blockedUntil` moves
  later and no longer equals the annotation: the controller sees an automatic
  lock from then on, and Dex answers it as one. Dex reads the annotation (100)
  and never writes it; the kubernetes storage keeps object metadata on update.
- the LDAP and Atlassian Crowd connectors return `{UserID, Email}` for a wrong
  password of an existing user; Crowd only when its lookup returns that very
  user name.
- config `passwordPolicy.lockout`: `maxAttempts`, `lockDuration` (`15m`,
  `1d`), `applyToConnectors` (connector types; local users are always
  covered).

Known limits:

- static passwords of the config file cannot be written: their failures are
  not counted;
- the kubernetes storage does not retry a conflicting write of a `Password`;
  the guard retries five times, and a count it still cannot write is logged and
  not counted.

**Consumer in Deckhouse:** ModuleConfig `passwordPolicy.lockout` (`LDAP` →
`ldap`, `Crowd` → `atlassian-crowd` in `templates/dex/config.yaml`);
`UserAccount` and `User.status` (`blockedUntil`, counters); `UserOperation
Lock` and `Unlock`; basic-auth-proxy (password grant).

**Upstream status:** Deckhouse only. Upstream checks `blockedUntil` only in
`finalizeLogin` with sessions on and has no automatic lockout.

**Depends on:** 100 (counters, `AdminLockedUntil`), 200 (`renderNotAllowed` at the
`finalizeLogin` callers), 210 (`useridentity.CheckBlocked`, `BlockedError`,
`grants.identityError`, the refusal of `finalizeLogin`), 220 (the change page,
`cmd/dex/passwordpolicy.go`).

### 240-ip-ratelimit.patch

**Zone:** rate limit. Limits failed attempts per client address on the routes
where a client proves a secret:

- `/token`: the client authentication and the password grant. Once the
  client is authenticated, the other grants (refresh, code exchange, token
  exchange, device code) give their token back whatever the answer
  (`ratelimit.Exempt`, called by the token endpoint): their tokens cannot be
  guessed, and dex-authenticator refreshes the tokens of all its users from
  the address of its pod. They are still refused while the address has used
  up its failures;
- `/auth/{connector}/login` (login form of password connectors);
- `/password/change` (the password change page of 220);
- `/mfa` and everything below it: `/mfa/totp`, `/mfa/webauthn/...`; the second
  factor had no limit (upstream marks it with TODOs in
  `server/mfa/totp.go` and `server/mfa/webauthn.go`, left in place).

A token bucket per client address, in the memory of each replica. A request
takes a token before it reaches the handler; a response below 400 gives it
back, so only failures count and a successful login or refresh never uses up
the budget. An empty bucket answers 429 with `Retry-After` (rounded up, at
least 1 s) without reaching the handler; the first refusal of a streak is
logged once. A bucket that has filled up again is dropped, so memory follows
the addresses that failed recently; no background goroutine.

The client address is the one the router resolved from `web.clientRemoteIP`
(with 245: the header of a request that presents the trusted proxy secret),
otherwise the peer address, so clients behind one ingress pod have buckets of
their own and a direct caller cannot choose its bucket.

Config keys and defaults as the v2.44-based Dex renders them:
`rateLimit.enabled`, `rateLimit.perIP.requestsPerMinute` (10),
`rateLimit.perIP.burst` (10); a value that is not positive takes the default.
New package `server/ratelimit`. A route is limited by adding its pattern to
`rateLimitedRoutes` in `server/server.go`; a pattern ending in `/` covers the
subtree.

Differences from v2.44: the second factor and the password change page are
limited; the 10-minute cleanup goroutine is replaced by dropping full buckets;
a request without any address counts against its peer address instead of one
shared bucket; at the token endpoint a failed refresh or code exchange does not
count. The lockout half of the v2.44 patch is 230.

**Consumer in Deckhouse:** ModuleConfig `rateLimit`
(`openapi/config-values.yaml`), rendered unchanged by
`templates/dex/config.yaml`.

**Upstream status:** not upstream.

**Depends on:** 220 (`authflow.PasswordChangePath`). Uses the client address of
245 when present.

### 245-client-ip-header.patch

**Zone:** client address. A proxy whose address cannot be listed in
`web.clientRemoteIP.trustedProxies` (ingress-nginx on the host network of any
node) proves itself with its client certificate,
`web.clientRemoteIP.trustedProxyCAFile` and `trustedProxyCertOrganization`,
or with a secret, `web.clientRemoteIP.trustedProxySecret`. With either set and
`trustedProxies` empty:

- the configured header (`X-Real-IP`) is trusted only on a request whose
  `X-Deckhouse-Trusted-Proxy` equals the secret (constant-time comparison,
  both non-empty), or whose connection presented a client certificate that
  verifies against the CA file for client authentication and names the
  organization. The HTTPS listener asks every client for a certificate but
  requires none and does not judge it in the handshake, so a client without
  one, or with another one, connects as before;
- when that header is missing or not a single address, the client address is
  the last address of `X-Forwarded-For`, which the proxy appended (a Gateway
  removes the client's `X-Real-IP` and appends the client address to the
  chain); a last entry that is not an address gives the peer address, since
  everything to its left came from the client;
- any other request (no secret header, a wrong one, no certificate or one
  of another CA or organization) is its peer address, whatever its headers
  say;
- the router removes `X-Deckhouse-Trusted-Proxy` from every request before a
  handler sees it, so the secret is neither logged nor passed on, and no
  startup warning names it.

With neither `trustedProxies`, the secret nor the CA file the header is
ignored, as upstream does, with a startup warning; a non-empty
`trustedProxies` keeps its upstream meaning, and a secret or CA file set with
it is ignored (startup warning); a CA file without an organization trusts
every client certificate the CA signed (startup warning).
v2.46 alone puts every client behind one ingress pod at the ingress address.
The v2.44-based Dex took the header from every peer, so any pod that reaches
the Service could name any address: a fresh bucket of the rate limit (240) on
every request, `/mfa` included, the bucket of another address used up, any
`client_remote_addr` in the sign-in log (260).

**Consumer in Deckhouse:** `templates/dex/config.yaml`
(`clientRemoteIP.header: X-Real-IP`, `trustedProxySecret`, `trustedProxyCAFile`
with the CA of kube-rbac-proxy and `trustedProxyCertOrganization:
ingress-nginx:auth`, no `trustedProxies`). The Dex Ingresses present the client
certificate of ingress-nginx (`proxy-ssl-use-controller-certificate`, or
`proxy_ssl_certificate` in the snippet of older controllers); the Gateway route
and basic-auth-proxy send the secret in `X-Deckhouse-Trusted-Proxy`. The rate
limit (240) and the sign-in log (260, `client_remote_addr`) use the address.

**Upstream status:** Deckhouse only. Upstream up to v2.45 took the header
from every peer (#3661); v2.46 takes it only from `trustedProxies` (#4933).

**Depends on:** nothing.

### 250-upstream-refresh.patch

**Zone:** upstream refresh, with sessions off (`DEX_SESSIONS_ENABLED` unset:
the refresh grant asks the connector). With sessions on, v2.46 does not call
the connector at refresh and none of this applies.

- One holder per upstream credential. A login keeps the credential it obtained
  on the refresh token it issues and only seeds an offline session that has
  none. A refresh offers the token's own credential first, then the shared one
  of the offline session, and writes a rotated credential back to the object it
  was read from. A request that lost the race for the shared credential
  continues from the winner's successor (at most two more attempts, then a
  server error: the credential is alive). A shared credential the provider
  refused and nobody rotated is dropped, so the next login of any client
  restores it. A credential rotated during a request that fails afterwards is
  still stored. Refreshes inside the reuse interval keep the stored credential.
  Upstream credentials are no longer logged. New file
  `server/tokens/refresh_upstream.go` (`RefreshStore.RotateUpstream`,
  `tokens.Upstream`).
- Detached context. The connector call runs on `context.WithoutCancel` of the
  request, so a client that hangs up does not lose a rotated refresh token:
  GitLab, GitHub and OIDC providers with rotation. It is bound to 5 s: the
  call runs under the lock of the refresh token, which the kubernetes storage
  holds for 10 s, and a call that outlives the lock lets a request on another
  replica take it and spend the same upstream credential. Half of the lock
  leaves room for the API calls under it and for clocks of the replicas that
  differ. A provider slower than that gets 500, as an unreachable one; a
  credential it rotated after the bound is lost, so the next refresh of that
  token is refused by the provider and answers `invalid_grant`.
- `invalid_grant` on a refusal (behaviour change). When the provider refuses
  the user at refresh, the token endpoint answers 400 `invalid_grant` ("The
  identity provider refused to refresh the identity.") instead of 500: refresh
  token refused (`invalid_grant`, GitHub's `bad_refresh_token`), user deleted
  or deactivated, access token revoked, `connector.UserNotInRequiredGroupsError`.
  `connector.ErrIdentityGone` carries that answer from LDAP (user not found, the
  username now names another entry), Atlassian Crowd (`USER_NOT_FOUND`,
  `INACTIVE_ACCOUNT`), GitHub, GitLab and Bitbucket Cloud (401 from the API)
  and the local password store (user deleted or replaced). GitLab, GitHub,
  OIDC, Google and Bitbucket wrap the token error with `%w`. A provider that
  fails to answer (unreachable, 5xx, timeout) stays 500 and touches no stored
  credential. The refresh token is kept either way.
- SAML. The connector data of SAML is the identity of a login, not a
  credential. A SAML login replaces it on the offline session, and every
  refresh token refreshes to the offline session's, the latest login's; the
  token's copy only seeds a session that has none.

Already in v2.46 and not duplicated: the connector is called only when a token
is actually minted. Kept anyway: the call is memoized per request, because the
kubernetes storage reruns the refresh token updater on conflicts, which would
spend a rotated upstream token twice.

Not changed: a token left with no credential at all gets 500 until a login, as
in upstream. Memory storage only (not Deckhouse): a refresh of a local user
deadlocks, as upstream does one step earlier.

**Consumer in Deckhouse:** no config. kubeconfig clients and DexAuthenticator
start a new login on `invalid_grant` instead of failing with a server error.

**Upstream status:** not upstream; the bugs fixed here (one credential shared
by all clients, the request context) are present in v2.46 with sessions off.

**Depends on:** nothing (context lines of 200 and 210 in
`server/grants/refresh.go` only).

### 260-sign-in-logging.patch

**Zone:** sign-in log. The one place that owns the attributes and the levels of
the sign-in log lines, package `server/signinlog`, across the login form, the
connector callback, SPNEGO, the password change page, the TOTP step, the
password, refresh-token and token-exchange grants, the lockout, the
allow-lists, the rate limit and the upstream refresh.

- Attributes: always `connector_id` and `client_id`; `user_id`, `username`,
  `preferred_username`, `email` when known, the email as the provider gives it;
  `groups` on a success; `user_groups`, `user_groups_count` and
  `allowed_groups` on a refusal for groups; `reason` on every refusal (`rule`
  with `not_allowed`); `err` when an error caused the line; never `user` or
  `error`. `client_remote_addr` and `request_id` come from the request context.
- Levels: a refusal caused by the user, the identity provider's answer
  included, is `WARN`; a failure of Dex or of the provider is `ERROR`; a
  success is `INFO`.
- Values from users and providers go through `pkg/logsafe`: control
  characters dropped, 256 characters, 50 items.
- Message texts are those of upstream and of the v2.44-based Dex.

| Message | Level | Reason |
|---|---|---|
| `login successful` | INFO | |
| `failed login attempt: Invalid credentials.` (form; password grant with `grant_type`) | WARN | `invalid_credentials` |
| `password change refused: request not bound to the login`, `password change refused: the second factor of the login is not done` | WARN | `invalid_credentials` |
| `failed TOTP attempt: Invalid credentials.` (wrong or reused code, with `authenticator`) | WARN | `invalid_totp` |
| `failed to authenticate`, `failed to login user`, `SPNEGO authentication error` (group refusal of the provider) | WARN | `not_in_allowed_groups` |
| the same messages, any other connector error | ERROR | |
| `access denied`, `password grant denied by access filter`, `refresh denied by client access filter`, `refresh denied by access filter after upstream refresh`, `token exchange denied by access filter` | WARN | `not_allowed` |
| `login attempt for locked account`, `login rejected for locked account`, `account locked due to too many failed attempts`, `token request for locked account`, `refresh denied: account locked` | WARN | `account_locked` |
| `client address rate limited` (with `path`; no user or client) | WARN | `rate_limited` |
| `refresh refused by the upstream provider` | WARN | `upstream_refused` or `not_in_allowed_groups` |
| `failed to refresh identity` (per upstream credential) | WARN when the provider refused, ERROR when it failed | |
| `user was forced to change password due to password object requirement` (`… complexity policy settings`, `… rotation policy settings`), `password changed successfully`, `token exchange successful` | INFO | |
| `refresh token expired`, `refresh token expired due to inactivity` (the normal end of a refresh token; upstream logs `ERROR`) | INFO | |
| `refresh token claimed twice` (reused after rotation), `trying to claim token for different client` (upstream logs `ERROR`) | WARN | |
| `refresh denied: credentials revoked`, `token request denied: credentials revoked` (code exchange, `/auth` response), `session login refused: credentials revoked` (290) | WARN | `revoked` |
| `token revoked` (290, `/token/revoke`) | INFO | |

A refusal decided in `finalizeLogin` is logged once, where it is decided, not
again as `failed to finalize login`.

The OIDC, SAML, GitHub, GitLab, Google, Bitbucket Cloud, Atlassian Crowd,
Microsoft and OpenShift connectors refuse a user outside their required groups
with `connector.UserNotInRequiredGroupsError`, which now names the user and the
groups read from the provider (`UserGroups` stays nil when the connector did not
read them). The callback answers upstream's 403 "You are not a member of any of
the required groups to authenticate." instead of 500, and a refresh the
provider refuses this way gets `invalid_grant` (250). The LDAP connector logs
a wrong password at `WARN`.

**Consumer in Deckhouse:** FAQ "How to find out why a sign-in was refused?"
(the `reason` values); log searches in Loki.

**Upstream status:** Deckhouse only. Upstream logs `login successful` with the
same keys and has `logger.excludeFields`, which can drop any of these keys.

**Depends on:** 200 (`NotAllowedError` carries the groups), 210
(`identityError`, `BlockedError`), 220 (the change page), 230 (the lockout
guard takes the client), 240 (the rate limiter), 250 (the upstream refresh
lines).

### 270-web-cache-busting.patch

**Zone:** static cache busting. Every page passes `BuildIdentifier` to the
header template, which appends `?cache=<id>` to the stylesheet URLs; this
includes the pages new in v2.46 (TOTP and WebAuthn verification, home, logout),
the error page and the password change page of 220. The identifier is a hash
of the web assets, read at startup, so it changes whenever the assets do,
including a logo mounted in EE.

The static and theme assets answer with `Cache-Control`: `public,
max-age=31536000, immutable` when the URL carries the build identifier of this
Dex (`?cache=<id>`) and the asset is served, `no-store` otherwise (another
identifier, or none, as for the logo and the favicon). In a rollout a page of
one release may load its stylesheet from a pod of the other, and the browser
keeps what it got under the URL it asked for: an old pod, without
`Cache-Control`, cannot be helped, and a new one never leaves its assets in a
browser under a URL of other ones.

**Consumer in Deckhouse:** `images/dex/src/web/templates/header.html` (uses
`.BuildIdentifier`); every page template includes the header.

**Upstream status:** not upstream.

**Depends on:** 220 (`server/templates/passwordchange.go`).

### 290-revocation.patch

**Zone:** revocation, the Dex side of global logout and instant revocation.
New package `server/revocation`.

Every credential of a (user, connector) pair carries the generation the pair
had at its login (fields of 100): the auth request, the code, the refresh
token, the browser session, and the claim `d8_gen` of the ID and access tokens
(absent for 0; every consumer reads its absence as 0, so tokens issued before
this patch stay valid). The user-authn controller raises the generation to
revoke everything issued to the pair. Only the current generation is valid: a
lower one was revoked, a higher one was issued to a record of the pair that
was deleted and created again. Nothing compares times.

- the login form, connector callbacks, SPNEGO and the password change page
  take the generation from the same write as the lock check
  (`useridentity.Record` returns it, or, when the write is given up (210),
  from the read that checks the lock); a session login takes the identity's;
- refused: the code exchange, device flow included (`invalid_grant`, "Invalid
  or expired code parameter.", as any spent code); the `/auth` response (400
  "The login is no longer valid. Please sign in again.", the auth request is
  gone); a refresh (`invalid_grant`, "The credentials of this user were
  revoked. Sign in again.", and the refresh token goes with its
  offline-session reference, `RefreshStore.RevokeGrant`; a lock is checked
  first and answers 429 as before); a session login (the session is deleted,
  the cookie cleared, the user signs in again); introspection answers
  `active: false`; userinfo answers 401 `invalid_token`;
- not refused: the password grant and token exchange authenticate anew and
  issue the current generation (`useridentity.State` reads the lock and the
  generation at once); tokens of client credentials have no pair;
- `/token/revoke` (RFC 7009) for refresh tokens, with the client
  authentication of `/token`, in the discovery document
  (`revocation_endpoint`, `revocation_endpoint_auth_methods_supported`) and in
  the rate limit of 240, where, as at `/token`, only failed client
  authentication counts. The token is looked up before its client is compared,
  so its secret is checked first. `token_type_hint` is ignored: the form of the
  token tells its type (RFC 7009 §2.1). An access or ID token gets
  `unsupported_token_type`; a token Dex does not know or no longer honors 200;
  a token of another client `invalid_grant` (WARN `token revocation refused:
  the token was issued to another client`). A `grant` revocation is written
  before the token is deleted; when it cannot be written, 503
  `temporarily_unavailable` and nothing is deleted;
- `/logout` and `/logout/callback` write a `session` revocation (the `sid` and
  the clients of the session) before anything of the session goes; when it
  cannot be written, 503 and the session is kept;
- the sign-in log reason `revoked` (table of 260);
- a storage without revocations (SQL, ent, etcd) is logged once at start,
  "revocation events are disabled: the storage does not support them", and
  records nothing.

A revocation between a check and a write lets one more token out at the old
generation: the subscribers refuse it on the revocation event, Dex on the
next refresh.

**Consumer in Deckhouse:** user-authn-controller (`UserOperation` raises the
generation; the revocation feed of DexAuthenticator); dex-authenticator
(`/token/revoke` on sign out); the user-authz webhook and the
`AuthenticationConfiguration` of kube-apiserver (`d8_gen`); basic-auth-proxy;
`templates/dex/rbac-for-us.yaml` (`revocations`: `create`).

**Upstream status:** Deckhouse only. v2.46 has no token revocation endpoint
and nothing that ends the tokens of a user at once.

**Depends on:** 100 (the `generation` fields, `Revocation`), 210
(`useridentity.Record`, the lock check that `State` joins), 240
(`rateLimitedRoutes`), 260 (the sign-in log scheme).

### 300-sessions-logout.patch

**Zone:** sessions and logout: what the browser sessions of Dex
(`DEX_SESSIONS_ENABLED`) need before Deckhouse turns them on. Nothing here
runs with sessions off.

- **logout by `client_id`**: `/logout` takes the client from `client_id` when
  there is no `id_token_hint` (RP-Initiated Logout §2), so an application signs
  its users out by sending the browser to Dex with no token in the address;
  `post_logout_redirect_uri` is checked against the `postLogoutRedirectURIs` of
  that client, and a `client_id` that contradicts the hint is refused (400);
- **confirmation**: a GET without a hint goes through when it is a top-level
  navigation from the site of Dex (`Sec-Fetch-Site` `same-origin`, `same-site`
  or `none`, `Sec-Fetch-Mode: navigate`, `Sec-Fetch-Dest: document`). The site
  is the registrable domain: any host under it, an application of another tenant
  too, can sign the user out this way, though only to an address the client
  registered. A POST without a hint goes through only from Dex's own origin
  (`Sec-Fetch-Site: same-origin`, or the `Origin` of the issuer from a browser
  that sends no fetch metadata), which is where the confirmation form posts
  from. Any other request — another site, a form of another host of the site, a
  frame, a request without fetch metadata — gets the confirmation page, whose
  form posts the client, the address to return to and the state back;
- **three states of the storage**: a session that cannot be read is not "no
  session". `/logout` and `/logout/callback` answer 503 and keep the session
  and the cookie; a login goes on interactively and keeps the cookie; a refresh
  that cannot read the offline session answers 500 and revokes nothing
  (`session.Manager.CurrentSession`);
- **a bound grant outlives the idle session**: a refresh token bound to its
  session (`refreshTokenLifetime: session`) is no longer refused because the
  session went idle, reached its lifetime or was replaced; it ends when the
  session is ended on purpose (logout revokes it, 290 announces it) or its pair
  is revoked. Introspection judges it the same way. The idle and absolute
  lifetimes close the window for signing in without a password, not the
  applications already signed in;
- **a code minted in a session is spent by logout**: the authorization code the
  browser carries to the token endpoint names the session it was minted in
  (`sid`). Redeeming it is refused as a spent code (`invalid_grant`) once that
  session has ended, so signing out also voids a code the application had not yet
  exchanged — a logout does not raise the generation of the pair, so the 290
  revocation check alone would let such a code through, and the token it would
  mint would name a session that signing out was meant to end. The same gate
  covers the device flow, which redeems its code through the same path; a code
  that names no session (a non-browser login, or sessions off) is unaffected
  (`grants.ExchangeAuthCode`, `session.Manager.Alive`);
- **an application in use keeps its session**: the refresh of a token issued
  through a live session counts as a use of the session (of the same user,
  connector and generation), so its idle timeout starts again and the browser
  signs in to the other applications without a password while one of them is
  in use, up to the absolute lifetime. The session is written at most once a
  tenth of `validIfNotUsedFor` (at least every 5 minutes); an ended session
  stays ended. Upstream counts only the logins through the session;
- **upstream logout off by default**: the OIDC connector sends the browser to
  the provider's `end_session_endpoint` only with `upstreamLogout: true`;
- **checks of the session login**: the allow-lists of the client and the
  connector (which also narrow the groups), and for a local user a password
  that is still there and need not be changed; a refused session login goes on
  to the interactive one and writes nothing to the session. The state of a
  client signed in through SSO is recorded only once the checks let the user
  in, so a refused attempt neither lists the client in the session (for logout
  and back-channel logout) nor counts as activity;
- **the time of the browser**: `max_age` and `auth_time` of a login from the
  session count from the authentication on this browser (the client's state, or
  that of the client whose session it shares), not from `LastLogin` of the pair,
  which a login on any other device moves;
- **a new secret at every login**: a login on a session the browser already
  holds, and a second factor recorded on the session, give the session a new
  secret and the browser a new cookie with the remember-me choice of the session
  (`rememberMe`), so a copy of the cookie taken before the user authenticated
  again, or between the password and the second factor, does not open it;
- **the second factor once per session**: the response of a login that passed
  the second factor records the MFA chain of its client on the browser session
  (of the same user, connector and generation); a login from the session to a
  client whose chain was passed asks for no second factor again. Upstream asks
  for it at every login from a session, as the session is made after the first
  factor. A reset of the second factor, a lock and a logout end the session with
  the generation of the pair;
- **offline access only interactively**: a client asking for `offline_access`
  whose refresh tokens are not bound to the session (kubeconfig, device flow,
  an application keeping its own tokens) is not signed in from a session, so a
  stolen session cookie mints no credential that outlives the session.

**Consumer in Deckhouse:** `templates/dex/config.yaml` (the `sessions`
parameters of the module), dex-authenticator (sign out through `/logout` by
`client_id`, `refreshTokenLifetime: session`), `DexProvider` (`upstreamLogout`),
`DexClient` (`postLogoutRedirectURIs`, `refreshTokenLifetime`).

**Upstream status:** Deckhouse only. Upstream requires `id_token_hint` for
`post_logout_redirect_uri`, ends a bound grant with the idle session, treats a
storage error as no session, logs out of an OIDC provider whenever it
advertises `end_session_endpoint`, writes the SSO state of a client before the
checks of the session login, takes `max_age` and `auth_time` from `LastLogin`
of the pair, and keeps the secret of a session for its whole life.

**Depends on:** 100 (`mfaAuthenticators`, `rememberMe`), 200 (`checkAllowLists`), 220 (the
password change flag), 290 (the `session` revocation of `/logout`).

### 310-session-identity-freshness.patch

**Zone:** freshness of the identity while sessions are on. Upstream serves the
claims stored at the last login and never asks the connector again, so a user
deleted, locked or moved between groups at the provider keeps the access Dex
gave. Here the stored claims are served while younger than
`sessions.identityRefresh.maxAge` (default 0: read at every refresh, as
without sessions), counted from `claimsRefreshedAt` (100) or, for an identity
stored before it, from the last login.

- a refresh with stale claims reads the identity through the connector the
  way a refresh without sessions does (250, with the groups whatever the token
  asked for) and stores it (`useridentity.Refreshed`);
- the provider answering that the user is gone or no longer let in
  (`connector.ErrIdentityGone`, the groups the connector requires) ends the
  pair: every browser session and refresh token of the pair is recorded as a
  revocation with the reason `IdentityGone` (290) and goes; the refresh gets
  `invalid_grant`. A refused credential alone ends only the token, as in 250;
- a provider that does not answer leaves the stored claims served up to
  `identityRefresh.gracePeriod` past `maxAge` (default 0), and the refresh
  fails with 500 after it, the token kept;
- a session login with stale claims signs a user of a provider in through the
  provider again, rather than spend here the provider credential the refresh
  tokens of the pair rotate (250); a local user's claims are read from the
  password at once (a local user's are never served stored);
- a login moves `claimsRefreshedAt` like the last login of the password grant:
  when older than a minute;
- a refresh without a stored identity reads the connector, as without
  sessions, instead of failing with 500.

**Consumer in Deckhouse:** `templates/dex/config.yaml`
(`sessions.identityRefresh`, `maxAge` defaults to `idTokenTTL`).

**Upstream status:** Deckhouse only.

**Depends on:** 100 (`claimsRefreshedAt`), 210 (`useridentity.Record`), 250
(`RotateUpstream`, `UpstreamError`), 290 (`Recorder`), 300 (the order of the
session login checks).

### 320-devices.patch

**Zone:** devices: a user signs in to one client from several devices.
Upstream keeps one refresh token per (user, connector, client): a login on a
second laptop or a second kubeconfig signs the first one out.

- **grants by login**: the password grant (basic-auth-proxy repeats it) keeps
  one grant per client and replaces it; a client bound to the browser session
  keeps one grant per session, so signing in again in the same browser replaces
  it; any other login (another browser, kubeconfig, device flow, a client
  without sessions) adds a grant. The grants a login replaces are read in the
  same write of `OfflineSessions` that adds its own, and their refresh tokens
  are deleted after it: logins of one browser at once (restored tabs) keep one
  grant of the session, and a failed write leaves no reference to a deleted
  token. The first logins of a user at once, which all find no
  `OfflineSessions`, add their grants to the one the first of them created
  instead of failing. A client keeps at most `devices.maxPerClient`
  grants of a pair (10 by default): the one used longest ago is evicted and
  recorded as a `grant` revocation with the reason `Evicted` (290), so its
  DexAuthenticator sessions end within seconds. A grant younger than
  `devices.trustedAfter` (24h by default) goes first; `0s` prefers none. The
  new `devices` section of the config takes these and `maxKnown`; the module
  renders them from its `devices` settings.
  The browser session the evicted grant was issued under drops the client from
  its client states, as a logout of the client does: it no longer lists the
  application and does not sign the browser in to it again without a password;
- **references by grant**: `OfflineSessions.grants` holds every grant, the
  refresh and revocation paths look a grant up by its ID; `refresh[client]`
  names the last grant of the client for the code that knows one. An offline
  session written before reads as the grants of `refresh`, and an older Dex that
  rewrites the object drops `grants`: the grants it does not name are refused
  at their next use;
- **expired grants go**: a refresh token past its absolute lifetime or unused
  longer than allowed is deleted with its reference when a refresh is refused
  as expired, and the garbage collection deletes the others once an hour (they
  belong to clients that never came back). Upstream keeps them, and each one
  stayed a device of the user in `UserAccount`;
- **logout of one browser**: `/logout` revokes the grants of the clients bound
  to the session that were issued in that session, not the grants the same
  clients hold on the other devices of the user;
- **the device of a grant**: the address and the User-Agent of the browser at
  `/auth` go with the auth request and the code into the grant; the password
  grant takes those of the token request;
- **`d8_gid`**: the ID and access tokens issued with a refresh token, or
  refreshed from one, carry its ID, so a token without a browser session names
  its device;
- **device mark** (`server/devicemark`): at `/auth/{connector}` every browser
  gets a long-lived random cookie (`__Host-dex_device`, 400 days, renewed at
  every login), kept apart from the session cookie, so a logout does not clear
  it. Dex keeps only its SHA-256 hash: on the auth request, the code, the grant
  and the browser session, and in `UserIdentity.knownDevices` once a login of
  the browser passed every factor (at most `devices.maxKnown` browsers, 20 by
  default, the one seen longest ago goes; `0` remembers none). A device is as old as the first login of its browser: the
  user-authn controller lists it as `knownSince`, user-api counts the 24 hours
  from it. Dex never forgets a browser; the controller removes the browsers of
  the devices someone else ended and clears the list when it revokes the pair.

**Consumer in Deckhouse:** user-authn-controller (`UserAccount.status.sessions`
with `knownSince`, `UserOperation Logout` of one grant, the known devices it
forgets), user-api (the devices of the user and the 24-hour rule).

**Upstream status:** Deckhouse only.

**Depends on:** 100 (the fields), 290 (`Recorder`), 300 (the logout of a
session), 310 (the end of a pair over every grant).

### 330-backchannel-revoked-sessions.patch

**Zone:** back-channel logout of the browser sessions that others end. Dex
sends the OpenID Connect Back-Channel Logout of a session it ends itself
(`/logout`, the identity provider no longer lets the user in); the
user-authn controller ends sessions too (`UserOperation`: Logout, Lock,
ResetPassword, Reset2FA; a deleted user) and records each in a `session`
revocation, but cannot sign a logout token.

- `storage.RevocationClaimer` (kubernetes, memory): the `session` revocations
  not claimed yet, and the claim of one. Kubernetes keeps the claim in the
  label `user-authn.deckhouse.io/backchannel-claimed`: the list is served from
  the cache of the API server (`resourceVersion=0`), the claim reads the object
  afresh and writes it back with its `resourceVersion`, so of two replicas
  claiming at once one gets a conflict and leaves the revocation to the other;
- every 5 seconds a replica claims the pending revocations and sends the
  logout token of each session (`backchannel.Notifier.NotifyRevoked`) to the
  clients of the revocation that have a `backchannelLogoutURI`, best effort as
  at `/logout`;
- the `session` revocations Dex writes itself are created claimed; one an
  earlier Dex wrote (source `dex`, not claimed) and one older than an hour are
  claimed without a logout.

**Consumer in Deckhouse:** user-authn-controller (a `session` revocation for
every browser session it ends, also when it revokes the whole pair); the
`backchannelLogoutURI` of `DexClient`; `templates/dex/rbac-for-us.yaml`
(`revocations`: `update`).

**Upstream status:** Deckhouse only.

**Depends on:** 290 (`Revocation`, `Recorder`), 300 (`backchannel.Notifier`).

### 998-fix-cve.patch

**Zone:** CVE. Dependency bumps only: the patch changes only `go.mod` and
`go.sum`. Do not edit it by hand, rebuild it.

- `golang.org/x/net` v0.59.0 → v0.60.0: GO-2026-6603 (CVE-2026-78659),
  GO-2026-6610 (CVE-2026-78660), GO-2026-6611 (CVE-2026-78669), GO-2026-6612
  (CVE-2026-78663), GO-2026-6617 (CVE-2026-97032).

Source of the list: `govulncheck ./...` on v2.46.0 (v1.8.0, vulnerability
database of 2026-10-08). The standard library findings are fixed by the Go
version of the builder image, not here. GO-2026-5932
(`golang.org/x/crypto/openpgp`) has no fix and is not imported; it is
covered by `../known_vulnerabilities.vex` for `golang.org/x/crypto@v0.57.0`.

How to rebuild it (for example, to add or drop a bump), with the Dex tag from
`oss.yaml`:

```shell
git clone --branch v2.46.0 https://github.com/dexidp/dex.git && cd dex
# Apply every patch numbered below 998, in order.
for p in $(ls /path/to/patches/*.patch | grep -v -e '/998-' -e '/999-'); do git apply "$p"; done
git add -A && git commit -q -m "patches below 998"
# Find the vulnerable modules that a newer version fixes.
govulncheck ./...
# Bump each of them (the current list is above), then tidy and build.
go get golang.org/x/net@v0.60.0
go mod tidy && go build ./...
git diff -- go.mod go.sum > /path/to/patches/998-fix-cve.patch
```

Drop a bump once Dex requires that version or newer, and keep the list above
in sync with the patch.

**Consumer in Deckhouse:** image vulnerability scans,
`known_vulnerabilities.vex`.

**Upstream status:** upstream bumps on its own schedule; drop lines as it
catches up.

**Depends on:** every earlier patch that touches `go.mod` (none now).

### 999-fuzz-handlers.patch

**Zone:** fuzz tests, `server/fuzz_handlers_test.go`. Each target sends its
input through `Server.ServeHTTP` to the route of one handler package;
`FuzzServeHTTP` takes any route and `FuzzAllHandlers` sends one input to every
route. The fuzz server has sessions, a TOTP factor, every grant type, the local
connector with a password policy and a lockout, and the rate limit of 240.
Seeds reach every handler package without fuzzing:

- a client whose allow-lists (200) refuse every user, with seeds for the
  refusals of the login form, the callback and the grants;
- the signed URL of the password change page (220), with and without the
  cookie that binds it to the browser.

| Target | Route | Package |
|---|---|---|
| `FuzzHandleToken` | `/token` | grants |
| `FuzzHandlePublicKeys` | `/keys` | discovery |
| `FuzzHandleUserInfo` | `/userinfo` | userinfo |
| `FuzzHandleIntrospect` | `/token/introspect` | introspection |
| `FuzzHandleRevoke` | `/token/revoke` | grants |
| `FuzzHandleAuthorization` | `/auth` | authflow |
| `FuzzHandleConnectorLogin` | `/auth/{connector}` | authflow |
| `FuzzHandlePasswordLogin` | `/auth/{connector}/login` | authflow |
| `FuzzHandleConnectorCallback` | `/callback[/{connector}]` | authflow |
| `FuzzHandlePasswordChange` | `/password/change` | authflow |
| `FuzzHandleApproval` | `/approval` | consent |
| `FuzzHandleMFA` | `/mfa` | mfa |
| `FuzzHandleTOTPVerify` | `/mfa/totp` | mfa |
| `FuzzHandleWebAuthn` | `/mfa/webauthn[/...]` | mfa |
| `FuzzHandleDeviceExchange`, `FuzzVerifyUserCode`, `FuzzHandleDeviceCode`, `FuzzHandleDeviceCallback` | `/device...` | device |
| `FuzzHandleLogout` | `/logout[/callback]` | logout |
| `FuzzHandleHome` | `/` | home |

`*_test.go` is not compiled into the Dex image. Run from a patched source tree:

```bash
go test -vet=off -run=^$ -fuzz=^FuzzAllHandlers$ -fuzztime=12h -fuzzminimizetime=10x ./server
```

Each input builds its own server, so bound `-fuzzminimizetime`: the default
60 s per new input shows up as minutes of `0/sec`.

**Consumer in Deckhouse:** the `dex-fuzz` image (`FUZZ_PACKAGES: "./server"`).

**Upstream status:** Deckhouse only.

**Depends on:** 100, 200, 220, 230, 240, 290; applied last so it compiles
against every patch.

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `998-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

| Module | Upstream | Patched |
|---|---|---|
| `golang.org/x/net` | v0.59.0 | v0.60.0 |
<!-- cvefix:end -->
