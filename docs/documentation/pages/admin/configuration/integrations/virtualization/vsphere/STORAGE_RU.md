---
title: Хранилище и балансировка
permalink: ru/admin/integrations/virtualization/vsphere/storage.html
lang: ru
---

## Хранилище

Для хранения данных Kubernetes-кластера в VMware vSphere используются:

- Datastore — для размещения root-дисков виртуальных машин;
- CNS-диски (Container Native Storage) — для автоматического создания PersistentVolume’ов через CSI.

Deckhouse Kubernetes Platform автоматически создаёт StorageClass для каждого Datastore и DatastoreCluster, тегированных как `zone`.  
Можно указать:

- имя StorageClass по умолчанию ([`default`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-default));
- исключения через [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) — список имен или шаблонов StorageClass, которые не нужно создавать.

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

Deckhouse Kubernetes Platform поддерживает Online Resize PersistentVolume, начиная с версии vSphere 7.0U2. Однако из-за особенностей CSI и API vSphere после изменения размера PVC требуется выполнить дополнительные действия:

1. Выполните `d8 k cordon <имя_узла>`.
1. Удалите под, использующий PVC.
1. Дождитесь завершения операции Resize:
   - убедитесь, что у PVC нет condition `Resizing`;
   - `FileSystemResizePending` можно игнорировать.
1. Выполните `d8 k uncordon <имя_узла>`.

## Балансировка нагрузки

Варианты организации балансировки входящего трафика:

1. **Через внешний балансировщик.** Если в инфраструктуре уже есть внешний балансировщик (например, NSX-T), можно направлять трафик напрямую на frontend-узлы кластера.

1. **Через MetalLB.** Для отказоустойчивой балансировки внутри кластера рекомендуется использовать MetalLB в режиме BGP. В этом случае:

   - frontend-узлы получают два сетевых интерфейса;
   - требуется отдельный VLAN для BGP-трафика;
   - необходим DHCP и доступ в интернет в этой сети;
   - указываются IP-адреса и ASN BGP-роутеров;
   - задаётся пул IP-адресов, который будет анонсироваться.

{% alert level="info" %}
Необходимо обеспечить связь между BGP-роутерами и frontend-узлами в выделенном VLAN.
{% endalert %}

## CSI

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения их размера на лету. Но также поддерживается работа и в legacy-режиме с использованием FCD-дисков. Поведение подсистемы устанавливается с помощью [параметра `compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag).

## Важная информация об увеличении размера PVC

Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer CSI и vSphere API, после увеличения размера PVC нужно сделать следующее:

1. На узле, где находится под, выполните команду `d8 k cordon <имя_узла>`.
1. Удалите под.
1. Убедитесь, что изменение размера прошло успешно. В объекте PVC *не будет* condition `Resizing`.
   > Состояние `FileSystemResizePending` не является проблемой.
1. На узле, где находится под, выполните команду `d8 k uncordon <имя_узла>`.

## Настройка Datastore

Для корректной работы PersistentVolume необходимо, чтобы datastore был доступен на всех ESXi.

Назначьте теги:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
