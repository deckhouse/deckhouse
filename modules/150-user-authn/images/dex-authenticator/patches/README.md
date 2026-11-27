## Patches

The patches apply to the oauth2-proxy tag from `modules/150-user-authn/oss.yaml` in the order of their numbers.
Each patch owns one zone and carries its tests. Every patch is a plain diff that starts with `diff --git`, without a
commit header or message: the patches except `998-fix-cve.patch` are the diffs of the commits of a branch with one
commit per patch (`git diff <commit>^ <commit>`); `998-fix-cve.patch` is the diff of `go.mod` and `go.sum` after the
dependency bumps, rebuilt by hand as its section describes.

Fuzz targets (`999-fuzz-handlers.patch`) and the tests of the other patches are `*_test.go` files, so they do not get
into the image. To run them, apply the patches to a clone of the tag and run `go test ./...` there.

### 002-cookie-refresh.patch

- **Zone:** the session ticket cookie and the persistent session store (`pkg/sessions/persistence`), the refresh token
  in the cookie included (`--cookie-encrypt-refresh-token`); the stored session loader when a refresh cannot be made
  now or is answered late (`pkg/middleware`) and the answers of the handlers to it; the ticket of a new sign-in
  (`OAuthCallback`, `SignIn`).
- **Consumer in Deckhouse:** every DexAuthenticator. Redis is a sidecar of the pod without a disk, so a restart of the
  pod or a request to another replica finds no session data.
- **What it does:** the session cookie carries the session ticket and the refresh token as JSON. When the store has
  no data for the ticket, the proxy gets a stub session with the refresh token of the cookie, the placeholder user
  `Does not found in Redis` and a creation time far in the past. With `--cookie-refresh` above zero the stub is
  refreshed through dex at once instead of being trusted. Any other error of the store than the one below produces
  the stub, an unreachable Redis of the `redis` store included: the proxy clears the cookie on a load error, and the
  cookie holds the only copy of the refresh token. A session the ticket secret cannot decrypt, or a cookie without a
  refresh token, is an error. The stub refreshes under the lock of its ticket in Redis, as a stored session does, so
  parallel requests with one cookie make one refresh through dex per replica.
- **When the store does not answer:** a store that cannot tell whether it has the session returns
  `persistence.ErrStoreUnavailable` (the `redis-peers` store of patch 017 does when no filled Redis answered). The
  stub of such a session is marked and never refreshed: the request is answered 503 as below, the cookie is kept,
  and the window of a dex that does not answer does not start. When every Redis of a replica stalled at once, the
  refresh of every session through dex overloaded the API priority level of dex, and two replicas refreshed the
  same refresh token, which dex refuses the second time and ends the session. The mark is checked again under the
  lock of the session, so a store that stops answering after the first load does not let the stub be refreshed in
  place of the stored session either. A refresh dex answers late is saved although the store does not answer the
  check whether the session has ended meanwhile; the save goes where the store can put it. A sign-out still
  revokes the refresh token of the cookie through the marked stub (patch 011).
- **Restores at once:** a process refreshes at most 32 stubs at once. A stub that gets no slot within 2 seconds is
  answered 503 as below and keeps its cookie, and the window of a dex that does not answer does not start. The slot
  is held until dex answers, also when the request has stopped waiting (see the late refresh below), so a slow dex
  gets no more of them at once. dex refreshes a session through the Kubernetes API, 7 to 9 requests one after
  another: two replicas keep at most 64 requests of dex in flight for the stubs, and at about 0.1 second a refresh
  a replica restores about 300 sessions a second. The refresh of a stored session takes no slot.
