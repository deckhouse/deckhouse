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

Bump dependencies to fix CVEs:
- [CVE-2026-33186](https://github.com/advisories/GHSA-fw5q-2xv9-49qr) — `google.golang.org/grpc` bumped from v1.66.0 to v1.80.0.
- [CVE-2026-24051](https://github.com/advisories/GHSA-9h8m-3fm2-qjrq) — `go.opentelemetry.io/otel/sdk` bumped from v1.29.0 to v1.43.0.
- [CVE-2026-39883](https://github.com/advisories/GHSA-c98q-8jvw-w7p2) — `go.opentelemetry.io/otel/sdk` bumped from v1.29.0 to v1.43.0.
- [CVE-2026-39882](https://github.com/advisories/GHSA-pqrx-pwhc-3wf2) — `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` bumped from v1.29.0 to v1.43.0.
