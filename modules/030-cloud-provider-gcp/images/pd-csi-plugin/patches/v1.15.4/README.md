## Patches

### 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

### 002-go-mod.patch

Bump go.mod dependencies to fix known CVEs:
- CVE-2026-56854 (CRITICAL), CVE-2026-56855, CVE-2026-78662: golang.org/x/crypto -> v0.56.0
- CVE-2026-84304, GHSA-hrxh-6v49-42gf: google.golang.org/grpc -> v1.83.1
- transitively required by the above: golang.org/x/net -> v0.58.0, golang.org/x/text -> v0.41.0, golang.org/x/mod -> v0.40.0
