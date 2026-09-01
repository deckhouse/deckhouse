---
title: Хранилище и балансировка нагрузки в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/storage.html
lang: ru
---

## Хранилище

Для хранения данных Kubernetes-кластера в VMware vSphere используются:

- Datastore — для размещения root-дисков виртуальных машин;
- CNS-диски (Container Native Storage) — для автоматического создания PersistentVolume’ов через CSI.

Deckhouse Kubernetes Platform автоматически создаёт StorageClass для каждого Datastore и DatastoreCluster, маркированных как `zone`.  
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
  version: 2
  enabled: true
  settings:
    storageClass:
      default: fast-lun102
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

### Политики хранения SPBM

Если в vSphere настроены политики хранения SPBM (Storage Policy Based Management), DKP обнаруживает их и дополнительно создаёт StorageClass для каждого сочетания Datastore и политики. Когда вы заказываете PersistentVolume через такой StorageClass, vSphere применяет к тому соответствующую политику.

Имя StorageClass складывается из имени Datastore и имени политики. DKP приводит его к нижнему регистру, заменяет пробелы на дефисы и удаляет остальные символы, кроме дефиса и точки. Например, для Datastore `lun_1` и политики `Gold Policy` получается StorageClass `lun1-gold-policy`.

Чтобы DKP обнаружил политики, учётной записи vSphere нужна привилегия `StorageProfile.View`.

Политики применяются к томам в любом сценарии, где работает модуль `cloud-provider-vsphere`, включая гибридный кластер. Выбрать политику для отдельного StorageClass вручную нельзя, набор StorageClass формируется автоматически.

Ограничения:

- StorageClass с политиками создаются только для объектов Datastore, для DatastoreCluster они не создаются;
- параметр [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) сопоставляется с именем Datastore, поэтому он убирает вместе с базовым StorageClass и все StorageClass с политиками для этого Datastore.

### Политика хранения для дисков узлов

Диски виртуальных машин, которые создаёт установщик, получают политику хранения из параметра [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) ресурса VsphereClusterConfiguration. В параметре указывается идентификатор политики SPBM.

Параметр действует на master-узлы и на статические узлы, которые создаёт установщик. Узлы, заказываемые по VsphereInstanceClass, политику из этого параметра не получают. В гибридном кластере, где ресурс VsphereClusterConfiguration не используется, задать политику для дисков узлов нельзя.

### Изменение размера тома (PVC)

Deckhouse Kubernetes Platform поддерживает Online Resize PersistentVolume, начиная с версии vSphere 7.0U2. Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer CSI и API vSphere после изменения размера PVC выполните следующие действия:

1. Выполните `d8 k cordon <имя_узла>` для узла, на котором работает под.
1. Удалите под, использующий PVC.
1. Дождитесь завершения операции Resize. У PVC не должно остаться condition `Resizing`, при этом состояние `FileSystemResizePending` не является проблемой.
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

## Настройка Datastore

{% alert %}
Pазметку **Datastore** также можно сделать через **vSphere Client** по инструкции [«Настройка через vSphere Client»](authorization.html#настройка-datastore-с-использованием-vsphere-client). Ниже описана настройка через **`govc`**.
{% endalert %}

Для корректной работы PersistentVolume необходимо, чтобы datastore был доступен на всех ESXi.

Назначьте теги:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```
