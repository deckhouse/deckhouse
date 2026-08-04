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

### 006-return-200-on-success-and-header-on-fail.patch

Oauth2-proxy returns 200 (instead of 202) when the request is authenticated and adds "X-Auth-Request-Result" header on fail.

### 007-add-json-logging.patch

Add json logging.

### 999-fix-cve.patch

#### Fix CVEs

- CVE-2024-28180
- CVE-2025-22868
- CVE-2025-30204
- CVE-2026-25680
- CVE-2026-25681
- CVE-2026-27136
- CVE-2026-33186
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
