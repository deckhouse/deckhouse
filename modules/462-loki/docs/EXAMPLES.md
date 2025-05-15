---
title: "Module loki: examples"
---


{% raw %}

## Reading Pod logs from a specified namespace and sending them to Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 30
    retentionPeriodHours: 168
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingConfig
metadata:
  name: development-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchExpressions:
        - key: "kubernetes.io/metadata.name"
          operator: In
          values: [development]
  destinationRefs:
    - d8-loki
```

For more examples, see the [log-shipper](../log-shipper/examples.html) module page.

{% endraw %}