- **When dex cannot answer:** a refresh that gets no answer from dex (connection refused, TLS error, timeout) or gets
  429 or a 5xx status does not end a session that cannot be trusted without the refresh: the stub, or a session in
  Redis whose ID token has expired. The session and its cookie are kept, `/auth`, `/oauth2/userinfo` and the proxied
  requests answer 503 with `Retry-After` of a random 5 to 15 seconds, so that the clients answered at once do not
  come back in the same second, and the same cookie works again once dex answers. A lock that cannot be taken within
  5 seconds, or an unreachable Redis, is answered the same way instead of clearing the session; the pod of the
  `redis` store is not ready without Redis anyway. A refresh dex refuses (`invalid_grant`, `invalid_client`, or
  `access_denied`,
  which dex answers with 429 for a locked account) still clears the session and the cookie, and so does any other 4xx
  answer for such a session. Sign-out does not need dex and clears
  the cookie.
  - The session is kept for one `--cookie-refresh` period (`idTokenTTL`) from the first refresh dex did not answer;
    the next failure past it ends the session, clears the cookie and sends the user to sign in. dex answers 5xx for
    a session it can never refresh too (its upstream provider failing for that user), and without the bound such a
    user would get 500 on every reload. A refresh that succeeds or is refused starts the window again. The time of the
    first failure is kept in the memory of the replica by the SHA-256 of the refresh token, which only a refresh
    changes: the cookie cannot carry it (ingress-nginx passes no `Set-Cookie` of a 503 auth answer on), and the Redis
    of a pod is its own. So each replica ends the session one period after the first failure it saw, a restart of the
    pod starts its window again, at most 100000 sessions are tracked (one beyond them is kept as before), and one not
    tried again for a day is forgotten.
  - The refresh and the validation after it have a deadline of their own, 10 seconds, after which dex counts as not
    answering. ingress-nginx gives the auth subrequest 60 seconds and then answers 504 itself, so without the deadline
    a dex that drops packets kept the request hanging past it: the answer of the proxy, 503 or the end of the session,
    never reached the user. 10 seconds is far above a refresh through dex, its upstream provider included, and with
    the 5-second wait for the lock well below the timeout of the ingress. A client that gives up does not cancel the
    refresh: its outcome still counts, and a rotated refresh token is still saved.
  - The lock of the session lives as long as the refresh may take and 5 seconds more, 15 seconds instead of the 2 of
    upstream, and is released even when the client has hung up. With 2 seconds the lock expired in the middle of a
    refresh through a loaded dex, the next request of the session took it and refreshed the same token again, and the
    first holder logged `unable to release lock`. A request that waits for the lock gets the session the holder
    refreshed, or the 503 above once its own 5 seconds run out; it never refreshes the session a second time. After
    a refresh dex did not answer, the stub, which has no ID token, is not verified, so its failure no longer logs
    `malformed jwt`.
  - A refresh dex completes after the deadline is not lost. The deadline ends the wait of the request, not the
    refresh: the request is answered 503 as above, and the refresh goes on for up to 60 seconds from its start,
    holding the lock of the session, so the other requests of the session wait for the lock and present no token
    that is being replaced. A slow dex, as opposed to one that is down, completes the refresh and rotates the refresh
    token, and the answer used to be dropped with the request: the browser kept a cookie with a token dex had
    replaced, and the user had to sign in again. Now the answer is saved in the store together with the SHA-256 of
    the token it replaced. A refresh that fails after the deadline changes nothing and is left to the next request,
    which has a browser to answer. A session that was in the store when the refresh started and is gone when dex
    answers was ended meanwhile, by a sign-out for one, and the answer is dropped. The refresh revokes nothing: the
    store cannot tell a sign-out from data it lost or cannot read, and a session the store lost is restored from
    its cookie, whose refresh token dex still accepts. The sign-out revokes the token of the cookie itself (patch
    011), also when the lock of the refresh kept the session from it.
  - Which refresh token a loaded session gets: the one of the cookie, since another replica may have rotated the
    token stored here, unless the stored session says its refresh replaced exactly the token of the cookie. Then the
    cookie is behind the session: the answer with the new cookie did not reach the browser, or dex answered after the
    deadline. The stored token is used, and the answer of the request writes the cookie again, under the lock of
    the session and with the session loaded under it, so that the write does not put back the tokens a refresh of
    another request has just replaced. Every refresh records the token it replaced and the time of the answer, so
    a browser that dropped the answer of a refresh is not signed out either. The record is made only when dex has
    issued another token, and it counts for 2 minutes after the answer of the refresh: the requests the browser
    sent before the new cookie reached it, and its first request after a late refresh, whose answer comes at most
    a minute after the start of the refresh. A cookie that comes back later keeps its own token, and the next
    refresh of the session presents that token to dex. The session keeps the token of the store apart, in memory
    only, so that a sign-out revokes it as well (patch 011): the grant goes on with that token, and a copy of the
    cookie that made the refresh carries it. A record written before the time was kept does not count.
  - Limits of the above. The record lives in the store that made the refresh, and the Redis of a pod is its own: a
    request that reaches another replica of the same DexAuthenticator finds no record and presents the token of the
    cookie. One replaced token is remembered, so a cookie that missed two refreshes in a row is not recognised. The
    stub of a session restored from the cookie has nothing in the store to compare with, so a sign-out during its
    late refresh is not noticed, and the answer is saved under a ticket the browser no longer has. A sign-out that
    clears the session after the late refresh has checked the store and before it has saved the answer is not
    noticed either: the check and the save are two calls to Redis, and the clear of the sign-out takes no lock. The
    sign-out has revoked the grant in both cases, so the saved tokens are refused at their next refresh. A store
    that loses a session while dex rotates its token late drops the answer as after a sign-out: the cookie keeps the
    token the refresh replaced, and the next refresh of the session presents it to dex.
  - Behind ingress-nginx a 503 of the auth subrequest is neither 401 nor 403, so ingress-nginx does not redirect to
    the sign-in page: it answers its own 500 page and does not pass `Retry-After` on. The user reloads the page once
    dex is back and is still signed in, or, past the period above, is sent to sign in.
  - Behind Envoy `ext_authz` (Gateway API) a 5xx of the authorizer is a failure of the check: Envoy answers
    `status_on_error`, 403 unless the route sets another status, and never passes the request on unless
    `failure_mode_allow` is set.
- **New sign-in:** the session of a new sign-in is saved under a new ticket, not under the ticket of the cookie the
  browser still has. Another replica that keeps the previous session under the old ticket in its Redis would
  otherwise serve the previous identity, `sid` and generation until its next refresh, up to `--cookie-refresh`.
- **Refresh token in the cookie:** with `--cookie-encrypt-refresh-token` the cookie carries the refresh token
  encrypted: AES-256-GCM with a key derived from the cookie secret by HKDF-SHA256, so the key differs from the one
  that signs the cookie. The value has the `EncryptedRefreshToken` field instead of `RefreshToken`. A proxy reads both forms whatever its flag is, and the flag only chooses the form
  it writes: a cookie written before the flag was set keeps working and is replaced by the next refresh of its
  session. A proxy with another cookie secret, or one built without this change, cannot read the encrypted token and
  refuses the cookie. An encrypted token that cannot be decrypted is an error, not a session without a refresh
  token. The module does not set the flag yet.
- **Ticket format:** oauth2-proxy 7.6.0 and later write the ticket as `v2.<id>.<secret>`. They read the older
  `<id>.<secret>` too, but the older proxies cannot read the new format, so an upgrade from 7.5.1 logs out the users
  that reach an old replica with a cookie of a new one, once.
- **Security:** sign-out does not invalidate a previously issued cookie. A copy of the cookie restores the session
  while the refresh token in it is valid in dex and the cookie has not expired (`--cookie-expire`, set from
  `keepUsersLoggedInFor`). The one exception is the record of the replaced token above: for 2 minutes after the
  answer of a refresh, a copy that carries the token this refresh replaced gets the tokens of the stored session
  and a new cookie, though its own token has been replaced. A copy that comes later keeps its replaced token and
  presents it at the next refresh, where a provider that refuses a replaced refresh token ends the session of that
  copy, as before the record. Two copies of one cookie thus keep each other signed in only while one of them comes
  within 2 minutes after every refresh the other makes, as the requests of one browser do. The cookie is signed; its
  refresh token is plain unless `--cookie-encrypt-refresh-token` is set, and the ticket secret in it is always plain.
  Patch 011 revokes the refresh token in dex on sign-out, the token of the store as well when the cookie carries
  another one, and the token of the cookie also when a late refresh holds the lock of the session; patch 010
  refuses the stub of a revoked grant before it reaches dex.
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
- **Feed of a process with a table of clients:** with a token file instead of a client (`Options.TokenFile`,
  `--revocation-feed-token-file` of patch 018) the process subscribes once, with the token of its ServiceAccount as
  a bearer token, read again for every connection since the kubelet replaces it. The feed then sends the
  revocations of every client of the process in one stream, each key made with the salt of its client. The keys of
  two clients never meet, so the process keeps them in one state, and a client reads it with the salt of its own
  secret (`Feed.ForClient`): it finds its keys and none of another client. The freshness of the feed and the limit
  of forced refreshes are those of the process. Without a token in the file the process makes no request and tries
  again.
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
  When the cookie carries another refresh token than the session in the store, as a cookie that came back long
  after a refresh it missed does (patch 002), the token of the store is revoked too, in a request of its own: the
  grant goes on with it, and a copy of the cookie may carry it. Equal tokens are revoked once.
  A sign-out the stored session loader gives no session, as while a late refresh of patch 002 holds the lock of
  the session, loads the session from the store without the lock: the session of the store, or the stub of the
  cookie when the store has none, carries the token of the cookie, and that token is revoked. The late refresh
  revokes nothing.
