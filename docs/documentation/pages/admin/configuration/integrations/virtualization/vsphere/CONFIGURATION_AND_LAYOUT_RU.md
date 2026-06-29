---
title: Схемы размещения и настройка VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/layout.html
lang: ru
---

## Standard

Схема Standard предназначена для размещения кластера внутри инфраструктуры vSphere с возможностью управления ресурсами, сетями и хранилищем.

Особенности:

- Использование vSphere Datacenter в качестве региона ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region));
- Использование vSphere Cluster в качестве зоны ([`zone`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones));
- Поддержка нескольких зон и размещения узлов по зонам;
- Использование различных datastore для дисков и volume’ов;
- Поддержка подключения сетей, включая дополнительную сетевую изоляцию (например, MetalLB + BGP).

![resources](../../../../images/cloud-provider-vsphere/vsphere-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11345&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  zones:
    - ru-central1-a
    - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
  - name: khm
    replicas: 1
    zones:
      - ru-central1-a
    instanceClass:
      numCPUs: 4
      memory: 8192
      template: dev/golden_image
      datastore: dev/lun_1
      mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
  - ru-central1-a
  - ru-central1-b
```

Обязательные параметры [ресурса VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration):

- `layout` — схема размещения (`Standard`);
- `provider` — параметры подключения к vCenter;
- `masterNodeGroup` — описание master-узлов (в `instanceClass` обязательны `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`);
- `region` — тег, присвоенный объекту Datacenter;
- `zoneTagCategory` и `regionTagCategory` — категории тегов, по которым распознаются регионы и зоны;
- `vmFolderPath` — путь до папки, в которой будут размещаться виртуальные машины кластера;
- `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
- `zones` — список зон, доступных для размещения узлов.

{% alert level="info" %}
Все узлы, размещённые в разных зонах, должны иметь доступ к общим datastore с аналогичными тегами зоны.
{% endalert %}

## Сетевые параметры {#сетевые-параметры}

Сетевые настройки модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) задаются в [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html), [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) и [`ModuleConfig`](/modules/cloud-provider-vsphere/configuration.html). Таблица ниже суммирует обязательность параметров и их применимость по типам узлов.

| Параметр | Где задаётся | CloudPermanent | CloudEphemeral | Назначение |
|----------|--------------|----------------|----------------|------------|
| [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) | `instanceClass` / `VsphereInstanceClass` | **Обязателен** | **Обязателен** (если не указан в `VsphereInstanceClass`, используется значение master-узла, когда доступно) | Port group основного NIC при создании ВМ |
| [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) | `VsphereClusterConfiguration` | **Обязателен**, если у master указан `additionalNetworks`; иначе не используется | Не используется | Назначение статических IP master-узлам с дополнительными NIC (Terraform) |
| [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) | `VsphereClusterConfiguration` / `ModuleConfig` | Опционален | Опционален | `vsphere-cloud-controller-manager`: `InternalIP` в `Node.status.addresses` |
| [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) | `VsphereClusterConfiguration` / `ModuleConfig` | Опционален | Опционален | `vsphere-cloud-controller-manager`: `ExternalIP` в `Node.status.addresses` |
| [`resourcePool`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-resourcepool) | `instanceClass` / `VsphereInstanceClass` | Опционален (DKP может создать вложенный pool) | Опционален (если указан — должен уже существовать) | Размещение ВМ в Resource Pool |

`mainNetwork` определяет сеть, к которой подключается ВМ при создании. `internalNetworkNames` и `externalNetworkNames` не влияют на создание ВМ — только на публикацию IP-адресов узла в Kubernetes API после того, как ВМ создана.

### mainNetwork {#mainnetwork}

**Обязательный** параметр в `instanceClass`:

- для `masterNodeGroup` и `nodeGroups` ресурса `VsphereClusterConfiguration` (узлы [CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html), создаются через Terraform);
- для ресурса [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) при создании узлов [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) (создаются через machine-controller-manager) — если не указан, используется значение из конфигурации master-узлов (когда доступно).

Задаёт port group, к которой подключается основной сетевой интерфейс виртуальной машины (маршрут по умолчанию).

**Формат значения** — путь к сети **относительно Datacenter** (не просто отображаемое имя сети и не полный inventory path от корня vCenter):

| Значение | Описание |
|----------|----------|
| `net3-k8s` | Имя port group в корне раздела Networks датацентра |
| `k8s-msk/test_187` | Port group `test_187` в папке `k8s-msk` |
| `"PROD NET"` | Имя port group с пробелом — в YAML указывайте в кавычках |

```yaml
instanceClass:
  mainNetwork: "PROD NET"
  # или с папкой:
  # mainNetwork: "k8s-networks/PROD NET"
```

### Узлы CloudPermanent и CloudEphemeral {#cloudpermanent-and-cloudephemeral-nodes}

