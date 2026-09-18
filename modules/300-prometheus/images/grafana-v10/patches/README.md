## Patches

Patches are applied by `werf.inc.yaml` with a glob (`git apply /patches/*.patch`), so they are applied
in file-name order and a new file is picked up automatically.

The `go` directive is `1.25.0` through the whole chain (both `go.mod` and `go.work`): the builder
`builder/golang-alpine` pinned in `candi/base_images.yml` resolves to the `builder/golang-alpine-1.25`
digest and ships Go 1.25.13, and there is no 1.26/1.27 key there.

### 001-go-mod.patch

Update dependencies to fix CVEs and set the `go` directive to `1.25.0` in `go.mod` and `go.work`.
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 002-nodejs.patch — removed

`002-nodejs.patch` pinned `react-router` to `6.30.2` in `/src/grafana`'s `package.json`/`yarn.lock`.
It was dead code: the image never runs `yarn`, and the whole frontend (`/usr/share/grafana/public`,
including `public/app/plugins/datasource/tempo/node_modules/react-router` that the scanner reports)
is imported prebuilt from `grafana-deps.git` at tag `v10.4.19` (see `werf.inc.yaml`, the
`-src-artifact` and `-grafana-distr` stages). The four react-router findings are covered by VEX in
`images/grafana-v10/known_vulnerabilities.vex` instead. The numbering of `003`+ is deliberately left
unshifted so the patch files stay comparable with the external `flant/prometheus` repo.

### 003-cve-go-mod.patch