- **Upstream:** not proposed.
- **Depends on:** 002 (the late refresh), 009 (the sign-out of a session without an ID token).

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
- **Changed by:** 014 moves the restart redirect into `restartLogin`, shared by both restarts, without changing it.

### 014-restart-login-without-csrf-cookie.patch

- **Zone:** the OAuth2 callback without its CSRF cookie (`OAuthCallback`), `--restart-login-without-csrf-cookie`, and
  the restart mark in the `state` of a restarted flow (`doOAuthStart`).
- **Consumer in Deckhouse:** every DexAuthenticator. The CSRF cookie of a sign-in flow lives 10 minutes
  (`--cookie-csrf-expire=10m`), so a user who left the sign-in of dex open longer than that, or a callback opened again
  from the history, came back to the bare 403 «Unable to find a valid CSRF token» of oauth2-proxy.
- **What it does:** a callback whose CSRF cookie is missing, does not validate or has expired:
  - a script (`fetch`/XHR, decided as in patch 005) gets 401 with `{}`;
  - a browser navigation gets the 403 page of an expired sign-in (message
    `Login Failed: The sign-in has expired. Unable to find a valid CSRF token. Please sign in again.`, which the
    templates of patch 015 tell apart by its prefix), whose **Sign in again** button goes to the sign-in page with
    the `rd` of the `state`, validated (`/` when it is not an allowed redirect);
  - with `--restart-login-without-csrf-cookie` a browser navigation is redirected to the sign-in page with that `rd`
    and `login_restarted=1` instead, the same way as patch 013. The module sets the flag while the sessions of dex
    are on (`sessions.enabled`): a live session signs the user in again without a password. Without the sessions the
    restart would show the user the dex sign-in once more, so the page asks first. The flag is a container argument,
    so a change of `sessions.enabled` rolls the DexAuthenticator pods.
- **Loop guard:** a flow without its cookie cannot read the mark of patch 013, so the sign-in of a restarted flow also
  prefixes the nonce in its `state` with `r.` (a nonce hash, unpadded base64url, has no dot); the callback strips the
  mark before it names the CSRF cookie and checks the state. A callback of a marked flow without its cookie gets the
  page, so a browser that does not keep the cookie does not loop.
- **Security:** a callback without its CSRF cookie never redeems its code, in either mode and for any request, and
  sets no session; it trusts nothing of the lost flow but the validated `rd`. A restart starts a new flow from
  scratch, with a new state, CSRF cookie and PKCE verifier, so a code slipped into another browser cannot become its
  session (`TestForeignCodeWithoutCSRFCookieIsNeverRedeemed`, `TestRestartStartsANewFlow`). A forged callback only
  shows the page or sends the browser to the sign-in page, which anyone can link to anyway.
- **Logs:** the missing cookie keeps its auth log line (`, restarting the login` is appended for a restart), so the
  `D8_DEX_AUTHENTICATOR_LOGIN_FAILED` security event still fires; a state that does not match the cookie
  (`CSRF token mismatch`, `D8_DEX_AUTHENTICATOR_CSRF_MISMATCH`) stays the 403 of upstream.
- **Upstream:** not proposed.
- **Depends on:** 013 (the restart redirect and the mark in the CSRF cookie), 005-sign-in-401-for-scripts and
  008-auth-log-login-failures.
- **Order:** right after 013; its hunks in `oauthproxy.go` sit inside those of 013. 015 sets the language in
  `errorPageTo`, the error page with a given redirect that 014 adds.

### 015-deckhouse-pages.patch

- **Zone:** the data of the sign-in and error pages (`pkg/app/pagewriter`, `OAuthProxy.ErrorPage`).
- **Consumer in Deckhouse:** every DexAuthenticator, with the templates of `images/dex-authenticator/templates`
  (`--custom-templates-dir=/usr/share/dex-authenticator/templates`).
- **What it does:** the pages get `Lang`, `ru` when the `Accept-Language` of the request prefers Russian to English
  and `en` otherwise (`PreferredLanguage`; the first 32 entries of the header are read, a range with `q=0` or a
  weight that does not parse is skipped), and the error page gets `LogoData`, the logo of `--custom-sign-in-logo`
  that upstream gives only to the sign-in page. The templates get `HasPrefix` to tell the generic messages of the
  error page apart. Dex itself shows its pages in one language (`frontend.extra.lang` of the dex config, `en`), so
  the pages of DexAuthenticator follow the browser instead.
- **The pages:** `error.html` and `sign_in.html` are not in the patch: the image takes them from
  `images/dex-authenticator/templates`, and `template_tests/authenticator_pages_test.go` renders them. With
  `--skip-provider-button` only the error page is shown; `sign_in.html` and `robots.txt` are there so that the proxy
  does not log the missing files at startup. The stylesheet, fonts and images are those of the Dex theme
  (`images/dex/src/web/static`): the image copies them into `static/` before the build, and `go:embed static/*`
  serves them under `/dex-authenticator/static/`. The deployment passes the Deckhouse logo as
  `--custom-sign-in-logo`, or `-` when the module has a custom logo, which the namespace of the authenticator cannot
  read.
- **Upstream:** not proposed.
- **Order:** after 014; it adds one line to `errorPageTo`, the error page of 014.

