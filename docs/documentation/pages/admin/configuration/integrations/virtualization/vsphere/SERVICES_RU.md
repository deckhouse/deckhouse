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

### Проверка в vSphere

В vSphere Client виртуальные машины кластера размещаются в папке [`vmFolderPath`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-vmfolderpath). Имена ВМ формируются по шаблону `<префикс>-<имя_группы>-<индекс>`.

При проблемах с узлами проверьте:

- состояние ВМ в vSphere Client (powered on, VMware Tools running);
- доступность vCenter из кластера;
- логи компонентов:

  ```shell
  d8 k -n d8-cloud-provider-vsphere logs -l app=cloud-controller-manager --tail=50
  d8 k -n d8-cloud-instance-manager logs -l app=machine-controller-manager --tail=50
  ```

## Диагностика типичных проблем

| Симптом | Возможная причина | Что проверить |
|---------|-------------------|---------------|
| `dhctl converge` завершается ошибкой | Недостаточно прав в vSphere, нехватка ресурсов | [Привилегии](authorization.html#список-необходимых-привилегий), свободное место на Datastore, Resource Pool |
| Узел в `NotReady` | Проблемы с сетью или bootstrap | `cloud-init` логи на ВМ, доступность Kubernetes API |
| Неверные IP-адреса на узле | Неверно указаны сети | `externalNetworkNames`, `internalNetworkNames`, соответствие port group |
| ВМ создаётся, но не присоединяется к кластеру | Проблемы SSH/bootstrap | SSH-ключ в [`sshPublicKey`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-sshpublickey), сетевая связность |
