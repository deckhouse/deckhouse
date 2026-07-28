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

### 005-fix-cve-bump.patch

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-fw5q-2xv9-49qr) — `google.golang.org/grpc` bumped from v1.66.0 to v1.80.0.
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq) — `go.opentelemetry.io/otel/sdk` bumped from v1.29.0 to v1.43.0.
- [CVE-2026-39883](https://github.com/advisories/GHSA-c98q-8jvw-w7p2) — `go.opentelemetry.io/otel/sdk` bumped from v1.29.0 to v1.43.0.
- [CVE-2026-39882](https://github.com/advisories/GHSA-pqrx-pwhc-3wf2) — `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` bumped from v1.29.0 to v1.43.0.
- x/crypto→v0.53.0, x/net→v0.56.0, x/sys→v0.46.0, x/text→v0.39.0, `go.mongodb.org/mongo-driver`→v1.17.7: x/crypto CVE-2026-39828/39829/39830/39831/39832/39835, CVE-2026-42508, CVE-2026-46595/46597; x/net CVE-2026-25680/25681/27136/33814/39821, CVE-2026-42502/42506, CVE-2026-46600; x/sys CVE-2026-39824; x/text CVE-2026-56852; mongo-driver CVE-2026-2303.