### 016-auth-request-host.patch

- **Zone:** the host of an auth subrequest (`pkg/middleware`, the first step of the request chain) and the parsing
  of `X-Original-URL` (`pkg/requests/util`).
- **Consumer in Deckhouse:** every DexAuthenticator behind ingress-nginx; the log collection and the security events
  of `templates/security-events.yaml`, which take the host of an event from the log.
- **Why:** the proxy runs with `--reverse-proxy` and takes the host of a request from `X-Forwarded-Host`. On the auth
  subrequest ingress-nginx sets `Host` to the Service of the authenticator and `X-Original-URL` to the request being
  authorized, and passes `X-Forwarded-Host` on as the browser sent it. A client could thus write any host into the
  request log, the auth log and the security events of its requests. Access was not affected: the session and the
  signature of the cookie do not depend on the host.
- **What it does:** on the exact path `<proxy prefix>/auth` a request with `X-Original-URL` and without
  `X-Forwarded-Proto` gets `X-Forwarded-Host` rewritten to the host of its `X-Original-URL`. That pair of headers
  is the subrequest of ingress-nginx: it builds `X-Original-URL` from the `Host` header of the request, by which
  it chooses the server (see the limits for the one case where it does not), overwriting whatever the browser sent
  under this name, and removes `X-Forwarded-Proto` from the subrequest; on the
  route of an application it sets `X-Forwarded-Proto` and `X-Forwarded-Host` itself. So a request with
  `X-Forwarded-Proto` came through a route and is left as it is, with the `X-Original-URL` of the browser ignored.
- **Parsing:** the header must be there once and start with `http://` or `https://`; the host is what stands before
  the first slash, a DNS name of letters, digits, hyphens and dots with an optional numeric port, written in lower
  case, without a trailing dot and without the default port of the scheme. It is not parsed as a URL on purpose: for
  `b.example.com:443@w.example.com` nginx serves `b.example.com`, and a URL parser answers `w.example.com`. Anything
  else, an address in brackets included, names no host: `X-Forwarded-Host` is then removed, and the host of the
  request is its `Host` header, the Service of the authenticator.
- **Limits:** a request that reaches the Service of the authenticator from inside the cluster, not through the
  ingress controller, names its headers itself, as before. The check of another ingress controller, which carries no
  `X-Original-URL`, is left as it came. A request with an absolute URI in its request line
  (`GET https://app.example.com/x HTTP/1.1` with `Host: other.example.com`) still names its host itself: nginx
  chooses the server by the host of the request line, and ingress-nginx builds `X-Original-URL` from `$http_host`,
  the `Host` header as it came. The logs and the security events of such a request carry the host of the header,
  not the one nginx served; access is not affected. Closing it takes `$host` in the auth subrequest, which is set
  where the Ingress of the application is written, not in the proxy.
- **Upstream:** not proposed.

### 017-session-store-redis-peers.patch

- **Zone:** a new session store, `pkg/sessions/redispeers`, chosen with `--session-store-type=redis-peers`, and the
  memory of the process it falls back to, `pkg/sessions/memory`. The stores `cookie` and `redis` are not changed.
- **Consumer in Deckhouse:** the shared DexAuthenticator (`templates/shared-dex-authenticator`), whose replicas
  each have a Redis without a disk in the pod.
