---
title: Примеры конфигурации
permalink: ru/admin/configuration/logging/loki/configuration-examples.html
lang: ru
---

## Чтение логов со всех подов из указанного пространства имён и отправка в loki

Пример конфигурации `log-shipper` для сбора логов со всех подов в пространстве имён `development` и сохранения в `loki`.
Дополнительно, в конфигурации указывается настройка хранения данных для `loki`,
включая используемый StorageClass, размер диска и период хранения логов.

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
