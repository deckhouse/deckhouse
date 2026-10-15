## Patches

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-25680
- CVE-2026-25681
- CVE-2026-27136
- CVE-2026-33814
- CVE-2026-39821
- CVE-2026-39824
- CVE-2026-39827
- CVE-2026-39828
- CVE-2026-39829
- CVE-2026-39830
- CVE-2026-39831
- CVE-2026-39832
- CVE-2026-39833
- CVE-2026-39834
- CVE-2026-39835
- CVE-2026-42502
- CVE-2026-42506
- CVE-2026-42508
- CVE-2026-46595
- CVE-2026-46597
- CVE-2026-46598
- CVE-2026-46600
- CVE-2026-56852
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78662
- CVE-2026-81870
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445

GHSA:
- GHSA-gcjh-h69q-9w9g
- GHSA-hrxh-6v49-42gf

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `999-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

| Module | Upstream | Patched |
|---|---|---|
| `cel.dev/expr` | v0.25.1 | v0.25.2 |
| `cloud.google.com/go/auth/oauth2adapt` | v0.2.3 | v0.2.8 |
| `cloud.google.com/go/auth` | v0.7.2 | v0.18.2 |
| `cloud.google.com/go/monitoring` | v1.20.1 | v1.24.3 |
| `cloud.google.com/go/trace` | v1.10.11 | v1.11.7 |
| `github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp` | v1.31.0 | v1.33.0 |
| `github.com/antlr4-go/antlr/v4` | v4.13.0 | v4.13.1 |
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `github.com/google/cel-go` | v0.26.0 | v0.29.0 |
| `github.com/google/s2a-go` | v0.1.7 | v0.1.9 |
| `github.com/googleapis/enterprise-certificate-proxy` | v0.3.2 | v0.3.11 |
| `github.com/googleapis/gax-go/v2` | v2.13.0 | v2.17.0 |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.28.0 | v2.29.0 |
| `go.opentelemetry.io/contrib/detectors/gcp` | v1.39.0 | v1.44.0 |
| `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | v0.60.0 | v0.61.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.40.0 | v1.45.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.40.0 | v1.45.0 |
| `go.opentelemetry.io/otel/metric` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk/metric` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/proto/otlp` | v1.10.0 | v1.11.0 |
| `golang.org/x/crypto` | v0.49.0 | v0.57.0 |
| `golang.org/x/net` | v0.52.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.35.0 | v0.36.0 |
| `golang.org/x/sync` | v0.20.0 | v0.23.0 |
| `golang.org/x/sys` | v0.42.0 | v0.48.0 |
| `golang.org/x/term` | v0.41.0 | v0.46.0 |
| `golang.org/x/text` | v0.35.0 | v0.42.0 |
| `google.golang.org/api` | v0.189.0 | v0.264.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260401024825-9d38bb4040a9 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260401024825-9d38bb4040a9 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto` | v0.0.0-20240722135656-d784300faade | v0.0.0-20260128011058-8636f8732409 |
| `google.golang.org/grpc` | v1.80.0 | v1.83.2 |

The `go` directive of `go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.
<!-- cvefix:end -->
