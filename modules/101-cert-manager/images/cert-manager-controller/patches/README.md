## Patches

### 001-certificate_owner_ref.patch

Adds `CertificateOwnerRef` flag to Certificate CRD. `CertificateOwnerRef` flag is whether to set the certificate resource as an owner of a secret where the TLS certificate is stored. When this flag is enabled, the secret will be automatically removed when the certificate resource is deleted.
https://github.com/cert-manager/cert-manager/pull/5158

The patch also adds the `gen.SetCertificateOwnerRef` test helper
and unit tests that check how `CertificateOwnerRef` overrides the `--enable-certificate-owner-ref` flag.

The post-issuance policy checks `SecretOwnerReferenceMismatch` and `SecretOwnerReferenceManagedFieldMismatch`
take the same effective value: `CertificateOwnerRef` when it is set, otherwise the flag.
Without this, a certificate whose `CertificateOwnerRef` differs from the flag triggers a Secret update on every reconcile,
and an owner reference requested in `CertificateOwnerRef` reaches an existing Secret only on the next issuance.

### 002-solver_test_ipv6_host.patch

Encloses the IPv6 host in square brackets in the request URL of the `TestSolver` HTTP-01 solver test.
Starting with the `go 1.26` directive that `999-fix-cve.patch` sets, `net/url` rejects an IPv6 host without brackets.
The change is a backport of upstream commit
[127488f](https://github.com/cert-manager/cert-manager/commit/127488f59fb627c0653f0cb1f94b213021629ea2)
and affects the test only.

### 003-gateway_discovery_retry.patch

Retries the Gateway API discovery at controller startup instead of exiting on the first error.
Upstream exits when the API server does not answer the discovery request, so a controller that starts
during an API server outage goes into `CrashLoopBackOff` and, once the API server is back, waits for
the kubelet back-off of up to five minutes.
The patch retries transient errors with a back-off of up to 10 seconds for at most five minutes.
The retries stop as soon as the controller receives a shutdown signal, so a pod deleted while it waits
exits at once instead of waiting for `SIGKILL`.
A `NotFound` answer is retried for one minute: a kube-apiserver that has just started answers `NotFound`
for a served CRD group until its discovery controller catches up, while a lasting `NotFound` means the
Gateway API CRDs are absent and still fails.
Unit tests cover the transient errors, a transient `NotFound` and a lasting one.

The check runs only when the module passes `--enable-gateway-api=true`, that is, when the cluster
serves the Gateway API CRDs; without them the patch changes nothing.
The health endpoint of the controller starts after this check, so the Deployment carries a startup
probe long enough for the whole retry period.

### 999-fix-cve.patch

Fix CVEs:
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
- GHSA-mpwr-8vm7-h73f

<!-- cvefix:begin -->
### Dependency overrides

`cvefix` from modules-ci maintains this section from `999-fix-cve.patch`. Do not edit it by hand.
It lists the `go.mod` requirements that the patch set changes over the upstream release.

#### cmd/acmesolver

The changes of `cmd/acmesolver/go.mod`:

| Module | Upstream | Patched |
|---|---|---|
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `golang.org/x/net` | v0.55.0 | v0.60.0 |
| `golang.org/x/sys` | v0.45.0 | v0.48.0 |
| `golang.org/x/text` | v0.37.0 | v0.42.0 |

The `go` directive of `cmd/acmesolver/go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.

#### cmd/cainjector

The changes of `cmd/cainjector/go.mod`:

| Module | Upstream | Patched |
|---|---|---|
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `golang.org/x/crypto` | v0.52.0 | v0.57.0 |
| `golang.org/x/net` | v0.55.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.35.0 | v0.36.0 |
| `golang.org/x/sync` | v0.20.0 | v0.23.0 |
| `golang.org/x/sys` | v0.45.0 | v0.48.0 |
| `golang.org/x/term` | v0.43.0 | v0.46.0 |
| `golang.org/x/text` | v0.37.0 | v0.42.0 |

The `go` directive of `cmd/cainjector/go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.

#### cmd/controller

The changes of `cmd/controller/go.mod`:

| Module | Upstream | Patched |
|---|---|---|
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.27.1 | v2.29.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/metric` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/proto/otlp` | v1.7.0 | v1.11.0 |
| `golang.org/x/crypto` | v0.52.0 | v0.57.0 |
| `golang.org/x/mod` | v0.35.0 | v0.41.0 |
| `golang.org/x/net` | v0.55.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.35.0 | v0.36.0 |
| `golang.org/x/sync` | v0.20.0 | v0.23.0 |
| `golang.org/x/sys` | v0.45.0 | v0.48.0 |
| `golang.org/x/term` | v0.43.0 | v0.46.0 |
| `golang.org/x/text` | v0.37.0 | v0.42.0 |
| `golang.org/x/tools` | v0.44.0 | v0.49.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260128011058-8636f8732409 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260319201613-d00831a3d3e7 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/grpc` | v1.79.3 | v1.83.2 |
| `software.sslmate.com/src/go-pkcs12` | v0.7.0 | v0.7.2 |

The `go` directive of `cmd/controller/go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.

#### cmd/webhook

The changes of `cmd/webhook/go.mod`:

| Module | Upstream | Patched |
|---|---|---|
| `cel.dev/expr` | v0.25.1 | v0.25.2 |
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `github.com/google/cel-go` | v0.26.0 | v0.29.0 |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.27.1 | v2.29.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/metric` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/proto/otlp` | v1.7.0 | v1.11.0 |
| `golang.org/x/crypto` | v0.52.0 | v0.57.0 |
| `golang.org/x/net` | v0.55.0 | v0.60.0 |
| `golang.org/x/oauth2` | v0.35.0 | v0.36.0 |
| `golang.org/x/sync` | v0.20.0 | v0.23.0 |
| `golang.org/x/sys` | v0.45.0 | v0.48.0 |
| `golang.org/x/term` | v0.43.0 | v0.46.0 |
| `golang.org/x/text` | v0.37.0 | v0.42.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260128011058-8636f8732409 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260319201613-d00831a3d3e7 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/grpc` | v1.79.3 | v1.83.2 |

The `go` directive of `cmd/webhook/go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.

#### go.mod

The changes of `go.mod`:

| Module | Upstream | Patched |
|---|---|---|
| `cel.dev/expr` | v0.25.1 | v0.25.2 |
| `github.com/go-logr/logr` | v1.4.3 | v1.4.4 |
| `github.com/google/cel-go` | v0.26.0 | v0.29.0 |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.27.1 | v2.29.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.37.0 | v1.45.0 |
| `go.opentelemetry.io/otel/metric` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel/trace` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/otel` | v1.43.0 | v1.45.0 |
| `go.opentelemetry.io/proto/otlp` | v1.7.0 | v1.11.0 |
| `golang.org/x/crypto` | v0.52.0 | v0.56.0 |
| `golang.org/x/mod` | v0.35.0 | v0.38.0 |
| `golang.org/x/net` | v0.55.0 | v0.58.0 |
| `golang.org/x/oauth2` | v0.35.0 | v0.36.0 |
| `golang.org/x/sync` | v0.20.0 | v0.22.0 |
| `golang.org/x/sys` | v0.45.0 | v0.47.0 |
| `golang.org/x/term` | v0.43.0 | v0.45.0 |
| `golang.org/x/text` | v0.37.0 | v0.41.0 |
| `golang.org/x/tools` | v0.44.0 | v0.48.0 |
| `google.golang.org/genproto/googleapis/api` | v0.0.0-20260128011058-8636f8732409 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260319201613-d00831a3d3e7 | v0.0.0-20260803160001-6ac0973c030d |
| `google.golang.org/grpc` | v1.79.3 | v1.83.2 |
| `software.sslmate.com/src/go-pkcs12` | v0.7.0 | v0.7.2 |

The `go` directive of `go.mod` is raised from 1.25.0 to 1.26.0: the patched dependencies require it.
<!-- cvefix:end -->
