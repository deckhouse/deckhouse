## Patches

Patches are applied by `werf.inc.yaml` with a glob (`git apply /patches/*.patch`), so they are applied
in file-name order and a new file is picked up automatically.

The `go` directive is `1.25.0` through the whole chain (both `go.mod` and `go.work`), and no
`toolchain` directive is added: the builder `builder/golang-alpine` pinned in `candi/base_images.yml`
resolves to the `builder/golang-alpine-1.25` digest and ships Go 1.25.13, and there is no 1.26/1.27
key there.

### 001-go-mod.patch

Update dependencies to fix CVEs and set the `go` directive to `1.25.0` in `go.mod` and `go.work`.
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-nodejs.patch

Pre-existing patch (added in #17597). It pins `react-router` to `6.30.2` in `/src/grafana`'s
`package.json`/`yarn.lock`. Note that it does not affect the shipped artifact: the image never runs
`yarn`, and the whole frontend (`/usr/share/grafana/public`, including
`public/app/plugins/datasource/tempo/node_modules/react-router` that the scanner reports) is
imported prebuilt from `grafana-deps.git` at tag `v10.4.19` (see `werf.inc.yaml`). That is why the
four react-router findings are covered by VEX in `images/grafana-v10/known_vulnerabilities.vex`.
The patch itself is left untouched by this CVE round.

### 003-cve-go-mod.patch

Single cumulative dependency bump for the whole CVE round. It replaces the former incremental chain
`003-cve-go-mod.patch` + `004`/`005`/`006`/`007-fix-cve-bump.patch`, which walked the same
dependencies up in several steps. It touches only `go.mod`, `go.sum` and `go.work.sum` — there is no
source-level patch in the chain any more (see "kin-openapi is deliberately not bumped" below).

Final versions after the patch:

| Module | `001` | `003` |
|---|---|---|
| `google.golang.org/grpc` | `v1.71.0` | `v1.83.2` |
| `go.opentelemetry.io/otel` family | `v1.35.0` | `v1.45.0` |
| `github.com/getkin/kin-openapi` | `v0.120.0` | `v0.120.0` (not bumped, VEX) |
| `github.com/grafana/grafana-plugin-sdk-go` | `v0.218.0` | `v0.218.0` (not bumped) |
| `github.com/grafana/grafana-aws-sdk` | `v0.25.1` | `v0.25.1` (not bumped) |
| `go.mongodb.org/mongo-driver` | `v1.17.3` | `v1.17.7` |
| `golang.org/x/crypto` | `v0.47.0` | `v0.55.0` |
| `golang.org/x/net` | `v0.48.0` | `v0.58.0` |
| `golang.org/x/mod` | `v0.31.0` | `v0.40.0` |
| `golang.org/x/sys` | `v0.40.0` | `v0.47.0` |
| `golang.org/x/text` | `v0.33.0` | `v0.41.0` |

CVEs closed by the bump:

- `google.golang.org/grpc` → `v1.83.2`:
  [CVE-2026-33186](https://github.com/advisories/GHSA-p77j-4mvh-x3m3),
  [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf),
  [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303),
  [CVE-2026-84304](https://github.com/advisories/GHSA-vp52-pcj8-j9qc),
  [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445).
- `go.opentelemetry.io/otel{,/sdk,/trace,/metric,/exporters/...}` → `v1.45.0`:
  [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq),
  [CVE-2026-39883](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx),
  [CVE-2026-39882](https://github.com/advisories/GHSA-w8rr-5gcm-pp58),
  [CVE-2026-81870](https://nvd.nist.gov/vuln/detail/CVE-2026-81870).
- `github.com/russellhaering/goxmldsig` `v1.4.0` → `v1.6.0`:
  [CVE-2026-33487](https://github.com/advisories/GHSA-479m-364c-43vc).
- `github.com/go-jose/go-jose/v3` `v3.0.4` → `v3.0.5`:
  [CVE-2026-34986](https://github.com/advisories/GHSA-78h2-9frx-2jm8).
- `github.com/cloudflare/circl` `v1.6.1` → `v1.6.3`:
  [CVE-2026-1229](https://github.com/advisories/GHSA-q9hv-hpm4-hj6x).
- `github.com/moby/spdystream` `v0.2.0` → `v0.5.1`:
  [CVE-2026-35469](https://github.com/advisories/GHSA-pc3f-x583-g7j2).
- `github.com/Azure/go-ntlmssp` → `v0.1.1`:
  [CVE-2026-32952](https://github.com/advisories/GHSA-pjcq-xvwq-hhpj).
- `go.mongodb.org/mongo-driver` → `v1.17.7`:
  [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303).
- `golang.org/x/crypto` → `v0.55.0`:
  [CVE-2024-45337](https://nvd.nist.gov/vuln/detail/CVE-2024-45337),
  [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827) … [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835),
  [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508),
  [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595),
  [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597),
  [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598).
- `golang.org/x/net` → `v0.58.0`:
  [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680),
  [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681),
  [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136),
  [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814),
  [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821),
  [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502),
  [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506),
  [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600).
- `golang.org/x/mod` → `v0.40.0`:
  [CVE-2026-56864](https://nvd.nist.gov/vuln/detail/CVE-2026-56864),
  [CVE-2026-56865](https://nvd.nist.gov/vuln/detail/CVE-2026-56865) — both `golang.org/x/mod/sumdb`
  transparency-log verification flaws; `x/mod` is a direct Grafana dependency linked into
  `./pkg/cmd/grafana`.
- `golang.org/x/sys` → `v0.47.0`:
  [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824).
- `golang.org/x/text` → `v0.41.0`:
  [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852).

Decisions baked into the patch:

**grpc `v1.83.2`, not `v1.83.1`** (which is what the external `flant/prometheus` repo took): the DKP
1.76 finding set includes `CVE-2026-84445`, whose fixed versions are `1.82.2` / `1.83.2`, so
`v1.83.1` would leave it open.

**`golang.org/x/crypto` stays at `v0.55.0`.** `v0.56.0` declares `go 1.26.0` in its own `go.mod`,
which would raise this module's `go` directive to `1.26.0` and break the build on the Go 1.25.13
builder. The two findings that are only fixed in `v0.56.0` —
[CVE-2026-56855](https://nvd.nist.gov/vuln/detail/CVE-2026-56855) (GO-2026-6355) and
[CVE-2026-78662](https://nvd.nist.gov/vuln/detail/CVE-2026-78662) (GO-2026-6354), both confined to
`golang.org/x/crypto/ssh` — are covered by VEX. `grpc v1.83.2` itself only requires
`x/crypto v0.55.0`, so nothing in the graph forces `v0.56.0`. Because `x/crypto` moved off its
earlier version, the `GO-2026-5932` statement in `images/grafana-v10/known_vulnerabilities.vex` uses
the versionless `pkg:golang/golang.org/x/crypto` product so it keeps matching across this and future
bumps.

**kin-openapi is deliberately not bumped; the three findings are covered by VEX.** Bumping
`github.com/getkin/kin-openapi` alone breaks the build:
`grafana-plugin-sdk-go@v0.218.0/experimental/e2e/storage/openapi.go:142` does
`cannot range over op.Responses (variable of type *openapi3.Responses)` — the `Responses` type stopped
being a map in kin-openapi `v0.124.0`, and the SDK carried an explicit
`replace github.com/getkin/kin-openapi => v0.120.0` up to and including `v0.226.0` for exactly this
reason. Fixing that means `grafana-plugin-sdk-go v0.250.0`, which in turn drops `build.Info.Hash`
that `grafana-aws-sdk@v0.25.1/pkg/awsds/utils.go` reads in four places, so `grafana-aws-sdk v0.31.2`
follows, and that release changes the `sigv4.New` signature, which forces a source patch on
`pkg/infra/httpclient/httpclientprovider/sigv4_middleware.go`. Four coupled bumps plus a source patch
on a pinned upstream tag were judged too much risk for three findings that are not reachable, so the
whole chain was rolled back: kin-openapi stays at `v0.120.0`, `grafana-plugin-sdk-go` at `v0.218.0`,
`grafana-aws-sdk` at `v0.25.1`, and the former `007-sigv4-middleware.patch` is deleted.

All three kin-openapi findings — [CVE-2026-73502](https://nvd.nist.gov/vuln/detail/CVE-2026-73502)
(Medium, `openapi3filter/req_resp_decoder.go`),
[CVE-2026-76905](https://github.com/advisories/GHSA-mmfr-pmjx-hw9w) (High, GO-2026-6274,
`openapi3filter/validation_error_encoder.go`) and
[GHSA-R277-6W6Q-XMQW](https://github.com/advisories/GHSA-r277-6w6q-xmqw) (Critical, CVE-2026-73501,
`openapi3filter/validation_handler.go`) — are confined to the `openapi3filter` subpackage, which is
not linked into the image. They are waived in `images/grafana-v10/known_vulnerabilities.vex`
(`not_affected` / `vulnerable_code_not_in_execute_path`, product
`pkg:golang/github.com/getkin/kin-openapi@v0.120.0`), together with the pre-existing
`CVE-2025-30153` (kin-openapi) and `CVE-2024-8986` (`grafana-plugin-sdk-go@v0.218.0`) statements,
whose purls match the shipped versions again after the rollback. Evidence recorded in the VEX
statements:

```sh
# empty — openapi3filter is not in the import closure
go list -deps -tags oss ./pkg/cmd/grafana | grep -x 'github.com/getkin/kin-openapi/openapi3filter'
# positive control — the filter works, these subpackages are there
go list -deps -tags oss ./pkg/cmd/grafana | grep 'github.com/getkin/kin-openapi'
#   github.com/getkin/kin-openapi/openapi3
#   github.com/getkin/kin-openapi/routers
#   github.com/getkin/kin-openapi/routers/gorillamux
# symbol table of an UNSTRIPPED binary (built without -s -w, otherwise the control is void):
# 18 openapi3 symbols, 5 routers/gorillamux symbols, 0 openapi3filter symbols
CGO_ENABLED=1 go build -tags oss -o /tmp/grafana-nostrip ./pkg/cmd/grafana
go tool nm /tmp/grafana-nostrip | grep 'kin-openapi'
```

**grpc needed an `otelgrpc` `replace`.** grpc `v1.83.x` drags `google.golang.org/api` to `v0.264.0`,
which requires `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc >= v0.61.0`.
otelgrpc `v0.61.0` removed the deprecated `UnaryClientInterceptor`/`StreamClientInterceptor`, which
`k8s.io/apiserver@v0.29.0` (pinned by Grafana 10.4) still calls in
`pkg/storage/storagebackend/factory/etcd3.go`, so the build fails with
`undefined: otelgrpc.UnaryClientInterceptor`. `v0.60.0` is the last release that still exports them,
so the patch adds
`replace go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => ... v0.60.0`.
The alternative — bumping the whole `k8s.io/*` stack off `v0.29.0` — is a far larger change to a
pinned upstream tag. otelgrpc has no open finding in the scan at either version. The `replace` is
still required after the kin-openapi rollback: it is `google.golang.org/api v0.264.0` (pulled by
grpc `v1.83.2`), not `grafana-plugin-sdk-go`, that raises otelgrpc, and dropping the `replace`
reproduces `undefined: otelgrpc.UnaryClientInterceptor` in `k8s.io/apiserver@v0.29.0` verbatim.

Notable transitive bumps: `google.golang.org/api` → `v0.264.0`, `google.golang.org/genproto` and
`.../genproto/googleapis/{api,rpc}` to the 2026 revisions, `cloud.google.com/go` and its
`{auth,iam,kms,longrunning,storage}` modules, `github.com/googleapis/gax-go/v2` → `v2.17.0`,
`github.com/spf13/afero`, `golang.org/x/time`, `golang.org/x/sync` → `v0.22.0`,
`golang.org/x/term` → `v0.45.0`, `golang.org/x/tools` → `v0.49.0`, plus the grpc `v1.83.x`
dependency set (`github.com/spiffe/go-spiffe/v2`, `github.com/cncf/xds/go`, `github.com/envoyproxy/*`,
`cel.dev/expr`).

These bumps shift the workspace's combined module graph, so `go.work.sum` is part of the patch too.
`go.work` itself is untouched — `001-go-mod.patch` already set its `go` directive.

`github.com/grafana/tempo` is deliberately left at `v1.5.1-0.20230524121406-1dc1bfe7085b`
(CVE-2026-21728, CVE-2026-27878, CVE-2026-28377) — see the VEX file: only the five generated
`pkg/tempopb*` protobuf packages are linked in, and the advisory's fix is a pseudo-version three
years newer that drags the whole Kubernetes stack forward while Grafana 10.4 is pinned to
`k8s.io/apiserver v0.29.0`.

`github.com/prometheus/prometheus` stays at the `replace ... => v0.49.0` pinned by Grafana 10.4
(CVE-2026-40179, CVE-2026-42151, CVE-2026-42154, CVE-2026-44903) — covered by VEX: the whole
`github.com/prometheus/prometheus/web` subtree is absent from the binary's import closure.

Generated with:
```sh
git clone --depth 1 --branch v10.4.19 https://github.com/grafana/grafana.git .
git apply /path/to/001-go-mod.patch --verbose
git add -A && git commit -m "baseline"
export GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
# NB: Grafana is in workspace mode (go.work), so GOFLAGS=-mod=mod must NOT be set —
# go errors with "-mod may only be set to readonly or vendor when in workspace mode".
go get google.golang.org/grpc@v1.83.2 \
       go.opentelemetry.io/otel@v1.45.0 go.opentelemetry.io/otel/sdk@v1.45.0 \
       go.opentelemetry.io/otel/trace@v1.45.0 go.opentelemetry.io/otel/metric@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.45.0 \
       go.opentelemetry.io/otel/exporters/zipkin@v1.45.0 \
       github.com/russellhaering/goxmldsig@v1.6.0 github.com/go-jose/go-jose/v3@v3.0.5 \
       github.com/cloudflare/circl@v1.6.3 github.com/moby/spdystream@v0.5.1 \
       github.com/Azure/go-ntlmssp@v0.1.1 go.mongodb.org/mongo-driver@v1.17.7 \
       golang.org/x/crypto@v0.55.0 golang.org/x/net@v0.58.0 \
       golang.org/x/mod@v0.40.0 golang.org/x/sys@v0.47.0 golang.org/x/text@v0.41.0
go mod edit -replace go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc=go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.60.0
go mod tidy
wire gen -tags oss ./pkg/server
CGO_ENABLED=1 go build -tags oss ./pkg/cmd/grafana
# go.work.sum only settles once the workspace is resolved for the target package:
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go list -deps -tags oss ./pkg/cmd/grafana >/dev/null
git diff -- go.mod go.sum go.work.sum > 003-cve-go-mod.patch
```

### 007-sigv4-middleware.patch — removed

It adapted `pkg/infra/httpclient/httpclientprovider/sigv4_middleware{,_test}.go` to the
`sigv4.New` signature of `grafana-aws-sdk v0.31.2`. With `grafana-aws-sdk` back at `v0.25.1` the
upstream sources compile unchanged, so the patch is deleted. The `007` slot is left free rather than
renumbering, so the patch files stay comparable with the external `flant/prometheus` repo.

### Verification

The whole chain (`001`, `002`, `003`) applies cleanly with a single `git apply /patches/*.patch` to a
pristine `v10.4.19` clone, `wire gen -tags oss ./pkg/server` succeeds and
`CGO_ENABLED=1 go build -tags oss ./pkg/cmd/grafana` builds.

### entrypoint/go.mod

Not a patch — the small `entrypoint` module lives in this repository. Bumped alongside the chain:
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` `v0.11.0` → `v0.44.0` (`/usr/local/bin/entrypoint` in the scan)
- `go` directive `1.24.6` → `1.25.0`, matching the rest of the image.
