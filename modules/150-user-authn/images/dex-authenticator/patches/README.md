## Patches

### 001-go-mod.patch

Update deps.

### 002-cookie-refresh.patch

There is a problem when we are using nonpersistent Redis for session storage. If Redis was killed or flushed, the user should be authenticated again. That makes oauth2 proxy to be stateful application (or sort of).
Storing refresh token in cookie adds the possibility to restore access- and id- token even if there is no data in Redis.

Upstream PR - https://github.com/oauth2-proxy/oauth2-proxy/pull/313

A persistent store that holds no data for a ticket returns a stub session instead of an error.
The stub holds the refresh token from the cookie and the placeholder user `Does not found in Redis`.
With `--cookie-refresh` above zero, the proxy refreshes the stub instead of trusting it.
If the cookie carries no refresh token, the store returns an error, so a stub never passes
as an authenticated session.
The session store tests expect this behaviour.

Sign-out does not invalidate a previously issued cookie.
A copy of the cookie restores the session while both conditions hold:
the refresh token in the cookie is still valid in dex,
and the cookie itself has not expired (`--cookie-expire`, set from `keepUsersLoggedInFor`).

### 003-remove-groups.patch

Prevents sending groups auth request header (may cause uncontrollable headers grows).
Two options to fix this without patch:

Add a new flag: https://github.com/oauth2-proxy/oauth2-proxy/issues/2144
Migrate to a structured config (alpha config): https://oauth2-proxy.github.io/oauth2-proxy/docs/configuration/alpha-config

### 004-add-redis-retries.patch

Prevents oauth2-proxy from failing with exit 1 if Redis has not started in time. Adds a loop to retry sendRedisConnectionTest.

### 005-fix-cves.patch

This patch fixes:

- CVE-2025-30204
- CVE-2025-22868
- CVE-2024-28180
- CVE-2026-34986
- CVE-2026-33186

The login.gov provider embeds `jwt.RegisteredClaims` in its ID token claims, as `golang-jwt/jwt/v5`
requires. An embedded `jwt.Claims` interface stays nil and makes token validation panic.
The provider parses the ID token with `jwt.WithIssuedAt()`, so a token with an `iat` claim in the
future is rejected, as with `StandardClaims` of `jwt/v3`.

The patch also adapts upstream tests to the newer toolchain and dependencies:

- the JSON decoding error messages of Go 1.24 and later, including the field names of Go 1.27;
- an unreachable upstream address that still parses under the strict URL parsing of Go 1.26;
- valid JSON in a mocked Google Directory API response.

### 006-return-200-on-success-and-header-on-fail.patch

Oauth2-proxy returns 200 (instead of 202) when the request is authenticated and adds "X-Auth-Request-Result" header on fail.

### 007-add-json-logging.patch

Add json logging.

### 998-fix-cve.patch

Additional dependency bumps on top of `005-fix-cves.patch`.

Fix CVEs:
- CVE-2025-29923
- CVE-2026-25680
- CVE-2026-25681
- CVE-2026-27136
- CVE-2026-33814
- CVE-2026-39821
- CVE-2026-39824
- CVE-2026-42502
- CVE-2026-42506
- CVE-2026-46600
- CVE-2026-56852
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78662
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445

GHSA:
- GHSA-hrxh-6v49-42gf

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

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `998-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

| Module | Upstream | Patched |
|---|---|---|
| `cloud.google.com/go/auth/oauth2adapt` | — | v0.2.8 |
| `cloud.google.com/go/auth` | — | v0.18.2 |
| `github.com/felixge/httpsnoop` | — | v1.0.4 |
| `github.com/go-logr/stdr` | — | v1.2.2 |
| `github.com/google/s2a-go` | v0.1.4 | v0.1.9 |
| `github.com/googleapis/enterprise-certificate-proxy` | v0.2.3 | v0.3.11 |
| `github.com/googleapis/gax-go/v2` | v2.11.0 | v2.17.0 |
| `github.com/redis/go-redis/v9` | v9.0.2 | v9.6.3 |
| `github.com/stretchr/testify` | v1.8.4 | v1.11.1 |
| `go.opentelemetry.io/auto/sdk` | — | v1.2.1 |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | — | v0.61.0 |
| `go.opentelemetry.io/otel/metric` | — | v1.44.0 |
| `go.opentelemetry.io/otel/trace` | — | v1.44.0 |
| `go.opentelemetry.io/otel` | — | v1.44.0 |
| `golang.org/x/crypto` | v0.46.0 | v0.57.0 |
| `golang.org/x/net` | v0.48.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.34.0 | v0.36.0 |
| `golang.org/x/sync` | v0.19.0 | v0.23.0 |
| `golang.org/x/sys` | v0.39.0 | v0.48.0 |
| `golang.org/x/text` | v0.32.0 | v0.42.0 |
| `google.golang.org/api` | v0.126.0 | v0.264.0 |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20251202230838-ff82c1b0f217 | v0.0.0-20260526163538-3dc84a4a5aaa |
| `google.golang.org/grpc` | v1.79.3 | v1.83.2 |
| `google.golang.org/protobuf` | v1.36.10 | v1.36.11 |

The `go` directive of `go.mod` is raised from 1.24.0 to 1.26.0: the patched dependencies require it.
<!-- cvefix:end -->
