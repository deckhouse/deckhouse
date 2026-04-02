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
