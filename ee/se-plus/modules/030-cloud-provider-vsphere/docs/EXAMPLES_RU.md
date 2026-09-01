---
title: "Cloud provider — VMware vSphere: примеры"
---

На этой странице собраны типовые сценарии настройки облачного провайдера VMware vSphere.
Примеры расположены от простого к сложному.
Полное описание параметров приведено в разделах [«Настройки»](configuration.html) и [«Custom Resources»](cr.html).

## Создание группы узлов

Инстанс-класс описывает параметры виртуальных машин, а группа узлов задаёт их количество и зоны размещения.
В примере создаётся группа из двух узлов, каждому из которых выделяется два vCPU и 4096 МиБ памяти.

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: worker
spec:
  numCPUs: 2
  memory: 4096
  rootDiskSize: 40
  template: <TEMPLATE_PATH>
  mainNetwork: <NETWORK_PATH>
  datastore: <DATASTORE_PATH>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: VsphereInstanceClass
      name: worker
    minPerZone: 2
    maxPerZone: 2
    zones:
      - <ZONE_TAG_NAME>
```

Значения плейсхолдеров:

- `<TEMPLATE_PATH>` — путь к шаблону виртуальной машины относительно Datacenter, например `dev/golden_image`;
- `<NETWORK_PATH>` — путь к сети относительно Datacenter, к которой подключается основной интерфейс узла;
- `<DATASTORE_PATH>` — путь к Datastore относительно Datacenter, например `dev/lun_1`;
- `<ZONE_TAG_NAME>` — имя тега зоны, назначенного объекту Cluster.

Размер корневого диска задаётся в ГиБ, объём памяти — в МиБ.

После применения манифеста Deckhouse Kubernetes Platform (DKP) создаёт виртуальные машины во vSphere.
Дождитесь, когда узлы перейдут в состояние `Ready`:

```shell
d8 k get nodes -l node.deckhouse.io/group=worker
```

## Подключение к vCenter в существующем кластере

В кластере, который создан без участия установщика, параметры подключения к vCenter задаются в ModuleConfig.
В примере используется проверка TLS-сертификата vCenter по цепочке сертификатов корпоративного центра сертификации.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    host: <VCENTER_FQDN>
    username: <USERNAME>@<DOMAIN>
    password: <PASSWORD>
    caBundle: |
      -----BEGIN CERTIFICATE-----
      <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
      -----END CERTIFICATE-----
    vmFolderPath: <VM_FOLDER_PATH>
    regionTagCategory: k8s-region
    zoneTagCategory: k8s-zone
    region: <REGION_TAG_NAME>
    zones:
      - <ZONE_TAG_NAME>
    internalNetworkNames:
      - <PORT_GROUP_NAME>
    sshKeys:
      - <SSH_PUBLIC_KEY>
```

Значения плейсхолдеров:

- `<VCENTER_FQDN>` — адрес vCenter;
- `<USERNAME>@<DOMAIN>` — имя пользователя вместе с доменом, например `deckhouse@vsphere.local`;
- `<VM_FOLDER_PATH>` — директория, в которой создаются виртуальные машины;
- `<REGION_TAG_NAME>` — имя тега региона, назначенного объекту Datacenter;
- `<PORT_GROUP_NAME>` — имя сети без полного пути, по которой определяется внутренний адрес узла.

Проверьте, что модуль перешёл в состояние `Ready`:

```shell
d8 k get module cloud-provider-vsphere -o wide
```

## Ограничение набора StorageClass

DKP создаёт StorageClass для каждого размеченного тегами Datastore, а при настроенных политиках хранения SPBM — ещё и для каждого сочетания Datastore и политики.
В примере из кластера исключаются StorageClass медленных хранилищ, чтобы разработчики не заказывали на них тома.

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
        - slow-lun103
        - ".*-lun101-.*"
```

В параметре `exclude` перечисляются имена StorageClass или регулярные выражения для них.
Выражение сопоставляется с именем Datastore, поэтому исключение убирает вместе с основным StorageClass и все StorageClass с политиками хранения для этого Datastore.

Убедитесь, что лишние StorageClass пропали из кластера:

```shell
d8 k get storageclass
```

Чтобы задать StorageClass по умолчанию, используйте глобальный параметр [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass).

## Узлы с несколькими сетями и ограничением ресурсов

Виртуальные машины можно подключать к дополнительным сетям и ограничивать потребление ресурсов на стороне vSphere.
В примере узел подключается к сети хранения данных, а его потребление процессора ограничено 4000 МГц.

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: storage-worker
spec:
  numCPUs: 8
  memory: 16384
  rootDiskSize: 60
  template: <TEMPLATE_PATH>
  mainNetwork: <NETWORK_PATH>
  additionalNetworks:
    - <STORAGE_NETWORK_PATH>
  datastore: <DATASTORE_PATH>
  runtimeOptions:
    cpuLimit: 4000
    memoryReservation: 100
```

Значения плейсхолдеров:

- `<STORAGE_NETWORK_PATH>` — путь к дополнительной сети относительно Datacenter.

Параметр `cpuLimit` задаёт предел потребления процессора в МГц.
Параметр `memoryReservation` резервирует память в процентах от значения `memory`, по умолчанию он равен 80.

{% alert level="warning" %}
Занижение параметра `cpuLimit` замедляет работу узла.
{% endalert %}

## Балансировщик нагрузки через NSX-T

Если в инфраструктуре развёрнут NSX-T, DKP может заказывать в нём балансировщики для сервисов с типом LoadBalancer.
В примере задаётся пул адресов по умолчанию и проверка TLS-сертификата NSX-T.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    nsxt:
      host: <NSXT_HOST>
      user: <NSXT_USERNAME>
      password: <NSXT_PASSWORD>
      defaultIpPoolName: <IP_POOL_NAME>
      tier1GatewayPath: <TIER1_GATEWAY_PATH>
      caBundle: |
        -----BEGIN CERTIFICATE-----
        <CA_CERTIFICATE_CHAIN_IN_PEM_FORMAT>
        -----END CERTIFICATE-----
```

Значения плейсхолдеров:

- `<IP_POOL_NAME>` — имя пула адресов для сервисов без аннотации `loadbalancer.vmware.io/class`;
- `<TIER1_GATEWAY_PATH>` — путь к шлюзу Tier-1, в котором создаются балансировщики.

Чтобы направить сервис в другой пул адресов, опишите класс балансировщика в параметре `nsxt.loadBalancerClass` и укажите его в аннотации `loadbalancer.vmware.io/class` сервиса.
