## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-cve-grpc.patch

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) — `google.golang.org/grpc` bumped from `v1.58.3` to `v1.79.3` (authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
- [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) — `go.opentelemetry.io/otel` bumped from `v1.18.0` to `v1.43.0` (multi-value `baggage` header extraction causes excessive allocations).

`go.opentelemetry.io/otel v1.43.0` requires `go >= 1.25.0` in its `go.mod`,
so the `go` directive is bumped from `1.24.0` to `1.25.8`.

Generated with:

```sh
go mod edit -go=1.25.8
go get google.golang.org/grpc@v1.79.3 \
       go.opentelemetry.io/otel@v1.43.0 \
       go.opentelemetry.io/otel/metric@v1.43.0 \
       go.opentelemetry.io/otel/trace@v1.43.0 \
       go.opentelemetry.io/otel/sdk@v1.43.0
go mod tidy
```

`go mod tidy` pulls a few transitive bumps (`google.golang.org/genproto/*`,
`golang.org/x/oauth2`, …) that grpc `v1.79.x` and otel `v1.43.x` require.
