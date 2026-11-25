---
title: Хранилище в Google Cloud Platform
permalink: ru/admin/integrations/public/gcp/load-balancing-and-storage.html
lang: ru
---

## Хранилище

Deckhouse Platform автоматически создаёт StorageClass'ы, покрывающие все варианты дисков в GCP:

| Тип | Репликация | Имя StorageClass |
|---|---|---|
| standard | none | `pd-standard-not-replicated` |
| standard | regional | `pd-standard-replicated` |
| balanced | none | `pd-balanced-not-replicated` |
| balanced | regional | `pd-balanced-replicated` |
| ssd | none | `pd-ssd-not-replicated` |
| ssd | regional | `pd-ssd-replicated` |

Можно отфильтровать ненужные StorageClass'ы, для этого укажите их в [параметре `exclude`](/modules/cloud-provider-gcp/configuration.html#parameters-storageclass-exclude).

## Добавление дополнительных дисков

Чтобы подключить к инстансам дополнительные диски (например, для узлов хранилища LINSTOR, Ceph, NFS и аналогичных решений), задайте их в параметре `additionalDisks`:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: storage-nodes
spec:
  machineType: n1-standard-8
  additionalDisks:
  - sizeGb: 200
    type: pd-ssd
  - sizeGb: 500
    type: pd-standard
    autoDelete: true
```
