---
title: Управление режимом HA
permalink: ru/admin/configuration/high-reliability-and-availability/enable.html
description: Управление режимом HA
lang: ru
---

{% alert level="info" %}
Обратите внимание, что если в кластере **более одного master-узла**, режим HA **включается автоматически**. Это справедливо как для развёртывания кластера сразу с тремя master-узлами, так и при увеличении количества master-узлов с одного до трёх.
{% endalert %}

## Глобальное включение режима HA

Включить режим отказоустойчивости для всей платформы DKP можно одним из следующих способов.

### Через кастомный ресурс ModuleConfig/global

1. Установите в `ModuleConfig/global` [параметр `settings.highAvailability`](../../../reference/api/global.html#parameters-highavailability) в значение `true`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: global
   spec:
     version: 2
     settings: 
       highAvailability: true
   ```

1. Убедитесь, что режим включился. Например, проверьте количество подов `deckhouse` в пространстве имён `d8-system`, выполнив команду:

   ```shell
   d8 k -n d8-system get po | grep deckhouse
   ```

   Количество подов `deckhouse` должно быть больше одного, как показано в примере вывода ниже:

   ```text
   deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
   deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
   deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
   ```

### Через веб-интерфейс Deckhouse

Если в кластере включен модуль [`console`](/modules/console/), откройте веб-интерфейс Deckhouse, перейдите в раздел «Deckhouse» — «Глобальные настройки» — «Глобальные настройки модулей» и установите переключатель «Режим отказоустойчивости» в положение «Да».

## Настройка режима HA c двумя master-узлами и arbiter-узлом

В Deckhouse Kubernetes Platform возможна настройка режима HA c двумя master-узлами и arbiter-узлом. Такой подход позволяет обеспечить требования по HA в условиях ограниченных ресурсов.

На arbiter-узле размещается только etcd, без остальных компонентов control plane. Этот узел используется для обеспечения кворума etcd.

Требования к arbiter-узлу:

* не менее 2 ядер CPU;
* не менее 4 ГБ RAM;
* не менее 8 ГБ дискового пространства под etcd.

Требования к сетевым задержкам для arbiter-узла аналогичны требованиям для master-узлов.

### Настройка в облачном кластере

Пример ниже актуален для облачного кластера с тремя master-узлами.
Чтобы настроить режим HA c двумя master-узлами и arbiter-узлом в облачном кластере, необходимо удалить из кластера один master-узел и добавить один arbiter-узел.

Для этого выполните следующие действия:

{% alert level="warning" %}
- Описанные ниже шаги необходимо выполнять с первого по порядку master-узла кластера (`master-0`). Это связано с тем, что кластер всегда масштабируется по порядку: например, невозможно удалить узлы `master-0` и `master-1`, оставив `master-2`.

- Если в кластере используется модуль [`stronghold`](/modules/stronghold/), перед добавлением или удалением master-узла убедитесь, что модуль находится в полностью работоспособном состоянии. Перед началом любых изменений рекомендуется создать [резервную копию данных модуля](/modules/stronghold/auto_snapshot.html).  
{% endalert %}

1. Сделайте [резервную копию etcd](../backup/backup-and-restore.html#резервное-копирование-etcd) и директории `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь DKP пуста:

   ```shell
   d8 system queue list
   ```

1. **На локальной машине** запустите контейнер установщика DKP соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.ru/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   Измените настройки облачного провайдера:

   * В параметре `masterNodeGroup.replicas` укажите `2`.
   * Создайте NodeGroup для arbiter-узла. На arbiter-узле **обязательно** должен быть лейбл `node-role.deckhouse.io/etcd-only: ""` и taint, предотвращающий размещение на нем пользовательской нагрузки. Пример описания NodeGroup для arbiter-узла:

     ```yaml
     nodeGroups:
       - name: arbiter
         replicas: 1
         nodeTemplate:
           labels:
             node.deckhouse.io/etcd-arbiter: ""
           taints:
             - key: node.deckhouse.io/etcd-arbiter
               effect: NoSchedule
         zones:
           - europe-west3-b
         instanceClass:
           machineType: n1-standard-4
       # ... остальная часть манифеста
     ```

   * Сохраните изменения.

   > Для **Yandex Cloud** при использовании внешних адресов на master-узлах количество элементов массива в параметре `masterNodeGroup.instanceClass.externalIPAddresses` должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов) количество элементов в массиве все равно должно соответствовать количеству master-узлов.
   >
   > Например, при одном master-узле (`masterNodeGroup.replicas: 1`) и автоматическом заказе адресов параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST>
   ```

   > **Важно**. Для **OpenStack** и **VK Cloud (OpenStack)** после подтверждения удаления узла обязательно проверьте удаление диска `<prefix>kubernetes-data-N` в самом OpenStack.
   >
   > Например, при удалении узла `cloud-demo-master-2` в веб-интерфейсе OpenStack или в OpenStack CLI необходимо проверить отсутствие диска `cloud-demo-kubernetes-data-2`.
   >
   > В случае, если диск `kubernetes-data` останется, при увеличении количества master-узлов могут возникнуть проблемы в работе etcd.

1. Проверьте очередь Deckhouse с помощью следующей команды и убедитесь, что отсутствуют ошибки:

   ```shell
   d8 system queue list
   ```

### Настройка в статическом кластере

Чтобы настроить режим HA c двумя master-узлами и arbiter-узлом в статическом кластере, выполните следующие действия:

1. Создайте NodeGroup для arbiter-узла. На arbiter-узле **обязательно** должен быть лейбл `node-role.deckhouse.io/etcd-only: ""` и taint, предотвращающий размещение на нем пользовательской нагрузки. Пример описания NodeGroup для arbiter-узла:

   ```yaml
   apiVersion: deckhouse.io/v1
     kind: NodeGroup
     metadata:
       name: arbiter
     spec:
       nodeType: Static
       nodeTemplate:
         labels:
           node.deckhouse.io/etcd-arbiter: ""
         taints:
           - key: node.deckhouse.io/etcd-arbiter
             effect: NoSchedule
     # ... остальная часть манифеста
   ```

1. Добавьте [удобным вам способом](../platform-scaling/node/bare-metal-node.html#добавление-узлов-в-bare-metal-кластере) в кластер узел, который будет использовать как arbiter-узел.
1. [Убедитесь](/modules/control-plane-manager/faq.html#как-посмотреть-список-узлов-кластера-в-etcd), что добавленный arbiter-узел находится в списке членов кластера etcd.
1. [Удалите](../platform-scaling/control-plane/scaling-and-changing-master-nodes.html#удаление-роли-master-с-узла-без-удаления-самого-узла) один master-узел из кластера.

## Включение режима HA для отдельных компонентов

Некоторые модули DKP могут иметь собственные настройки режима HA. Чтобы включить режим высокой надежности в конкретном модуле, установите параметр `settings.highAvailability` в его настройках. При этом работа режима HA в отдельных модулях не зависит от состояния глобального режима HA.

Перечень модулей, для которых доступно управление режимом HA:

* [`deckhouse`](/modules/deckhouse/);
* [`openvpn`](/modules/openvpn/);
* [`istio`](/modules/istio/);
* [`dashboard`](/modules/dashboard/);
* [`multitenancy-manager`](/modules/multitenancy-manager/);
* [`user-authn`](/modules/user-authn/);
* [`ingress-nginx`](/modules/ingress-nginx/);
* [`prometheus-monitoring`](/modules/prometheus/);
* [`monitoring-kubernetes`](/modules/monitoring-kubernetes/);
* [`snapshot-controller`](/modules/snapshot-controller/).

Чтобы вручную включить режим HA в конкретном модуле, добавьте в его конфигурацию параметр `settings.highAvailability`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    highAvailability: true
```

Чтобы убедиться, что режим включился, проверьте количество подов выбранного модуля. Например, для проверки работы режима в модуле `deckhouse`, проверьте количество подов в пространстве имён `d8-system`, выполнив следующую команду:

```shell
d8 k -n d8-system get po | grep deckhouse
```

Количество подов `deckhouse` должно быть больше одного:

```text
deckhouse-57695f4d68-8rk6l                           2/2     Running   0             3m49s
deckhouse-5764gfud68-76dsb                           2/2     Running   0             3m49s
deckhouse-fgrhy4536s-fhu6s                           2/2     Running   0             3m49s
```