- **What it does:** every replica of the proxy has a Redis of its own, and none of them is a master for long. A
  replica writes each session to its own Redis and then to the Redis of its neighbours, and reads it from its own
  Redis, then from a neighbour, then from the memory of the process; a session a filled Redis says it does not have
  is restored from its cookie (patch 002), but only when no other filled Redis failed the same load. Otherwise the
  session may well be in a Redis that did not answer: one Redis took the writes while the other stalled, then the
  other came back and the first one stalls. The load then returns `persistence.ErrStoreUnavailable`, and the
  request is answered 503 without a refresh (patch 002); so it is when no filled Redis is in use at all. Once the
  failing Redis has been left, the answer of the others counts again, and a session only the left Redis had costs
  one refresh, under the lock taken in every filled Redis. A neighbour that is being filled does not answer
  for a session. The memory serves only what no Redis took. The store sits behind the same `persistence.Manager`
  and ticket cookie as `redis`.
  - **Neighbours:** `--redis-peers-dns-name` resolves to the address of every replica, as a headless Service with
    `publishNotReadyAddresses` does; `--redis-peers-self-address` is the address of the replica itself. The own
    Redis, its password and the port of every Redis are those of `--redis-connection-url` and `--redis-password`.
    The list is looked up every second.
  - **Password:** every Redis of the replicas has one password, set both as `requirepass` and as `masterauth`, and
    the proxy refuses to start in this mode without it in `--redis-password` or `--redis-connection-url`.
    `requirepass`: the Redis listens on the address of the pod for its neighbours, and without a password whoever
    reaches the port reads every session, removes them (`FLUSHALL`) or makes the Redis a replica of a Redis of its
    own (`REPLICAOF`, `CONFIG SET`). `masterauth`: a Redis that is filled signs in to its neighbour with it; without
    it the link of `REPLICAOF` never comes up, every filling fails after `--redis-peers-seed-timeout`, and the Redis
    stays out of use, a replica without its mark.
  - **Filling an empty Redis:** a Redis without its mark has restarted. The mark is the key
    `deckhouse.redis-peers.seeded` holding the run ID of the Redis, so the key a copy brings from a neighbour is
    not the mark of the copy. The proxy makes the Redis a copy of a neighbour that has its mark and is not a
    replica: `REPLICAOF`, wait for the full copy (a Redis that answers `LOADING` is still copying),
    `CONFIG SET replica-read-only no`, wait the time of one request, wait until it has applied all the neighbour
    had sent by then, drop the copied refresh locks, set the mark, `REPLICAOF NO ONE`. The mark is set before the
    Redis is detached: in the other order a step that fails after the detaching leaves a master with a full copy
    that looks empty. A Redis that carries its mark and is still a replica is not in use and is filled again. A
    copy that fails leaves the Redis without the mark and out of use, and is tried again, from another neighbour
    if this one no longer answers. The Redis starts empty only when every neighbour answers and none has anything
    to give; a neighbour that is being filled itself or does not answer, and a lookup of the neighbours that
    fails, are waited for. The replica is not ready (`/ready`) until this is over or
    `--redis-peers-seed-timeout` (default `15s`) has passed. Redis needs `--repl-diskless-sync-delay 0` and
    `--repl-diskless-load swapdb`; the Redis image of the module pins the 7.4 line this was measured on.
  - **Without Redis:** a Redis that fails five requests in a row, for at least 2 seconds, is left and probed every
    second; `/ready` does not depend on Redis. Each request is bounded by `--redis-peers-timeout`, default `500ms`:
    far above the time Redis takes, it is the time a Redis may stall before a request goes on to the next one. A
    loaded node delays a process by hundreds of milliseconds now and then, and with `200ms` and three failures
    every Redis of such a node was left at once, again and again. A Redis that is down is left within a few seconds
    without requests too, since the probe asks it every second. The failures of a Redis are cleared by three
    answers in a row, requests or probes, not by one answer between stalls: a neighbour that fails most requests
    but answers the probe every second is left all the same. A Redis that was left is taken back after it has
    answered three probes in a row, and starts without failures. The Redis of the replica is filled and brought back apart from the probes, which
    go on watching the neighbours meanwhile.
  - **Fail-closed:** while no filled Redis answers, the replica answers 503 to every request with a session cookie,
    for as long as that lasts: nothing restores such a session from its cookie, and a new sign-in does not help
    either, since the cookie is kept. There is no switch for the former behaviour. A new sign-in during that time
    is kept in the memory of the replica that made it, so the next request of the user that reaches another
    replica gets 503 there too. A session no Redis took is kept in the memory of the process. A removal the own
    Redis did not take is kept there too and hides the session at
    once. Both are given to the own Redis when it takes writes again: on its return before it is read, and by
    every probe. A session a neighbour took is not kept in the memory: the own Redis is left with its older copy,
    which costs one refresh. A move waits for the requests only while they write to the own Redis, not while they
    write to the neighbours.
  - **Memory:** `--session-store-memory-max-bytes` (default 64 MiB) bounds it: past the limit the records not used
    for the longest time are dropped, the expired ones first; a record counts as its key, its value and 256 bytes
    of overhead. `--session-store-memory-ttl` (default `0`, off) caps the life of a record, which is otherwise
    `--cookie-expire`. A record the memory has dropped costs one refresh through dex, never a sign-in. The memory
    is not a store type of its own.
  - **Refresh lock:** taken in every Redis that answers and carries its mark, the own one included, in the order of
    their addresses; busy in one of them is busy. A Redis that stalls is left by each replica on its own, so for a
    while the replicas see different Redis: with the lock in the lowest of them only, each replica took it in
    another Redis and both refreshed the same refresh token. Taken in all, the locks of two replicas meet in every
    Redis both of them see. A Redis that fails the lock fails the request, which is answered 503, and the locks
    taken so far are released. The memory of the process holds the locks only of a replica without neighbours; a
    replica with neighbours and no filled Redis answers 503 to the requests that need a refresh.
- **Metrics:** `oauth2_proxy_session_store_redis_peers_`: `loads_total` by `step` (`own`, `peer`, `memory`,
  `cookie`), `saves_total` by `target` (`own`, `peer`, `memory`), `peer_writes_total` by `operation` and `result`
  (`ok`, `readonly`, `timeout`, `error`), `moved_total`, `seed_duration_seconds` by `outcome` (`copied`, `empty`,
  `kept`, `timeout`, `error`), `mode` and `mode_switches_total` (`redis`, `peer`, `memory`), `lock_home` by
  `address` (the Redis a lock is taken in first), `locks_total` by `home` and `result`, `peers` by `state`.
  `loads_total` with `step="unavailable"` counts the loads no filled Redis answered. Of the memory:
  `oauth2_proxy_session_store_memory_entries`, `_bytes`, `_evictions_total` by `reason` (`size`, `expired`),
  `_loads_total` by `result` (`hit`, `miss`). No metric carries a client, a host or a user.
- **Limits:** two replicas that each have left the Redis the other one uses, and see no other (a network split
  between the pods), take the refresh lock in different Redis; no TLS between the Redis of the replicas; a write a
  neighbour has missed is not sent again: a save
  costs one refresh there; a removal leaves the session there, and the session can come back on every replica, the
  one of the sign-out included, until its next refresh (after a sign-out only a copy of the cookie gets that far,
  and the sign-out has revoked its refresh token, patch 011); a session a replica keeps in its memory comes back on
  that replica the same way; a session found at a
  neighbour is not written to the own Redis; a session no Redis took is written to the own Redis on its return even
  if a neighbour has removed it meanwhile; the lock of a replica that was killed is not taken over (it expires); a
  removal kept in the memory can be dropped by its size limit; a replica whose neighbours do not answer waits for
  them for the whole `--redis-peers-seed-timeout` and then starts empty with its mark.
- **Tests:** `pkg/sessions/redispeers` on fake Redis and miniredis; `integration_test.go` (build tag
  `integration`) runs the filling on two real Redis with sessions that do not compress, addresses in
  `REDIS_PEERS_TEST_ADDRS`. `pkg/sessions/memory`: the TTL, the eviction by bytes, concurrent use under the race
  detector, the lock, the metrics, and the suite every session store of oauth2-proxy passes.
  `pkg/middleware/deckhouse_memory_store_refresh_test.go`: the lock tests of patch 002 over the memory and its
  lock. `pkg/validation/deckhouse_sessions_test.go`: the store is refused without a Redis password and is not
  tested for a connection, the flags of the module included.
- **Upstream:** not proposed.

### 018-client-table.patch

- **Zone:** `main.go` and the construction of `OAuthProxy` (`NewOAuthProxy` is split into building the proxy and
  starting its servers); two new files, `deckhouse_client_table.go` and `deckhouse_dispatcher.go`; the scopes of
  `validation.Validate`; the remote key sets of `pkg/providers/oidc`; two fields of the JSON log lines of patch 007.
- **Consumer in Deckhouse:** none yet: the shared DexAuthenticator, one process for the applications of many
  DexAuthenticator objects, each a dex client of its own. Without `--client-table` the proxy works as before.
