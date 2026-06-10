# Patches

## 001-go-mod.patch

    Fix CVEs in crypto/net packages and bump the `go` directive to `1.25.8`
    (required by `go.opentelemetry.io/otel v1.43.0`, see `004-cve-grpc-jsonparser.patch`).
    ```sh
    go mod edit -go=1.25.8
    go get golang.org/x/crypto v0.31.0
    go get golang.org/x/net v0.33.0
    go mod tidy
    ```

## 002-Allow-delete-logs.patch

Enable/disable `/loki/api/v1/delete` endpoints by setting `ALLOW_DELETE_LOGS` env value to true/false.

## 003-Force-expiration.patch

Automatically delete old logs by setting `force_expiration_threshold` higher than 0.

## 004-cve-grpc-jsonparser.patch

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) — `google.golang.org/grpc` bumped from `v1.59.0` to `v1.79.3` (authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
- [CVE-2026-32285](https://github.com/advisories/GHSA-) — `github.com/buger/jsonparser` bumped from `v1.1.1` to `v1.1.2`.
- [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) — `go.opentelemetry.io/otel` (and `otel/metric`, `otel/sdk`, `otel/trace`) bumped from `v1.21.0` (upstream Loki v2.9.15) to `v1.43.0` (multi-value `baggage` header extraction causes excessive allocations).

`go.opentelemetry.io/otel v1.43.0` requires `go >= 1.25.0` in its `go.mod`,
so the `go` directive is bumped from `1.24.0` to `1.25.8` in `001-go-mod.patch`.

The Loki upstream `replace go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp => ... v0.44.0`
is left untouched because newer otelhttp releases drop API surface the pinned
weaveworks/common code still relies on; otel `v1.43.0` is wire-compatible with
this older otelhttp version.

Generated with:

```sh
# Apply 001-go-mod.patch (with go 1.25.8), 002-Allow-delete-logs.patch and
# 003-Force-expiration.patch first, then:
go get google.golang.org/grpc@v1.79.3
go get github.com/buger/jsonparser@v1.1.2
# Minimum google.golang.org/api version that uses keyed-field initialization
# of grpcgoogle.DefaultCredentialsOptions (vendored google API code does not
# compile against grpc >= 1.64 otherwise).
go get google.golang.org/api@v0.155.0
go get \
  go.opentelemetry.io/otel@v1.43.0 \
  go.opentelemetry.io/otel/sdk@v1.43.0 \
  go.opentelemetry.io/otel/metric@v1.43.0 \
  go.opentelemetry.io/otel/trace@v1.43.0
go mod tidy -e
```

`go mod tidy` pulls a few transitive bumps (`google.golang.org/genproto/*`,
`golang.org/x/oauth2`, …) required by the new grpc/otel versions.

The patch also adds a small `healthCheckWithList` wrapper in `pkg/loki/loki.go`
because dskit's `grpcutil.HealthCheck` (pinned at the loki v2.9.15 version) does
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
