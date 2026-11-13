## Patches

Dex v2.46.0 (the `dex` version in `oss.yaml`) plus one patch per zone of
responsibility, applied in file name order (`git apply /patches/*.patch`).
Numbering: 1xx storage, 2xx login, 3xx sessions, 4xx connectors, 998
dependency bumps, 999 fuzz tests.
A patch changes lines of an earlier patch only when it is a feature on top of
it; a fix goes into the patch it fixes. Each patch is the plain diff of one
commit on top of the previous patches (`git diff <commit>^ <commit>`): it
starts with `diff --git` and has no commit message, sign-off or diffstat.

Each entry names the zone, what the patch changes, its consumer in Deckhouse,
its upstream status and the patches it depends on.

The patches change the Dex CRD manifests; `crds/external/` is copied from the
patched source tree (`crds/pull_dex_crds.sh`), and the image build checks it.

### 100-storage-schema.patch

**Zone:** storage schema. The patch that adds fields to Dex storage, apart
from the one prefix of a connector (435), the last use of a WebAuthn
credential (340, inside the credentials every storage keeps as JSON) and the
lease on the upstream credential of an offline session (250,
`OfflineSessions.upstreamLease`), for
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
  `pair`, `session` or `grant`; the user-authn controller also writes
  `connector`, every token of a token-exchange connector, which Dex does not
  read), the pair (`connectorID`, `userID`, the
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

### 120-signing-keys-cache.patch

**Zone:** storage reads. The local signer reads the signing keys through a
cache in memory (`server/signer/keys_cache.go`).

Every token Dex issues asks the signer for the keys twice, for the algorithm
and for the signature. In v2.46 the signer, opened over the storage in
`cmd/dex/serve.go`, reads them from the storage every time, bypassing the key
cache of the server: four reads of the `signingkeies` object from the
Kubernetes API for every login and every refresh (two tokens each), measured
at 4.0 of 17.8 requests per refresh. The v2.44-based Dex read them through
the cache of the server.

- The keys are served from memory until their `NextRotation`; keys past it
  are read every time until rotated, a failed read is not kept.
- The rotation of the signer refreshes the cache every time it looks at the
  keys, every 30 seconds, so keys replaced other than by rotation (the object
  deleted and created by another replica) are served within that time.
- `TestTokenRequestsReadNoSigningKeys`: no read of the keys for a password
  login and for refreshes (4 each without the cache).

**Consumer in Deckhouse:** every token request; the API server and its APF
level of Dex (`templates/dex/flowcontrol.yaml`).

**Upstream status:** Deckhouse only; upstream v2.46.0 reads the keys from the
storage for each token.

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

The password form's "Select another login method" link goes back to the page
that offers the client its connectors, so `handleConnectorLogin` shows it only
when that page would offer more than one: the stored connectors that allow the
grant and that the client's `allowedConnectors` lets through. Upstream counts
every connector of the server, which left a link to the same form for a client
limited to one provider. The page and the link share one filter
(`offeredConnectors`), and only the password form, the one page with the link,
reads the connectors for it.

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
- an MFA authenticator that cannot be built stops the start: a config that
  does not parse, a WebAuthn authenticator that Dex refuses (a timeout that
  is no duration, for example) or an unknown type. Upstream logged the error
  and went on without the authenticator, and a login that asked for it then
  passed with no second factor, since the chain of a login keeps only the
  authenticators that exist. `buildMFAProviders` of `cmd/dex/serve.go`
  returns the error;
- WebAuthn registration refuses an identity that already has a credential for
  the authenticator (a further key comes with 360, after a registered key
  confirms the sign-in). Upstream checks that only when it chooses the HTML mode,
  and `register/begin` and `register/finish` nil the stored credentials, so a
  password alone can add a second key and mark the login's second factor done.
  Both endpoints refuse (`403` and `400`), and `register/finish` checks again
  inside `UpdateUserIdentity`, so two ceremonies started while the account had
  no key cannot both store one;
- for the same reason, the chain of a client that names an authenticator Dex
  does not have (an `mfaChain` of a client in the storage; the configuration
  checks only its own chains) stops the login with a 500 instead of skipping
  the authenticator; the page says that the second factor of the application
  is misconfigured, instead of upstream's "Database error." (with 280, the
  page carries `ErrCode` `mfa_misconfigured`, which the theme explains in the
  language of the frontend), and the log line names the client and the
  authenticator. `validateMFA` refuses a `connectorTypes` entry that is
  no connector type of Dex (`crowd` for `atlassian-crowd`), which would bind
  the authenticator to no connector;
- a chain that asks for every factor is passed in the order for the user
  (`server/mfa/order.go`): first the factors the user has, then the
  registration of the missing ones, each group in the order of the chain. A
  TOTP secret not confirmed yet counts as missing. Upstream sent every login
  to the first factor of the chain, so a user who had only a key registered a
  code with the password alone before the key was asked for. The factors a
  login passed are kept in the auth request, next to the WebAuthn session
  (`WebAuthnSessionData`, key `deckhousePassed`), and a registration before
  every factor the user has is passed sends the page back to `/mfa`, and the
  WebAuthn API answers it with `403`, also inside `UpdateUserIdentity`. A user
  with every factor, or none, passes the chain in its order, as upstream. The
  passes record the user and the connector they belong to
  (`deckhousePassedUserID`, `deckhousePassedConnectorID`): passes of another
  user, or recorded without one, count for nothing;
