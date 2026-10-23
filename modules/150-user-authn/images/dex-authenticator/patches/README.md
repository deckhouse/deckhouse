## Patches

The patches apply to the oauth2-proxy tag from `modules/150-user-authn/oss.yaml` in the order of their numbers.
Each patch owns one zone and carries its tests. Every patch is a plain diff that starts with `diff --git`, without a
commit header or message: the patches except `998-fix-cve.patch` are the diffs of the commits of a branch with one
commit per patch (`git diff <commit>^ <commit>`); `998-fix-cve.patch` is the diff of `go.mod` and `go.sum` after the
dependency bumps, rebuilt by hand as its section describes.

Fuzz targets (`999-fuzz-handlers.patch`) and the tests of the other patches are `*_test.go` files, so they do not get
into the image. To run them, apply the patches to a clone of the tag and run `go test ./...` there.

### 002-cookie-refresh.patch

- **Zone:** the session ticket cookie and the persistent session store (`pkg/sessions/persistence`); the stored
  session loader when a refresh cannot be made now (`pkg/middleware`) and the answers of the handlers to it; the
  ticket of a new sign-in (`OAuthCallback`, `SignIn`).
- **Consumer in Deckhouse:** every DexAuthenticator. Redis is a sidecar of the pod without a disk, so a restart of the
  pod or a request to another replica finds no session data.
- **What it does:** the session cookie carries the session ticket and the refresh token as JSON. When the store has
  no data for the ticket, the proxy gets a stub session with the refresh token of the cookie, the placeholder user
  `Does not found in Redis` and a creation time far in the past. With `--cookie-refresh` above zero the stub is
  refreshed through dex at once instead of being trusted. Any error of the store produces the stub, an unreachable
  Redis included: the proxy clears the cookie on a load error, and the cookie holds the only copy of the refresh
  token. A session the ticket secret cannot decrypt, or a cookie without a refresh token, is an error. The stub
  refreshes under the lock of its ticket in Redis, as a stored session does, so parallel requests with one cookie
  make one refresh through dex per replica.
- **When dex cannot answer:** a refresh that gets no answer from dex (connection refused, TLS error, timeout) or gets
  429 or a 5xx status does not end a session that cannot be trusted without the refresh: the stub, or a session in
  Redis whose ID token has expired. The session and its cookie are kept, `/auth`, `/oauth2/userinfo` and the proxied
  requests answer 503 with `Retry-After: 10`, and the same cookie works again once dex answers. A lock that cannot be
  taken within 5 seconds, or an unreachable Redis, is answered the same way instead of clearing the session; the pod
  is not ready without Redis anyway. A refresh dex refuses (`invalid_grant`, `invalid_client`, or `access_denied`,
  which dex answers with 429 for a locked account) still clears the session and the cookie, and so does any other 4xx
  answer for such a session. Sign-out does not need dex and clears
  the cookie.
  - Behind ingress-nginx a 503 of the auth subrequest is neither 401 nor 403, so ingress-nginx does not redirect to
    the sign-in page: it answers its own 500 page and does not pass `Retry-After` on. The user reloads the page once
    dex is back and is still signed in.
  - Behind Envoy `ext_authz` (Gateway API) a 5xx of the authorizer is a failure of the check: Envoy answers
    `status_on_error`, 403 unless the route sets another status, and never passes the request on unless
    `failure_mode_allow` is set.
- **New sign-in:** the session of a new sign-in is saved under a new ticket, not under the ticket of the cookie the
  browser still has. Another replica that keeps the previous session under the old ticket in its Redis would
  otherwise serve the previous identity, `sid` and generation until its next refresh, up to `--cookie-refresh`.
- **Ticket format:** oauth2-proxy 7.6.0 and later write the ticket as `v2.<id>.<secret>`. They read the older
  `<id>.<secret>` too, but the older proxies cannot read the new format, so an upgrade from 7.5.1 logs out the users
  that reach an old replica with a cookie of a new one, once.
- **Security:** sign-out does not invalidate a previously issued cookie. A copy of the cookie restores the session
  while the refresh token in it is valid in dex and the cookie has not expired (`--cookie-expire`, set from
  `keepUsersLoggedInFor`). The cookie is signed, not encrypted. Patch 011 revokes the refresh token in dex on
  sign-out, and patch 010 refuses the stub of a revoked grant before it reaches dex.
- **Upstream:** <https://github.com/oauth2-proxy/oauth2-proxy/pull/313>, closed without merge. Needed until the
  sessions move to a shared store.

### 003-remove-groups.patch

- **Zone:** the `X-Auth-Request-*` response headers of `--set-xauthrequest`.
- **Consumer in Deckhouse:** the ingress-nginx `auth-url` of every application behind a DexAuthenticator. Many groups
  outgrow the buffers of ingress-nginx and break the request.
