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

### 004-cve-go-mod.patch

Single cumulative dependency bump for the whole CVE round (it replaces the former incremental
chain `004-cve-grpc.patch`, `005`/`006`/`007-fix-cve-bump.patch`, which walked the same
dependencies up in several steps). It touches only `go.mod` and `go.sum` and is applied on top of
`001-go-mod.patch`.

Final versions after the patch:

| Module | `001` | `004` |
|---|---|---|
| `google.golang.org/grpc` | `v1.58.3` | `v1.83.2` |
| `go.opentelemetry.io/otel{,/metric,/trace}` | `v1.18.0` | `v1.44.0` |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | `v0.44.0` | `v0.61.0` |
| `go.mongodb.org/mongo-driver` | `v1.8.3` | `v1.17.7` |
| `github.com/gorilla/websocket` | `v1.5.0` | `v1.5.3` |
| `golang.org/x/crypto` | `v0.47.0` | `v0.55.0` |
| `golang.org/x/net` | `v0.48.0` | `v0.58.0` |
| `golang.org/x/sys` | `v0.40.0` | `v0.47.0` |
| `golang.org/x/text` | `v0.33.0` | `v0.41.0` |
| `golang.org/x/mod` | `v0.31.0` | `v0.38.0` |
| `google.golang.org/protobuf` | `v1.33.0` | `v1.36.11` |

CVEs closed by the bump:

- `google.golang.org/grpc` → `v1.83.2`:
  [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4) (authorization bypass via the
  HTTP/2 `:path` pseudo-header), [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf),
  [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303),
  [CVE-2026-84304](https://github.com/advisories/GHSA-vp52-pcj8-j9qc),
  [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445).
- `go.opentelemetry.io/otel` → `v1.44.0`:
  [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475) (multi-value `baggage` header
  extraction causes excessive allocations).
- `go.mongodb.org/mongo-driver` → `v1.17.7`:
  [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303).
- `github.com/gorilla/websocket` → `v1.5.3`:
  [GHSA-w67g-5rqw-f597](https://github.com/advisories/GHSA-w67g-5rqw-f597).
- `golang.org/x/crypto` → `v0.55.0`:
  [CVE-2024-45337](https://nvd.nist.gov/vuln/detail/CVE-2024-45337),
  [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827) … [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835),
  [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508),
  [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595),
  [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597),
  [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598),
  [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854).
- `golang.org/x/net` → `v0.58.0`:
  [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680),
  [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681),
  [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136),
  [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814),
  [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821),
  [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502),
  [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506),
  [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600).
- `golang.org/x/sys` → `v0.47.0`:
  [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824).
- `golang.org/x/text` → `v0.41.0`:
  [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852).

Constraints baked into the patch:

- **`golang.org/x/crypto` stays at `v0.55.0`, not `v0.56.0`.** `v0.56.0` declares `go 1.26.0` in its
  own `go.mod`, which would raise this module's `go` directive to `1.26.0` and break the build: the
  monorepo builder `builder/golang-alpine` is pinned in `candi/base_images.yml` to the
  `builder/golang-alpine-1.25` digest (Go 1.25.13); there is no 1.26/1.27 key there. The two
  findings that are only fixed in `v0.56.0` —
  [CVE-2026-56855](https://nvd.nist.gov/vuln/detail/CVE-2026-56855) (GO-2026-6355) and
  [CVE-2026-78662](https://nvd.nist.gov/vuln/detail/CVE-2026-78662) (GO-2026-6354), both confined to
  `golang.org/x/crypto/ssh` — are covered by VEX statements in
  `images/promxy/known_vulnerabilities.vex`; `golang.org/x/crypto/ssh` is not in the import graph of
  `./cmd/promxy`.
- **grpc `v1.83.2`, not `v1.83.1`.** The DKP 1.76 finding set includes `CVE-2026-84445`, whose fixed
  versions are `1.82.2` / `1.83.2`; `v1.83.1` would leave it open.
- The `go` directive stays `1.25.0` and no `toolchain` directive is added.

Other transitive movement forced by MVS: the `envoyproxy/go-control-plane` module split
(`v0.11.1` → `github.com/envoyproxy/go-control-plane/envoy v1.37.0`),
`github.com/envoyproxy/protoc-gen-validate` `v1.0.2` → `v1.3.3`, `github.com/cncf/xds/go`,
`google.golang.org/api` `v0.128.0` → `v0.264.0`,
`google.golang.org/genproto/googleapis/{api,rpc}` to the 2026-05-26 revision,
`cloud.google.com/go/auth` `v0.18.2` and `.../auth/oauth2adapt` `v0.2.8` (new),
`github.com/googleapis/gax-go/v2` `v2.12.0` → `v2.17.0`,
`github.com/googleapis/enterprise-certificate-proxy` `v0.2.5` → `v0.3.11`,
`github.com/google/s2a-go` `v0.1.4` → `v0.1.9`,
`github.com/felixge/httpsnoop` `v1.0.3` → `v1.0.4`,
`golang.org/x/oauth2` `v0.28.0` → `v0.36.0`, `golang.org/x/time` `v0.11.0` → `v0.14.0`.
`go.opencensus.io`, `google.golang.org/appengine` and `github.com/golang/groupcache` drop out of the
graph.

The vendored `github.com/prometheus/prometheus` stays at the `jacksontj/prometheus` pseudo-version
pinned by the existing `replace`, so `002-op-functions.patch`,
`003-printer-op-top-aggregate-string.patch` and the `op_*.go.tpl` files still apply. Verified by
reproducing the werf recipe locally on a pristine `v0.0.93` clone (apply `001`, `004`,
`go mod vendor`, copy the `.tpl` files, apply `002`/`003` into
`vendor/github.com/prometheus/prometheus`, `goyacc`, `go build ./cmd/promxy`) — the build succeeds.

Note that `werf.inc.yaml` applies the `go.mod` patches **by name**, not by glob, so every new patch
here needs its own `git apply` line in the `-src-artifact` stage.

Generated with:

```sh
git clone --depth 1 --branch v0.0.93 https://github.com/jacksontj/promxy.git /src && cd /src
git apply 001-go-mod.patch && git commit -am base
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get google.golang.org/grpc@v1.83.2 \
       go.opentelemetry.io/otel@v1.44.0 \
       go.opentelemetry.io/otel/metric@v1.44.0 \
       go.opentelemetry.io/otel/trace@v1.44.0 \
       go.mongodb.org/mongo-driver@v1.17.7 \
       github.com/gorilla/websocket@v1.5.3 \
       golang.org/x/crypto@v0.55.0 \
       golang.org/x/net@v0.58.0 \
       golang.org/x/sys@v0.47.0 \
       golang.org/x/text@v0.41.0
go mod tidy
git diff -- go.mod go.sum > 004-cve-go-mod.patch
```

`github.com/docker/docker` stays at `v25.0.6+incompatible` (an indirect dependency of the pinned
`prometheus/prometheus` fork). `CVE-2026-33997`, `CVE-2026-41567`, `CVE-2026-41568` and
`CVE-2026-42306` are covered by VEX in `images/promxy/known_vulnerabilities.vex`: only the Docker
client SDK is linked, and `v29.3.1` does not exist as a Go module — the newest published version of
`github.com/docker/docker` is `v28.5.2+incompatible`, `github.com/docker/docker/v29` has no published
versions, and upstream `moby/moby` has no `v29` tag.

### op_func.go.tpl, op_top.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after `go mod vendor`.

New Go source files adding custom PromQL op-functions (`op_defined`, `op_replace_nan`,
`op_smoothie`, `op_zero_if_none`) and the `op_top` aggregate operator to the vendored
Prometheus engine. These are adapted for the older Prometheus API used by the
`jacksontj/prometheus` fork (uses `Point.V` instead of `FPoint.F`, etc.).
