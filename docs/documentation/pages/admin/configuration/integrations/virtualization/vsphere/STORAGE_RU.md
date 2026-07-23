---
title: Хранилище и балансировка нагрузки в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/storage.html
lang: ru
---

## Обзор

В кластере Deckhouse Kubernetes Platform (DKP) на VMware vSphere используются два независимых типа хранилища:

| Назначение | Технология | Где настраивается |
|------------|------------|-------------------|
| Root-диски виртуальных машин (узлов кластера) | Файлы ВМ на Datastore | [`datastore`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-datastore) в `VsphereInstanceClass`; [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) — только в `VsphereClusterConfiguration` (облачный кластер) |
| PersistentVolume для приложений | CNS-диски (Container Native Storage) через CSI | Автоматически через теги на Datastore; настройка — в [`ModuleConfig`](/modules/cloud-provider-vsphere/configuration.html) модуля `cloud-provider-vsphere` |

Root-диск узла и том приложения могут размещаться на одном и том же Datastore или на разных — это задаётся независимо.

{% alert level="info" %}
Подготовка Datastore (теги, доступность на ESXi) описана в разделе [«Подключение и авторизация»](authorization.html#настройка-datastore-с-использованием-vsphere-client). Ниже — настройка хранилища на стороне кластера Kubernetes.
{% endalert %}

## Root-диски виртуальных машин

### Где поддерживается `storagePolicyID`

Параметр [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) задаёт SPBM-политику (Storage Policy Based Management) для root-дисков виртуальных машин. В коде модуля он определён **только** в ресурсе `VsphereClusterConfiguration` и применяется компонентом Terraform (`dhctl`) при создании узлов [CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html).

| Сценарий | Ресурс конфигурации | `storagePolicyID` | Как выбрать хранилище |
|----------|---------------------|-------------------|------------------------|
| Облачный кластер, узлы CloudPermanent | `VsphereClusterConfiguration` | **Поддерживается** | `datastore` в `instanceClass` + опционально `storagePolicyID` на уровне кластера |
| Гибридный кластер, узлы CloudEphemeral | `VsphereInstanceClass` | **Не поддерживается** | Только `datastore` в `VsphereInstanceClass` |
| Гибридный кластер, узлы CloudStatic | — (ВМ создаётся вручную) | **Не поддерживается** | Datastore и политика задаются в vSphere при создании ВМ |
| PersistentVolume приложений | `StorageClass` (создаётся автоматически) | **Не через `storagePolicyID`** | Выбор `StorageClass` с параметром `StoragePolicyName` — см. [CSI и PersistentVolume](#csi-и-persistentvolume) |

{% alert level="warning" %}
В гибридной интеграции ресурс `VsphereClusterConfiguration` **не используется**, поэтому параметр `storagePolicyID` там недоступен. Не ищите его в `ModuleConfig` или `VsphereInstanceClass` — в схемах этих ресурсов поля `storagePolicyID` нет.
{% endalert %}

### Настройка `datastore` для root-диска

При создании узлов DKP клонирует шаблон ВМ и размещает root-диск на Datastore, указанном в конфигурации группы узлов:

```yaml
instanceClass:
  datastore: dev/lun_1   # путь относительно Datacenter
  rootDiskSize: 50       # размер root-диска в ГиБ (опционально)
```

Параметр `datastore` поддерживается и в `VsphereClusterConfiguration` (облачный кластер), и в [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) (гибридный сценарий).

DKP создаёт диски с типом `eagerZeroedThick`, но итоговый тип может быть изменён политикой хранения vSphere.

### Политика хранения для root-дисков (`storagePolicyID`)

Применимо **только** к облачному кластеру. Параметр задаётся один раз в `VsphereClusterConfiguration` и распространяется на root-диски всех узлов, создаваемых Terraform: master-узлов и групп из секции `nodeGroups`.

ID политики можно получить в vSphere Client (**Policies and Profiles** → VM Storage Policies) или через `govc`:

```shell
govc storage.policy.ls
```

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
storagePolicyID: "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
provider:
  server: "<VCENTER_FQDN>"
  username: "<USERNAME@DOMAIN.LOCAL>"
  password: "<PASSWORD>"
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: test-region
zones:
  - test-zone-1
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
```

### Гибридный сценарий: выбор хранилища без `storagePolicyID`

Если нужен аналогичный контроль над размещением root-дисков в гибридном кластере:

- **Root-диск узла CloudEphemeral** — укажите целевой Datastore в `VsphereInstanceClass.spec.datastore`. SPBM-политика для root-диска через DKP не настраивается; vSphere применит политику, связанную с выбранным Datastore, согласно настройкам инфраструктуры.
- **Том приложения (PVC)** — используйте автоматически созданный `StorageClass`. Если в vCenter настроены VM Storage Policies, для каждой комбинации «Datastore + политика» создаётся отдельный StorageClass (например, `dev-lun-102-gold-policy`). Укажите его в `PersistentVolumeClaim.spec.storageClassName`.

Подробнее о гибридной интеграции — в разделе [«Гибридный кластер с vSphere»](../../hybrid/vsphere-hybrid.html#хранилище-в-гибридном-сценарии).

{% alert %}
Подробнее о подготовке шаблона ВМ и политиках дисков — в разделе [«Подключение и авторизация»](authorization.html#требования-к-образу-виртуальной-машины).
{% endalert %}

## CSI и PersistentVolume

### Как работает автоматическое обнаружение хранилищ

Компонент `cloud-data-discoverer` периодически опрашивает vCenter и формирует список доступных Datastore. В список попадают объекты, которые:

1. Находятся в Datacenter с тегом региона (категория [`regionTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-regiontagcategory), по умолчанию `k8s-region`).
2. Имеют тег зоны (категория [`zoneTagCategory`](/modules/cloud-provider-vsphere/configuration.html#parameters-zonetagcategory), по умолчанию `k8s-zone`) из списка зон кластера.
3. Доступны на всех ESXi-хостах зоны (shared datastore).

На основе обнаруженных Datastore модуль `cloud-provider-vsphere` создаёт объекты `StorageClass` в кластере Kubernetes.

### Имена StorageClass

Имя StorageClass формируется из пути Datastore в инвентаре vSphere: символы приводятся к нижнему регистру, пробелы заменяются на дефисы. Например, Datastore `dev/lun_102` может стать StorageClass `dev-lun-102`.

Если в vCenter настроены политики хранения (VM Storage Policies), для каждой комбинации «Datastore + политика» создаётся отдельный StorageClass с именем вида `<datastore>-<policy-name>`.

### Datastore и DatastoreCluster

Обнаруживаются как отдельные Datastore, так и кластеры Datastore (DatastoreCluster). Однако режим создания StorageClass зависит от используемого CSI-драйвера:

| Тип объекта vSphere | CNS (режим по умолчанию) | Legacy (FCD) |
|---------------------|--------------------------|--------------|
| Datastore | StorageClass создаётся | StorageClass создаётся |
| DatastoreCluster | StorageClass **не** создаётся | StorageClass создаётся |

Для динамического заказа PVC в стандартном режиме (CNS) используйте отдельные Datastore с корректными тегами зоны.

### Параметры StorageClass

Созданные StorageClass имеют следующие характеристики:

- **Provisioner:** `csi.vsphere.vmware.com` (CNS) или `vsphere.csi.vmware.com` (Legacy).
- **volumeBindingMode:** `WaitForFirstConsumer` (CNS) / `Immediate` (Legacy) — том создаётся на ESXi, где запланирован под.
- **allowVolumeExpansion:** `true` — поддерживается увеличение размера PVC (в режиме CNS, начиная с vSphere 7.0U2).
- **allowedTopologies:** ограничение по зонам — PVC будет создан только в Datastore с тегом соответствующей зоны.

Пример созданного StorageClass (CNS):

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: dev-lun-102
provisioner: csi.vsphere.vmware.com
parameters:
  DatastoreURL: "ds:///vmfs/volumes/..."
  StoragePolicyName: "Gold Policy"   # если политика задана
allowedTopologies:
- matchLabelExpressions:
  - key: failure-domain.beta.kubernetes.io/region
    values: ["test-region"]
  - key: failure-domain.beta.kubernetes.io/zone
    values: ["test-zone-1"]
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

### Настройка StorageClass в кластере

Через `ModuleConfig` модуля `cloud-provider-vsphere` можно:

- **Исключить** ненужные StorageClass — параметр [`storageClass.exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude). Принимает точные имена или regex-выражения.
- **Задать StorageClass по умолчанию** — используйте глобальный параметр [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass). Параметр `storageClass.default` в модуле устарел.

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
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

Если StorageClass по умолчанию не задан явно, используется первый (по алфавиту) StorageClass, созданный модулем.

## Режимы CSI-драйвера

Поведение подсистемы хранения определяется параметром [`storageClass.compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag):

| Значение | Драйвер | Тип дисков | Online resize | Снимки томов |
|----------|---------|------------|---------------|--------------|
| не задано (по умолчанию) | `csi.vsphere.vmware.com` | CNS | Да (vSphere 7.0U2+) | Да |
| `Legacy` | `vsphere.csi.vmware.com` | FCD (First Class Disk) | Нет | Нет |
| `Migration` | оба драйвера одновременно | CNS + FCD | Да для CNS | Да для CNS |

Режим `Migration` предназначен для перехода с устаревшего FCD-драйвера на CNS. После миграции всех PVC установите `compatibilityFlag` в пустое значение (или удалите параметр), чтобы отключить legacy-драйвер.

{% alert level="warning" %}
Перед миграцией PVC с FCD на CNS убедитесь, что шаблоны ВМ используют hardware version 15 или выше. Подробнее — в [документации модуля](/modules/cloud-provider-vsphere/configuration.html#csi).
{% endalert %}

## Увеличение размера PVC

DKP поддерживает online resize PersistentVolume в режиме CNS (vSphere 7.0U2+). Из-за [особенностей](https://github.com/kubernetes-csi/external-resizer/issues/44) работы volume-resizer и vSphere API после изменения размера PVC требуются дополнительные действия:

1. Выполните `d8 k cordon <имя_узла>`, на котором работает под с томом.
1. Удалите под, использующий PVC.
1. Дождитесь завершения операции Resize:
   - убедитесь, что у PVC **нет** condition `Resizing`;
   - состояние `FileSystemResizePending` можно игнорировать.
1. Выполните `d8 k uncordon <имя_узла>`.

## Снимки томов (Volume Snapshots)

При включённом модуле [`snapshot-controller`](/modules/snapshot-controller/) DKP автоматически создаёт `VolumeSnapshotClass` с именем `vsphere` для CSI-драйвера CNS. Снимки поддерживаются только в стандартном режиме (не в `Legacy`).

Пример создания снимка:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
spec:
  volumeSnapshotClassName: vsphere
  source:
    persistentVolumeClaimName: my-pvc
```

## Настройка Datastore для PVC

Для корректной работы динамического заказа PersistentVolume Datastore должен быть доступен на **каждом** ESXi-хосте в зоне (shared datastore).

Назначьте теги региона и зоны на объекты Datastore. Это можно сделать через vSphere Client — см. [«Настройка Datastore»](authorization.html#настройка-datastore-с-использованием-vsphere-client), или через `govc`:

```shell
govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName1>
govc tags.attach -c k8s-zone test-zone-1 /<DatacenterName>/datastore/<DatastoreName1>

govc tags.attach -c k8s-region test-region /<DatacenterName>/datastore/<DatastoreName2>
govc tags.attach -c k8s-zone test-zone-2 /<DatacenterName>/datastore/<DatastoreName2>
```

{% alert level="warning" %}
Все Cluster в пределах одной зоны должны иметь доступ ко всем Datastore с тегом этой зоны. Подробнее о модели регионов и зон — в разделе [«Подключение и авторизация»](authorization.html#создание-тегов-и-категорий-тегов-с-использованием-vsphere-client).
{% endalert %}

## Балансировка нагрузки

Варианты организации балансировки входящего трафика в кластере на vSphere:

### Внешний балансировщик

Если в инфраструктуре уже есть внешний балансировщик (например, аппаратный или на базе NSX-T в режиме reverse proxy), можно направлять трафик напрямую на frontend-узлы кластера.

### MetalLB (BGP)

Для отказоустойчивой балансировки внутри кластера рекомендуется использовать MetalLB в режиме BGP:

- frontend-узлы получают два сетевых интерфейса;
- требуется отдельный VLAN для BGP-трафика;
- необходим DHCP и доступ в интернет в этой сети;
- указываются IP-адреса и ASN BGP-роутеров;
- задаётся пool IP-адресов, который будет анонсироваться.

{% alert level="info" %}
Необходимо обеспечить связь между BGP-роутерами и frontend-узлами в выделенном VLAN.
{% endalert %}

### NSX-T Load Balancer (через cloud-controller-manager)

Модуль `cloud-provider-vsphere` поддерживает создание сервисов типа `LoadBalancer` через интеграцию с NSX-T. Для этого в `ModuleConfig` настраивается секция [`nsxt`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  settings:
    nsxt:
      defaultIpPoolName: pool1
      tier1GatewayPath: /infra/tier-1s/gateway1
      host: nsx-manager.example.com
      user: admin
      password: "<PASSWORD>"
      insecureFlag: true
```

После настройки сервисы типа `LoadBalancer` получают внешний IP из пула NSX-T. Для использования альтернативных профилей и пулов IP задайте аннотацию `loadbalancer.vmware.io/class` на Service — подробнее в [документации модуля](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt).
