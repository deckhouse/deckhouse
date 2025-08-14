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

Update dependencies

### 004-hardfix_bug_with_dropped_unknown_samples.patch

Add loading chunk snapshots in remote-write to solve problem with unknown series's samples drop.