- **What it does:** `--client-table=<file>` names a JSON file with the clients of the process. Each client is an
  `OAuthProxy` built from the flags of the process followed by the flags of its record, through the same option
  loading and validation as the only client of a process without the table. A dispatcher on the application server
  gives each request to the client the name of a Service in its `Host` header names, and otherwise to the client
  its host names.
  - **Table:** `{"generation": N, "clients": [...]}`. A client is a DexAuthenticator object: `name`, `namespace`,
    `hosts`, `clientID`, `clientSecret`, `cookieSecret` (required) and `cookieExpire`, `allowAccessToKubernetes`
    (adds the scopes of the `kubernetes` audience to the scope of the process), `sendAuthorizationHeader`,
    `providerLogout`, `services`, `sharedHosts`. These are all a client may set: there is no field for arbitrary
    flags, and every other setting is that of the process. Hosts are DNS names without a port; a redirect is
    allowed to the hosts of the client, the shared ones included.
  - **Two clients on one host:** `services` are the names of the Service of the object, as an ingress controller
    writes them into `Host`: the host of `auth-url` in the auth subrequest of ingress-nginx, and the value of the
    `upstream-vhost` annotation on a route. A request with such a `Host` came through that Service and is given to
    its client whatever host of the client it is for; a request for a host the client does not have has no client.
    `hosts` are the hosts a client is chosen by when `Host` names no Service, one client per host. `sharedHosts`
    are the hosts of a client that another client is chosen by: the client serves them through its Service only,
    so they need `services`, and two clients may name one. This is how two DexAuthenticator objects publish one
    domain through two Ingress classes. Who may send a request through the Service of a client is decided where
    the routes are written, as it was when every client had pods of its own. A record without `services` is chosen
    by its hosts only. `auth-url` is written by the author of the application with the name of the Service under
    the domain of the cluster or under an alias of it: ingress-nginx resolves no shorter name, a sender inside the
    cluster may use one. So an auth subrequest names the Service of a client by `<service>.<namespace>`, alone or
    followed by `svc` and any domain; a route names it by the exact name of the table, the one the controller puts
    into `upstream-vhost`. Two records whose names give one `<service>.<namespace>` claim one Service: the first
    keeps it. The choice by the host must not answer for an object the table does not have: the object is gone
    while the Ingress of its application still sends the auth subrequest to its Service and the ingress controller
    still resolves the name, or `auth-url` names another namespace or is misspelt. The client that is chosen by
    the host would open the application of that object to its own users. So an auth subrequest whose `Host` has
    the form `<object>-dex-authenticator.<namespace>`, alone or followed by `svc` and any domain, and names no
    Service of the table has no client, whatever host it is for. A `Host` of any other form is chosen by the host
    as before, and so is any `Host` for a client whose record has no `services`. The Service of an object whose
    name was cut to fit the length of a name does not have the form, so the rule does not cover it.
  - **A record that is refused:** a record that fails a check, or has the cookie secret of an earlier record of
    the file, is refused alone. So is a record that claims a host or a name of the Service that a client of the
    table in use is chosen by and its record in the file still names: the name stays with the client that is served
    by it, whatever the order of the file. A host or a Service that two records claim and no client is chosen by
    goes to none of them: both records are refused. With the order of the file, a record put before the record of
    a client would take its host, and the requests of its application would be checked with the cookie secret, the
    users and the groups of another client; and a process that starts has no table in use, so by the order it
    could give the name to another client than the process next to it that runs. The writer of the table gives a
    name to one client, and the process does not rely on it. The other clients are served by the new table, and
    that client by the record it had: by its previous `OAuthProxy`, on the hosts and by the names of the Service of
    that record that no client of the new table has taken (a Service is taken also when a client of the new table
    names it by another of its DNS names), and only while no client of the new table has its
    previous cookie secret. A name it gave up comes back to it once it is free. A new client whose record is
    refused is not served. The first record of a client is its record, refused or not: a second one is dropped. Two
    clients are never chosen by one host or one name of a Service, and never served with one cookie secret; cookie
    secrets are compared as the proxy reads them, so a secret and its base64 form are one. Only a
    file that is not a table is refused as a whole and leaves the table in use: an unknown field, more than one
    JSON value, or no list of clients.
  - **Sign-out through dex:** a process with `--provider-logout-url` refuses a record without `providerLogout`:
    dex answers the sign-out of a client that has no address to return to with an error page and keeps its session.
  - **Generation:** the writer of the table numbers its versions. The process answers `GET /client-table`, by the
    exact path, on the application server and outside the proxy prefix, so that no route of an application leads
    there, with
    `{"generation": N, "clients": M, "failed": ["<identifier>", ...]}`: the generation it serves, the number of
    its clients, and the clients whose record was refused or could not be built. The answer names no client: every
    pod of the cluster reaches the port, and the names are the namespaces and the objects of other tenants. A
    failed client is given by the first 16 bytes, in hex, of the HMAC-SHA256 of `<namespace>/<name>` under the
    cookie secret of its record, which only the writer of the record can compute. The writer learns from the
    answer when every replica has loaded a version and which of its records were refused.
  - **Sessions of the clients:** the clients share the session store, and each keeps its records and refresh locks
    there under its own name (`persistence.NewScopedStore`, the key is `<length of the name>:<name>:<ticket>`). A
    cookie of one client is not a session of another by two checks: the cookie secret signs it, and behind the
    signature the other client finds no record under its name and asks dex, which refuses a refresh token of
    another client (`TestClientTable_storeScope`). The second check needs the refresh, so the process refuses to
    start without `--cookie-refresh` above zero: without it the stub of patch 002 is trusted as it is. It also
    needs `--skip-oidc-discovery`: a client is built while the table is loaded, and a provider that is asked for
    its configuration and does not answer would hold the load.
  - **Key of a request:** the name of a Service in `Host` chooses its client, and the host of the application
    then only has to be one of that client; without it the host of the application chooses the client. The host of
    the application: for the auth subrequest of ingress-nginx, the exact path `<proxy prefix>/auth` with
    `X-Original-URL` and without `X-Forwarded-Proto`, the host of `X-Original-URL` (patch 016); a header that names
    no plain host is no key. For any other request with the name of a Service in `Host`, the only
    `X-Forwarded-Host`, which ingress-nginx sets to the host of the application when `upstream-vhost` replaces
    `Host`; a browser that writes it through an ingress controller that trusts forwarded headers gets the client
    of the Service, and only on a host of that client. For every other request the `Host` header: a browser path
    that came through the route of an application, or the auth check of an ingress controller that sends the
    `Host` of the application. `X-Forwarded-Host` is read in no other case and is overwritten with the host of the
    application before the handler of the client runs. The host is written as the table has it, in lower case and
    without a trailing dot, with the port only when it is not the default one of the scheme: the client builds its
    `redirect_uri` from it, and dex compares that with the registered one as a string.
  - **What the key relies on:** ingress-nginx overwrites `X-Original-URL` on its subrequest. The auth check of
    another ingress controller is told from it only by the headers: it must carry `X-Forwarded-Proto` or no
    `X-Original-URL`, or a client of that controller could name the host of another application there. The `Host`
    of a subrequest of ingress-nginx is the Service of the authenticator, so the writer of the table must not give
    that name to a client as a host.
  - **A request that names no client**, by a host no client has or by the Service of one client and the host of
    another, is answered 403 on the auth path and 404 on any other. ingress-nginx passes a 403 of the auth
    subrequest to the browser and turns a 404 into its own 500.
  - **Reload:** the file is read again on a file event (the symlink swap of a Secret volume included) and every 10
    seconds. A client whose record did not change keeps its `OAuthProxy`; the others are built, and the new table
    replaces the old one at once. A request already given to a client finishes there. A client that cannot be built
    is served as a refused one is. Without the file the table is empty and the process is ready.
  - **Shared by the clients:** the session store, the pages, the upstreams, the remote key set of dex
    (`ShareRemoteKeySets`), the logger, the TLS settings of the requests to dex, and the memory of the refreshes
    dex did not answer (patch 002), whose bound is then one for the process. With `--skip-oidc-discovery`, as in
    Deckhouse, building a client makes no request. Of its own a client has the provider client, the cookie
    settings, the redirect allowlist and the response headers.
  - **Validation:** `validation.Validate` sets the state of the process (the logger, the transport, the Redis
    connection test of patch 004). `ValidateProcess` does that once for the flags of the process, which name no
    client, and `ValidateClient` checks a client without touching the process or the network.
  - **Log:** the JSON lines of a request served by a client of the table carry `authenticator` and
    `authenticator_namespace`, the DexAuthenticator object of the client, right after `host`; the lines of a process
    without the table do not have them. A load of the table writes one line per client it builds (its name, client
    ID and hosts), one per record it refuses and one with the totals and the generation. The lines an `OAuthProxy`
    writes about itself at start are left to the process without the table, and the warning about
    `--trusted-proxy-ip` is written once per process. No line carries a secret.
  - **Health checks:** the process answers `--ping-path` and `--ready-path` itself, for any host.
  - **Revocation feed:** a subscription per client would be hundreds of streams, and the feed allows 1000. With
    `--client-table` the process subscribes once, with `--revocation-feed-token-file` (patch 010), and every client
    reads that one feed with its own secret. The proxy refuses to start with `--revocation-feed-url` and neither
    the token file nor `--revocation-feed-disable`, so a process never runs without the feed unnoticed; without the
    feed revoked sessions last until their next refresh.
  - **Not supported with the table:** `--authenticated-emails-file`, `--htpasswd-file`, the `cookie` session store.
  - **Metrics:** `oauth2_proxy_dispatch_request_duration_seconds` by `outcome` (`hit`, `unknown`), whose count is
    the number of requests; `oauth2_proxy_client_table_clients`, `oauth2_proxy_client_table_hosts`,
    `oauth2_proxy_client_table_generation`; `oauth2_proxy_client_table_reload_duration_seconds` by `result` (`ok`,
    `partial`, `error`; every read of a file that is missing, unreadable or refused as a whole is an `error`, so it shows as
    a rate while the file stays so);
    `oauth2_proxy_client_table_client_changes_total` by `change` (`built`, `reused`, `failed`, `removed`);
    `oauth2_proxy_client_table_last_reload_timestamp_seconds`. No label carries a host, a client or a user.
