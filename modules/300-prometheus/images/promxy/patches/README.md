## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Register `OP_TOP` as a keyword and aggregate operator in the parser lexer and grammar
- Handle `op_top` argument parsing in `newAggregateExpr`
- Add `resultModifier` to the query struct and `ExtractOptTop` calls in `NewInstantQuery`
  and `NewRangeQuery` in the engine

The parser is regenerated from the `.y` grammar using `goyacc` during the build.

### 003-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function;

### 004-cve-grpc.patch

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) — `google.golang.org/grpc` bumped from `v1.58.3` to `v1.79.3` (authorization bypass via the HTTP/2 `:path` pseudo-header in gRPC-Go).
- [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) — `go.opentelemetry.io/otel` bumped from `v1.18.0` to `v1.43.0` (multi-value `baggage` header extraction causes excessive allocations).

`go.opentelemetry.io/otel v1.43.0` requires `go >= 1.25.0`, which `001-go-mod.patch` already sets, so
no `go` directive bump is needed here. The directive stays at `1.25.0` through the whole chain: the
builder `builder/golang-alpine` pinned in `candi/base_images.yml` ships Go 1.25.13.

Transitive bumps forced by the above: `github.com/envoyproxy/go-control-plane` `v0.11.1` →
`github.com/envoyproxy/go-control-plane/envoy` `v1.36.0` (module split),
`github.com/envoyproxy/protoc-gen-validate` `v1.0.2` → `v1.3.0`, `github.com/cncf/xds/go`,
`google.golang.org/genproto/googleapis/{api,rpc}`, `google.golang.org/protobuf` `v1.33.0` → `v1.36.10`,
`golang.org/x/oauth2` `v0.28.0` → `v0.34.0`, `golang.org/x/sys` `v0.40.0` → `v0.42.0`.

Generated with:

```sh
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get google.golang.org/grpc@v1.79.3 \
       go.opentelemetry.io/otel@v1.43.0 \
       go.opentelemetry.io/otel/metric@v1.43.0 \
       go.opentelemetry.io/otel/trace@v1.43.0 \
       go.opentelemetry.io/otel/sdk@v1.43.0
go mod tidy
```

