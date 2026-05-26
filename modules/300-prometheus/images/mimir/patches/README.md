## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-22868](https://github.com/advisories/GHSA-6v2p-p543-phr9)
- [CVE-2025-27144](https://github.com/advisories/GHSA-c6gw-w398-hv78)
- [CVE-2025-30204](https://github.com/advisories/GHSA-mh63-6h87-95cp)
- [CVE-2024-45339](https://github.com/advisories/GHSA-6wxm-mpqj-6jpf)

### 002-fix-cves-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) — `google.golang.org/grpc` bumped from v1.65.0 (replace pin) / v1.66.0 (require) to v1.79.3.
- [CVE-2026-34986](https://github.com/advisories/GHSA-78h2-9frx-2jm8) — `github.com/go-jose/go-jose/v4` bumped from v4.0.5 to v4.1.4.
- [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) — `go.opentelemetry.io/otel` bumped from v1.29.0 to v1.43.0 (multi-value `baggage` header extraction causes excessive allocations).
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq) — `go.opentelemetry.io/otel/sdk` bumped from v1.28.0 to v1.43.0 (Darwin `ioreg` PATH hijacking in resource detection).
- [CVE-2026-39883](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx) — `go.opentelemetry.io/otel/sdk` bumped from v1.28.0 to v1.43.0 (BSD/Solaris `kenv` PATH hijacking in resource detection).

The `replace google.golang.org/grpc => google.golang.org/grpc v1.65.0` pin
from upstream `go.mod` is removed by this patch.

`go.opentelemetry.io/otel v1.43.0` requires `go >= 1.25.0` in its `go.mod`,
so the `go` directive is bumped from `1.24.0` to `1.25.8`.

Generated with:

```sh
go mod edit -dropreplace=google.golang.org/grpc -go=1.25.8
go get google.golang.org/grpc@v1.79.3 \
       github.com/go-jose/go-jose/v4@v4.1.4 \
       go.opentelemetry.io/otel@v1.43.0 \
       go.opentelemetry.io/otel/metric@v1.43.0 \
       go.opentelemetry.io/otel/trace@v1.43.0 \
       go.opentelemetry.io/otel/sdk@v1.43.0
go mod tidy
```

`go mod tidy` pulls a few transitive bumps that grpc `v1.79.x` and otel
`v1.43.x` require.

### 003-grpc-health-list.patch

Applied to vendored `github.com/grafana/dskit` after `go mod vendor`.

gRPC-Go v1.72+ added a `List` method to the `grpc_health_v1.HealthServer` interface,
but the dskit revision pinned by mimir 2.14.3 (`v0.0.0-20240920183844-560bb26f205e`)
predates that change. After bumping `google.golang.org/grpc` to `v1.79.3` in
`002-fix-cves-bump.patch`, the embedded `*grpcutil.HealthCheck` no longer satisfies
the interface, breaking the build.

The patch back-ports the `List` implementation from upstream
[grafana/dskit#689](https://github.com/grafana/dskit/pull/689) onto the vendored
`grpcutil/health_check.go` so `grpc_health_v1.RegisterHealthServer` accepts
dskit's `HealthCheck` again. We patch the vendored copy (rather than bumping
dskit itself) because newer dskit revisions also rework `DialOption`, `SpanLogger`,
etc. — incompatible API changes that would require a much larger update.
