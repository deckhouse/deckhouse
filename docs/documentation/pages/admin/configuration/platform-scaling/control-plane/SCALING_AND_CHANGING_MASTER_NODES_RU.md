---
title: "Масштабирование и изменение master-узлов"
permalink: ru/admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html
lang: ru
---

## Масштабирование и переход single-master/multi-master

### Режимы работы control plane

Deckhouse Kubernetes Platform (DKP) поддерживает два режима работы control plane:

1. **Single-master**:
   - `kube-apiserver` использует только локальный экземпляр etcd;
   - на узле запускается прокси-сервер, который принимает запросы на `localhost`;
   - `kube-apiserver` "слушает" только на IP-адресе master-узла.

1. **Multi-master**:
   - `kube-apiserver` работает со всеми экземплярами etcd в кластере;
   - на всех узлах настраивается дополнительный прокси:
     - если локальный `kube-apiserver` недоступен, запросы автоматически переадресуются к другим узлам;
   - это обеспечивает отказоустойчивость и возможность масштабирования.

### Автоматическое масштабирование master-узлов

Deckhouse Kubernetes Platform (DKP) позволяет автоматически добавлять и удалять master-узлы, используя лейбл `node-role.kubernetes.io/control-plane=""`.

Автоматическое управление master-узлами:

- Добавление лейбла `node-role.kubernetes.io/control-plane=""` на узел:
  - разворачиваются все компоненты control plane;
  - узел подключается к etcd-кластеру;
  - автоматически регенерируются сертификаты и конфигурационные файлы.

- Удаление лейбла `node-role.kubernetes.io/control-plane=""` с узла:
  - компоненты control plane удаляются;
  - узел корректно исключается из etcd-кластера;
  - обновляются связанные конфигурационные файлы.

{% alert level="info" %}
Переход с 2 master-узлов до 1 требует ручной корректировки etcd. В остальных случаях изменение количества master-узлов выполняется автоматически.
{% endalert %}

### Типовые сценарии масштабирования

Deckhouse Kubernetes Platform (DKP) поддерживает автоматическое и ручное масштабирование master-узлов как в облачных, так и в bare-metal кластерах:

1. **Миграция single-master → multi-master**:

   - добавьте один или несколько новых master-узлов;
   - установите им лейбл `node-role.kubernetes.io/control-plane=""`;
   - DKP автоматически:
     - развернёт все компоненты control plane;
     - настроит узлы для работы с etcd-кластером;
     - синхронизирует сертификаты и конфигурационные файлы.

1. **Миграция multi-master → single-master**:

   - снимите лейблы `node-role.kubernetes.io/control-plane=""` и `node-role.kubernetes.io/master=""` со всех лишних master-узлов;
   - для **bare-metal кластеров**:
     - чтобы корректно исключить узлы из etcd:
       - выполните команду `d8 k delete node <имя-узла>`;
       - выключите соответствующие виртуальные машины или серверы.

{% alert level="warning" %}
В облачных кластерах все необходимые действия автоматически выполняются с помощью команды `dhctl converge`.
{% endalert %}

1. **Изменение числа master-узлов в облачном кластере**:

   - Аналогично добавлению/удалению узлов, но чаще всего выполняется с помощью команды `dhctl converge` или вручную через облачные инструменты.

{% alert level="warning" %}
Для стабильности кластера необходимо поддерживать нечётное число master-узлов для обеспечения кворума etcd.
{% endalert %}

### Удаление роли master с узла без удаления самого узла

Если необходимо вывести узел из состава master-узлов, но сохранить его в кластере для других задач, выполните следующие шаги:

1. Снимите лейблы, чтобы узел больше не рассматривался как master:

   ```bash
   d8 k label node <имя-узла> node-role.kubernetes.io/control-plane-
   d8 k label node <имя-узла> node-role.kubernetes.io/master-
   d8 k label node <имя-узла> node.deckhouse.io/group-
   ```

1. Удалите статические манифесты компонентов control plane, чтобы они больше не запускались на узле и лишние файлы PKI:

   ```bash
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Проверьте статус узла в etcd-кластере с помощью `etcdctl member list`.

   Пример:

   ```bash
   d8 k -n kube-system exec -ti \
   $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

После выполнения этих шагов узел больше не будет считаться master-узлом, но останется в кластере и может использоваться для других задач.

### Изменение образа ОС master-узлов в мультимастерном кластере

