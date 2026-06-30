---
title: Интеграция со службами VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/services.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) интегрируется с инфраструктурой VMware vSphere и использует [ресурсы VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера Kubernetes.

Основные возможности:

- Заказ и удаление виртуальных машин через vCenter API;
- Размещение узлов кластера в разных кластерах ([`zones`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-zones)) и датацентрах ([`region`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-region));
- Использование шаблонов виртуальных машин с `cloud-init`;
- Поддержка сетей с DHCP, статической адресацией и дополнительными интерфейсами;
- Работа с хранилищем: заказ root-дисков и PVC на базе Datastore или CNS-дисков;
- Поддержка механизмов балансировки входящего трафика:
  - через внешние балансировщики;
  - через MetalLB (в режиме BGP).

{% alert level="info" %}
Для подключения vSphere к статическому кластеру см. раздел [«Гибридный кластер с vSphere»](../../hybrid/vsphere-hybrid.html).
{% endalert %}

## Типы узлов

В облачном кластере на vSphere узлы имеют тип [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) и управляются через секции [`masterNodeGroup`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup) и [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) ресурса `VsphereClusterConfiguration`.

## Параметры виртуальных машин

Параметры ВМ задаются в секции `instanceClass` ресурса `VsphereClusterConfiguration`:

| Параметр | Описание |
|----------|----------|
| [`numCPUs`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-numcpus) | Количество vCPU |
| [`memory`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-memory) | Объём RAM в МиБ |
| [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) | Путь к шаблону ВМ относительно Datacenter |
| [`datastore`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-datastore) | Путь к Datastore для root-диска |
| [`rootDiskSize`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-rootdisksize) | Размер root-диска в ГиБ (по умолчанию 20) |
| [`mainNetwork`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-mainnetwork) | Основная сеть (port group) с маршрутом по умолчанию. Путь относительно Datacenter — см. [«Сетевые параметры»](layout.html#сетевые-параметры) |
| [`additionalNetworks`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-additionalnetworks) | Дополнительные сетевые интерфейсы |
| [`resourcePool`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-resourcepool) | Resource Pool относительно зоны (vSphere Cluster). Для CloudEphemeral должен существовать заранее — см. [«Сетевые параметры»](layout.html#resourcepool) |
| [`mainNetworkIPAddresses`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masterinstanceclass-mainnetworkipaddresses) | Статические IP-адреса вместо DHCP (только в `VsphereClusterConfiguration`) |
| [`runtimeOptions`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-runtimeoptions) | Дополнительные параметры ВМ: CPU/memory shares, limits, nested virtualization |

Пример `instanceClass` для worker-группы:

```yaml
instanceClass:
  numCPUs: 4
  memory: 8192
  template: Templates/ubuntu-24.04
  datastore: lun10
  mainNetwork: net3-k8s
  rootDiskSize: 50
  additionalNetworks:
    - K8S_INTERNAL
  runtimeOptions:
    nestedHardwareVirtualization: false
```

{% alert %}
При использовании статических IP-адресов (`mainNetworkIPAddresses`) в образе ОС должен быть настроен интерфейс `ens192` — см. [«Подключение и авторизация»](authorization.html#требования-к-образу-виртуальной-машины).
{% endalert %}

### Размещение узлов по зонам

Список зон в секции `zones` группы узлов ограничивает, в каких vSphere Cluster могут создаваться ВМ. Узлы распределяются по зонам **в алфавитном порядке**: первый узел — в зоне с наименьшим именем, второй — в следующей и т.д. Если узлов больше, чем зон, распределение начинается сначала.

```yaml
nodeGroups:
- name: worker
  replicas: 4
  zones:
    - zone-a
    - zone-b
  instanceClass:
    # ...
```

В этом примере узлы будут размещены: `zone-a`, `zone-b`, `zone-a`, `zone-b`.

## Управление ресурсами vSphere

### Общий принцип

Изменения конфигурации узлов в облачном кластере на vSphere выполняются в два шага:

1. Отредактировать [`VsphereClusterConfiguration`](/modules/cloud-provider-vsphere/cluster_configuration.html):

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Применить изменения через `dhctl converge` [в установочном контейнере DKP](/products/kubernetes-platform/documentation/v1/installing/#установка) той же редакции и версии, что и кластер:

   ```shell
   dhctl converge \
     --ssh-host <IP-АДРЕС_MASTER-УЗЛА> \
     --ssh-user <ИМЯ_ПОЛЬЗОВАТЕЛЯ> \
     --ssh-agent-private-keys /tmp/.ssh/<ИМЯ_ПРИВАТНОГО_SSH-КЛЮЧА>
   ```

Команда `dhctl converge` запускает Terraform, который создаёт, изменяет или удаляет виртуальные машины в vSphere, выполняет bootstrap новых узлов и регистрирует их в кластере Kubernetes.

Проверить состояние Terraform перед применением:

```shell
dhctl terraform check \
  --ssh-host <IP-АДРЕС_MASTER-УЗЛА> \
  --ssh-user <ИМЯ_ПОЛЬЗОВАТЕЛЯ> \
  --ssh-agent-private-keys /tmp/.ssh/<ИМЯ_ПРИВАТНОГО_SSH-КЛЮЧА>
```

### Увеличение количества узлов

Чтобы добавить узлы в группу `CloudPermanent`:

1. Откройте конфигурацию vSphere:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Увеличьте значение [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) в нужной группе `nodeGroups`. Например, с 2 до 4:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 4
     zones:
     - zone-a
     - zone-b
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

1. Примените конфигурацию через `dhctl converge` (см. [выше](#общий-принцип)).

1. Дождитесь завершения и проверьте узлы:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=worker
   ```

   Новые виртуальные машины появятся в vSphere Client в папке, указанной в [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath).

### Добавление новой группы узлов

Чтобы создать новую группу worker-узлов (например, `frontend`):

1. Добавьте описание группы в секцию `nodeGroups`:

   ```yaml
   nodeGroups:
   - name: frontend
     replicas: 2
     zones:
     - zone-a
     instanceClass:
       numCPUs: 2
       memory: 4096
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
     nodeTemplate:
       labels:
         node-role.deckhouse.io/frontend: ""
       taints:
       - effect: NoExecute
         key: dedicated.deckhouse.io
         value: frontend
   ```

   Секция [`nodeTemplate`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-nodetemplate) задаёт labels и taints для узлов Kubernetes.

1. Примените конфигурацию через `dhctl converge`.

1. Убедитесь, что создан объект NodeGroup и узлы в состоянии `Ready`:

   ```shell
   d8 k get nodegroup frontend
   d8 k get nodes -l node.deckhouse.io/group=frontend
   ```

### Изменение параметров виртуальных машин

Параметры `instanceClass` (CPU, RAM, шаблон, datastore, сети) можно изменить в конфигурации и применить через `dhctl converge`.

{% alert level="warning" %}
Изменение аппаратных параметров (CPU, RAM, шаблон) или datastore для **существующих** узлов может потребовать пересоздания виртуальных машин. Рекомендуется:

1. Увеличить `replicas` на 1, применить `dhctl converge` — создастся узел с новыми параметрами.
1. Перенести нагрузку со старого узла (drain).
1. Уменьшить `replicas`, применить `dhctl converge` — лишний узел будет удалён.

Для изменения только `rootDiskSize` Terraform увеличит диск без пересоздания ВМ, если новый размер больше текущего.
{% endalert %}

### Управление master-узлами

Master-узлы настраиваются в секции [`masterNodeGroup`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup):

```yaml
masterNodeGroup:
  replicas: 3
  zones:
    - zone-a
    - zone-b
    - zone-c
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: Templates/ubuntu-24.04
    datastore: lun10
    mainNetwork: net3-k8s
```

{% alert level="warning" %}
Количество master-узлов (`replicas`) должно быть **нечётным** для обеспечения кворума etcd. После изменения `masterNodeGroup` обязательно выполните `dhctl converge`.
{% endalert %}

### Уменьшение количества узлов

Параметр [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) определяет целевое количество виртуальных машин в группе.

{% alert level="warning" %}
Не удаляйте описание группы из секции [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups), пока значение `replicas` больше нуля. Преждевременное удаление описания группы может рассинхронизировать состояние Terraform с виртуальными машинами в vSphere.
{% endalert %}

Чтобы уменьшить количество узлов:

1. Откройте конфигурацию: `d8 system edit provider-cluster-configuration`.
1. Уменьшите `replicas` до требуемого значения.
1. Примените `dhctl converge`.
1. Проверьте результат:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<ИМЯ_ГРУППЫ>
   ```

{% alert level="info" %}
Перед уменьшением количества узлов рекомендуется убедиться, что на удаляемых узлах нет критичных подов. При необходимости выполните [drain](#drain-cordon-и-обслуживание-узлов) вручную до применения `dhctl converge`.
{% endalert %}

### Полное удаление группы узлов

Удаление группы `CloudPermanent` выполняется в два этапа:

1. Установите `replicas: 0` и примените `dhctl converge`. Дождитесь удаления всех узлов и виртуальных машин.
1. Удалите описание группы из `nodeGroups` и повторно примените `dhctl converge`.

Пошаговая инструкция:

1. Откройте конфигурацию и установите `replicas: 0` для удаляемой группы. **Не удаляйте** описание группы на этом этапе.
1. Примените `dhctl converge`.
1. Убедитесь, что узлов не осталось:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<ИМЯ_ГРУППЫ>
   ```

   Проверьте в vSphere Client, что виртуальные машины удалены.
1. Удалите описание группы из `nodeGroups` и снова выполните `dhctl converge`.
1. Убедитесь, что NodeGroup удалён:

   ```shell
   d8 k get nodegroup <ИМЯ_ГРУППЫ>
   ```

## Drain, cordon и обслуживание узлов

### Ручной вывод узла из эксплуатации

Для планового обслуживания виртуальной машины в vSphere (миграция, обновление гипервизора, замена оборудования) выведите узел из планирования и эвакуируйте поды:

```shell
d8 k cordon <имя_узла>
d8 k drain <имя_узла> --ignore-daemonsets --delete-emptydir-data
```

После завершения работ верните узел в работу:

```shell
d8 k uncordon <имя_узла>
```

### Автоматический drain при обновлениях

При disruptive-обновлениях (обновление `containerd`, kubelet, перезагрузка) DKP может автоматически выполнять drain узла. Режим задаётся в [`disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) ресурса `NodeGroup`:

| Режим | Поведение |
|-------|-----------|
| `Automatic` | Drain выполняется автоматически перед обновлением (по умолчанию) |
| `Manual` | Требуется ручное подтверждение аннотацией `update.node.deckhouse.io/disruption-approved=` |

Для ручного подтверждения обновления:

```shell
d8 k annotate node <имя_узла> update.node.deckhouse.io/disruption-approved=
```

Таймаут drain настраивается параметром [`nodeDrainTimeoutSecond`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond) в NodeGroup (по умолчанию — 10 минут).

В процессе drain на узле появляются аннотации:

| Аннотация | Значение |
|-----------|----------|
| `update.node.deckhouse.io/draining` | Drain запрошен (значение — источник, например `bashible`) |
| `update.node.deckhouse.io/drained` | Drain завершён |

Подробнее — в разделе [«Основы управления узлами»](../../platform-scaling/node/node-management.html#обновления-требующие-прерывания-работы-узла).

## Мониторинг состояния узлов и групп

### Проверка узлов Kubernetes

```shell
# Все узлы кластера
d8 k get nodes -o wide

# Узлы конкретной группы
d8 k get nodes -l node.deckhouse.io/group=<ИМЯ_ГРУППЫ>

# Детальная информация об узле (адреса, taints, conditions)
d8 k describe node <имя_узла>
```

Cloud Controller Manager проставляет адреса узлов на основе сетей, указанных в [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) и [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames).

### Состояние NodeGroup

```shell
d8 k get nodegroup <ИМЯ_ГРУППЫ> -o yaml
```

Основные conditions ресурса NodeGroup:

| Condition | Значение `True` означает |
|-----------|--------------------------|
| `Ready` | В группе достаточно узлов в состоянии `Ready` |
| `Updating` | Идёт обновление хотя бы одного узла |
| `WaitingForDisruptiveApproval` | Ожидается ручное подтверждение disruptive-обновления |
| `Scaling` | Идёт масштабирование |
| `Error` | Ошибка при создании узла (подробности в `status.error`) |

### Проверка в vSphere {#checking-in-vsphere}

В vSphere Client виртуальные машины кластера размещаются в папке [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath). Имена ВМ формируются по шаблону `<префикс>-<имя_группы>-<индекс>`.

При проблемах с узлами проверьте:

- состояние ВМ в vSphere Client (powered on, VMware Tools running);
- доступность vCenter из кластера;
- логи компонентов:

  ```shell
  d8 k -n d8-cloud-provider-vsphere logs -l app=cloud-controller-manager --tail=50
  d8 k -n d8-cloud-instance-manager logs -l app=machine-controller-manager --tail=50
  ```

При сбоях заказа узлов см. [«Диагностика проблем при заказе узлов»](layout.html#troubleshooting-node-provisioning).

## Диагностика типичных проблем {#диагностика-типичных-проблем}

| Симптом | Возможная причина | Что проверить |
|---------|-------------------|---------------|
| `dhctl converge` завершается ошибкой | Недостаточно прав в vSphere, нехватка ресурсов | [Привилегии](authorization.html#список-необходимых-привилегий), свободное место на Datastore, Resource Pool |
| Сбой заказа узла (CloudEphemeral) | Права на сеть, неверный `mainNetwork`, отсутствует `resourcePool` | [логи machine-controller-manager](#проверка-в-vsphere), [`govc permissions.ls`](layout.html#network-permissions-in-vsphere), [`mainNetwork`](layout.html#mainnetwork), [`resourcePool`](layout.html#resourcepool) |
| Сбой заказа узла (только CloudPermanent) | Поиск сети в Terraform отличается от MCM | [`mainNetwork`](layout.html#mainnetwork), [узлы CloudPermanent и CloudEphemeral](layout.html#cloudpermanent-and-cloudephemeral-nodes) |
| Узел в `NotReady` | Проблемы с сетью или bootstrap | `cloud-init` логи на ВМ, доступность Kubernetes API |
| Неверные IP-адреса на узле | Неверно указаны сети | `externalNetworkNames`, `internalNetworkNames`, соответствие port group |
| ВМ создаётся, но не присоединяется к кластеру | Проблемы SSH/bootstrap | SSH-ключ в [`sshPublicKey`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-sshpublickey), сетевая связность |


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