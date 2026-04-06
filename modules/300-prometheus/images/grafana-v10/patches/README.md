## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-nodejs.patch

Update dependencies to fix CVEs
- [CVE-2025-68470](https://github.com/advisories/GHSA-9jcx-v3wj-wh4m)

### 003-fix-cves.patch

Fix CVE-2026-33186 (`google.golang.org/grpc`).
```sh
go get google.golang.org/grpc@v1.79.3
go mod tidy
```

### 004-fix-cves.patch

Fix CVE-2026-24051 (`go.opentelemetry.io/otel/sdk`).
```sh
go get go.opentelemetry.io/otel/sdk@v1.40.0
go mod tidy
```

### 005-fix-cves.patch

Fix CVE-2026-33487 (`github.com/russellhaering/goxmldsig`) and CVE-2026-34986 (`github.com/go-jose/go-jose/v3`).
```sh
go get github.com/russellhaering/goxmldsig@v1.6.0 github.com/go-jose/go-jose/v3@v3.0.5
go mod tidy
```

### 006-fix-cves.patch

Fix CVE-2026-1229 (`github.com/cloudflare/circl`).
```sh
go get github.com/cloudflare/circl@v1.6.3
go mod tidy
```