- **What it does:** removes `X-Auth-Request-Groups`. The groups stay in the session: `allowed_groups` and
  `allowed_emails` in the `/auth` query keep working, which `TestAuthOnlyAllowListWithoutGroupsHeader` checks with
  the flags and the Redis store of the DexAuthenticator. A shared authenticator will check the policy of an
  application this way.
- **Upstream:** <https://github.com/oauth2-proxy/oauth2-proxy/issues/2144>, closed as not planned. The alternative is
  the alpha config, which replaces the whole flag configuration.

### 004-add-redis-retries.patch

- **Zone:** the Redis connection test at startup (`pkg/validation`).
- **Consumer in Deckhouse:** every DexAuthenticator: the Redis sidecar may start after the proxy.
- **What it does:** retries the test 10 times, 2 seconds apart, instead of exiting with code 1 at once.
- **Upstream:** not proposed. A native sidecar for Redis would make the patch unnecessary, but it changes the pod
  of every DexAuthenticator.

### 005-sign-in-401-for-scripts.patch

- **Zone:** the sign-in page (`SignIn`).
- **Consumer in Deckhouse:** single-page applications behind a DexAuthenticator. ingress-nginx redirects every request
  without a session to `auth-signin`, and the redirect of a `fetch` or XHR to dex ended in a CORS error.
- **What it does:** the sign-in page starts the login only for a browser navigation: `Sec-Fetch-Mode: navigate`, or,
  without Fetch Metadata, a request without `Accept: application/json` and `X-Requested-With: XMLHttpRequest`.
  Other requests get 401 with `{}` and no CSRF cookie; the application can reload the page to log in.
- **Upstream:** not proposed. Upstream answers JSON errors only in the proxy mode, which DexAuthenticator does not use.
- **Order:** in the fork branch this commit comes after 008-auth-log-login-failures; its hunks do not overlap
  those of 006, 007 and 008-auth-log-login-failures, so applying the patches by their numbers gives the same tree.

### 006-return-200-on-success-and-header-on-fail.patch

- **Zone:** the status of `/auth`.
- **Consumer in Deckhouse:** Gateway API routes of DexAuthenticator: Envoy `ext_authz` treats only 200 as success.
  ingress-nginx accepts any 2xx. `X-Auth-Request-Result: Denied` is meant for the `alb` module, which lives outside
  this repository.
- **What it does:** `/auth` answers 200 instead of 202 to an authenticated request and sets
  `X-Auth-Request-Result: Denied` next to the 401.
- **Upstream:** not proposed.

### 007-add-json-logging.patch

- **Zone:** log formats (`pkg/logger`, `--logging-format`).
- **Consumer in Deckhouse:** the log collection of the cluster; every DexAuthenticator runs with
  `--logging-format=json`.
- **What it does:** `--logging-format=json` writes one JSON object per line for the standard, auth and request logs;
  the `*-logging-format` templates are ignored. Sign-out logs a `User signed out` auth event. A test checks that a
  failed CSRF check of the callback logs cookie names, not values (GHSA-hhqp-vx7f-5c6m, fixed upstream in 7.15.5).
- **Upstream:** <https://github.com/oauth2-proxy/oauth2-proxy/issues/1057>, open.

### 008-auth-log-login-failures.patch

Writes the login failures of the OAuth2 callback and the failed session refreshes to the auth log,
so each line carries the client address, the request ID and the user.
The security events of `templates/security-events.yaml` are built from these lines.

- A missing CSRF cookie is already logged with `PrintAuthf` in v7.15.5, so the patch leaves that line as it is.
- An error returned by the identity provider and a failure to redeem the authorization code
  get an auth log line next to the existing error line.
- A failed session refresh gets an auth log line with the email of the session,
  at most once per session per `--cookie-refresh` period.
  The session is kept unsaved after a failed refresh, so every following request retries it;
  without the throttle each of them would write one more line.

### 009-sign-out-without-id-token.patch

- **Zone:** the sign-out redirect (`SignOut`, `backendLogout`).
- **Consumer in Deckhouse:** the sign-out through dex (`/logout` with `id_token_hint={id_token}`), planned together
  with the global logout of user-authn.
- **What it does:** when there is no session or no ID token to put into `{id_token}`, the redirect goes to the
  `post_logout_redirect_uri` of the target if it is an allowed redirect (`--whitelist-domain`), and to `/` otherwise.
  The raw placeholder never reaches dex, which answers 400 to it. A user signs out without a session after a global
  logout ended it. The session is looked up for a backend logout only when `--backend-logout-url` is set, so a
  sign-out without a session no longer logs an error.
- **Upstream:** not proposed.

### 010-revocation-feed.patch