- **Tests:** `deckhouse_dispatcher_test.go`: a forged `Host`, `X-Forwarded-Host` or `X-Original-URL` reaches the
  handler of no other client (`TestDispatcher_ServeHTTP`, the fuzz target `FuzzDispatcherRoute`), and the name of
  a Service opens no host its client does not have (`TestDispatcher_ServeHTTP_byService`, the fuzz target
  `FuzzDispatcherRouteByService`). `deckhouse_client_table_test.go`: the table format, the refused records, the
  reload, the generation, a request in flight during a reload, the file events, and `TestClientTable_isolation`,
  `TestClientTable_sharedHost` and `TestClientTable_storeScope`, where the session of one real client is refused
  by another. `pkg/sessions/persistence/scoped_store_test.go`: the scopes.
- **Upstream:** not proposed.
- **Order:** after 016 and 017: it chooses the client with the host parsing of 016 and shares the store of 017. It
  moves the body of `NewOAuthProxy` into `newOAuthProxy`, so a patch below it that changes `NewOAuthProxy` moves its
  hunks.

### 019-provider-http1.patch

- **Zone:** `pkg/requests` (a new file, `deckhouse_provider_transport.go`), the TLS settings of the provider in
  `pkg/validation`, and the HTTP client the OIDC provider (`providers/oidc.go`) and its key set
  (`pkg/providers/oidc`) put into the context of `go-oidc` and `golang.org/x/oauth2`.
- **Consumer in Deckhouse:** the shared dex-authenticator: its requests to the token endpoint of Dex (the code
  exchange and the refresh) and to its keys.