Узлы [CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) и [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) во vSphere создаются разными компонентами — Terraform (`dhctl`) и [machine-controller-manager](https://github.com/gardener/machine-controller-manager) соответственно.

Оба принимают путь к сети относительно Datacenter, но механизм поиска объекта в vSphere Inventory отличается. Из-за этого проблемы с сетью могут проявляться по-разному для разных типов узлов: для одного типа сеть может находиться корректно, а для другого заказ может падать из-за прав, пути или способа поиска сети.

Например, ошибки вида «network not found» при корректной конфигурации master-узлов могут проявляться только при создании ephemeral-узлов — проверьте `mainNetwork` в `VsphereInstanceClass` и права [`Network.Assign`](authorization.html#проверка-прав-на-сеть-с-использованием-govc) на целевую сеть.

### internalNetworkCIDR

**Опциональный** параметр `VsphereClusterConfiguration`.

| Тип узлов | Обязателен? | Используется |
|-----------|-------------|--------------|
| CloudPermanent (master с `additionalNetworks`) | **Да** | Terraform — назначает IP-адреса master-узлам из указанной подсети (начиная с десятого адреса: для `192.168.199.0/24` — с `192.168.199.10`) |
| CloudPermanent (master без `additionalNetworks`) | Нет | Не используется |
| CloudPermanent (worker в `nodeGroups`) | Нет | Не используется |
| CloudEphemeral | Нет | Не используется |

### internalNetworkNames и externalNetworkNames {#internalnetworknames-and-externalnetworknames}

**Опциональные** параметры. Задаются в `VsphereClusterConfiguration` и/или в настройках модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/configuration.html) (`ModuleConfig`).

| Тип узлов | Обязателен? | Используется |
|-----------|-------------|--------------|
| CloudPermanent | Нет | `vsphere-cloud-controller-manager` |
| CloudEphemeral | Нет | `vsphere-cloud-controller-manager` |

Используются для заполнения полей `InternalIP` и `ExternalIP` в `Node.status.addresses`. Не влияют на выбор сети при создании ВМ.

{% alert level="info" %}
Указывается **только имя сети** (port group) — без пути, в том виде, как оно отображается в свойствах сетевого адаптера ВМ в vSphere. Это отличается от формата параметра `mainNetwork`.
{% endalert %}

Рекомендуется задавать, если у узлов несколько сетевых интерфейсов и необходимо явно разделить internal/external адреса для Kubernetes.

Пример:

```yaml
internalNetworkNames:
  - K8S_INTERNAL
externalNetworkNames:
  - PUBLIC_NET
```

### Права на сеть в vSphere {#network-permissions-in-vsphere}

У сервисной учётной записи должна быть привилегия [`Network.Assign`](authorization.html#список-необходимых-привилегий) на каждую port group, указанную в `mainNetwork` (и в `additionalNetworks`, если используется). При [гранулярной модели прав](authorization.html#гранулярная-модель-прав) привилегия должна быть назначена на **каждую** целевую port group или унаследована от родительского объекта.

Проверьте права сервисной учётной записи на целевую сеть:

```shell
export GOVC_URL="https://<VCENTER_FQDN>/sdk"
export GOVC_USERNAME="<USERNAME@DOMAIN.LOCAL>"
export GOVC_PASSWORD="<PASSWORD>"
export GOVC_INSECURE=true

govc permissions.ls -r "/<Datacenter>/network/<NETWORK_NAME>"
```

Пример для сети с пробелом в имени:

```shell
govc permissions.ls -r "/<Datacenter>/network/PROD NET"
```

В выводе команды должна быть роль для учётной записи DKP с привилегией `Network.Assign`. Флаг `-r` (`--recursive`) показывает права, унаследованные от родительских объектов.

Примеры путей и подробности — в разделе [«Проверка прав на сеть с использованием govc»](authorization.html#проверка-прав-на-сеть-с-использованием-govc).

### resourcePool {#resourcepool}

**Опциональный** параметр в `instanceClass`.

| Тип узлов | Поведение |
|-----------|-----------|
| **CloudPermanent** | При [`useNestedResourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool): `true` (по умолчанию) DKP автоматически создаёт вложенный resource pool в каждой зоне. Значение `resourcePool` в `instanceClass` переопределяет pool по умолчанию |
| **CloudEphemeral** | Если `resourcePool` указан явно в `VsphereInstanceClass` или настройках NodeGroup, соответствующий resource pool **должен уже существовать** во vSphere — machine-controller-manager не создаёт Resource Pool автоматически. При отсутствии pool с указанным путём создание ВМ завершится ошибкой |

{% alert level="warning" %}
Если `resourcePool` указан в `VsphereInstanceClass` или настройках NodeGroup, Resource Pool должен уже существовать во vSphere. Machine-controller-manager не создаёт Resource Pool автоматически. Если указанный Resource Pool не найден, MCM завершит заказ узла с ошибкой.

Это важно для сценария, когда разные NodeGroup размещают узлы в разных Resource Pool.
{% endalert %}

По умолчанию для ephemeral-узлов в облачном кластере используется `resourcePoolPath` из `VsphereCloudDiscoveryData`, созданный при развёртывании кластера.

## Диагностика проблем при заказе узлов {#troubleshooting-node-provisioning}

При сбоях заказа узлов проверьте следующее:

1. **Логи machine-controller-manager** (для узлов CloudEphemeral):

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

1. **Права сервисной учётной записи на сеть** — проверьте через [`govc permissions.ls`](#network-permissions-in-vsphere), что у учётной записи есть `Network.Assign` на port group из `mainNetwork`.

1. **Имя/путь сети в `mainNetwork`** — используйте формат пути относительно Datacenter; для имён с пробелами указывайте значение в кавычках в YAML (например, `mainNetwork: "PROD NET"`). См. [mainNetwork](#mainnetwork).

1. **Существование `resourcePool`** — если параметр задан в `VsphereInstanceClass`, убедитесь, что Resource Pool уже существует во vSphere.

1. **Разные механизмы создания permanent и ephemeral узлов** — узлы CloudPermanent создаются через Terraform (`dhctl`), CloudEphemeral — через machine-controller-manager. Проблема с сетью может проявляться только для одного типа узлов, даже если `mainNetwork` выглядит корректным для другого.

Дополнительная диагностика — в разделе [«Диагностика типичных проблем»](services.html#диагностика-типичных-проблем).

## Привилегии vSphere

Полный список необходимых привилегий, инструкции по созданию роли и варианты гранулярной модели прав описаны в разделе [«Подключение и авторизация»](authorization.html#список-необходимых-привилегий).