### 005-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) — `go.mongodb.org/mongo-driver` bumped from `v1.8.3` to `v1.17.7`.
- `golang.org/x/crypto` bumped from `v0.47.0` to `v0.52.0`, fixing
  [CVE-2024-45337](https://nvd.nist.gov/vuln/detail/CVE-2024-45337),
  [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827) … [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835),
  [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508),
  [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595),
  [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597),
  [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598).
- `golang.org/x/net` bumped from `v0.48.0` to `v0.55.0`, fixing
  [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680),
  [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681),
  [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136),
  [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814),
  [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821),
  [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502),
  [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506).
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` bumped from `v0.42.0` to `v0.45.0` (the CVE's own fixed version `v0.44.0` is exceeded by MVS resolution).

Generated with:

```sh
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get go.mongodb.org/mongo-driver@v1.17.7 \
       golang.org/x/crypto@v0.52.0 \
       golang.org/x/net@v0.55.0 \
       golang.org/x/sys@v0.45.0
go mod tidy
```

`go mod tidy` pulls a few additional transitive bumps (`golang.org/x/mod`, `golang.org/x/sync`,
`golang.org/x/term`, `golang.org/x/text`, `golang.org/x/tools`) required by the above.

### 006-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf) — `google.golang.org/grpc` bumped from `v1.79.3` to `v1.82.1`.
- [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) — `golang.org/x/net` bumped from `v0.55.0` to `v0.56.0`.
- [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) — `golang.org/x/text` bumped from `v0.37.0` to `v0.39.0`.

MVS also lifts `golang.org/x/crypto` to `v0.53.0` and `golang.org/x/sys` to `v0.46.0` here.

Generated with:

```sh
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get google.golang.org/grpc@v1.82.1 \
       golang.org/x/net@v0.56.0 \
       golang.org/x/text@v0.39.0
go mod tidy
```

### op_func.go.tpl, op_top.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after `go mod vendor`.

New Go source files adding custom PromQL op-functions (`op_defined`, `op_replace_nan`,
`op_smoothie`, `op_zero_if_none`) and the `op_top` aggregate operator to the vendored
Prometheus engine. These are adapted for the older Prometheus API used by the
`jacksontj/prometheus` fork (uses `Point.V` instead of `FPoint.F`, etc.).

### 007-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595), [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) — `golang.org/x/crypto` bumped from `v0.53.0` to `v0.55.0` (source-address restrictions returned by SSH authentication callbacks were not enforced).
- [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303), [CVE-2026-84304](https://github.com/advisories/GHSA-vp52-pcj8-j9qc), [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445) — `google.golang.org/grpc` bumped from `v1.82.1` to `v1.83.2`.
- [GHSA-w67g-5rqw-f597](https://github.com/advisories/GHSA-w67g-5rqw-f597) — `github.com/gorilla/websocket` bumped from `v1.5.0` to `v1.5.3`.

Two deliberate differences from the external `flant/prometheus` repo, where the same round landed as
`241876b`:

- **grpc `v1.83.2`, not `v1.83.1`.** The DKP 1.76 finding set includes `CVE-2026-84445`, whose fixed
  versions are `1.82.2` / `1.83.2`; `v1.83.1` would leave it open.
- **`golang.org/x/crypto` `v0.55.0`, not `v0.56.0`.** `v0.56.0` declares `go 1.26.0` in its own
  `go.mod`, which would raise this module's `go` directive to `1.26.0` and break the build: the
  monorepo builder `builder/golang-alpine` is pinned in `candi/base_images.yml` to the
  `builder/golang-alpine-1.25` digest (Go 1.25.13); there is no 1.26/1.27 key there. The external
  repo could take `v0.56.0` only because it moved to base images `v2.1.10` with Go 1.27.
  The two findings that are only fixed in `v0.56.0` — [CVE-2026-56855](https://nvd.nist.gov/vuln/detail/CVE-2026-56855)
  (GO-2026-6355) and [CVE-2026-78662](https://nvd.nist.gov/vuln/detail/CVE-2026-78662) (GO-2026-6354),
  both confined to `golang.org/x/crypto/ssh` — are covered by VEX statements in
  `images/promxy/known_vulnerabilities.vex`; `golang.org/x/crypto/ssh` is not in the import graph of
  `./cmd/promxy`.

Transitive bumps forced by grpc `v1.83.2` via MVS: `google.golang.org/api` `v0.128.0` → `v0.264.0`,
`google.golang.org/genproto/googleapis/{api,rpc}` to the 2026-05-26 revision,
`cloud.google.com/go/auth` `v0.18.2` and `.../auth/oauth2adapt` `v0.2.8` (new),
`github.com/googleapis/gax-go/v2` `v2.12.0` → `v2.17.0`,
`github.com/googleapis/enterprise-certificate-proxy` `v0.2.5` → `v0.3.11`,
`github.com/google/s2a-go` `v0.1.4` → `v0.1.9`, `github.com/felixge/httpsnoop` `v1.0.3` → `v1.0.4`,
`go.opentelemetry.io/otel{,/metric,/trace}` `v1.43.0` → `v1.44.0`,
`go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` `v0.44.0` → `v0.61.0`,
the rest of the `golang.org/x/*` family and `golang.org/x/time` `v0.11.0` → `v0.14.0`.
`go.opencensus.io`, `google.golang.org/appengine` and `github.com/golang/groupcache` drop out of the graph.
The `go` directive stays `1.25.0`.

The vendored `github.com/prometheus/prometheus` stays at the `jacksontj/prometheus` pseudo-version
pinned by the existing `replace`, so `002-op-functions.patch`,
`003-printer-op-top-aggregate-string.patch` and the `op_*.go.tpl` files still apply. Verified by
reproducing the werf recipe locally on a pristine `v0.0.93` clone (apply `001`, `004`-`007`,
`go mod vendor`, copy the `.tpl` files, apply `002`/`003` into
`vendor/github.com/prometheus/prometheus`, `goyacc`, `go build ./cmd/promxy`) — the build succeeds.

Note that `werf.inc.yaml` applies the `go.mod` patches **by name**, not by glob, so every new patch
here needs its own `git apply` line in the `-src-artifact` stage.

Generated with:

```sh
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get golang.org/x/crypto@v0.55.0 google.golang.org/grpc@v1.83.2 github.com/gorilla/websocket@v1.5.3
go mod tidy
git diff -- go.mod go.sum > 007-fix-cve-bump.patch
```

`github.com/docker/docker` stays at `v25.0.6+incompatible` (an indirect dependency of the pinned
`prometheus/prometheus` fork). `CVE-2026-33997`, `CVE-2026-41567`, `CVE-2026-41568` and
`CVE-2026-42306` are covered by VEX in `images/promxy/known_vulnerabilities.vex`: only the Docker
client SDK is linked, and `v29.3.1` does not exist as a Go module — the newest published version of
`github.com/docker/docker` is `v28.5.2+incompatible`, `github.com/docker/docker/v29` has no published
versions, and upstream `moby/moby` has no `v29` tag.
