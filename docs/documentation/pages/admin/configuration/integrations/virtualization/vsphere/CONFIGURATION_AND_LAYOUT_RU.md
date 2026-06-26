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

## Сетевые параметры

### mainNetwork

**Обязательный** параметр в `instanceClass`:

- для `masterNodeGroup` и `nodeGroups` ресурса `VsphereClusterConfiguration` (узлы [CloudPermanent](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html));
- для ресурса [`VsphereInstanceClass`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) при создании узлов [CloudEphemeral](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) — если не указан, используется значение из конфигурации master-узлов.

Задаёт port group, к которой подключается основной сетевой интерфейс виртуальной машины (маршрут по умолчанию).

**Формат значения** — путь к сети **относительно Datacenter** (не полный inventory path от корня vCenter):

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

{% alert level="warning" %}
Узлы CloudPermanent и CloudEphemeral создаются разными компонентами — Terraform (`dhctl`) и [machine-controller-manager](https://github.com/gardener/machine-controller-manager) соответственно. Оба принимают путь к сети относительно Datacenter, но механизм поиска объекта в vSphere Inventory отличается. Ошибки вида «network not found» при корректной конфигурации master-узлов могут проявляться только при создании ephemeral-узлов — проверьте `mainNetwork` в `VsphereInstanceClass` и права [`Network.Assign`](authorization.html#проверка-прав-на-сеть-с-использованием-govc) на целевую сеть. Подробнее о CloudEphemeral — в разделе [«Гибридный кластер с vSphere»](../../hybrid/vsphere-hybrid.html).
{% endalert %}

### internalNetworkCIDR

**Опциональный** параметр `VsphereClusterConfiguration`.

**Обязателен**, если в `masterNodeGroup.instanceClass` указан [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks). В этом случае Terraform назначает IP-адреса master-узлам из указанной подсети (начиная с десятого адреса: для `192.168.199.0/24` — с `192.168.199.10`).

Для worker-групп в `nodeGroups` параметр не используется.

### internalNetworkNames и externalNetworkNames

**Опциональные** параметры. Задаются в `VsphereClusterConfiguration` и/или в настройках модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/configuration.html) (`ModuleConfig`).

Используются компонентом `vsphere-cloud-controller-manager` для заполнения полей `InternalIP` и `ExternalIP` в `Node.status.addresses`.

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

### resourcePool

**Опциональный** параметр в `instanceClass`.

| Тип узлов | Поведение |
|-----------|-----------|
| **CloudPermanent** | При [`useNestedResourcePool`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-usenestedresourcepool): `true` (по умолчанию) DKP автоматически создаёт вложенный resource pool в каждой зоне. Значение `resourcePool` в `instanceClass` переопределяет pool по умолчанию |
| **CloudEphemeral** | Если `resourcePool` указан явно в `VsphereInstanceClass`, соответствующий resource pool **должен уже существовать** в vSphere — machine-controller-manager не создаёт его автоматически. При отсутствии pool с указанным путём создание ВМ завершится ошибкой |

По умолчанию для ephemeral-узлов в облачном кластере используется `resourcePoolPath` из `VsphereCloudDiscoveryData`, созданный при развёртывании кластера.

## Привилегии vSphere

Полный список необходимых привилегий, инструкции по созданию роли и варианты гранулярной модели прав описаны в разделе [«Подключение и авторизация»](authorization.html#список-необходимых-привилегий).
