## Patches

### 001-go-mod.patch

Update dependencies. The `go 1.22.1` + `toolchain go1.22.2` pair is replaced by a bare
`go 1.25.0`: that is the highest directive every module in the graph accepts, and the image
builds with `builder/golang-alpine`, which is Go 1.25 in `candi/base_images.yml` on this branch.

### 002-pma-log-flooding.patch

Log CPU metrics fetch failures as info instead of error for pods created within the monitoring window, reducing noise without lowering overall log verbosity.

### 003-cve-go-mod.patch

Dependency bumps that close CVEs in the module graph of prometheus-adapter v0.12.0. Only
`go.mod` and `go.sum` are touched; the patch applies on top of 001 and 002.

| CVE | module | bumped to |
| --- | --- | --- |
| CVE-2026-33186, GHSA-hrxh-6v49-42gf, CVE-2026-84303, CVE-2026-84304, CVE-2026-84445 | `google.golang.org/grpc` | v1.60.1 -> v1.83.2 |
| CVE-2026-24051, CVE-2026-39883, CVE-2026-81870 | `go.opentelemetry.io/otel`, `otel/metric`, `otel/sdk`, `otel/trace`, `otel/exporters/otlp/otlptrace`, `otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.21.0 -> v1.45.0 |
| CVE-2026-39827 … CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597, CVE-2026-46598, CVE-2026-56854 | `golang.org/x/crypto` | v0.47.0 -> v0.55.0 |
| CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502, CVE-2026-42506, CVE-2026-46600 | `golang.org/x/net` | v0.48.0 -> v0.58.0 |
| CVE-2026-39824 | `golang.org/x/sys` | v0.40.0 -> v0.47.0 |
| CVE-2026-56852 | `golang.org/x/text` | v0.33.0 -> v0.41.0 |

Notes on the two non-obvious moves:

- `google.golang.org/grpc` v1.83.2 closes CVE-2026-84445 (a request carrying neither the
  `:authority` nor the `Host` header makes `RouteAndProcess` index an empty slice, and the
  resulting panic is not recovered per RPC). It requires `golang.org/x/net` v0.58.0, which it
  pulls in itself.
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace` v1.45.0 requires
  `github.com/cenkalti/backoff/v5`, so the module path changes from
  `github.com/cenkalti/backoff/v4` v4.2.1 to `/v5` v5.0.3. Both are indirect-only.

`golang.org/x/crypto` is deliberately capped at v0.55.0: v0.56.0 declares `go 1.26.0`, which
the Go 1.25 builder of this branch cannot satisfy. The findings only fixed in v0.56.0
(CVE-2026-56855, CVE-2026-78662) live in `golang.org/x/crypto/ssh`, which is not linked into
`/adapter`, and are waived in `known_vulnerabilities.vex` instead.

Regenerated against a pristine tree, on top of the two patches that precede it:

```sh
git clone --depth 1 --branch v0.12.0 https://github.com/kubernetes-sigs/prometheus-adapter.git /tmp/pma-src
cd /tmp/pma-src
git apply --verbose .../patches/001-go-mod.patch .../patches/002-pma-log-flooding.patch
export GOFLAGS=-mod=mod GOTOOLCHAIN=local
go get golang.org/x/crypto@v0.55.0 google.golang.org/grpc@v1.83.2 \
  go.opentelemetry.io/otel@v1.45.0 go.opentelemetry.io/otel/metric@v1.45.0 \
  go.opentelemetry.io/otel/sdk@v1.45.0 go.opentelemetry.io/otel/trace@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0
go mod tidy
# diff only the incremental part, i.e. against the state right after 001 and 002
```

Then assert: exactly two `diff --git` headers (go.mod, go.sum); the `go` directive is still
`go 1.25.0` with no `toolchain` line; and `go build -ldflags '-s -w' -o adapter ./cmd/adapter/adapter.go`
succeeds.
