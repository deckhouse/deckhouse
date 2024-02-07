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
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: development-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - development
  destinationRefs:
    - d8-loki
```

Больше примеров в описании модуля [log-shipper](../460-log-shipper/examples.html).

{% endraw %}