1. Сделайте [резервную копию etcd](../../backup/backup-and-restore.html#резервное-копирование-etcd) и директории `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь Deckhouse пуста.
1. **На локальной машине** запустите контейнер установщика DKP соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы проверить состояние перед началом работы:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Ответ должен сообщить, что Terraform не нашел расхождений и изменений не требуется.

1. **В контейнере с инсталлятором** выполните следующую команду и укажите необходимый образ ОС в параметре `masterNodeGroup.instanceClass` (укажите адреса всех master-узлов в параметре `--ssh-host`):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы провести обновление узлов:

   Внимательно изучите действия, которые планирует выполнить `converge`, когда запрашивает подтверждение.

   При выполнении команды узлы будут замены на новые с подтверждением на каждом узле. Замена будет выполняться по очереди в обратном порядке (2,1,0).

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Следующие действия (п. 9-12) **выполняйте поочередно на каждом** master-узле, начиная с узла с наивысшим номером (с суффиксом 2) и заканчивая узлом с наименьшим номером (с суффиксом 0).

1. **На созданном узле** откройте журнал systemd-юнита `bashible.service`. Дождитесь окончания настройки узла — в журнале должно появиться сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Проверьте, что узел etcd отобразился в списке узлов кластера:

   ```bash
   d8 k -n kube-system exec -ti \
   $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Убедитесь, что [`control-plane-manager`](/modules/control-plane-manager/) функционирует на узле.

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Перейдите к обновлению следующего узла.

### Изменение образа ОС в кластере с одним master-узлом

1. Преобразуйте кластер с одним master-узлом в мультимастерный в соответствии с [инструкцией](#добавление-master-узлов-в-облачном-кластере).
1. Обновите master-узлы в соответствии с [инструкцией](#изменение-образа-ос-master-узлов-в-мультимастерном-кластере).
1. Преобразуйте мультимастерный кластер в кластер с одним master-узлом в соответствии с [инструкцией](#уменьшение-числа-master-узлов-в-облачном-кластере).

## Добавление master-узлов в облачном кластере

Далее описана конвертация кластера с одним master-узлом в мультимастерный кластер.

{% alert level="warning" %}
Перед добавлением узлов убедитесь в наличии необходимых квот.
Важно иметь нечетное количество master-узлов для обеспечения кворума.
{% endalert %}

1. Сделайте [резервную копию etcd](../../backup/backup-and-restore.html#резервное-копирование-etcd) и директории `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать созданию новых master-узлов.
1. Убедитесь, что очередь Deckhouse пуста:

   ```shell
   d8 platform queue list
   ```

1. **На локальной машине** запустите контейнер установщика DKP соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы проверить состояние перед началом работы:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   Ответ должен сообщить, что Terraform не нашел расхождений и изменений не требуется.

1. **В контейнере с инсталлятором** выполните следующую команду и укажите требуемое количество master-узлов в параметре `masterNodeGroup.replicas`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

   > Для **Yandex Cloud**, при использовании внешних адресов на master-узлах, количество элементов массива в параметре `masterNodeGroup.instanceClass.externalIPAddresses` должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов), количество элементов в массиве все равно должно соответствовать количеству master-узлов.
   >
   > Например, при трех master-узлах (`masterNodeGroup.replicas: 3`) и автоматическом заказе адресов, параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > - "Auto"
   > - "Auto"
   > ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Дождитесь появления необходимого количества master-узлов в статусе `Ready` и готовности всех экземпляров [`control-plane-manager`](/modules/control-plane-manager/):

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

## Уменьшение числа master-узлов в облачном кластере

Далее описана конвертация мультимастерного кластера в кластер с одним master-узлом.

{% alert level="warning" %}
Описанные ниже шаги необходимо выполнять с первого по порядку master-узла кластера (`master-0`). Это связано с тем, что кластер всегда масштабируется по порядку: например, невозможно удалить узлы `master-0` и `master-1`, оставив `master-2`.
{% endalert %}

1. Сделайте [резервную копию etcd](../../backup/backup-and-restore.html#резервное-копирование-etcd) и директории `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет алертов, которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь DKP пуста:

   ```shell
   d8 platform queue list
   ```

1. **На локальной машине** запустите контейнер установщика DKP соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду и укажите `1` в параметре `masterNodeGroup.replicas`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   > Для **Yandex Cloud** при использовании внешних адресов на master-узлах количество элементов массива в параметре `masterNodeGroup.instanceClass.externalIPAddresses` должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов) количество элементов в массиве все равно должно соответствовать количеству master-узлов.
   >
   > Например, при одном master-узле (`masterNodeGroup.replicas: 1`) и автоматическом заказе адресов параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. Снимите следующие лейблы с удаляемых master-узлов:
   * `node-role.kubernetes.io/control-plane`;
   * `node-role.kubernetes.io/master`;
   * `node.deckhouse.io/group`.

   Команда для снятия лейблов:

   ```bash
   d8 k label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что удаляемые master-узлы пропали из списка узлов кластера etcd:

   ```bash
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Выполните `drain` для удаляемых узлов:

   ```bash
   d8 k drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальные машины, соответствующие удаляемым узлам, удалите инстансы соответствующих узлов из облака и подключенные к ним диски (`kubernetes-data-master-<N>`).

1. Удалите в кластере поды, оставшиеся на удаленных узлах:

   ```bash
   d8 k delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. Удалите в кластере объекты `Node` удаленных узлов:

   ```bash
   d8 k delete node <MASTER-NODE-N-NAME>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

### Доступ к контроллеру DKP в мультимастерном кластере

В кластерах с несколькими master-узлами DKP запускается в режиме высокой доступности (в нескольких экземплярах). Для доступа к активному контроллеру DKP можно использовать следующую команду (на примере команды `deckhouse-controller queue list`):

```shell
d8 platform queue list
```
