# Patches

## 001-go-mod.patch

Single source of all `go.mod`/`go.sum` changes for the image (the dependency bumps
from `004-cve-grpc-jsonparser.patch` are folded in here, because `go.sum` is one
re-resolved artifact that cannot be cleanly split across two patches). Bumps the `go`
directive to `1.25.8` (required by `go.opentelemetry.io/otel v1.43.0`) and fixes CVEs by
bumping:

- `golang.org/x/crypto` → `v0.53.0` (SSH advisories GO-2026-5006/5013–5023, fixed in 0.52.0).
- `golang.org/x/net` → `v0.56.0` (net/html XSS GO-2026-5025/5027/5029/5030, idna GO-2026-5026, http2 GO-2026-4918, dnsmessage GO-2026-5942).
- `golang.org/x/sys` → `v0.46.0` (GO-2026-5024; also the minimum required by `x/crypto v0.53.0`).
- `golang.org/x/text` → `v0.39.0` (GO-2026-5970 norm.Iter infinite loop).
- `go.mongodb.org/mongo-driver` → `v1.17.7` (CVE-2026-2303 GSSAPI heap out-of-bounds read).

It also carries the grpc/otel/protobuf/jsonparser bumps described under
`004-cve-grpc-jsonparser.patch`. `GO-2026-5932` (`golang.org/x/crypto/openpgp` is
unmaintained) has no fixed release and is handled via `known_vulnerabilities.vex`.

Regenerated with (against a fresh `grafana/loki` v2.9.15 clone, after applying `002`/`003`
and the grpc/otel/api bumps below):

```sh
go get golang.org/x/crypto@v0.53.0 golang.org/x/net@v0.56.0 \
       golang.org/x/sys@v0.46.0 golang.org/x/text@v0.39.0 \
       go.mongodb.org/mongo-driver@v1.17.7
go mod tidy
```

## 002-Allow-delete-logs.patch

Enable/disable `/loki/api/v1/delete` endpoints by setting `ALLOW_DELETE_LOGS` env value to true/false.

## 003-Force-expiration.patch

Automatically delete old logs by setting `force_expiration_threshold` higher than 0.

## 004-cve-grpc-jsonparser.patch

Code-only shim that accompanies the grpc/otel/jsonparser CVE bumps (whose `go.mod`/`go.sum`
changes now live in `001-go-mod.patch`). Those bumps were:
- [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) — `google.golang.org/grpc` from `v1.59.0` to `v1.79.3` (authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
- [CVE-2026-32285](https://github.com/advisories/GHSA-) — `github.com/buger/jsonparser` from `v1.1.1` to `v1.1.2`.
- [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) — `go.opentelemetry.io/otel` (and `otel/metric`, `otel/sdk`, `otel/trace`) from `v1.21.0` (upstream Loki v2.9.15) to `v1.43.0` (multi-value `baggage` header extraction causes excessive allocations).

`go.opentelemetry.io/otel v1.43.0` requires `go >= 1.25.0` in its `go.mod`,
so the `go` directive is bumped to `1.25.8` in `001-go-mod.patch`.

This patch itself contains only a small `healthCheckWithList` wrapper in `pkg/loki/loki.go`,
required because dskit's `grpcutil.HealthCheck` (pinned at the loki v2.9.15 version) does
not implement the `List` RPC that grpc >= 1.64 added to the
`grpc_health_v1.HealthServer` interface. Bumping dskit to a version that
implements `List` would cascade into incompatible memberlist/prometheus changes.

## known_vulnerabilities.vex

OpenVEX statements attached to the loki image via the `vex mitigation` werf
template (see `.werf/defines/vex.tmpl` and `werf.inc.yaml`). Marks Loki as
`not_affected` by the following CVEs in `github.com/prometheus/prometheus`,
which is pulled in only as a library (Loki does not run the Prometheus server,
`/api/v1/read` endpoint, or the legacy web UI):
- [CVE-2026-42154](https://github.com/advisories/GHSA-8rm2-7qqf-34qm) — Prometheus remote read endpoint DoS via crafted snappy payload.
- [CVE-2026-44903](https://github.com/advisories/GHSA-fw8g-cg8f-9j28) — Stored XSS in the legacy Prometheus web UI heatmap (`--enable-feature=old-ui`).

It also marks Loki `not_affected` by:
- [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) — the `golang.org/x/crypto/openpgp` package is unmaintained and unsafe by design. It has no fixed release (bumping `x/crypto` to `v0.53.0` does not clear it). The `/usr/bin/loki` binary does not import `openpgp` and performs no OpenPGP operations.
