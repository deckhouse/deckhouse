## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-cve-grpc.patch

Bump `google.golang.org/grpc` from `v1.58.3` to `v1.79.3` to fix
[CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4)
(authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
Generated with:

```sh
go get google.golang.org/grpc@v1.79.3
go mod tidy
```

`go mod tidy` pulls a few transitive bumps (`google.golang.org/genproto/*`,
`go.opentelemetry.io/otel/*`, `golang.org/x/oauth2`, …) that grpc `v1.79.x`
requires.