- **Zone:** the subscription to the revocation feed of user-authn and the session check (`pkg/deckhouse/revocation`,
  the stored session loader in `pkg/middleware`, `--revocation-feed-url`, `--revocation-feed-ca-file`).
- **Consumer in Deckhouse:** every DexAuthenticator. The feed is served by user-authn-controller
  (`https://revocation-feed.d8-user-authn/revocations`); a lock, a password reset, a deleted user or "sign out
  everywhere" reaches the applications behind every replica within seconds.
- **What it does:** one goroutine per process subscribes to the feed, a `text/event-stream`, with the client ID and
  secret of the authenticator (HTTP Basic) and trusts only the CA of the feed. It resumes with `Last-Event-ID`,
  reconnects after 1 to 30 seconds with jitter, after 60 seconds on 401 and after `Retry-After` on 429. It keeps the
  revoked pairs of a user and a connector with their generation, the ended dex sessions and the revoked grants as
  keys salted with the client secret, merged monotonically, with their expiry times; no tokens and no names. The
  stored session loader refuses a session whose ID token carries an older `d8_gen` of its pair (none is 0) or an
  ended `sid`, or whose refresh token belongs to a revoked grant: it clears the session, logs
  `Session revoked (reason: pair|session|grant)` and `/auth` answers 401 as without a session. The check runs
  before a refresh, so the stub of patch 002 is refused by its grant without a call to dex, and again on the tokens
  a refresh brings. While the feed is not fresh (no whole snapshot yet, or no event for 30 seconds) a session older
  than 30 seconds is refreshed through dex, which refuses revoked refresh tokens: 5 a second with a burst of 10 per
  pod, and above the limit the last state of the feed decides. The feed goes stale for every DexAuthenticator at once,
  so dex and the apiserver then get up to 5 refreshes a second times the number of DexAuthenticator pods in the
  cluster, each a few apiserver calls; a revocation reaches the sessions over the limit only once the feed is back. Readiness does not depend on the feed. Without
  `--revocation-feed-url` nothing changes.
- **Upstream:** not proposed.
- **Depends on:** 002 (the stub session, checked by its grant).

### 011-sign-out-revoke.patch

- **Zone:** the revocation of the refresh token on sign-out (`SignOut`, `--revocation-url`).
- **Consumer in Deckhouse:** every DexAuthenticator, with `--revocation-url=https://dex.d8-user-authn/token/revoke`
  (the RFC 7009 endpoint of dex patch 290).
- **What it does:** before the session is cleared on sign-out, posts its refresh token to the endpoint with
  `token_type_hint=refresh_token` and the client credentials (HTTP Basic), within 3 seconds and over the TLS of dex
  (`--provider-ca-file`). Dex revokes the grant and the revocation feed tells the other replicas, so a copy of the
  cookie no longer restores the session. Any failure is a warning; the session is cleared anyway. The address
  comes from the flag only, the sign-out redirect does not change it. Without `--revocation-url` nothing changes.
- **Upstream:** not proposed.
- **Depends on:** 009 (the sign-out of a session without an ID token).

### 012-sign-out-through-the-provider.patch

- **Zone:** where the browser goes after sign-out (`SignOut`, `--provider-logout-url`).
- **Consumer in Deckhouse:** every DexAuthenticator while the sessions of dex are on (the `sessions.enabled` parameter
  of the module), with `--provider-logout-url=https://<dex>/logout`.
- **What it does:** after sign-out (the refresh token revoked by 011, the cookie cleared) sends the browser to the
  logout endpoint of dex with `client_id` and `post_logout_redirect_uri=https://<host of the request>/` instead of the
  sign-out redirect. Dex ends its session in this browser, and the revocation feed signs the other applications of
  the browser out; dex patch 300 accepts the client without an ID token in the address and asks the user to confirm
  only for a request from another site. Dex returns only to the addresses registered for the client. The address
  comes from the flag only, `rd` does not change it; the flag takes an absolute https URL. Without
  `--provider-logout-url` nothing changes.
- **Upstream:** not proposed.
- **Depends on:** 011 (the revocation before the session is cleared).

### 013-restart-login-on-refused-code.patch

- **Zone:** the OAuth2 callback and the code exchange (`OAuthCallback`, `OIDCProvider.Redeem`), and the mark of a
  restarted flow in the CSRF cookie (`pkg/cookies`, `OAuthStart`).
- **Consumer in Deckhouse:** every DexAuthenticator while the sessions of dex are on (the `sessions.enabled` parameter
  of the module). Dex patch 300 refuses to exchange an authorization code minted in a browser session that has ended
  between the code being issued and redeemed (`invalid_grant`).
