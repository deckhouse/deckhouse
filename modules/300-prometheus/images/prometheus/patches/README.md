## Patches

### 001-sample_limit_annotation.patch

Limit the number of metrics which Prometheus scrapes from a target.

```yaml
metadata:
  annotations:
    prometheus.deckhouse.io/sample-limit: "5000"
```

### 002-successfully_sent_metric.patch

Exports gauge metric with the count of successfully sent alerts.

### 003-fix-cve.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)

### 004-hardfix_bug_with_dropped_unknown_samples.patch

Add loading chunk snapshots in remote-write to solve problem with unknown series's samples drop.

### 005-op_functions.patch

Added op functions (op_top, op_defined, op_replace_nan, op_smoothie, op_zero_if_none)

### 006-printer-op-top-aggregate-string.patch

Applied to vendored `github.com/prometheus/prometheus` after `go mod vendor`.

Patches existing vendored Prometheus files to:
- Add to the `String` method of the `AggregateExpr` struct to print the expression with the `op_top` function;

### 007-fix-cve-bump.patch

Bump Go dependencies to fix CVEs. Applied on top of `001`-`006`; `go.mod`/`go.sum` only
(Prometheus `v2.55.1` has no `go.work`). The final `go` directive stays `1.25.0` — the builder
`builder/golang-alpine` pinned in `candi/base_images.yml` ships Go 1.25.13.

