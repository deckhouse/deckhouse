---
title: Интеграция со службами VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с инфраструктурой VMware vSphere и использует [ресурсы VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) для описания характеристик виртуальных машин, создаваемых в составе кластера Kubernetes.

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
DKP поддерживает гибридную интеграцию с VMware vSphere. Подробнее о настройке можно узнать в разделе [«Гибридный кластер с vSphere»](../../hybrid/vsphere-hybrid.html).
{% endalert %}

## Управление ресурсами vSphere

### Удаление CloudPermanent-узлов в vSphere

Узлы типа [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html) создаются на основании конфигурации групп узлов, заданной в секции [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) ресурса VsphereClusterConfiguration.

Параметр [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) определяет необходимое количество виртуальных машин в группе. После изменения конфигурации необходимо выполнить команду `dhctl converge` для запуска Terraform и актуализации состояния виртуальных машин в VMware vSphere в соответствии с указанным количеством реплик.

Чтобы уменьшить количество узлов в группе, уменьшите значение `replicas` и выполните `dhctl converge`.

{% alert level="warning" %}
Не удаляйте описание группы из секции [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups), пока значение параметра [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) больше нуля.

Если удалить описание группы до уменьшения количества реплик до нуля, состояние Terraform может рассинхронизироваться с состоянием виртуальных машин и дисков в VMware vSphere. В результате последующее выполнение `dhctl converge` может завершиться ошибкой и потребовать ручного восстановления состояния.
{% endalert %}

#### Уменьшение количества узлов

Чтобы уменьшить количество узлов в группе `CloudPermanent`:

1. Откройте конфигурацию vSphere для редактирования:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. В секции [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) найдите необходимую группу и уменьшите значение параметра [`replicas`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-replicas) до требуемого количества узлов.

   Например, чтобы уменьшить количество узлов в группе `worker` с трёх до двух, установите `replicas: 2`:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 2
     zones:
     - zone-a
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

   Сохраните изменения.

1. [В установочном контейнере DKP](/products/kubernetes-platform/documentation/v1/installing/#установка) примените изменённую конфигурацию:

   ```shell
   dhctl converge \
     --ssh-host <IP-АДРЕС_MASTER-УЗЛА> \
     --ssh-user <ИМЯ_ПОЛЬЗОВАТЕЛЯ> \
     --ssh-agent-private-keys /tmp/.ssh/<ИМЯ_ПРИВАТНОГО_SSH-КЛЮЧА>
   ```

   {% alert level="info" %}
   Используйте установочный контейнер той же редакции и версии, что и в кластере.
   {% endalert %}

1. Дождитесь завершения `dhctl converge` и проверьте количество узлов в группе:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<ИМЯ_ГРУППЫ>
   ```

#### Полное удаление группы узлов

Полное удаление группы `CloudPermanent` выполняется в два этапа. Сначала необходимо уменьшить количество реплик до нуля и дождаться удаления узлов и виртуальных машин. После этого можно удалить описание группы из конфигурации vSphere.

Чтобы полностью удалить группу `CloudPermanent`:

1. Откройте конфигурацию vSphere для редактирования:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. В секции [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups) найдите удаляемую группу и установите для неё значение `replicas: 0`. Не удаляйте описание группы на этом этапе. Например:

   ```yaml
   nodeGroups:
   - name: worker
     replicas: 0
     zones:
     - zone-a
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-24.04
       datastore: datastore-1
       mainNetwork: network-1
   ```

   Сохраните изменения.

1. [В установочном контейнере DKP](/products/kubernetes-platform/documentation/v1/installing/#установка) примените изменённую конфигурацию:

   ```shell
   dhctl converge \
     --ssh-host <IP-АДРЕС_MASTER-УЗЛА> \
     --ssh-user <ИМЯ_ПОЛЬЗОВАТЕЛЯ> \
     --ssh-agent-private-keys /tmp/.ssh/<ИМЯ_ПРИВАТНОГО_SSH-КЛЮЧА>
   ```

1. Дождитесь успешного завершения `dhctl converge` и убедитесь, что в группе не осталось узлов:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<ИМЯ_ГРУППЫ>
   ```

   Команда не должна возвращать узлы удаляемой группы.

   Также убедитесь с помощью vSphere Client, что связанные с группой виртуальные машины удалены.

1. Повторно откройте конфигурацию vSphere:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Удалите описание группы из секции `nodeGroups` и сохраните изменения.

1. Повторно примените конфигурацию:

   ```shell
   dhctl converge \
     --ssh-host <IP-АДРЕС_MASTER-УЗЛА> \
     --ssh-user <ИМЯ_ПОЛЬЗОВАТЕЛЯ> \
     --ssh-agent-private-keys /tmp/.ssh/<ИМЯ_ПРИВАТНОГО_SSH-КЛЮЧА>
   ```

1. Убедитесь, что объект NodeGroup удалён:

   ```shell
   d8 k get nodegroup <ИМЯ_ГРУППЫ>
   ```

   Команда должна завершиться сообщением о том, что запрашиваемый ресурс не найден.