- **What it does:** on `invalid_grant` from the code exchange — a tab that reached the callback after the user signed
  out — the callback starts the login again instead of answering an empty HTTP 500. A browser navigation is redirected
  to the sign-in page keeping where it was going (`rd`, validated), so the user signs in again and lands where they
  meant to; a script (`fetch`/XHR, decided as in patch 005) gets 401, not a cross-origin redirect it cannot follow.
  Other redemption errors — the provider is unreachable or answered a 5xx — stay a server error. `OIDCProvider.Redeem`
  wraps the token-exchange error with `%w` so the callback reads the `*oauth2.RetrieveError` underneath and tells
  `invalid_grant` from the rest. The login is restarted at most once per sign-in flow: the restart redirect carries
  `login_restarted=1`, the sign-in copies it into the CSRF cookie of the new flow, and a code refused again in that
  flow is the server error of any other redemption failure. The CSRF cookie of the refused flow is cleared. The mark
  lives in the cookie of the flow, not in a cookie of the browser, so parallel sign-ins (one CSRF cookie per flow,
  `--cookie-csrf-per-request`) do not take each other's restart. The refused code gets the auth log line of a failed
  redemption (`Invalid authentication via OAuth2: unable to redeem the code: ...`), so the
  `D8_DEX_AUTHENTICATOR_LOGIN_FAILED` security event still fires.
- **Upstream:** not proposed; the behavior pairs with the browser sessions of dex.
- **Depends on:** 005-sign-in-401-for-scripts (the navigation-versus-script test and the 401 for scripts) and
  008-auth-log-login-failures (the auth log line of a failed redemption).

### 998-fix-cve.patch

- **Zone:** `go.mod` and `go.sum`.
- **Consumer in Deckhouse:** the image scanners.
- **What it does:** bumps the modules with known vulnerabilities that the tag does not fix yet. The patch changes only
  `go.mod` and `go.sum`; do not edit it by hand, rebuild it.
- **How to rebuild it** (for example, to add or drop a bump), with the tag from `oss.yaml`:

  ```shell
  git clone --branch v7.15.5 https://github.com/oauth2-proxy/oauth2-proxy.git && cd oauth2-proxy
  # Apply every patch numbered below 998, in order.
  for p in $(ls /path/to/patches/*.patch | grep -v -e '/998-' -e '/999-'); do git apply "$p"; done
  git add -A && git commit -q -m "patches below 998"
  # Find the vulnerable modules that a newer version fixes.
  govulncheck ./...
  # Bump each of them (the current list is below), then tidy and build.
  go get golang.org/x/net@v0.60.0
  go mod tidy && go build ./...
  git diff -- go.mod go.sum > /path/to/patches/998-fix-cve.patch
  ```

  Drop a bump once the tag requires that version or newer. List the vulnerabilities of every bump below.
- **Current bumps:** `golang.org/x/net` v0.60.0 (GO-2026-6603, GO-2026-6610, GO-2026-6611, GO-2026-6612,
  GO-2026-6617).
- **Not covered by the patch:** the advisories of the Go standard library are fixed by the Go version of the builder
  image; GO-2026-5932 (`golang.org/x/crypto/openpgp`, no fix) is not reachable and is described in
  `known_vulnerabilities.vex`.
- **Upstream:** the next releases of oauth2-proxy bump the modules themselves.

### 999-fuzz-handlers.patch

- **Zone:** Go native fuzz tests; applied last.
- **Consumer in Deckhouse:** the fuzzing pipeline (`FUZZ_PACKAGES` in `werf.inc.yaml`).
- **Targets:**
  - the HTTP mux via `ServeHTTP`: `/`, `/robots.txt`, `/oauth2/sign_in`, `/oauth2/sign_out`, `/oauth2/start`,
    `/oauth2/callback`, `/oauth2/auth`, `/oauth2/userinfo`, cookies, `Authorization`, `X-Forwarded-*`, the sign-in of
    a script (patch 005) and the sign-out redirect with `{id_token}` (patch 009);
  - `ManualSignIn`, `LoadCookiedSession`, `redeemCode`;
  - the JSON ticket cookie of patch 002 (`decodeTicket`, `decodeTicketFromRequest`);
  - the JWT bearer and basic session loader;
  - the subscriber of the revocation feed of patch 010: the SSE parser, the events, the state they build and the
    session check.
- **Upstream:** not proposed.

Run from a patched source tree:

```shell
go test -run=^$ -fuzz=FuzzServeHTTP -fuzztime=12h .
go test -run=^$ -fuzz=FuzzDecodeTicket$ -fuzztime=12h ./pkg/sessions/persistence
go test -run=^$ -fuzz=FuzzJwtSessionLoader$ -fuzztime=12h ./pkg/middleware
go test -run=^$ -fuzz=FuzzSSEStream$ -fuzztime=12h ./pkg/deckhouse/revocation
```

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `998-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

| Module | Upstream | Patched |
|---|---|---|
| `golang.org/x/net` | v0.59.0 | v0.60.0 |
<!-- cvefix:end -->
