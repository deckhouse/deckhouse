# Prometheus

Image with patched Prometheus v2.28.0.

## Patches

### Sample limit annotation

Limit the number of metrics which Prometheus scrapes from a target.  

```yaml
metadata:
  annotations:
    prometheus.deckhouse.io/sample-limit: "5000"
```

### Successfully sent metric

Exports gauge metric with the count of successfully sent alerts. 
