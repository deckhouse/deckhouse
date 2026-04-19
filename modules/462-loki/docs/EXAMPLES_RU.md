---
title: "Модуль loki: примеры"
---

{% raw %}

## Чтение логов из всех подов из указанного namespace и направление их в Loki

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

Больше примеров в описании модуля [log-shipper](../log-shipper/examples.html).

{% endraw %}
