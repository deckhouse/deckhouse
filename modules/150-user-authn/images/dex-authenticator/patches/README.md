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

Fixes
- CVE-2025-30204
- CVE-2025-22868
- CVE-2024-28180
- CVE-2025-47914
- CVE-2025-58181
- CVE-2026-34986
- CVE-2026-39828 (`golang.org/x/crypto` -> `v0.53.0`)
- CVE-2026-25680, CVE-2026-25681, CVE-2026-33814, CVE-2026-39821, CVE-2026-46600 (`golang.org/x/net` -> `v0.56.0`)
- CVE-2026-39824 (`golang.org/x/sys` -> `v0.46.0`)
- CVE-2026-56852 (`golang.org/x/text` -> `v0.39.0`)
- GHSA-hrxh-6v49-42gf (`google.golang.org/grpc` -> `v1.82.1`)

(previously split across 005/006; consolidated into one patch)