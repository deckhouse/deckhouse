## Patches

### 999-fix-cve.patch

Fix CVEs:
- CVE-2026-56851
- CVE-2026-56854
- CVE-2026-56855
- CVE-2026-78659
- CVE-2026-78660
- CVE-2026-78662
- CVE-2026-78663
- CVE-2026-78669
- CVE-2026-81870
- CVE-2026-84303
- CVE-2026-84304
- CVE-2026-84445
- CVE-2026-97032

GHSA:
- GHSA-2v4p-qf9q-27wj
- GHSA-8wmf-6v46-5gfg
- GHSA-qc2q-p7wx-3px3
- GHSA-vp52-pcj8-j9qc

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
| `github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp` | v1.32.0 | v1.33.0 |
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `github.com/google/s2a-go` | v0.1.7 | v0.1.9 |
| `github.com/googleapis/enterprise-certificate-proxy` | v0.3.2 | v0.3.11 |
| `github.com/googleapis/gax-go/v2` | v2.13.0 | v2.17.0 |
| `go.opentelemetry.io/contrib/detectors/gcp` | v1.43.0 | v1.44.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel/metric` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk/metric` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel/trace` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.44.0 | v1.45.0 |
| `go.opentelemetry.io/proto/otlp` | v1.10.0 | v1.11.0 |
| `golang.org/x/crypto` | v0.53.0 | v0.57.0 |
| `golang.org/x/net` | v0.56.0 | v0.60.0 |
| `golang.org/x/sync` | v0.22.0 | v0.23.0 |
| `golang.org/x/sys` | v0.46.0 | v0.48.0 |
| `golang.org/x/term` | v0.44.0 | v0.46.0 |
| `golang.org/x/text` | v0.39.0 | v0.42.0 |
| `google.golang.org/api` | v0.189.0 | v0.264.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260526163538-3dc84a4a5aaa | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260526163538-3dc84a4a5aaa | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto` | v0.0.0-20240722135656-d784300faade | v0.0.0-20260128011058-8636f8732409 |
| `google.golang.org/grpc` | v1.82.1 | v1.83.2 |
<!-- cvefix:end -->
