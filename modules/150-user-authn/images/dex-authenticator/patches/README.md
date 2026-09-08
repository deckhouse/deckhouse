## Patches

### 001-go-mod.patch

Update deps.

### 002-cookie-refresh.patch

There is a problem when we are using nonpersistent Redis for session storage. If Redis was killed or flushed, the user should be authenticated again. That makes oauth2 proxy to be stateful application (or sort of).
Storing refresh token in cookie adds the possibility to restore access- and id- token even if there is no data in Redis.

Upstream PR - https://github.com/oauth2-proxy/oauth2-proxy/pull/313

### 003-remove-groups.patch

Prevents sending groups auth request header (may cause uncontrollable headers grows).
Two options to fix this without patch:

Add a new flag: https://github.com/oauth2-proxy/oauth2-proxy/issues/2144
Migrate to a structured config (alpha config): https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/alpha-config

### 004-add-redis-retries.patch

Prevents oauth2-proxy from failing with exit 1 if Redis has not started in time. Adds a loop to retry sendRedisConnectionTest.

### 005-fix-cves.patch

Fixes CVE-2025-30204 CVE-2025-22868 CVE-2024-28180

### 006-return-200-on-success-and-header-on-fail.patch

Oauth2-proxy returns 200 (instead of 202) when the request is authenticated and adds "X-Auth-Request-Result" header on fail.


### 999-fuzz-handlers.patch

Go native fuzz tests for oauth2-proxy user-input surfaces. Applied last.

Targets:

- HTTP mux via `ServeHTTP`: `/`, `/robots.txt`, `/oauth2/sign_in`,
  `/oauth2/sign_out`, `/oauth2/start`, `/oauth2/callback`, `/oauth2/auth`,
  `/oauth2/userinfo`, cookies, `Authorization`, `X-Forwarded-*`
- `ManualSignIn`, `LoadCookiedSession`, `redeemCode`
- Deckhouse cookie-refresh JSON ticket parser (`decodeTicket`,
  `decodeTicketFromRequest`)
- JWT bearer/basic session loader

`*_test.go` is not compiled into the image. Run from a patched source tree:

```
go test -run=^$ -fuzz=FuzzServeHTTP -fuzztime=12h .
go test -run=^$ -fuzz=FuzzDecodeTicket$ -fuzztime=12h ./pkg/sessions/persistence
go test -run=^$ -fuzz=FuzzJwtSessionLoader$ -fuzztime=12h ./pkg/middleware
```