- [CVE-2026-33186](https://github.com/advisories/GHSA-fw5q-2xv9-49qr), [CVE-2026-84303](https://nvd.nist.gov/vuln/detail/CVE-2026-84303), [CVE-2026-84304](https://nvd.nist.gov/vuln/detail/CVE-2026-84304), [CVE-2026-84445](https://nvd.nist.gov/vuln/detail/CVE-2026-84445), [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf) — `google.golang.org/grpc` `v1.66.0` → `v1.83.2`.
  `v1.83.2` (not the `v1.83.1` used by the external `flant/prometheus` repo): CVE-2026-84445 is only
  fixed in `1.82.2` / `1.83.2`, and `v1.83.1` would leave it open.
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq), [CVE-2026-39883](https://github.com/advisories/GHSA-c98q-8jvw-w7p2), [CVE-2026-81870](https://nvd.nist.gov/vuln/detail/CVE-2026-81870) — `go.opentelemetry.io/otel`, `otel/sdk`, `otel/trace`, `otel/metric` `v1.29.0` → `v1.45.0`.
- [CVE-2026-39882](https://github.com/advisories/GHSA-pqrx-pwhc-3wf2), [CVE-2026-81870](https://nvd.nist.gov/vuln/detail/CVE-2026-81870) — `go.opentelemetry.io/otel/exporters/otlp/otlptrace`, `.../otlptracegrpc`, `.../otlptracehttp` `v1.29.0` → `v1.45.0`.
- [CVE-2026-2303](https://nvd.nist.gov/vuln/detail/CVE-2026-2303) — `go.mongodb.org/mongo-driver` `v1.14.0` → `v1.17.7`.
- [CVE-2024-45337](https://nvd.nist.gov/vuln/detail/CVE-2024-45337), [CVE-2026-39827](https://nvd.nist.gov/vuln/detail/CVE-2026-39827) … [CVE-2026-39835](https://nvd.nist.gov/vuln/detail/CVE-2026-39835), [CVE-2026-42508](https://nvd.nist.gov/vuln/detail/CVE-2026-42508), [CVE-2026-46595](https://nvd.nist.gov/vuln/detail/CVE-2026-46595), [CVE-2026-46597](https://nvd.nist.gov/vuln/detail/CVE-2026-46597), [CVE-2026-46598](https://nvd.nist.gov/vuln/detail/CVE-2026-46598), [CVE-2026-56854](https://nvd.nist.gov/vuln/detail/CVE-2026-56854) — `golang.org/x/crypto` `v0.47.0` → `v0.55.0`.
  **`v0.55.0` is the ceiling here**: `v0.56.0` declares `go 1.26.0`, which the Go 1.25.13 builder
  cannot compile. The two findings that are only fixed in `v0.56.0` (`CVE-2026-56855`,
  `CVE-2026-78662`, both confined to `golang.org/x/crypto/ssh`) are covered by VEX statements in
  `images/prometheus/known_vulnerabilities.vex`.
- [CVE-2026-25680](https://nvd.nist.gov/vuln/detail/CVE-2026-25680), [CVE-2026-25681](https://nvd.nist.gov/vuln/detail/CVE-2026-25681), [CVE-2026-27136](https://nvd.nist.gov/vuln/detail/CVE-2026-27136), [CVE-2026-33814](https://nvd.nist.gov/vuln/detail/CVE-2026-33814), [CVE-2026-39821](https://nvd.nist.gov/vuln/detail/CVE-2026-39821), [CVE-2026-42502](https://nvd.nist.gov/vuln/detail/CVE-2026-42502), [CVE-2026-42506](https://nvd.nist.gov/vuln/detail/CVE-2026-42506), [CVE-2026-46600](https://nvd.nist.gov/vuln/detail/CVE-2026-46600) — `golang.org/x/net` `v0.48.0` → `v0.58.0` (fix lands in `v0.56.0`; `v0.58.0` is what MVS settles on once grpc `v1.83.2` and x/crypto `v0.55.0` are in).
- [CVE-2026-56852](https://nvd.nist.gov/vuln/detail/CVE-2026-56852) — `golang.org/x/text` `v0.33.0` → `v0.41.0` (fix lands in `v0.39.0`; `v0.41.0` is required by x/crypto `v0.55.0` and grpc `v1.83.2`).
- [CVE-2026-39824](https://nvd.nist.gov/vuln/detail/CVE-2026-39824) — `golang.org/x/sys` `v0.40.0` → `v0.47.0` (fix lands in `v0.44.0`; `v0.47.0` is required by otel `v1.45.0`).
- [GHSA-w67g-5rqw-f597](https://github.com/advisories/GHSA-w67g-5rqw-f597) — `github.com/gorilla/websocket` `v1.5.0` → `v1.5.3`.
- [CVE-2026-10722](https://nvd.nist.gov/vuln/detail/CVE-2026-10722) — `github.com/cilium/ebpf` `v0.11.0` → `v0.22.0`.

Notable transitive bumps pulled in by the above (`go get` / `go mod tidy` MVS):
- `github.com/envoyproxy/go-control-plane` `v0.13.0` → `github.com/envoyproxy/go-control-plane/envoy` `v1.37.0` (module split), `github.com/envoyproxy/protoc-gen-validate` `v1.1.0` → `v1.3.3`, `github.com/cncf/xds/go` → `v0.0.0-20260202195803-dba9d589def2`
- `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` `v0.53.0` → `v0.61.0`, `go.opentelemetry.io/proto/otlp` `v1.3.1` → `v1.11.0`, `go.opentelemetry.io/auto/sdk` `v1.2.1` added
- `golang.org/x/oauth2` `v0.27.0` → `v0.36.0`, `golang.org/x/sync` `v0.19.0` → `v0.22.0`, `golang.org/x/mod` `v0.31.0` → `v0.38.0`, `golang.org/x/term`, `golang.org/x/tools` advanced accordingly
- `github.com/grpc-ecosystem/grpc-gateway/v2` `v2.22.0` → `v2.29.0`, `github.com/klauspost/compress` `v1.17.9` → `v1.18.0`, `github.com/google/go-cmp` `v0.6.0` → `v0.7.0`, `github.com/stretchr/testify` `v1.9.0` → `v1.11.1`
- `cloud.google.com/go/auth` `v0.9.3` → `v0.18.2`, `github.com/googleapis/gax-go/v2` `v2.13.0` → `v2.17.0`

Not fixed by this patch and covered by VEX in `images/prometheus/known_vulnerabilities.vex`:
`CVE-2026-33997`, `CVE-2026-41567`, `CVE-2026-41568`, `CVE-2026-42306` (`github.com/docker/docker` —
only the client SDK is linked; `v29.3.1` does not exist as a Go module on either
`github.com/docker/docker` or `github.com/docker/docker/v29`, and upstream `moby/moby` has no `v29`
tag at all, the newest being `v28.5.2`), `GO-2026-5932` (`golang.org/x/crypto/openpgp`, unmaintained,
no fixed version), `CVE-2026-56855` and `CVE-2026-78662` (`golang.org/x/crypto/ssh`, see above).

Generated with:
```sh
git clone --depth 1 --branch v2.55.1 https://github.com/prometheus/prometheus.git .
git apply /path/to/00{1,2,3,4,5,6}-*.patch --verbose
git add -A && git commit -m "post-existing-patches baseline"
export GOFLAGS=-mod=mod GOTOOLCHAIN=go1.25.13 GOPROXY=https://proxy.golang.org,direct
go get google.golang.org/grpc@v1.83.2 \
       go.opentelemetry.io/otel@v1.45.0 go.opentelemetry.io/otel/sdk@v1.45.0 \
       go.opentelemetry.io/otel/trace@v1.45.0 go.opentelemetry.io/otel/metric@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@v1.45.0 \
       go.mongodb.org/mongo-driver@v1.17.7 \
       golang.org/x/crypto@v0.55.0 golang.org/x/net@v0.58.0 golang.org/x/text@v0.41.0 \
       golang.org/x/sys@v0.47.0 \
       github.com/gorilla/websocket@v1.5.3 github.com/cilium/ebpf@v0.22.0
go mod tidy
git diff -- go.mod go.sum > 007-fix-cve-bump.patch
```
