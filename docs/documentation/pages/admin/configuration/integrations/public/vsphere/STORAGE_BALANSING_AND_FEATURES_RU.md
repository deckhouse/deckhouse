---
title: Хранилище и балансировка
permalink: ru/admin/integrations/public/vsphere/vsphere-storage.html
lang: ru
---

## Хранилище

Для хранения данных Kubernetes-кластера в VMware vSphere используются:

- Datastore — для размещения root-дисков виртуальных машин;
- CNS-диски (Container Native Storage) — для автоматического создания PersistentVolume’ов через CSI.

Deckhouse автоматически создаёт StorageClass для каждого Datastore и DatastoreCluster, тегированных как `zone`.  
Можно указать:

- имя StorageClass по умолчанию (`default`);
- исключения через `exclude` — список имен или шаблонов StorageClass, которые не нужно создавать.

Пример настройки через ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      default: fast-lun102
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

### Изменение размера тома (PVC)

Deckhouse поддерживает Online Resize PersistentVolume с версии vSphere 7.0U2. Однако из-за особенностей CSI и API vSphere после изменения размера PVC требуется выполнить дополнительные действия:

1. Выполнить `kubectl cordon <имя_узла>`.
1. Удалить под, использующий PVC.
1. Дождаться завершения операции Resize:
   - убедиться, что у PVC нет condition `Resizing`;
   - `FileSystemResizePending` можно игнорировать.
1. Выполнить `kubectl uncordon <имя_узла>`.

## Балансировка нагрузки

Варианты организации балансировки входящего трафика:

1. Через внешний балансировщик. Если в инфраструктуре уже есть внешний балансировщик (например, NSX-T), можно направлять трафик напрямую на frontend-узлы кластера.

1. Через MetalLB. Для отказоустойчивой балансировки внутри кластера рекомендуется использовать MetalLB в режиме BGP. В этом случае:

   - frontend-узлы получают два сетевых интерфейса;
   - требуется отдельный VLAN для BGP-трафика;
   - необходим DHCP и доступ в интернет в этой сети;
   - указываются IP-адреса и ASN BGP-роутеров;
   - задаётся пул IP-адресов, который будет анонсироваться.

> Необходимо обеспечить связь между BGP-роутерами и frontend-узлами в выделенном VLAN.