- a login into an auth request that another user has signed in is refused with
  400 ("This sign-in is already done for another account. Start the sign-in
  again."): the login form checks it before the password policy, and
  `finalizeLogin` checks it for every connector and again inside
  `UpdateAuthRequest`. Upstream overwrote the claims of the request and kept
  `MFAValidated`, so a user who passed their own second factor and then posted
  the password of another user to the same `state` got a code for that user
  without the second factor of that user. The same user signing in again (a
  second click, the back button) goes on;
- the password grant records the identity too; the password and token-exchange
  grants refuse a blocked pair with `access_denied` and HTTP 429, the
  refresh-token grant with `invalid_grant` and HTTP 400 and the same description
  (RFC 6749 §5.2: a client ends the session on it, where it waits on a 429),
  and all keep the refresh token. A pair without an identity is not locked
  (with 150, a pair whose old storage holds a lock in force reads as an
  identity with it);
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
`twoFactor` (rendered as `mfa.authenticators` without the sessions
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

The kept page opens only for the user the login is complete for, and the login
form writes neither the binding nor the kept page into a request another user
has signed in (400 "This sign-in is already done for another account. Start the
sign-in again.", as in 210). So the login of another user posted to the same
`state` at the same time, which writes its page before the first login
completes and is refused only after, leaves no page that changes the password
of that user after the second factor of the first one.

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
  (password and token-exchange grants), as in v2.44, and `invalid_grant` with
  400 "Account locked" (refresh-token grant, see 210). Answers to the lock of
  an administrator (`blockedUntil` equals `AdminLockedUntil` of 100): 403
  "Account locked by an administrator. Contact your administrator." (form),
  `access_denied` with 403 "Account locked by an administrator" (password and
  token-exchange grants; a refresh gets `invalid_grant` with 400 and the same
  description); v2.44 answered it as an automatic lock. With 150, a pair locked
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

**Zone:** upstream refresh. With sessions on or off, a refresh asks the
connector only when the claims stored for the pair are older than
`identityRefresh.maxAge` (310); then everything below applies.

- One upstream credential per pair (user, connector), the shared one of the
  offline session, spent by one request at a time. RFC 6749 §6: the client
  must discard a refresh token once a new one is issued; RFC 9700 §4.14.2: a
  provider may revoke the whole grant when a spent token is presented again.
  - A login replaces the shared credential with the one it obtained when that
    carries a refresh token (`connector.RefreshCredentialReporter`: OIDC and
    GitLab tell; other connectors count any data): a provider may end the
    refresh tokens of the earlier logins of the user to the same client, as
    dex does, so the latest login's is the one known to be alive. A login
    without a refresh token (Google without consent, `promptType: login`)
    keeps the shared one. A login that only seeded an empty session left a
    dead shared credential in place, and two applications of one user sent
    each other to the login. The identity of a SAML login always replaces the
    shared one (D10). The code exchange after the login leaves the session
    alone, so it does not undo a rotation another client made in between.
  - A refresh token keeps no copy of the credential (SAML keeps its identity):
    a copy is spent as soon as another client refreshes, and presenting it
    then is the reuse some providers revoke the grant for. A token issued
    before keeps its own copy, offered only when the session has none, moved
    to the session once spent, and dropped once the provider refuses it.
  - A lease on the offline session (`upstreamLease` of the OfflineSessions
    object; the SQL, ent and etcd storages do not keep it) lets one request of
    the pair, on any replica, read the connector at a time. It lasts 10 s: the
    5 s of the connector call, 3 s for a write and 2 s for clocks of the
    replicas that differ. The holder renews it in the background every third
    of that while it holds it, so a slow storage cannot let it expire under a
    live call; a holder that finds it lost before the call (taken over or not
    renewed in time) never presents the credential it read, but starts over
    from the current one, twice at most, then answers 503. The first write
    after the call publishes the successor, only over the credential the
    request spent; the lease is released once the stored claims and the token
    are written too, so a request waiting for it finds the claims fresh. A
    request that waits for the lease looks again after 50 ms, backing off to
    every 500 ms (24 looks in a lease time, each a read of the storage, where
    looking every 50 ms made 201), and, once it holds it, serves the claims the holder stored if they
    are fresh now, or reads the connector with the credential the holder
    left; after the lease time by the clock (the storage reads between the
    looks do not stretch it) it gets 503. A refresh inside the reuse
    interval mints nothing, and a pair with no credential to spend (a local
    user) has nothing to guard: neither takes the lease. The connector is
    always asked for offline access, so it returns the successor whatever
    the client asked for.
  - A request that lost the race for the shared credential to a dex without
    the lease (during an update) waits for the winner's successor up to the
    5 s of a connector call by the clock, looking again with the same backoff
    (14 looks; counting the looks alone made the wait 6.5 s on Kubernetes),
    then continues from it
    or gets 503 (`temporarily_unavailable`: the credential is alive). A shared
    credential the provider refused and nobody rotated is dropped, so the next
    login of any client restores it.
  - A credential is dead only on `invalid_grant`, GitHub's
    `bad_refresh_token`, or 400 `invalid_request` with "already been claimed"
    in its description, the answer of dex up to v2.45 to a refresh token it no
    longer holds (RFC 6749 §5.2). An `invalid_grant` "Account locked" or
    "Account locked by an administrator", the refusal of a locked account by
    Deckhouse dex (210; 1.77 with its patch 019), refuses the refresh but
    leaves the credential alive: it is not dropped and no rotation is waited
    for, so the refresh goes on once the lock ends, without a login.
  - A rotated credential is stored as soon as the token endpoint returned it:
    OIDC and GitLab report it with `connector.RefreshedError` when building
    the identity fails afterwards (user info, groups, the keys of the
    provider), and the refresh stores it before it answers.
  - The GitLab connector asks a login for the scopes of the groups (`openid`,
    and `read_api` with `inheritedGroups`) whatever the client asked for: the
    credential serves every client of the user.
  - Refreshes inside the reuse interval keep the stored credential. Upstream
    credentials are not logged. New files `server/tokens/refresh_upstream.go`
    (`RefreshStore.RotateUpstream`, `tokens.Upstream`) and
    `server/tokens/refresh_lease.go`.
- Detached context. The connector call runs on `context.WithoutCancel` of the
  request, so a client that hangs up does not lose a rotated refresh token:
  GitLab, GitHub and OIDC providers with rotation. It runs outside the
  storage transactions, under the lease of the offline session, and is bound
  to 5 s so that it and the writes after it end within the lease. A provider
  slower than that gets 503, as an unreachable one; a credential it rotated
  after the bound is lost, so the next refresh is refused by the provider and
  answers `invalid_grant`.
- Three answers to a failed upstream refresh (behaviour change). The token
  endpoint answers:
  - 400 `invalid_grant` ("The identity provider refused to refresh the
    identity.") instead of 500 when the provider answered and the answer cannot
    refresh the identity until the user logs in again: refresh token refused
    (`invalid_grant`, GitHub's `bad_refresh_token`), any other 4xx answer to the
    token request with an OAuth2 error of RFC 6749 §5.2 in its body
    (`invalid_request`, `unauthorized_client`, `unsupported_grant_type`,
    `invalid_scope`; `connector.OAuth2Refusal`) but 408, 429, `invalid_client`
    and any answer with 401, `access_denied` with a 4xx other than 401 and 408
    (`connector.AccessDenied`: the refusal of a locked account by Deckhouse dex
    up to v1.78, 429 or 403), user deleted or deactivated, access token revoked,
    `connector.UserNotInRequiredGroupsError`, nothing to refresh with (no
    credential stored; a connector that needs one, OIDC or GitLab, is not
    asked then). `connector.ErrIdentityGone` carries the answer about the
    user from LDAP (user not found, the username now names another entry),
    Atlassian Crowd (`USER_NOT_FOUND`, `INACTIVE_ACCOUNT`), GitHub, GitLab and
    Bitbucket Cloud (401 from the API) and the local password store (user
    deleted or replaced). `connector.ErrRefreshRefused` carries the rest from
    the OIDC connector: no upstream refresh token (the provider issued none, for
    example with refresh tokens off for the client), a token response that
    cannot be read, an ID token that does not verify for a reason other than
    the keys (`aud`, `iss`, expiry, signature), an ID token or user info without
    the claims of the user, user info refused with 4xx. GitLab, GitHub, OIDC,
    Google and Bitbucket wrap the token error with `%w`.
  - 503 `temporarily_unavailable` instead of 500 when the provider did not
    answer: the connection failed or timed out, the provider answered 408, 429
    without `access_denied` or a 5xx status (`connector.Unavailable`; the OIDC
    connector marks a 429 or 5xx of user info with
    `connector.ErrUpstreamUnavailable`, since go-oidc reports that status only
    in the text of the error), the OIDC connector could not fetch the keys of the provider to verify the ID token of the answer
    (the keys are fetched on every start of dex and every rotation of the keys;
    go-oidc v3.21.0 passes that failure on only as the text `fetching keys`, so
    the connector matches the text), other clients kept rotating the shared
    credential, or the password store of a local user did not answer (any
    failure of the read but "not found", the Kubernetes API throttling dex with
    429 among them; the local connector has no credential, so without the mark
    its failure counted as nothing to refresh with and ended the session), or
    the provider refused the client credentials of dex (`invalid_client`, or
    an OAuth2 error answered with 401). The last is a misconfiguration of dex,
    which a login hits too, so the sessions are kept until it is fixed. The
    description of this answer names none of `invalid_grant`, `invalid_client`
    and `access_denied`, which DexAuthenticator takes for a final refusal.
    `unauthorized_client` and `unsupported_grant_type` stay a refusal: they may
    last while logins work, and a 503 would keep users on an error page.
    Nothing stored is touched.
  - 500 as before for a failure the connector does not explain (LDAP errors
    other than a missing user, a plain error text, a 4xx answer to the token
    request without an OAuth2 error of the provider, which a proxy in front of
    the provider gives, `{"error":"Forbidden"}` of a Spring gateway among them)
    and for the storage of dex. A failure to read the connector of the token
    from the storage, but "not found", is 500 `server_error` too
    (`connectors.ErrStorageUnavailable`); it was 400 `invalid_request`,
    "Requested connector does not exist.", for every grant.
  - 400 `invalid_grant` instead of 500 for a refresh token whose ID no
    Kubernetes object can be named after (a garbage string, which is taken for
    a raw ID): the Kubernetes storage answers `storage.ErrNotFound` for it
    without asking the API server, as the other storages do.
  The refresh token is kept either way. The `failed to refresh identity` line
  is WARN for the first answer and ERROR for the others (260).

  A refresh token the provider issued is kept even when building the identity
  from the answer fails afterwards for a temporary reason (the keys of the
  provider cannot be fetched to verify the ID token, user info answers 429 or
  5xx or does not answer in time, the groups of GitLab cannot be read): the
  connector reports it with `connector.RefreshedError`, and the refresh stores
  it before it answers 503. A provider whose refresh tokens serve once
  (Keycloak with "Revoke Refresh Token", Okta after its grace period, Auth0
  with reuse detection) therefore accepts the next refresh.
- SAML. The connector data of SAML is the identity of a login, not a
  credential. A SAML login replaces it on the offline session, and every
  refresh token refreshes to the offline session's, the latest login's; the
  token's copy only seeds a session that has none.

Already in v2.46 and not duplicated: the connector is called only when a token
is actually minted. Kept anyway: the call is memoized per request, because the
kubernetes storage reruns the refresh token updater on conflicts, which would
spend a rotated upstream token twice.

**Consumer in Deckhouse:** no config. kubeconfig clients and DexAuthenticator
start a new login on `invalid_grant` instead of failing with a server error.
DexAuthenticator keeps a session on 503 (and 500) for one `--cookie-refresh`
period from the first failure (its patch 002).

**Upstream status:** not upstream; the bugs fixed here (one credential shared
by all clients, the request context) are present in v2.46 with sessions off.

**Depends on:** nothing (context lines of 200 and 210 in
`server/grants/refresh.go` only).

### 260-sign-in-logging.patch

**Zone:** sign-in log. The one place that owns the attributes and the levels of
the sign-in log lines, package `server/signinlog`, across the login form, the
connector callback, SPNEGO, the password change page, the TOTP step, the
password, refresh-token and token-exchange grants, the lockout, the
allow-lists, the rate limit and the upstream refresh. The zone also takes the
answer of a connector to a user without groups (the LDAP line and the SAML
assertion without the attribute below): the level of the line and whether the
user is refused are one decision, so they stay in one patch rather than split
between this one and the connector zone.

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
- A user in no group is not a failure. The LDAP connector logs `ldap: groups
  search returned no groups` at `DEBUG` (upstream: `ERROR`, at every login of
  such a user), and the SAML connector returns an empty list of groups when the
  assertion carries no attribute for them, where upstream fails the login with
  `no attribute with name "groups"` and the user sees a 500. With `allowedGroups`
  set, a user without the attribute is in none of the allowed groups and gets
  the refusal of the provider, as any user outside them.
- The lines of the connectors that print the data of a user are at `DEBUG`
  (upstream: `INFO`), which the default level of Dex does not show: SAML
  `parsed and verified saml response attributes` (every attribute: email, name,
  groups, roles); LDAP `performing ldap search` (the filter carries the entered
  username or the DN), `username mapped to entry` and `LDAP search returned no
  groups`; Kerberos `principal authenticated`, `LDAP user found` and `SPNEGO
  login succeeded`; GitHub `user in org but no teams` and `user not in org or
  application not authorized to read org data`. LDAP `no results returned for
  filter`, the line of an unknown user, is at `DEBUG` too (upstream: `ERROR`):
  its filter carries the entered username, and the attempt is still logged as
  `failed login attempt: Invalid credentials.` at `WARN`. The lines of the
  sign-in log above keep their levels.

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
| `invalid 'state' parameter provided` (280, a step of an expired sign-in; with `path`, and `connector_id` when the address names it; no client or user; upstream logs `ERROR` and only for the form and the callback) | WARN | `login_expired` |

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

### 280-login-request-expired.patch

**Zone:** the expired sign-in. A sign-in lives `expiry.authRequests` from its
start (10 minutes in Deckhouse); a step that comes later finds no auth request
(after the garbage collection) or one past its expiry (before it). Upstream
answers the login form and the connector callback with 400 "Requested resource
does not exist." and an `ERROR` line, the approval and the dispatcher with
"User session error.", the second factor with 500 "Database error.", the
password change page (220) with "Requested resource does not exist." and the
authorization response with "User session has expired.". The patch answers all
of them with one page:

- `templates.LoginExpired` (`server/templates/expired.go`) renders the error
  page with 400, the message "The sign-in has expired. Go back to the
  application and sign in again.", `ErrCode` `login_expired` and `ReturnURL`.
  The error page gets `ErrCode` and `ReturnURL` on every error, empty on the
  others, so a theme tells the case apart without matching the message; a theme
  that does not know the code shows the message. `templates.RenderErrorWithCode`
  renders another case with its own code: the misconfigured second factor of
  210 (`mfa_misconfigured`, 500).
- `ReturnURL` is the application to go back to: the origin (`https://host[:port]/`)
  of the `redirect_uri` in the connector selection link of the login form
  (`back`, `/auth?client_id=...&redirect_uri=...`). The connector selection
  itself is not offered: it would start the same sign-in of the client, whose
  own state has expired too, so without the sessions of Dex the user would
  enter the password twice. Dex puts `back` into the login form only when it has
  more than one connector, and the other steps have none: then there is no button.
- **Security of `ReturnURL`:** `back` comes from the address of the form, so it
  is checked as `/auth` checks a sign-in (`loginExpiredReturnURL`): the client
  exists in the storage, and the `redirect_uri` passes `validateRedirectURI` and
  is https, which makes it one of the `RedirectURIs` of the client exactly (the
  loopback, out-of-band and device addresses a public client may use are not
  https). Only the scheme and the host are kept; the path, the query, the
  fragment and any userinfo are dropped, and `html/template` escapes the link.
  Anything else (no `back`, a link that does not parse, an unknown client, an
  address the client did not register, http) gets the page without the button.
- The login form and the connector callback also refuse an auth request past
  its expiry before the garbage collection, before the credentials are checked.
  The authorization response keeps its check and gets the same page; so does a
  response whose auth request another request has just removed.
- Logging: one `WARN` line `invalid 'state' parameter provided` with `reason`
  `login_expired` and `path`, `connector_id` when the address names one
  (`signinlog.LoginExpired`). The client and the user are unknown: they were in
  the auth request.

Other 400 and 404 errors of Dex keep their messages and pages. The upstream
tests of the approval that check "User session error." check the new page.

**Consumer in Deckhouse:** `images/dex/src/web/templates/error.html`, which
explains `login_expired` in the language of `frontend.extra.lang` and shows
the **Return to the application** button when there is a `ReturnURL`; FAQ "How
to find out why a sign-in was refused?" (`login_expired`).

**Upstream status:** not upstream. Upstream has no code for the error page and
logs the missing auth request at `ERROR`.

**Depends on:** 220 (the password change page), 260 (`server/signinlog`). 290,
300 and 320 apply to it with shifted line numbers and blob hashes only.

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

**Zone:** freshness of the identity, with sessions on or off. Upstream serves
the claims stored at the last login and never asks the connector again, so a
user deleted, locked or moved between groups at the provider keeps the access
Dex gave. Here the stored claims are served while younger than
`identityRefresh.maxAge` (default 0: read at every refresh), counted from
`claimsRefreshedAt` (100) or, for an identity stored before it, from the last
login. The top-level `identityRefresh` of the config file is read whether
sessions are on or not; `sessions.identityRefresh` stays accepted, and the
top-level one wins. With sessions off this is what spares the upstream
credential of the pair (250) a call at every refresh of every client: a user
disabled at the provider keeps refreshing tokens up to `maxAge`.

- a refresh with stale claims reads the identity through the connector the
  way a refresh without sessions does (250, with the groups whatever the token
  asked for) and stores it (`useridentity.Refreshed`);
- the provider answering that the user is gone or no longer let in
  (`connector.ErrIdentityGone`, the groups the connector requires) ends the
  pair: every browser session and refresh token of the pair is recorded as a
  revocation with the reason `IdentityGone` (290) and goes; the refresh gets
  `invalid_grant`. A refused credential alone ends no session: the token gets
  `invalid_grant`, and a shared credential nobody rotated is dropped, as in 250;
- a provider that does not answer leaves the stored claims served up to
  `identityRefresh.gracePeriod` past `maxAge` (default 0), and the refresh
  fails with 503 after it (500 for a failure the connector does not explain),
  the token kept; an answer that cannot refresh the identity (250) is
  `invalid_grant` at once;
- a session login with stale claims signs a user of a provider in through the
  provider again, rather than spend here the provider credential the refresh
  tokens of the pair rotate (250); a local user's claims are read from the
  password at once (a local user's are never served stored);
- a login moves `claimsRefreshedAt` like the last login of the password grant:
  when older than a minute;
- a refresh without a stored identity reads the connector, as without
  sessions, instead of failing with 500.

**Consumer in Deckhouse:** `templates/dex/config.yaml` renders the top-level
`identityRefresh` always, from the setting `sessions.identityRefresh`
(`maxAge` defaults to `idTokenTTL`, `gracePeriod` to 0s).

**Upstream status:** Deckhouse only.

**Depends on:** 100 (`claimsRefreshedAt`), 210 (`useridentity.Record`), 250
(`RotateUpstream`, `UpstreamError`, the stored-claims hooks `Upstream.Stored`
and `Upstream.Read` that run under the lease), 290 (`Recorder`), 300 (the order
of the session login checks).

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
  stayed a device of the user in `UserAccount`. The kubernetes storage lists
  the refresh tokens for it in pages of 500 (`ListRefreshTokens`, which
  upstream leaves unimplemented there);
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

### 340-mfa-one-of.patch

**Zone:** a choice of the second factor. Upstream asks a login for every
authenticator of its chain that applies to its connector type. For the
connector types of `mfa.oneOfConnectorTypes` a login passes one of them, of
the user's choice (`twoFactor.require: Any`):

- `/mfa`: with none of the chain registered (a confirmed TOTP secret, or a
  WebAuthn credential), the page `mfa_choose.html` offers each authenticator
  to register; with one, Dex goes to it; with several, to the one used last
  (the time step of the last TOTP code, the last sign-in with or the
  registration of a key, `WebAuthnCredential.LastUsedAt`), the TOTP first
  without a record. `/mfa?choose=1` offers the registered ones, and the TOTP
  and WebAuthn pages link to it (`OtherMethodURL`) when the user has several.
- a method is registered only while the user has no other one: the TOTP
  page refuses to show or confirm a secret, and the key page and
  `register/begin`/`register/finish` refuse a key, with 403, when another
  method is registered. The check is repeated inside `UpdateUserIdentity`, so
  a registration that loaded the identity before another method was stored is
  refused too. Otherwise a password alone would add a factor of its own to an
  account protected by another one.
- "another method" is any authenticator that applies to the connector type,
  in the chain of the login or not (`resolvedChain.factors`), and the same set
  gates the registration of a login that asks for every factor (`mayRegister`
  of 210). A client may name a chain of its own (`mfaChain`): through a client
  with `[totp]` a password would otherwise register a code for a user who has
  only a key, and that code alone would then pass a login with the choice. The
  login cannot pass a factor outside its chain, so its pages answer 403 instead
  of sending the login back to `/mfa`.
- `CompleteStep` completes the login after one factor of the chain, and,
  whatever the chain, refuses a factor that is not in the chain of the login:
  upstream marked a login validated after any factor that was not followed by
  another.
- the browser session records a choice as one entry, the authenticators joined
  with `|` (`ChainForClient`, `sessionChain`), so a session of a login that
  passed one factor never satisfies a login that asks for every one, after a
  change of `require` too.
- the configuration refuses a type of `oneOfConnectorTypes` that is no
  connector type of Dex, or to which no authenticator of `defaultMFAChain`
  applies: a choice among none would let its users in without a factor.

A Dex without the patch ignores `oneOfConnectorTypes` (the configuration is
read without strict field checks) and asks for the whole chain, never for
less. Without `oneOfConnectorTypes` there is no choice, and of this patch only
the factors outside the chain apply, to the registration.

**Consumer in Deckhouse:** `templates/dex/config.yaml` (`oneOfConnectorTypes`
from the types with `require: Any`), the theme (`mfa_choose.html`, the link on
`totp_verify.html` and `webauthn_verify.html`).

**Upstream status:** Deckhouse only.

**Depends on:** 210 (the chain stops at an unknown authenticator), 300
(`session.MFASatisfied`).

### 350-page-language.patch

**Zone:** the language of the pages. A theme chooses its texts by the frontend
extra `lang`, which upstream reads from the configuration, one language for
every browser. The patch keeps a set of the templates for each language of the
Deckhouse theme (`en`, `ru`), parsed with `lang` set to it
(`templates/language.go`, `loadLanguages`), and every page (login, password,
password change, approval, device, the second factor and its choice, home,
logout, the errors with and without a code) renders from the set of the
language the browser prefers:

- `PreferredLanguage` reads `Accept-Language` the way the pages of
  dex-authenticator do (its patch 015): `ru` when Russian has the highest
  weight, `en` when English or `*` has; a range matches by its primary subtag,
  `q=0` and a weight that does not parse skip it, of equal weights the first
  wins, and only the first 32 ranges count.
- with no range of a language of the pages (no header, other languages,
  garbage) the page keeps the configured `lang`.
- every page adds `Vary: Accept-Language` before its status, so a cache does
  not serve one language to a browser that asks for the other.

It comes after 340 because it covers the pages of 280, 300 and 340.

**Consumer in Deckhouse:** the theme (`images/dex/src/web/templates`, the
`extra "lang"` branches, `lang` of `<html>`, and `messages.html`, which
translates the messages Dex writes in English: the error texts and the rules
and refusals of the password policy of 220); `templates/dex/config.yaml` keeps
`lang: en` as the language without a preference.

**Upstream status:** Deckhouse only.

**Depends on:** 270 (the build identifier of the sets), 280, 300, 340 (their
pages).

### 360-webauthn-more-keys.patch

**Zone:** several WebAuthn keys per user. 210 lets an account register a key
only while it has none. The WebAuthn config gets `maxCredentials` (zero or
unset means one, as before; a negative value stops the start), and a user
adds a key up to it:

- the key page, while the user has fewer keys than the limit, offers "Add a
  key" (`CanAddKey` of `WebAuthnVerifyWithKeys`). `login/finish` with
  `addKey=1` checks the registered key as a sign-in does and then, instead of
  completing the second factor, records in the auth request which
  authenticator's key has confirmed it (`deckhouseKeyConfirmedFor` in
  `WebAuthnSessionData`, next to the session of the ceremony, which an earlier
  dex reads the same).
- `register/begin` of a further key takes the confirmation of its
  authenticator over in the same `UpdateAuthRequest` as the session of its
  ceremony (`deckhouseKeyAddingFor`), and `register/finish` takes it from there
  in one update before the ceremony runs. A finish that fails, for any reason,
  leaves no confirmation, a second begin needs a key to confirm the sign-in
  again, and of parallel finishes one stores a key. A confirmation by a key of
  one authenticator adds no key to another, and a completed step, a new
  ceremony and the end of the login drop it. The confirmation records the user
  and the connector whose key gave it (`deckhouseKeyUserID`,
  `deckhouseKeyConnectorID`), as the passes of 210 do: a confirmation of
  another user, or recorded without one, counts for nothing.
- a further key needs the limit too, and `register/finish` checks it again
  inside `UpdateUserIdentity`, so a parallel registration cannot pass it. The
  keys the user has go to `excludeCredentials` of the ceremony
  (`WithExclusions`: the library adds none by itself), so the browser does not
  register one of them again, and `register/finish` refuses a credential ID the
  user has inside `UpdateUserIdentity`, for a browser that ignores the list.
  The login completes after the new key. A password alone never adds a key to
  an account that has one.
- a limit lowered below the keys a user has keeps them and adds none.
- with a choice of the second factor (340), adding a key is not a second
  method: the user already has the key.

**Consumer in Deckhouse:** `templates/dex/config.yaml`
(`maxCredentials` from `twoFactor.webAuthn.maxCredentials`, 3 by default), the
theme (the button and the flow on `webauthn_verify.html`).

**Upstream status:** Deckhouse only.

**Depends on:** 210 (one key per account), 340 (the choice), 350 (the page).

### 400-connector-open-deadline.patch

**Zone:** opening the connectors at startup. Dex opens every connector one by
one before it serves anything, the health checks included, and the OIDC
connector fetches the discovery document of its issuer while it opens.

- the discovery request of the OIDC connector has a deadline: the
  `discoveryTimeout` of the connector, `10s` when it is not set. An issuer that
  accepts the connection and never answers held the startup forever, and one
  that drops the packets held it for the 30 s dial timeout, until the liveness
  probe killed the pod. On timeout the connector fails to open with an error
  that names the issuer and the bound; with `DEX_CONTINUE_ON_CONNECTOR_FAILURE`
  Dex logs `server: Failed to open connector` and opens the next connector;
- `opening connector` with `connector_id` and `connector_type` is logged at
  INFO before each connector is opened, so a connector that hangs is named in
  the log even if the pod is killed first.

**Consumer in Deckhouse:** every `DexProvider` of type OIDC;
`DEX_CONTINUE_ON_CONNECTOR_FAILURE` and the startup probe of the `dex`
container in `templates/dex/deployment.yaml`, which gives each OIDC provider
15 s: the deadline plus one probe period.

**Upstream status:** Deckhouse only.

**Depends on:** none.

### 410-ldap-binary-id.patch

**Zone:** the user ID of the LDAP connector. With `idAttr` naming a binary
attribute of Active Directory, the connector returned the 16 raw bytes of the
GUID as the user ID. They are usually not valid UTF-8, and the user could not
sign in: the `sub` claim of the tokens is a proto3 message (`GenSubject`),
which refuses a string field with invalid UTF-8 (`string field contains invalid
UTF-8`), and the storage keeps the ID as JSON, which replaces the invalid
bytes, so the UserIdentity and the offline sessions read back no longer matched
the one the connector returned (`get user identity: wrong identity retrieved`,
`get offline session: wrong session retrieved`).

- when `idAttr` is `objectGUID` or `mS-DS-ConsistencyGuid` and the value holds
  16 bytes, the user ID is the string form of the GUID,
  `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx`, with the first three groups
  little-endian, as the tools of Active Directory show it;
- the attribute is recognised by its name, not by the value: a random GUID is
  valid UTF-8 now and then, and its user must get the same form as everyone
  else; a value that does not hold 16 bytes (a directory that returns the
  string form already) and every other attribute are returned as they are;
- sign-in, the identity of a wrong password, and refresh build the identity in
  one place, so they agree.

**Upgrade:** a user whose GUID happened to be valid UTF-8 could sign in before.
The user ID of that user becomes the GUID string, so `sub` changes, and an
application that tells users apart by `sub` sees a new user; the names of the
UserIdentity, the lock and the second factor change with it, so the user
enrolls TOTP again. Every other user of such a provider could not sign in
before.

**Consumer in Deckhouse:** every `DexProvider` of type LDAP whose
`userSearch.idAttr` is a binary GUID attribute of Active Directory.

**Upstream status:** Deckhouse only; not handled upstream.

**Depends on:** none.

### 420-token-exchange.patch

**Zone:** a connector that serves the token-exchange grant only, and the name
prefixes that only such a connector may assert. Upstream exchanges a token
through the OIDC connector, which does not check the audience of the subject
token (`SkipClientIDCheck`), reads the keys only through the discovery of the
`iss` URL, and requires the `name` and `email` claims; none of that fits a
service account token of another cluster, whose issuer URL every DKP cluster
shares.

- new connector type `tokenexchange` (`connector/tokenexchange`): no login,
  only `TokenIdentity` for a subject token of type `id_token`, a JWT. Its config
  mirrors the spec of a `TokenExchangeProvider`:
  - `issuer` must equal `iss`;
  - `keys` is either `discovery` (the issuer's discovery document, optional
    `url` and `caData`) or `jwks` (public keys pinned in the config; a private
    key is refused). Discovery is read at the first exchange, not when the
    connector opens, so an issuer that is down does not hold the startup of
    Dex. One read runs at a time: the exchanges that come meanwhile wait for
    it as long as their own request allows, and a failed read answers the
    exchanges of the next 10 seconds before the issuer is asked again. The
    HTTP client of the discovery and of the key set times out after 10
    seconds, so a hung JWKS endpoint does not hold the key refresh forever.
    `keys.discovery.url` (or the issuer, which gives the default address) and
    the `jwks_uri` of the discovery document must be https, and the client follows no redirect away from https: keys read over
    plain http could be swapped on the way;
  - signature, `iss`, `exp` and `nbf` are checked by go-oidc; `audiences`
    admits a token whose `aud` matches one `exact` value or `prefix`;
  - with pinned keys, a token that has `kid` is verified with the key of that
    `kid` only, even when one key is pinned, as discovery's key set does; a
    token without `kid` is tried with every pinned key (go-oidc's
    `StaticKeySet` ignores `kid`);
  - `subjectTokenMaxLifetime` (a Go duration, 24h by default) refuses a token
    valid for longer: `exp` minus `iat`, or `exp` minus now when there is no
    `iat` or it lies more than 5 minutes in the future (go-oidc does not
    check `iat`; 5 minutes leave room for an issuer clock that runs fast). A
    leaked long-lived token would otherwise buy fresh Dex tokens until it
    expires;
  - `claimValidation` requires claim values: a string equal to `equals`, a
    boolean or a number that prints as it, or a list that contains it;
  - `identity` names the subject: `username` (claim and prefix) is required
    and is also the email unless `email` is mapped, since the API server takes
    the user name from the email claim; `groups` take a string or list claim,
    drop the values without `trimPrefix`, and prepend `prefix`. `UserID` is
    `sub`; `EmailVerified` is true: the mapping vouches for the name;
  - `clients` lists the clients that may exchange through the connector. The
    grant asks the connector (`connector.ClientRestrictedConnector`) before
    the token is verified and refuses the others with `invalid_grant`:
    `Client.AllowedConnectors` cannot do it, being empty, and so open to every
    connector, for most clients;
- `reservedNamePrefixes` in the Dex config: the allow-lists of 200 refuse a user
  whose email or group starts with a reserved prefix that the connector does
  not own (rule `reserved_prefix`), on every path they cover (login, password,
  refresh-token and token-exchange grants). A name under a reserved prefix is
  allowed only when it starts with one of the `reservedPrefixes` of the
  connector config that lie under that prefix: a peer connector owns
  `peering:peer:moscow:`, not all of `peering:`, so it cannot name the subjects
  of another peer. Only a `tokenexchange` connector has them. A user of a
  sign-in connector with a group under a reserved prefix is refused as a whole,
  not stripped of the group: the API server refused such a user before too.

**Consumer in Deckhouse:** `TokenExchangeProvider` (the user-authn controller
writes the connector to the Dex storage); `reservedNamePrefixes` in
`templates/dex/config.yaml` (`system:`, `peering:`).

**Upstream status:** Deckhouse only. dexidp/dex#4397 asks upstream for the
audience check of the OIDC connector.

**Depends on:** 200 (allow-lists).

### 425-token-exchange-denied-clients.patch

**Zone:** the clients that may not use the token-exchange grant. The
kubeconfig clients share one secret, which the kubeconfig generator gives to
every user, and the `kubernetes` client trusts them as peers. With the
token-exchange grant open to them, anyone holding an ID token of the upstream
issuer of an OIDC provider, issued to any application of that issuer, could
exchange it for a token of this cluster: the OIDC connector does not check
the audience of the subject token.

- `tokenExchange.deniedClients` in the Dex config: the grant refuses these
  clients with `unauthorized_client` (HTTP 400) before it reads the subject
  token, whatever the connector, and logs `token exchange denied: client may
  not exchange tokens` at WARN;
- the list goes through `server.Config.TokenExchangeDeniedClients` to the
  grant; other grants of these clients are not affected;
- the grant refuses a cross-client scope (`audience:server:client_id:<peer>`)
  whose peer does not trust the client with `invalid_scope` (HTTP 400) before
  the exchange, as the other grants do. Upstream passes the scopes of the
  exchange through unvalidated, and such a scope failed at the signature with
  `server_error` (HTTP 500).

**Consumer in Deckhouse:** `templates/dex/config.yaml` lists the `kubernetes`
client, whose secret is the shared one, and the kubeconfig clients:
`kubeconfig-generator`, `kubeconfig-generator-<N>`,
`kubeconfig-publish-api` and `kubeconfig-<id>` of each
`kubeconfigGenerator` entry.

**Upstream status:** Deckhouse only.

**Depends on:** none.

### 430-token-exchange-metrics.patch

**Zone:** the metric of token exchanges.

- `dex_token_exchange_total{connector_id, connector_type, result}`: every
  request of the token-exchange grant that reached its connector, by outcome:
  `success`, `denied` (a refusal of the request, HTTP 4xx) or `error` (a
  failure of Dex, HTTP 5xx). A subject token the connector could not verify is
  `denied` even when the issuer was down: the connector reports both alike.
  The grant counts in `Authorize`, around the exchange
  itself; a request refused before it, for an unknown connector or wrong
  client credentials, is not counted;
- registered on the Prometheus registry of the server; without one nothing is
  counted; a failure to register is logged at WARN;
- at startup the `success` series of every connector that is not
  `tokenexchange` is created at zero: a series born at its first exchange
  starts at 1, and `increase()` over it misses that exchange. These are the
  connectors of the config file, and a change of them restarts Dex.
  `increase()` sees the first exchange once Prometheus has scraped the zero: an
  exchange between the start of a pod and its first scrape is still missed.

**Consumer in Deckhouse:** the
`D8UserAuthnDexProviderTokenExchangeDeprecated` alert
(`monitoring/prometheus-rules/dex-deprecation.yaml`): a successful exchange
through a connector that is not `tokenexchange` is an exchange through a
DexProvider.

**Upstream status:** Deckhouse only.

**Depends on:** 425 (the grant it wraps).

### 435-connector-group-prefix.patch

**Zone:** the groups a connector asserts. `GroupPrefix` on a connector, for
every connector type alike (the config file takes `groupPrefix` on each
connector, as it takes the allow-lists of 200), in the kubernetes and memory
storages like the fields of 100. Two providers can report a group of the same
name, and a rule for that name then applies to the members of both; the prefix
names the groups of one provider apart.

- `allowlist.Apply` puts the prefix in front of every group first. It is the
  one call every path that issues tokens makes with what the connector (or a
  refresh token that came from it) gave: the login form, the connector
  callback, SPNEGO and the password change page of 220 in `finalizeLogin`, the
  password, refresh-token and token-exchange grants, and the login from a
  browser session. So the lists of the client, the lists of the connector
  (`allowedGroups`), the reserved prefixes of 420 and the groups of the tokens
  all see the groups with the prefix;
- a group that starts with the prefix keeps its name, and an empty group is
  dropped. Applying the prefix twice therefore changes nothing, which the
  refresh needs: the claims stored on a refresh token pass through `Apply`
  again, and a connector that does not refresh the groups returns what it was
  given. Nothing marks a group as prefixed (a mark on the refresh token or in
  the answer of a connector would reach every zone that reads them), so the
  patch keeps the rule and makes it safe by what the Deckhouse side demands of
  the prefixes: every prefix ends with a colon (the schema of the DexProvider),
  and no prefix of a DexProvider is equal to, or starts with, another's or a
  group prefix of a TokenExchangeProvider (two admission webhooks). A name that
  starts with a prefix is a name of that provider only when every provider that
  can emit such names has its own prefix. A provider without a prefix passes
  the names as they are — OIDC, a GitHub team `org:team`, GitLab, SAML and the
  local groups — and can emit `<prefix>...`. When the rules rely on prefixed
  groups, set a prefix on every provider. What stays is inside one provider: its group
  `corp-ldap:admins` and its group `admins` are the same group once the prefix
  is applied, and the owner of the directory chooses both names;
- the claims stored on a refresh token are sometimes prefixed and sometimes not.
  The login and the password grant store what `Apply` returned, so the groups
  are prefixed. A refresh outside the reuse window stores what the connector
  (or the `UserIdentity`, which keeps the connector's own groups) gave, before
  `Apply` runs on it, so they are not. Inside the reuse window the stored claims
  are left as they are. The tokens a refresh issues then go through `Apply`
  with the current prefix. Only what reads the stored claims directly
  (introspection) sees them without that pass;
- a change of the prefix does not rewrite the claims already stored. Inside the
  reuse window (`reuseInterval`, 2 minutes in `templates/dex/config.yaml`) a
  refresh keeps those claims, and a second `Apply` puts the new prefix in front
  of the old one (`new:old:admins`) until the token expires or the controller
  deletes it. Outside the window the groups are read again without the prefix,
  and one `Apply` names `new:admins`. The local connector is created without a
  `groupPrefix` (`cmd/dex/serve.go`), and a browser session takes the groups
  from the user identity (or the local password), stored before the prefix, so
  neither stacks prefixes. With `allowedGroups` set to the new prefix the
  doubled name is refused, and it matches no rule for `new:admins`. The
  user-authn-controller deletes the refresh tokens of a DexProvider when its
  `groupPrefix` changes (`internal/controller/groupprefix`), on the first pass
  and once more 30 minutes later, since a sign-in that a Dex pod still on the
  old configuration served in between has the old prefix. The second pass waits
  out that margin and deletes the tokens issued before the change and those 30
  minutes. A token a pod still on the old configuration issues later than 30
  minutes can keep the old names. The users whose tokens went sign in again;
- the connector is read before the lists of the client, since they compare the
  groups with the prefix. A failed read waits for where it was handled before,
  after the refusals of the client, which never needed the connector, so the
  answers do not change for a connector without a prefix;
- a refusal reports the groups of the user with the prefix, as compared;
- what the prefix does not reach: the group filters inside a connector
  (`oidc.allowedGroups`, `gitlab.groups`, `ldap.groupSearch`) run before and
  name the groups of the identity provider; Basic Authentication of an OIDC or
  a Crowd provider, which basic-auth-proxy answers without Dex (the DexProvider
  refuses the two together); the groups on the refresh token itself, which
  introspection reports before any list narrowed them, as in 200; the
  `UserIdentity`, which keeps the connector's own groups.

**Consumer in Deckhouse:** DexProvider `spec.groupPrefix`, rendered on the
connector by `templates/dex/config.yaml`
(`dex_connector_identity_limits`); the identity-assign webhook of user-authz
reads it to tell which groups a provider can assert
(`webhooks/validating/identity_assign.py`, `dex_group_prefix`).

**Upstream status:** Deckhouse only. The OIDC connector of upstream has
`claimModifications.modifyGroupNames`, with a prefix and a suffix, for that
connector alone and before its own filters.

**Depends on:** 100 (the field in storage), 200 (`allowlist.Apply`, the
connector fields of the config file). 420 adds the reserved prefix check to
`Apply`, which compares the groups with the prefix as well.

### 998-fix-cve.patch

**Zone:** dependency bumps only: the modules that the Dex tag pins at a version
with a known vulnerability or a known defect. The patch changes only `go.mod`
and `go.sum`. Do not edit it by hand, rebuild it.

- `golang.org/x/net` v0.59.0 → v0.60.0: GO-2026-6603 (CVE-2026-78659),
  GO-2026-6610 (CVE-2026-78660), GO-2026-6611 (CVE-2026-78669), GO-2026-6612
  (CVE-2026-78663), GO-2026-6617 (CVE-2026-97032).
- `github.com/go-ldap/ldap/v3` v3.4.14 → v3.4.15: nil dereference in
  `DecodeControl`. In v3.4.14 the `ControlTypeSyncState`, `ControlTypeSyncDone`
  and `ControlTypeSyncInfo` branches write to `value.Description` without the
  `value == nil` check that the `ControlTypePaging` branch above them has.
  A control that arrives with a single child carries no value, so a response
  that contains one crashes the client. `DecodeControl` runs on server
  responses, including those of `Bind` and `Search`, which the LDAP connector
  uses. Fixed upstream in v3.4.15, which adds the check to all three branches.

Source of each bump:

- `golang.org/x/net`: `govulncheck ./...` on v2.46.0 (v1.8.0, vulnerability
  database of 2026-10-08);
- `github.com/go-ldap/ldap/v3`: a Svacer static analysis finding. The defect
  has no CVE, so `govulncheck` does not report it; keep the bump until Dex
  requires v3.4.15 or newer.

The standard library findings are fixed by the Go version of the builder
image, not here. GO-2026-5932 (`golang.org/x/crypto/openpgp`) has no fix and
is not imported; it is covered by `../known_vulnerabilities.vex` for `golang.org/x/crypto@v0.57.0`.

How to rebuild it (for example, to add or drop a bump), with the Dex tag from
`oss.yaml`:

```shell
git clone --branch v2.46.0 https://github.com/dexidp/dex.git && cd dex
# Apply every patch numbered below 998, in order.
for p in $(ls /path/to/patches/*.patch | grep -v -e '/998-' -e '/999-'); do git apply "$p"; done
git add -A && git commit -q -m "patches below 998"
# Find the vulnerable modules that a newer version fixes.
govulncheck ./...
# Bump each of them and every module of the current list above, including
# those that govulncheck does not report, then tidy and build.
go get golang.org/x/net@v0.60.0
go get github.com/go-ldap/ldap/v3@v3.4.15
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
| `github.com/go-jose/go-jose/v4` | v4.1.5 | v4.1.6 |
| `github.com/go-ldap/ldap/v3` | v3.4.14 | v3.4.15 |
| `golang.org/x/crypto` | v0.57.0 | v0.58.0 |
| `golang.org/x/net` | v0.59.0 | v0.62.0 |
| `golang.org/x/sync` | v0.23.0 | v0.24.0 |
| `golang.org/x/sys` | v0.48.0 | v0.49.0 |
| `golang.org/x/text` | v0.42.0 | v0.43.0 |
| `golang.org/x/tools` | v0.49.0 | v0.51.0 |
<!-- cvefix:end -->
