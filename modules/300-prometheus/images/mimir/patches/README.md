## Patches

Patches are applied in two places, so `werf.inc.yaml` lists them by name rather than by glob:

- `001-go-mod.patch` and `004-cve-go-mod.patch` are applied with `git apply` to the checked-out
  Mimir source tree, before `go mod vendor`;
- `002-op-functions.patch`, `003-printer-op-top-aggregate-string.patch`,
  `005-grpc-health-list.patch` and `op_parser_init.go.tpl` are applied to `vendor/` **after**
  `go mod vendor`.

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [CVE-2025-22868](https://github.com/advisories/GHSA-6v2p-p543-phr9)
- [CVE-2025-27144](https://github.com/advisories/GHSA-c6gw-w398-hv78)
- [CVE-2025-30204](https://github.com/advisories/GHSA-mh63-6h87-95cp)
- [CVE-2024-45339](https://github.com/advisories/GHSA-6wxm-mpqj-6jpf)

The `go` directive is raised to `1.25.0` — the highest version the DKP builder image supports
(`candi/base_images.yml`, `builder/golang-alpine` → `builder/golang-alpine-1.25`). It is required by
the dependencies bumped in `004-cve-go-mod.patch`.

### 002-op-functions.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus parser files to:
- Register `OP_TOP` as a keyword and aggregate operator in the lexer and grammar
- Handle `op_top` argument parsing in `newAggregateExpr`

No engine changes are needed since Mimir does not evaluate PromQL locally.

The parser is regenerated from the `.y` grammar using `goyacc` during the build.

### 003-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function

### 004-cve-go-mod.patch

Single cumulative `go.mod`/`go.sum` bump on top of `001-go-mod.patch` that closes every remaining
CVE found in Mimir's Go dependencies.

Direct fixes:

| Dependency | Version | CVEs |
|---|---|---|
| `google.golang.org/grpc` | v1.65.0 (`replace` pin) / v1.66.0 (`require`) → v1.83.2 | [CVE-2026-33186](https://github.com/advisories/GHSA-prj3-ccx8-p6x4), [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf), [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303), [CVE-2026-84304](https://github.com/advisories/GHSA-vp52-pcj8-j9qc), [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445) |
| `github.com/go-jose/go-jose/v4` | v4.0.5 → v4.1.4 | [CVE-2026-34986](https://github.com/advisories/GHSA-78h2-9frx-2jm8) |
| `go.opentelemetry.io/otel`, `otel/metric`, `otel/trace`, `otel/sdk` | v1.29.0 / v1.28.0 → v1.44.0 | [CVE-2026-29181](https://github.com/advisories/GHSA-mh2q-q3fh-2475), [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq), [CVE-2026-39883](https://github.com/advisories/GHSA-hfvc-g4fc-pqhx) |
| `go.mongodb.org/mongo-driver` | v1.14.0 → v1.17.7 | [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) |
| `golang.org/x/crypto` | v0.47.0 → v0.55.0 | [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827), [CVE-2026-39828](https://nvd.nist.gov/vuln/detail/CVE-2026-39828), [CVE-2026-39829](https://nvd.nist.gov/vuln/detail/CVE-2026-39829), [CVE-2026-39830](https://nvd.nist.gov/vuln/detail/CVE-2026-39830), [CVE-2026-39831](https://nvd.nist.gov/vuln/detail/CVE-2026-39831), [CVE-2026-39832](https://nvd.nist.gov/vuln/detail/CVE-2026-39832), [CVE-2026-39833](https://nvd.nist.gov/vuln/detail/CVE-2026-39833), [CVE-2026-39834](https://nvd.nist.gov/vuln/detail/CVE-2026-39834), [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835), [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508), [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595), [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597), [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598), [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) |
| `golang.org/x/net` | v0.48.0 → v0.58.0 | [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506), [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) |
| `golang.org/x/sys` | v0.40.0 → v0.47.0 | [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) |
| `golang.org/x/text` | v0.33.0 → v0.41.0 | [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) |

The `replace google.golang.org/grpc => google.golang.org/grpc v1.65.0` pin from upstream `go.mod`
is dropped by this patch.

Pulled in transitively via MVS (no CVE of their own): the rest of the `golang.org/x/*` family
(`mod`, `sync`, `time`, `tools`), `google.golang.org/api`, `google.golang.org/genproto*`,
`google.golang.org/protobuf`, `cloud.google.com/go*`, `github.com/googleapis/gax-go/v2`,
`go.opentelemetry.io/contrib/instrumentation/*` v0.54.0 → v0.61.0,
`github.com/prometheus/client_model` v0.6.1 → v0.6.2 and the grpc v1.83.x dependency set.

**`golang.org/x/crypto` v0.55.0 is the ceiling for DKP 1.76**: v0.56.0 declares `go 1.26.0` in its
own `go.mod`, while the builder image ships Go 1.25.x. The two remaining `golang.org/x/crypto/ssh`
findings that would require v0.56.0 (CVE-2026-56855, CVE-2026-78662) are handled by
`known_vulnerabilities.vex` instead, together with GO-2026-5932 (`golang.org/x/crypto/openpgp`).
`google.golang.org/grpc` v1.83.2 and `go.opentelemetry.io/otel` v1.44.0 declare `go 1.25.0`, so
Mimir's `go` directive stays at `1.25.0` and no `toolchain` line is introduced.

The dskit `grpc_health_v1.HealthServer` shim from `005-grpc-health-list.patch` remains necessary
and sufficient under grpc v1.83.2 — verified by reproducing the full build recipe (`go mod vendor`,
`op_parser_init.go.tpl`, patches 002/003 into `vendor/github.com/prometheus/prometheus`, patch 005
into `vendor/github.com/grafana/dskit`, `goyacc`, `go build ./cmd/mimir`), which succeeds.

Generated with:

```sh
git clone --depth 1 --branch mimir-2.14.3 https://github.com/grafana/mimir.git .
git apply /path/to/001-go-mod.patch --verbose
git add -A && git commit -m "baseline"
export GOFLAGS=-mod=mod GOPROXY=https://proxy.golang.org,direct
go mod edit -dropreplace=google.golang.org/grpc
go get google.golang.org/grpc@v1.83.2 \
       github.com/go-jose/go-jose/v4@v4.1.4 \
       go.opentelemetry.io/otel@v1.44.0 \
       go.opentelemetry.io/otel/metric@v1.44.0 \
       go.opentelemetry.io/otel/trace@v1.44.0 \
       go.opentelemetry.io/otel/sdk@v1.44.0 \
       go.mongodb.org/mongo-driver@v1.17.7 \
       golang.org/x/crypto@v0.55.0 \
       golang.org/x/net@v0.58.0 \
       golang.org/x/sys@v0.47.0 \
       golang.org/x/text@v0.41.0
go mod tidy
git diff HEAD -- go.mod go.sum > 004-cve-go-mod.patch
```

### 005-grpc-health-list.patch

Applied to vendored `github.com/grafana/dskit` after `go mod vendor`:

```sh
patch -d /src/vendor/github.com/grafana/dskit -p1 < /patches/005-grpc-health-list.patch
```

gRPC-Go v1.72+ added a `List` method to the `grpc_health_v1.HealthServer` interface, but the dskit
revision pinned by mimir 2.14.3 (`v0.0.0-20240920183844-560bb26f205e`) predates that change. After
bumping `google.golang.org/grpc` in `004-cve-go-mod.patch`, the embedded `*grpcutil.HealthCheck`
no longer satisfies the interface and `pkg/mimir/mimir.go:883` fails to compile with
`*grpcutil.HealthCheck does not implement grpc_health_v1.HealthServer (missing method List)`.
**This patch is mandatory, not optional** — without it the image does not build.

The patch back-ports the `List` implementation from upstream
[grafana/dskit#689](https://github.com/grafana/dskit/pull/689) onto the vendored
`grpcutil/health_check.go`. We patch the vendored copy (rather than bumping dskit itself) because
newer dskit revisions also rework `DialOption`, `SpanLogger`, etc. — incompatible API changes that
would require a much larger update.

### op_parser_init.go.tpl

Copied into vendored `github.com/prometheus/prometheus/promql/` after `go mod vendor`.

Registers custom PromQL op-functions (`op_defined`, `op_replace_nan`, `op_smoothie`,
`op_zero_if_none`) in `parser.Functions` and `FunctionCalls` with stub implementations.
Mimir only acts as a query-frontend that parses queries for splitting and caching; it
does not evaluate PromQL, so only parser-level recognition is needed.