Update Go dependencies to fix CVEs.
- [CVE-2026-33186](https://github.com/advisories/GHSA-p77j-4mvh-x3m3) — `google.golang.org/grpc` `v1.71.0` → `v1.80.0`
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq), [CVE-2026-39883](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx) — `go.opentelemetry.io/otel/sdk` `v1.35.0` → `v1.43.0`
- [CVE-2026-39882](https://github.com/advisories/GHSA-w8rr-5gcm-pp58) — `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` → `v1.43.0`
- [CVE-2026-33487](https://github.com/advisories/GHSA-479m-364c-43vc) — `github.com/russellhaering/goxmldsig` `v1.4.0` → `v1.6.0`
- [CVE-2026-34986](https://github.com/advisories/GHSA-78h2-9frx-2jm8) — `github.com/go-jose/go-jose/v3` `v3.0.4` → `v3.0.5`
- [CVE-2026-1229](https://github.com/advisories/GHSA-q9hv-hpm4-hj6x) — `github.com/cloudflare/circl` `v1.6.1` → `v1.6.3`
- [CVE-2026-35469](https://github.com/advisories/GHSA-pc3f-x583-g7j2) — `github.com/moby/spdystream` `v0.2.0` → `v0.5.1`
- [CVE-2026-32952](https://github.com/advisories/GHSA-pjcq-xvwq-hhpj) — `github.com/Azure/go-ntlmssp` → `v0.1.1`

`go.mod`/`go.sum` only — these bumps do not shift `go.work.sum`.

### 004-fix-cve-bump.patch

Update Go dependencies to fix CVEs.
- [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) — `go.mongodb.org/mongo-driver` `v1.17.3` → `v1.17.7`
- `golang.org/x/crypto` `v0.47.0` → `v0.52.0`: [CVE-2024-45337](https://nvd.nist.gov/vuln/detail/CVE-2024-45337), [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827) … [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835), [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508), [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595), [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597), [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598)
- `golang.org/x/net` `v0.48.0` → `v0.55.0`: [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506)
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` → `v0.45.0` (the CVE's own fix is `v0.44.0`; MVS resolves higher)

These bumps shift the workspace's combined module graph, so `go.work.sum` is part of the patch too.

### 005-fix-cve-bump.patch

Update Go dependencies to fix CVEs.
- [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf) — `google.golang.org/grpc` `v1.80.0` → `v1.82.1`
- [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) — `golang.org/x/net` `v0.55.0` → `v0.56.0`
- [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) — `golang.org/x/text` `v0.33.0` → `v0.39.0`

MVS also lifts `golang.org/x/crypto` to `v0.53.0` and `golang.org/x/sys` to `v0.46.0`.

### 006-fix-cve-bump.patch

Update Go dependencies to fix CVEs.
- [CVE-2026-56864](https://nvd.nist.gov/vuln/detail/CVE-2026-56864), [CVE-2026-56865](https://nvd.nist.gov/vuln/detail/CVE-2026-56865) — `golang.org/x/mod` `v0.31.0` → `v0.40.0`

Both are `golang.org/x/mod/sumdb` transparency-log verification flaws. `x/mod` is a direct Grafana
dependency and is linked into `./pkg/cmd/grafana`. It drags a whole `golang.org/x/*` generation
forward: `x/crypto` `v0.53.0` → **`v0.55.0`**, `x/net` `v0.56.0` → `v0.58.0`, `x/text` `v0.39.0` →
`v0.41.0`, `x/sys` `v0.46.0` → `v0.47.0`, `x/sync` → `v0.22.0`, `x/term` → `v0.45.0`,
`x/tools` → `v0.49.0`.

Because `x/crypto` moved off its earlier version, the `GO-2026-5932` statement in
`images/grafana-v10/known_vulnerabilities.vex` uses the versionless
`pkg:golang/golang.org/x/crypto` product so it keeps matching across this and future bumps.

### 007-fix-cve-bump.patch

Update Go dependencies to fix CVEs; the `go` directive stays `1.25.0`.
- [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303), [CVE-2026-84304](https://github.com/advisories/GHSA-vp52-pcj8-j9qc), [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445) — `google.golang.org/grpc` `v1.82.1` → **`v1.83.2`**
- [CVE-2026-81870](https://nvd.nist.gov/vuln/detail/CVE-2026-81870) — the whole `go.opentelemetry.io/otel` family (`otel`, `otel/sdk`, `otel/trace`, `otel/metric`, `exporters/otlp/otlptrace`, `.../otlptracegrpc`, `.../otlptracehttp`, `exporters/zipkin`) `v1.43.0` → **`v1.45.0`**
- [CVE-2026-76905](https://github.com/advisories/GHSA-mmfr-pmjx-hw9w) (GO-2026-6274), [CVE-2026-73502](https://nvd.nist.gov/vuln/detail/CVE-2026-73502), [GHSA-r277-6w6q-xmqw](https://github.com/advisories/GHSA-r277-6w6q-xmqw) — `github.com/getkin/kin-openapi` `v0.120.0` → `v0.149.0`
- `github.com/grafana/grafana-plugin-sdk-go` `v0.218.0` → `v0.250.0` and `github.com/grafana/grafana-aws-sdk` `v0.25.1` → `v0.31.2` — required by the kin-openapi bump, see below

**grpc `v1.83.2`, not `v1.83.1`** (which is what the external `flant/prometheus` repo took): the DKP
1.76 finding set includes `CVE-2026-84445`, whose fixed versions are `1.82.2` / `1.83.2`, so
`v1.83.1` would leave it open.

**`golang.org/x/crypto` stays at `v0.55.0`** (reached by `006`). `v0.56.0` declares `go 1.26.0` in its
own `go.mod`, which would raise this module's `go` directive to `1.26.0` and break the build on the
Go 1.25.13 builder. The two findings that are only fixed in `v0.56.0` —
[CVE-2026-56855](https://nvd.nist.gov/vuln/detail/CVE-2026-56855) (GO-2026-6355) and
[CVE-2026-78662](https://nvd.nist.gov/vuln/detail/CVE-2026-78662) (GO-2026-6354), both confined to
`golang.org/x/crypto/ssh` — are covered by VEX. `grpc v1.83.2` itself only requires
`x/crypto v0.55.0`, so nothing in the graph forces `v0.56.0`.

**kin-openapi needed a `grafana-plugin-sdk-go` bump.** Bumping kin-openapi alone breaks the build:
`grafana-plugin-sdk-go@v0.218.0/experimental/e2e/storage/openapi.go:142` does
`cannot range over op.Responses (variable of type *openapi3.Responses)` — the `Responses` type stopped
being a map in kin-openapi `v0.124.0`. The SDK carried an explicit
`replace github.com/getkin/kin-openapi => v0.120.0` up to and including `v0.226.0` for exactly this
reason. The SDK is taken to `v0.250.0`, which is also the first release outside the CVE-2024-8986
range. The kin-openapi target is `v0.149.0` rather than the advisory minimum `v0.141.0`, because
`CVE-2026-73502` and `GHSA-r277-6w6q-xmqw` are only fixed in `v0.144.0`; `v0.149.0` closes all three
findings at once and makes the pre-existing `kin-openapi@v0.120.0` waivers in the VEX file moot.
Verified afterwards that `github.com/getkin/kin-openapi/openapi3filter` is still absent from the
binary's import closure — only `openapi3`, `routers` and `routers/gorillamux` are linked in.

**The SDK bump needed `grafana-aws-sdk` and a two-line source fix.** `grafana-plugin-sdk-go v0.250.0`
dropped `build.Info.Hash`, which `grafana-aws-sdk@v0.25.1/pkg/awsds/utils.go` reads in four places.
`grafana-aws-sdk v0.31.2` is the first release without those reads, and its own `go.mod` requires
exactly `grafana-plugin-sdk-go v0.250.0` — upstream changed both in lockstep. `v0.31.2` in turn added
an `awsds.AuthSettings` parameter to `sigv4.New`, so
`pkg/infra/httpclient/httpclientprovider/sigv4_middleware.go` needs one extra argument. The
substitution is behaviour-preserving by construction: `v0.25.1`'s `createSigner` called
`awsds.ReadAuthSettingsFromEnvironmentVariables()` internally, and `v0.31.2` merely hoisted that call
to the caller, so passing `*awsds.ReadAuthSettingsFromEnvironmentVariables()` reproduces the old
behaviour exactly. The three stubs in `sigv4_middleware_test.go` get the same parameter;
`go test ./pkg/infra/httpclient/httpclientprovider/` passes. These two files are the only Grafana
source changes in this patch — everything else is `go.mod`/`go.sum`/`go.work.sum`.

**grpc needed an `otelgrpc` pin.** grpc `v1.83.x` drags `google.golang.org/api` to `v0.264.0`, which
requires `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc >= v0.61.0`.
otelgrpc `v0.61.0` removed the deprecated `UnaryClientInterceptor`/`StreamClientInterceptor`, which
`k8s.io/apiserver@v0.29.0` (pinned by Grafana 10.4) still calls in
`pkg/storage/storagebackend/factory/etcd3.go`, so the build fails with
`undefined: otelgrpc.UnaryClientInterceptor`. `v0.60.0` is the last release that still exports them,
so the patch adds
`replace go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc => ... v0.60.0`.
The alternative — bumping the whole `k8s.io/*` stack off `v0.29.0` — is a far larger change to a
pinned upstream tag. otelgrpc has no open finding in the scan at either version.

Notable transitive bumps: `google.golang.org/api` → `v0.264.0`, `google.golang.org/genproto` and
`.../genproto/googleapis/{api,rpc}` to the 2026 revisions, `cloud.google.com/go` and its
`{auth,iam,kms,longrunning,storage}` modules, `github.com/googleapis/gax-go/v2` → `v2.17.0`,
`github.com/spf13/afero`, `golang.org/x/time`, plus the grpc `v1.83.x` dependency set
(`github.com/spiffe/go-spiffe/v2`, `github.com/cncf/xds/go`, `github.com/envoyproxy/*`,
`cel.dev/expr`) and the kin-openapi `v0.149.x` set (`github.com/oasdiff/yaml`,
`github.com/oasdiff/yaml3`, `github.com/santhosh-tekuri/jsonschema/v6`, replacing
`github.com/invopop/yaml` and `github.com/perimeterx/marshmallow`).

`github.com/grafana/tempo` is deliberately left at `v1.5.1-0.20230524121406-1dc1bfe7085b`
(CVE-2026-21728, CVE-2026-27878, CVE-2026-28377) — see the VEX file: only the five generated
`pkg/tempopb*` protobuf packages are linked in, and the advisory's fix is a pseudo-version three
years newer that drags the whole Kubernetes stack forward while Grafana 10.4 is pinned to
`k8s.io/apiserver v0.29.0`.

`github.com/prometheus/prometheus` stays at the `replace ... => v0.49.0` pinned by Grafana 10.4
(CVE-2026-40179, CVE-2026-42151, CVE-2026-42154, CVE-2026-44903) — covered by VEX: the whole
`github.com/prometheus/prometheus/web` subtree is absent from the binary's import closure.

Verified: all six patches apply cleanly with a single `git apply /patches/*.patch` to a pristine
`v10.4.19` clone, `wire gen -tags oss ./pkg/server` succeeds and
`CGO_ENABLED=1 go build -tags oss ./pkg/cmd/grafana` builds.

Generated with:
```sh
git clone --depth 1 --branch v10.4.19 https://github.com/grafana/grafana.git .
git apply /path/to/00{1,3,4,5,6}-*.patch --verbose
git add -A && git commit -m "post-existing-patches baseline"
export GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
# NB: Grafana is in workspace mode (go.work), so GOFLAGS=-mod=mod must NOT be set —
# go errors with "-mod may only be set to readonly or vendor when in workspace mode".
go get google.golang.org/grpc@v1.83.2 \
       github.com/grafana/grafana-plugin-sdk-go@v0.250.0 \
       github.com/grafana/grafana-aws-sdk@v0.31.2 \
       github.com/getkin/kin-openapi@v0.149.0 \
       go.opentelemetry.io/otel@v1.45.0 go.opentelemetry.io/otel/sdk@v1.45.0 \
       go.opentelemetry.io/otel/trace@v1.45.0 go.opentelemetry.io/otel/metric@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.45.0 \
       go.opentelemetry.io/otel/exporters/zipkin@v1.45.0
go mod edit -replace go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc=go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc@v0.60.0
go mod tidy
# the sigv4 two-file source fix
wire gen -tags oss ./pkg/server
CGO_ENABLED=1 go build -tags oss ./pkg/cmd/grafana
# go.work.sum only settles once the workspace is resolved for the target package:
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go list -deps -tags oss ./pkg/cmd/grafana >/dev/null
git diff -- go.mod go.sum go.work.sum \
  pkg/infra/httpclient/httpclientprovider/sigv4_middleware.go \
  pkg/infra/httpclient/httpclientprovider/sigv4_middleware_test.go > 007-fix-cve-bump.patch
```

### entrypoint/go.mod

Not a patch — the small `entrypoint` module lives in this repository. Bumped alongside the chain:
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` `v0.11.0` → `v0.44.0` (`/usr/local/bin/entrypoint` in the scan)
- `go` directive `1.24.6` → `1.25.0`, matching the rest of the image.