- **What it does:** the OIDC provider gets a client and a transport of its own, `requests.ProviderHTTPClient` over
  `requests.ProviderTransport`, a copy of the default transport that speaks HTTP/1.1 only.
  - **Why HTTP/1.1:** Dex is reached through its Service, and a Service balances connections, not requests. Over
    HTTP/2 every request to Dex goes over the one connection a process keeps to it, so the processes of the shared
    dex-authenticator hold a few connections, and the pod of Dex that got most of them gets most of the refreshes
    of a wave. Over HTTP/1.1 a connection carries one request at a time: a wave opens as many connections as it
    has requests in flight, and the Service spreads them over the pods of Dex.
  - **HTTP/2 off:** `Protocols` is HTTP/1 only, `ForceAttemptHTTP2` is false and `TLSNextProto` is an empty map,
    so the TLS handshake does not offer `h2` and a later change of the protocols does not turn it on.
  - **Pool:** `MaxConnsPerHost` 64: twice the 32 restores of a session from its cookie a process runs at a time
    (patch 002), with room for the logins and the refreshes of stored sessions. Four processes open at most 256
    connections to Dex, and a request over the bound waits for a free connection within its context.
    `MaxIdleConnsPerHost` 32 keeps the connections of the restores between two waves, so a steady load does not
    open a TLS connection per request; the default of `net/http` is 2. `IdleConnTimeout` 30 s: Dex sets no idle
    timeout of its own and keeps an idle connection open, so the timeout is the choice of the proxy. A connection
    idle for 30 s is closed, and the next wave opens new ones that the Service spreads over the pods of Dex as they
    are then. Under a steady low load the transport reuses the connection used last, so a few busy connections stay
    on the pods they went to until a wave or an idle pause.
  - **TLS:** `--provider-ca-file` and `--ssl-insecure-skip-verify` set the TLS of both transports, the default one
    and that of the provider.
  - **Metrics** of the pool, no label with an address, a client or a user:
    `oauth2_proxy_provider_connections_open` (open connections, idle ones included),
    `oauth2_proxy_provider_connections_in_use` (connections that carry a request, until its answer is read),
    `oauth2_proxy_provider_connection_waiting_requests` (requests that wait for a free connection of the pool or for
    a new one), `oauth2_proxy_provider_connection_wait_seconds` by `connection` (`reused` from the pool, or `new`,
    whose wait includes the TLS handshake), and `oauth2_proxy_provider_connection_wait_failures_total` by `reason`:
    `timeout` (the deadline of the request passed while it waited), `canceled`, or `error` (the connection could not
    be opened). The transport has no wait timeout of its own: a request waits within its context. A full pool shows
    as `connections_in_use` at 64 with `connection_waiting_requests` above zero. The open connections are counted
    at the dialer, the rest through `net/http/httptrace`; the metrics are on the default registry of the process.
  - **Not changed:** every other outgoing client of the proxy: the upstreams, the revocation of a refresh token at
    sign-out (patch 011), the revocation feed (patch 010), the other providers. They keep the default transport and
    HTTP/2.
- **Tests:** `pkg/requests/deckhouse_provider_transport_test.go`: a TLS server that offers HTTP/2 gets HTTP/1.1
  from the transport, parallel requests take a connection each, the pool settings, and the default transport is
  not changed. `pkg/requests/deckhouse_provider_metrics_test.go`: with the pool full the metrics show the
  requests that wait, the connections in use and open, and the wait of each request; a request that gives up
  waiting is counted by its reason; a closed connection is no longer counted. `providers/deckhouse_provider_transport_test.go`: the code exchange and the refresh go over the
  transport of the provider. `pkg/validation/deckhouse_provider_transport_test.go`: the TLS of the provider reaches
  both transports.
- **Upstream:** not proposed.
- **Order:** after 018, whose `configureProviderTransport` it changes.

### 998-fix-cve.patch

- **Zone:** `go.mod` and `go.sum`.
- **Consumer in Deckhouse:** the image scanners.
- **What it does:** bumps the modules that the tag pins at a version with a known vulnerability or a known defect.
  The patch changes only `go.mod` and `go.sum`; do not edit it by hand, rebuild it.
- **How to rebuild it** (for example, to add or drop a bump), with the tag from `oss.yaml`:

  ```shell
  git clone --branch v7.15.5 https://github.com/oauth2-proxy/oauth2-proxy.git && cd oauth2-proxy
  # Apply every patch numbered below 998, in order.
  for p in $(ls /path/to/patches/*.patch | grep -v -e '/998-' -e '/999-'); do git apply "$p"; done
  git add -A && git commit -q -m "patches below 998"
  # Find the vulnerable modules that a newer version fixes.
  govulncheck ./...
  # Bump each of them (the current list is below), then tidy and build.
  go get golang.org/x/net@v0.62.0
  go get github.com/go-jose/go-jose/v4@v4.1.6
  go get github.com/redis/go-redis/v9@v9.23.0
  go mod tidy && go build ./...
  git diff -- go.mod go.sum > /path/to/patches/998-fix-cve.patch
  ```

  Drop a bump once the tag requires that version or newer. State the reason for every bump below.
- **Current bumps:**

  - `golang.org/x/net` v0.62.0 — GO-2026-6603, GO-2026-6610, GO-2026-6611, GO-2026-6612, GO-2026-6617,
    CVE-2026-78661, CVE-2026-97045.
  - `github.com/go-jose/go-jose/v4` v4.1.6 — CVE-2026-116585, CVE-2026-116586, CVE-2026-116587.
  - `github.com/redis/go-redis/v9` v9.23.0 — panic in the `AggMin` and `AggMax` aggregators. In v9.22.0 the branch
    that handles a failed `toFloat64` conversion calls `a.err.CompareAndSwap(nil, err)`, where `err` is nil, because
    the method returns earlier when it is not. `atomic.Value.CompareAndSwap` panics on a nil value, so a shard reply
    that is not numeric, for a command with the `agg_min` or `agg_max` response policy, panics instead of returning
    an error from `Result()`. Fixed upstream in redis/go-redis#4056 and released in v9.23.0.

  Minimal version selection also moves `go.uber.org/atomic` to v1.12.0, `github.com/klauspost/cpuid/v2` to v2.4.0,
  `golang.org/x/crypto` to v0.58.0, `golang.org/x/sync` to v0.24.0, `golang.org/x/sys` to v0.49.0,
  `golang.org/x/text` to v0.43.0 and `golang.org/x/tools` to v0.51.0.
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
| `github.com/go-jose/go-jose/v4` | v4.1.5 | v4.1.6 |
| `golang.org/x/net` | v0.59.0 | v0.62.0 |
<!-- cvefix:end -->
