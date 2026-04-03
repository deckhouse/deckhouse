## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [GO-2025-3900](https://github.com/advisories/GHSA-2464-8j7c-4cjm)

### 002-fix-cves.patch

Fix CVE-2026-33186 and CVE-2026-24051.
```sh
go get google.golang.org/grpc@v1.79.3
go get go.opentelemetry.io/otel/sdk@v1.40.0
go mod tidy
```
