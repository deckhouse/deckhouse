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

### node_group

Adds meta label __meta_kubernetes_<node/pod/endpointslices>_node_group to `kubernetes_sd` roles, that is take from `node.deckhouse.io/group` label on the node.
