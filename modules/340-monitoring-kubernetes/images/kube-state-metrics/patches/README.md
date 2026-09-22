## Patches

### 001-go-mod.patch

Update dependencies to fix CVEs
- [CVE-2025-47914](https://github.com/advisories/GHSA-f6x5-jh6r-wrfv)
- [CVE-2025-58181](https://github.com/advisories/GHSA-j5w8-q4qc-rx2x)
- [GO-2025-3900](https://github.com/advisories/GHSA-2464-8j7c-4cjm)
- [GHSA-gcjh-h69q-9w9g](https://github.com/advisories/GHSA-gcjh-h69q-9w9g) — bump `github.com/google/cel-go` to `v0.29.0`
- [CVE-2026-33186](https://github.com/advisories/GHSA-mhpq-9638-x6v6) — bump `google.golang.org/grpc`
  (advisory requires `v1.79.3`)
- [GHSA-hrxh-6v49-42gf](https://github.com/advisories/GHSA-hrxh-6v49-42gf) — bump `google.golang.org/grpc`
  (advisory requires `v1.82.1`)
- [CVE-2026-84303](https://github.com/advisories/GHSA-vp52-pcj8-j9qc), CVE-2026-84304 — bump
  `google.golang.org/grpc` (advisory `fixed: 1.83.1`)
- [CVE-2026-84445](https://avd.aquasec.com/nvd/cve-2026-84445) — bump `google.golang.org/grpc` to
  `v1.83.2` (Trivy reports `fixed: 1.82.2, 1.83.2, 1.85.0-dev.0.20260825072537-93e31b48545e`).
  Transitively pulls the Google Cloud / genproto set that `grpc v1.83.2` requires:
  `github.com/spiffe/go-spiffe/v2 v2.7.0`, `github.com/cenkalti/backoff/v5 v5.0.3`,
  `github.com/envoyproxy/go-control-plane/envoy v1.37.0`, `google.golang.org/api v0.264.0`,
  `google.golang.org/genproto/googleapis/{api,rpc}@20260803160001`, `google.golang.org/protobuf v1.36.11`.
  Since `v1.83.x` grpc ships `google.golang.org/grpc/stats/opentelemetry` itself, the separate module
  `google.golang.org/grpc/stats/opentelemetry` is dropped from `go.mod` (otherwise `go mod tidy` fails
  with `ambiguous import`)
- [CVE-2026-24051](https://github.com/advisories/GHSA-6q3w-4ccw-3463), CVE-2026-39883,
  [CVE-2026-81870](https://avd.aquasec.com/nvd/cve-2026-81870) — bump the whole
  `go.opentelemetry.io/otel` module set present in `go.mod` to `v1.45.0` (advisory `fixed: 1.45.0`;
  reported against `otel/exporters/otlp/otlptrace`, `otel/exporters/otlp/otlptrace/otlptracegrpc`
  and `otel/sdk`). The set is bumped as a whole — `otel`, `otel/metric`, `otel/trace`, `otel/sdk`,
  `otel/sdk/metric`, `otel/exporters/otlp/otlptrace`, `otel/exporters/otlp/otlptrace/otlptracegrpc`
  — so that `go mod tidy` keeps it consistent. Transitively pulls
  `go.opentelemetry.io/proto/otlp v1.11.0`, `go.opentelemetry.io/auto/sdk v1.2.1`,
  `go.opentelemetry.io/contrib/* v1.44.0 / v0.61.0`, `github.com/go-logr/logr v1.4.4` and
  `github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0`
- CVE-2026-39827, CVE-2026-39828, CVE-2026-39829, CVE-2026-39830, CVE-2026-39831, CVE-2026-39832,
  CVE-2026-39833, CVE-2026-39834, CVE-2026-39835, CVE-2026-42508, CVE-2026-46595, CVE-2026-46597,
  CVE-2026-46598 — bump `golang.org/x/crypto` (advisory requires `0.52.0`)
- CVE-2026-56854 (`GO-2026-6303`, fixed in `0.55.0`) — bump `golang.org/x/crypto` to `v0.55.0`.
  **`v0.55.0` is the ceiling here**: `v0.56.0` declares `go 1.26.0`, while the image is built by
  `builder/golang-alpine`, which is pinned to Go 1.25 in `candi/base_images.yml`
- CVE-2026-25680, CVE-2026-25681, CVE-2026-27136, CVE-2026-33814, CVE-2026-39821, CVE-2026-42502,
  CVE-2026-42506, CVE-2026-46600 — bump `golang.org/x/net` (advisory requires `0.56.0`;
  `google.golang.org/grpc v1.83.2` pulls `0.58.0`)
- CVE-2026-56852 — bump `golang.org/x/text` (advisory requires `0.39.0`; the dependency set pulls `0.41.0`)
- CVE-2026-39824 — bump `golang.org/x/sys` (advisory requires `0.44.0`; the dependency set pulls `0.47.0`)

The `go` directive is raised to `go 1.25.0` to match the builder (`builder/golang-alpine` = Go 1.25).

The patch is regenerated on top of the pristine `v2.17.0` tag with:

```bash
git clone --depth 1 --branch v2.17.0 https://github.com/kubernetes/kube-state-metrics.git
cd kube-state-metrics
sed -i 's/^go 1\.24\.0$/go 1.25.0/' go.mod
go get github.com/google/cel-go@v0.29.0 \
  go.opentelemetry.io/otel@v1.45.0 go.opentelemetry.io/otel/metric@v1.45.0 \
  go.opentelemetry.io/otel/trace@v1.45.0 go.opentelemetry.io/otel/sdk@v1.45.0 \
  go.opentelemetry.io/otel/sdk/metric@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace@v1.45.0 \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.45.0 \
  google.golang.org/grpc@v1.83.2 golang.org/x/crypto@v0.55.0
go mod edit -droprequire google.golang.org/grpc/stats/opentelemetry
go mod tidy
git diff -- go.mod go.sum > 001-go-mod.patch
```

`GO-2026-5932` (`golang.org/x/crypto/openpgp` unmaintained), `CVE-2026-56855` and `CVE-2026-78662`
(both `golang.org/x/crypto/ssh`, fixed only in `v0.56.0`, which needs Go 1.26) have no reachable
fix in this branch and are handled by `../known_vulnerabilities.vex` instead — neither `ssh` nor
`openpgp` is in the binary's build graph.
