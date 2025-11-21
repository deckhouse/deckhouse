---
title: "Управление control plane: FAQ"
---

<div id='как-добавить-master-узел'></div>

## Как добавить master-узел в статическом или гибридном кластере?

> Важно иметь нечетное количество master-узлов для обеспечения кворума.

В процессе установки Deckhouse Kubernetes Platform с настройками по умолчанию в NodeGroup `master` отсутствует секция [`spec.staticInstances.labelSelector`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) с настройками фильтра меток (label) по ресурсам `staticInstances`. Из-за этого после изменения количества узлов `staticInstances` в NodeGroup `master` (параметр [`spec.staticInstances.count`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)) при добавлении обычного узла с помощью Cluster API Provider Static (CAPS) он может быть «перехвачен» и добавлен в NodeGroup `master`, даже если в соответствующем ему `StaticInstance` (в `metadata`) указан лейбл с `role`, отличающейся от `master`.
Чтобы избежать этого «перехвата», после установки DKP измените NodeGroup `master` — добавьте в нее секцию [`spec.staticInstances.labelSelector`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-labelselector) с настройками фильтра меток (label) по ресурсам `staticInstances`. Пример NodeGroup `master` с `spec.staticInstances.labelSelector`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  staticInstances:
    count: 2
    labelSelector:
      matchLabels:
        role: master
```

Далее при добавлении в кластер master-узлов с помощью CAPS указывайте в соответствующих им `StaticInstance` лейбл, заданный в `spec.staticInstances.labelSelector` NodeGroup `master`. Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: StaticInstance
metadata:
  name: static-master-1
  labels:
    # Лейбл, указанный в spec.staticInstances.labelSelector NodeGroup master.
    role: master
spec:
  # Укажите IP-адрес сервера статического узла.
  address: "<SERVER-IP>"
  credentialsRef:
    kind: SSHCredentials
    name: credentials
```

{% alert level="info" %}
При добавлении новых master-узлов с помощью CAPS и изменении в NodeGroup `master` количества master-узлов (параметр [`spec.staticInstances.count`](../node-manager/cr.html#nodegroup-v1-spec-staticinstances-count)) учитывайте следующее:

При бутстрапе кластера в конфигурации указывается первый master-узел, на который происходит установка.
Если после бутстрапа нужно сделать мультимастер и добавить master-узлы с помощь CAPS, в параметре `spec.staticInstances.count` NodeGroup `master` необходимо указать количество узлов на один меньше желаемого.

Например, если нужно сделать мультимастер с тремя master-узлами в `spec.staticInstances.count` NodeGroup `master` укажите значение `2` и создайте два `staticInstances` для добавляемых узлов. После их добавления в кластер количество master-узлов будет равно трём: master-узел, на который происходила установка и два master-узла, добавленные с помощью CAPS.
{% endalert %}

В остальном добавление master-узла в статический или гибридный кластер аналогично добавлению обычного узла.
Воспользуйтесь для этого соответствующими [примерами](../node-manager/examples.html#добавление-статического-узла-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

<div id='как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master'></div>

## Как добавить master-узлы в облачном кластере?

Далее описана конвертация кластера с одним master-узлом в мультимастерный кластер.

> Перед добавлением узлов убедитесь в наличии необходимых квот.
>
> Важно иметь нечетное количество master-узлов для обеспечения кворума.

{% alert level="warning" %}
Если в кластере используется модуль [`stronghold`](/modules/stronghold/), перед добавлением или удалением master-узла убедитесь, что модуль находится в полностью работоспособном состоянии. Перед началом любых изменений настоятельно рекомендуется создать [резервную копию данных модуля](/modules/stronghold/auto_snapshot.html).  
{% endalert %}

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать созданию новых master-узлов.
1. Убедитесь, что очередь Deckhouse пуста:

   ```shell
   d8 system queue list
   ```

1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.ru/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
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

   > Для **Yandex Cloud**, при использовании внешних адресов на master-узлах, количество элементов массива в параметре [masterNodeGroup.instanceClass.externalIPAddresses](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalipaddresses) должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов), количество элементов в массиве все равно должно соответствовать количеству master-узлов.
   >
   > Например, при трех master-узлах (`masterNodeGroup.replicas: 3`) и автоматическом заказе адресов, параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
   >
   > ```bash
   > externalIPAddresses:
   > - "Auto"
   > - "Auto"
   > - "Auto"
   > ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Дождитесь появления необходимого количества master-узлов в статусе `Ready` и готовности всех экземпляров `control-plane-manager`:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

<div id='как-удалить-master-узел'></div>
<div id='как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master'></div>

## Как уменьшить число master-узлов в облачном кластере?

Далее описана конвертация мультимастерного кластера в кластер с одним master-узлом.

{% alert level="warning" %}
Описанные ниже шаги необходимо выполнять с первого по порядку master-узла кластера (master-0). Это связано с тем, что кластер всегда масштабируется по порядку: например, невозможно удалить узлы master-0 и master-1, оставив master-2.
{% endalert %}

{% alert level="warning" %}
Если в кластере используется модуль [`stronghold`](/modules/stronghold/), перед добавлением или удалением master-узла убедитесь, что модуль находится в полностью работоспособном состоянии. Перед началом любых изменений настоятельно рекомендуется создать [резервную копию данных модуля](/modules/stronghold/auto_snapshot.html).  
{% endalert %}

1. Сделайте [резервную копию etcd](../../backup/backup-and-restore.html#резервное-копирование-etcd) и директории `/etc/kubernetes`.
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

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```
   > Для **OpenStack** и **VKCloud(OpenStack)** после подтверждении удалении ноды крайне важно проверить удаление диска `<prefix>kubernetes-data-N` в самом Openstack.
   >
   > Например, при удалении ноды `cloud-demo-master-2` в веб-интерфейсе Openstack или в `OpenStack CLI` необходимо проверить отсутствие диска `cloud-demo-kubernetes-data-2`.
   >
   > В случае, если диск `kubernetes-data` останется, при увеличении количества master-узлов могут возникнуть проблемы в работе ETCD.

1. Выполните проверку очереди Deckhouse и убедитесь, что отсутствуют ошибки командой:

   ```shell
   d8 system queue list
   ```

## Как убрать роль master-узла, сохранив узел?

1. Сделайте [резервную копию etcd](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь Deckhouse пуста:

   ```shell
   d8 system queue list
   ```

1. Снимите следующие лейблы:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Команда для снятия лейблов:

   ```bash
   d8 k label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что удаляемый master-узел пропал из списка узлов кластера:

   ```bash
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

1. Зайдите на узел и выполните следующие команды:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

<div id='как-изменить-образ-ос-в-multi-master-кластере'></div>

## Как изменить образ ОС в мультимастерном кластере?

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что очередь Deckhouse пуста:

   ```shell
   d8 system queue list
   ```

1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.ru/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
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

   Внимательно изучите действия, которые планирует выполнить converge, когда запрашивает подтверждение.

   При выполнении команды узлы будут замены на новые с подтверждением на каждом узле. Замена будет выполняться по очереди в обратном порядке (2,1,0).

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Следующие действия (П. 9-12) **выполняйте поочередно на каждом** master-узле, начиная с узла с наивысшим номером (с суффиксом 2) и заканчивая узлом с наименьшим номером (с суффиксом 0).

1. **На созданном узле** откройте журнал systemd-юнита `bashible.service`. Дождитесь окончания настройки узла — в журнале должно появиться сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Проверьте, что узел etcd отобразился в списке узлов кластера:

   ```bash
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

1. Убедитесь, что `control-plane-manager` функционирует на узле.

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Перейдите к обновлению следующего узла.

<div id='как-изменить-образ-ос-в-single-master-кластере'></div>

## Как изменить образ ОС в кластере с одним master-узлом?

1. Преобразуйте кластер с одним master-узлом в мультимастерный в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master).
1. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ос-в-multi-master-кластере).
1. Преобразуйте мультимастерный кластер в кластер с одним master-узлом в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере)

<div id='как-посмотреть-список-memberов-в-etcd'></div>

## Как посмотреть список узлов кластера в etcd?

### Вариант 1

Используйте команду `etcdctl member list`.

Пример:

```shell
for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
  d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ member list -w table
  if [ $? -eq 0 ]; then
    break
  fi
done
```

**Внимание.** Последний параметр в таблице вывода показывает, что узел находится в состоянии [`learner`](https://etcd.io/docs/v3.5/learning/design-learner/), а не в состоянии `leader`.

### Вариант 2

Используйте команду `etcdctl endpoint status`. Для лидера в столбце `IS LEADER` будет указано значение `true`.

Пример:

```shell
for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
  d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ endpoint status --cluster -w table
  if [ $? -eq 0 ]; then
    break
  fi
done
```

## Что делать, если что-то пошло не так?

В процессе работы `control-plane-manager` автоматически создает резервные копии конфигурации и данных, которые могут пригодиться в случае возникновения проблем. Эти резервные копии сохраняются в директории `/etc/kubernetes/deckhouse/backup`. Если в процессе работы возникли ошибки или непредвиденные ситуации, вы можете использовать эти резервные копии для восстановления до предыдущего исправного состояния.

<div id='что-делать-если-кластер-etcd-развалился'></div>

## Что делать, если кластер etcd не функционирует?

Если кластер etcd не функционирует и не удается восстановить его из резервной копии, вы можете попытаться восстановить его с нуля, следуя шагам ниже.

1. Сначала на всех узлах, которые являются частью вашего кластера etcd, кроме одного, удалите манифест `etcd.yaml`, который находится в директории `/etc/kubernetes/manifests/`. После этого только один узел останется активным, и с него будет происходить восстановление состояния мультимастерного кластера.
1. На оставшемся узле откройте файл манифеста `etcd.yaml` и укажите параметр `--force-new-cluster` в `spec.containers.command`.
1. После успешного восстановления кластера, удалите параметр `--force-new-cluster`.

 {% alert level="warning" %}
 Эта операция является деструктивной, так как она полностью уничтожает текущие данные и инициализирует кластер с состоянием, которое сохранено на узле. Все pending-записи будут утеряны.
 {% endalert %}

### Что делать, если etcd постоянно перезапускается с ошибкой?

Этот способ может понадобиться, если использование параметра `--force-new-cluster` не восстанавливает работу etcd. Это может произойти, если converge master-узлов прошел неудачно, в результате чего новый master-узел был создан на старом диске etcd, изменил свой адрес в локальной сети, а другие master-узлы отсутствуют. Этот метод стоит использовать если контейнер etcd находится в бесконечном цикле перезапуска, а в его логах появляется ошибка: `panic: unexpected removal of unknown remote peer`.

1. Установите утилиту [etcdutl](https://github.com/etcd-io/etcd/releases).
1. С текущего локального снапшота базы etcd (`/var/lib/etcd/member/snap/db`) выполните создание нового снапшота:

   ```shell
   ./etcdutl snapshot restore /var/lib/etcd/member/snap/db --name <HOSTNAME> \
   --initial-cluster=HOSTNAME=https://<ADDRESS>:2380 --initial-advertise-peer-urls=https://ADDRESS:2380 \
   --skip-hash-check=true --data-dir /var/lib/etcdtest
   ```

   * `<HOSTNAME>` — название master-узла;
   * `<ADDRESS>` — адрес master-узла.

1. Выполните следующие команды для использования нового снапшота:

   ```shell
   cp -r /var/lib/etcd /tmp/etcd-backup
   rm -rf /var/lib/etcd
   mv /var/lib/etcdtest /var/lib/etcd
   ```

1. Найдите контейнеры `etcd` и `api-server`:

   ```shell
   crictl ps -a | egrep "etcd|apiserver"
   ```

1. Удалите найденные контейнеры `etcd` и `api-server`:

   ```shell
   crictl rm <CONTAINER-ID>
   ```

1. Перезапустите master-узел.

### Что делать, если объем базы данных etcd достиг лимита, установленного в quota-backend-bytes?

Когда объем базы данных etcd достигает лимита, установленного параметром `quota-backend-bytes`, доступ к ней становится "read-only". Это означает, что база данных etcd перестает принимать новые записи, но при этом остается доступной для чтения данных. Вы можете понять, что столкнулись с подобной ситуацией, выполнив команду:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | sed -n 1p) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ endpoint status -w table --cluster
   ```

Если в поле `ERRORS` вы видите подобное сообщение `alarm:NOSPACE`, значит вам нужно предпринять следующие шаги:

1. Найдите строку с `--quota-backend-bytes` в файле манифеста пода etcd, расположенного по пути `/etc/kubernetes/manifests/etcd.yaml` и увеличьте значение, умножив указанный параметр в этой строке на два. Если такой строки нет — добавьте, например: `- --quota-backend-bytes=8589934592`. Эта настройка задает лимит на 8 ГБ.

1. Сбросьте активное предупреждение (alarm) о нехватке места в базе данных. Для этого выполните следующую команду:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | sed -n 1p) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Измените параметр [maxDbSize](configuration.html#parameters-etcd-maxdbsize) в настройках `control-plane-manager` на тот, который был задан в манифесте.

## Как выполнить дефрагментацию etcd

{% alert level="warning" %}
Перед дефрагментацией [создайте резервную копию etcd](#как-сделать-резервную-копию-etcd-вручную).
{% endalert %}

Для просмотра размера БД etcd на определенном узле перед дефрагментацией и после ее выполнения используйте команду (здесь `NODE_NAME` — имя master-узла):

```bash
d8 k -n kube-system exec -it etcd-NODE_NAME -- /usr/bin/etcdctl \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  endpoint status --cluster -w table
```

Пример вывода (размер БД etcd на узле указывается в колонке `DB SIZE`):

```console
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
|          ENDPOINT           |        ID        | VERSION | STORAGE VERSION | DB SIZE | IN USE | PERCENTAGE NOT IN USE | QUOTA  | IS LEADER  | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS | DOWNGRADE TARGET VERSION | DOWNGRADE ENABLED |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.80:2379 | 489a8af1e7acd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |       true |      false |        56 |  258054684 |          258054684 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.81:2379 | 589a8ad1e7ccd7b0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
| https://192.168.199.82:2379 | 229a8cd1e7bcd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
+-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
```

### Как выполнить дефрагментацию etcd узла в кластере с одним master-узлом

{% alert level="warning" %}
Дефрагментация etcd — ресурсоемкая операция, которая на время полностью блокирует работу etcd на данном узле.
Учитывайте это при выборе времени для проведения операции в кластере с одним master-узлом.
{% endalert %}

Чтобы выполнить дефрагментацию etcd в кластере с одним master-узлом, используйте следующую команду (здесь `NODE_NAME` — имя master-узла):

```bash
d8 k -n kube-system exec -ti etcd-NODE_NAME -- /usr/bin/etcdctl \
  --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key \
  --endpoints https://127.0.0.1:2379/ defrag --command-timeout=30s
```

Пример вывода при успешном выполнении операции:

```console
Finished defragmenting etcd member[https://localhost:2379]. took 848.948927ms
```

> При появлении ошибки из-за таймаута увеличивайте значение параметра `–command-timeout` из команды выше, пока дефрагментация не выполнится успешно.

### Как выполнить дефрагментацию etcd в кластере с несколькими master-узлами

Чтобы выполнить дефрагментацию etcd в кластере с несколькими master-узлами:

1. Получите список подов etcd. Для этого используйте следующую команду:

   ```bash
   d8 k -n kube-system get pod -l component=etcd -o wide
   ```

   Пример вывода:

   ```console
   NAME           READY    STATUS    RESTARTS   AGE     IP              NODE        NOMINATED NODE   READINESS GATES
   etcd-master-0   1/1     Running   0          3d21h   192.168.199.80  master-0    <none>           <none>
   etcd-master-1   1/1     Running   0          3d21h   192.168.199.81  master-1    <none>           <none>
   etcd-master-2   1/1     Running   0          3d21h   192.168.199.82  master-2    <none>           <none>
   ```

1. Определите master-узел — лидер. Для этого обратитесь к любому поду etcd и получите список узлов — участников кластера etcd с помощью команды (где `NODE_NAME` — имя master-узла):

   ```bash
   d8 k -n kube-system exec -it etcd-NODE_NAME -- /usr/bin/etcdctl \
     --cert=/etc/kubernetes/pki/etcd/server.crt \
     --key=/etc/kubernetes/pki/etcd/server.key \
     --cacert=/etc/kubernetes/pki/etcd/ca.crt \
     endpoint status --cluster -w table
   ```

   Пример вывода (у лидера в колонке `IS LEADER` будет значение `true`):

   ```console
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   |          ENDPOINT           |        ID        | VERSION | STORAGE VERSION | DB SIZE | IN USE | PERCENTAGE NOT IN USE | QUOTA  | IS LEADER  | IS LEARNER | RAFT TERM | RAFT INDEX | RAFT APPLIED INDEX | ERRORS | DOWNGRADE TARGET VERSION | DOWNGRADE ENABLED |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.80:2379 | 489a8af1e7acd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |       true |      false |        56 |  258054684 |          258054684 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.81:2379 | 589a8ad1e7ccd7b0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   | https://192.168.199.82:2379 | 229a8cd1e7bcd7a0 |   3.6.1 |           3.6.0 |   76 MB |  62 MB |                   20% | 2.1 GB |      false |      false |        56 |  258054685 |          258054685 |        |                          |             false |
   +-----------------------------+------------------+---------+-----------------+---------+--------+-----------------------+--------+------------+------------+-----------+------------+--------------------+--------+--------------------------+-------------------+
   ```

1. Поочередно выполните дефрагментацию etcd узлов — участников etcd кластера. Для дефрагментации используйте команду (здесь `NODE_NAME` — имя master-узла):

   > Важно: дефрагментацию лидера необходимо выполнять в последнюю очередь.
   >
   > Восстановление etcd на узле после дефрагментации может занять некоторое время. Рекомендуется подождать не менее минуты прежде чем переходить к дефрагментации etcd следующего узла.

   ```bash
   d8 k -n kube-system exec -ti etcd-NODE_NAME -- /usr/bin/etcdctl \
     --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ defrag --command-timeout=30s
   ```

   Пример вывода при успешном выполнении операции:

   ```console
   Finished defragmenting etcd member[https://localhost:2379]. took 848.948927ms
   ```

   > При появлении ошибки из-за таймаута увеличивайте значение параметра `–command-timeout` из команды выше, пока дефрагментация не выполнится успешно.

## Как настроить дополнительные политики аудита?

1. Включите параметр [auditPolicyEnabled](configuration.html#parameters-apiserver-auditpolicyenabled) в настройках модуля:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     settings:
       apiserver:
         auditPolicyEnabled: true
   ```

2. Создайте Secret `kube-system/audit-policy` с YAML-файлом политик, закодированным в Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <base64>
   ```

   Минимальный рабочий пример `audit-policy.yaml` выглядит так:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   С подробной информацией по настройке содержимого файла `audit-policy.yaml` можно ознакомиться:
   * [В официальной документации Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy);
   * [В статье на Habr](https://habr.com/ru/company/flant/blog/468679/);
   * [В коде скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

### Как исключить встроенные политики аудита?

Установите параметр [apiserver.basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) модуля в `false`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      basicAuditPolicyEnabled: false
```

### Как вывести аудит-лог в стандартный вывод вместо файлов?

Установите параметр [apiserver.auditLog.output](configuration.html#parameters-apiserver-auditlog) модуля в значение `Stdout`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      auditLog:
        output: Stdout
```

### Как работать с журналом аудита?

Предполагается, что на master-узлах установлен «скрейпер логов»: [log-shipper](../log-shipper/cr.html#clusterloggingconfig), `promtail`, `filebeat`,  который будет мониторить файл с логами:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации логов в файле журнала предустановлены и их изменение не предусмотрено:

* Максимальное занимаемое место на диске `1000 МБ`.
* Максимальная глубина записи `30 дней`.

В зависимости от настроек политики (`Policy`) и количества запросов к `apiserver` логов может быть очень много, соответственно глубина хранения может быть менее 30 минут.

{% alert level="warning" %}
Текущая реализация функционала не гарантирует безопасность, так как существует риск временного нарушения работы control plane.

Если в Secret'е с конфигурационным файлом окажутся неподдерживаемые опции или опечатка, `apiserver` не сможет запуститься.
{% endalert %}

В случае возникновения проблем с запуском `apiserver`, потребуется вручную отключить параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить `apiserver` следующей командой:

```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
# Или (в зависимости используемого вами CRI).
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

После перезапуска будет достаточно времени исправить Secret или удалить его:

```bash
d8 k -n kube-system delete secret audit-policy
```

## Как ускорить перезапуск подов при потере связи с узлом?

По умолчанию, если узел в течении 40 секунд не сообщает свое состояние, он помечается как недоступный. И еще через 5 минут поды узла начнут перезапускаться на других узлах.  В итоге общее время недоступности приложений составляет около 6 минут.

В специфических случаях, когда приложение не может быть запущено в нескольких экземплярах, есть способ сократить период их недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой параметра `nodeMonitorGracePeriodSeconds`.
1. Установить меньший таймаут удаления подов с недоступного узла в параметре `failedNodePodEvictionTimeoutSeconds`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

В этом случае при потере связи с узлом приложения будут перезапущены примерно через 1 минуту.

Оба упомянутых параметра напрямую влияют на использование процессора и памяти control-plane'ом. Снижая таймауты, системные компоненты чаще отправляют статусы и проверяют состояние ресурсов.

При выборе оптимальных значений учитывайте графики использования ресурсов управляющих узлов. Чем меньше значения параметров, тем больше ресурсов может понадобиться для их обработки на этих узлах.

## Резервное копирование и восстановление etcd

### Что выполняется автоматически

Автоматически запускаются CronJob `kube-system/d8-etcd-backup-*` в 00:00 по UTC+0. Результат сохраняется в `/var/lib/etcd/etcd-backup.tar.gz` на всех узлах с `control-plane` в кластере (master-узлы).

<div id='как-сделать-бэкап-etcd-вручную'></div>

### Как сделать резервную копию etcd вручную

#### Используя Deckhouse CLI (Deckhouse Kubernetes Platform v1.65+)

Начиная с релиза Deckhouse Kubernetes Platform v1.65, стала доступна утилита `d8 backup etcd`, которая предназначена для быстрого создания снимков состояния etcd.

```bash
d8 backup etcd ./etcd-backup.snapshot
```

#### Используя bash (Deckhouse Kubernetes Platform v1.64 и старше)

Войдите на любой control-plane узел под пользователем `root` и используйте следующий bash-скрипт:

```bash
#!/usr/bin/env bash
set -e

pod=etcd-`hostname`
d8 k -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
mv /var/lib/etcd/"${pod##*/}.snapshot" etcd-backup.snapshot && \
cp -r /etc/kubernetes/ ./ && \
tar -cvzf kube-backup.tar.gz ./etcd-backup.snapshot ./kubernetes/
rm -r ./kubernetes ./etcd-backup.snapshot
```

В текущей директории будет создан файл `kube-backup.tar.gz` со снимком базы etcd одного из узлов кластера.
Из полученного снимка можно будет восстановить состояние кластера.

Рекомендуем сделать резервную копию директории `/etc/kubernetes`, в которой находятся:

* манифесты и конфигурация компонентов [control-plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
* [PKI кластера Kubernetes](https://kubernetes.io/docs/setup/best-practices/certificates/).

Данная директория поможет быстро восстановить кластер при полной потере control-plane узлов без создания нового кластера и без повторного присоединения узлов в новый кластер.

Рекомендуем хранить резервные копии снимков состояния кластера etcd, а также резервную копию директории `/etc/kubernetes/` в зашифрованном виде вне кластера Deckhouse.
Для этого вы можете использовать сторонние инструменты резервного копирования файлов, например [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/) и т.д.

О возможных вариантах восстановления состояния кластера из снимка etcd вы можете узнать [в документации](https://github.com/deckhouse/deckhouse/blob/main/modules/040-control-plane-manager/docs/internal/ETCD_RECOVERY.md).

### Как выполнить полное восстановление состояния кластера из резервной копии etcd?

Далее описаны шаги по восстановлению кластера до предыдущего состояния из резервной копии при полной потере данных.

<div id='восстановление-кластера-single-master'></div>

#### Восстановление кластера с одним master-узлом

Для корректного восстановления выполните следующие шаги на master-узле:

1. Найдите утилиту `etcdutl` на master-узле и скопируйте исполняемый файл в `/usr/local/bin/`:

   ```shell
   cp $(find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/ \
   -name etcdutl -print | tail -n 1) /usr/local/bin/etcdutl
   etcdutl version
   ```

   Должен отобразиться корректный вывод `etcdutl version` без ошибок.

   Также вы можете загрузить исполняемый файл [etcdutl](https://github.com/etcd-io/etcd/releases) на сервер (желательно, чтобы версия `etcdutl` была такая же, как и версия etcd в кластере):

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.6.1/etcd-v3.6.1-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.6.1-linux-amd64.tar.gz && mv etcd-v3.6.1-linux-amd64/etcdutl /usr/local/bin/etcdutl
   ```

   Проверить версию etcd в кластере можно выполнив следующую команду (команда может не сработать, если etcd и Kubernetes API недоступны):

   ```shell
   d8 k -n kube-system exec -ti etcd-$(hostname) -- etcdutl version
   ```

1. Остановите etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Сохраните текущие данные etcd.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Очистите директорию etcd.

   ```shell
   rm -rf /var/lib/etcd
   ```

1. Положите резервную копию etcd в файл `~/etcd-backup.snapshot`.

1. Восстановите базу данных etcd.

   ```shell
   ETCDCTL_API=3 etcdutl snapshot restore ~/etcd-backup.snapshot  --data-dir=/var/lib/etcd
   ```

1. Запустите etcd. Запуск может занять некоторое время.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
      ```

   Чтобы убедиться, что etcd запущена, воспользуйтесь командой:

   ```shell
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

   Пример вывода:

   ```console
   CONTAINER        IMAGE            CREATED              STATE     NAME      ATTEMPT     POD ID          POD
   4b11d6ea0338f    16d0a07aa1e26    About a minute ago   Running   etcd      0           ee3c8c7d7bba6   etcd-gs-test
   ```

1. Перезапустите master-узел.

<div id='восстановление-кластера-multi-master'></div>

#### Восстановление мультимастерного кластера

Для корректного восстановления выполните следующие шаги:

1. Включите режим High Availability (HA) с помощью глобального параметра [highAvailability](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability). Это необходимо для сохранения хотя бы одной реплики Prometheus и его PVC, поскольку в режиме кластера с одним master-узлом HA по умолчанию отключён.

1. Переведите кластер в режим с одним master-узлом в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере) для облачных кластеров, или самостоятельно выведите статические master-узлы из кластера.

1. На оставшемся единственном master-узле выполните шаги по восстановлению etcd из резервной копии в соответствии с [инструкцией](#восстановление-кластера-single-master) для кластера с одним master-узлом.

1. Когда работа etcd будет восстановлена, удалите из кластера информацию об уже удаленных в первом пункте master-узлах, воспользовавшись следующей командой (укажите название узла):

   ```shell
   d8 k delete node <MASTER_NODE_I>
   ```

1. Перезапустите все узлы кластера.

1. Дождитесь выполнения заданий из очереди Deckhouse:

   ```shell
   d8 system queue main
   ```

1. Переведите кластер обратно в режим мультимастерного в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master) для облачных кластеров или [инструкцией](#как-добавить-master-узел-в-статическом-или-гибридном-кластере) для статических или гибридных кластеров.

### Как восстановить объект Kubernetes из резервной копии etcd?

Чтобы получить данные определенных объектов кластера из резервной копии etcd:

1. Запустите временный экземпляр etcd.
1. Заполните его данными из [резервной копии](#как-сделать-бэкап-etcd-вручную).
1. Получите описания нужных объектов с помощью `auger`.

#### Пример шагов по восстановлению объектов из резервной копии etcd

В следующем примере `etcd-backup.snapshot` — [резервная копия](#как-сделать-бэкап-etcd-вручную) etcd (snapshot), `infra-production` — пространство имен, в котором нужно восстановить объекты.

* Для выгрузки бинарных данных из etcd потребуется утилита [auger](https://github.com/etcd-io/auger/tree/main). Ее можно собрать из исходного кода на любой машине с Docker (на узлах кластера это сделать невозможно).

  ```shell
  git clone -b v1.0.1 --depth 1 https://github.com/etcd-io/auger
  cd auger
  make release
  build/auger -h
  ```
  
* Получившийся исполняемый файл `build/auger`, а также `snapshot` из резервной копии etcd нужно загрузить на master-узел, с которого будет выполняться дальнейшие действия.

Данные действия выполняются на master-узле в кластере, на который предварительно был загружен файл `snapshot` и утилита `auger`:

1. Установите корректные права доступа для файла с резервной копией:

   ```shell
   chmod 644 etcd-backup.snapshot
   ```

1. Установите полный путь до `snapshot` и до утилиты в переменных окружения:

   ```shell
   SNAPSHOT=/root/etcd-restore/etcd-backup.snapshot
   AUGER_BIN=/root/auger 
   chmod +x $AUGER_BIN
   ```

1. Запустите под с временным экземпляром etcd:

   * Создайте манифест пода. Он будет запускаться именно на текущем master-узле, выбрав его по переменной `$HOSTNAME`, и смонтирует `snapshot` по пути `$SNAPSHOT` для загрузки во временный экземпляр etcd:

     ```shell
     cat <<EOF >etcd.pod.yaml 
     apiVersion: v1
     kind: Pod
     metadata:
       name: etcdrestore
       namespace: default
     spec:
       nodeName: $HOSTNAME
       tolerations:
       - operator: Exists
       initContainers:
       - command:
         - etcdutl
         - snapshot
         - restore
         - "/tmp/etcd-snapshot"
         - --data-dir=/default.etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-snapshot-restore
         # Раскоментируйте фрагмент ниже, чтобы задать лимиты для контейнера, если ресурсов узла недостаточно для его запуска.
         # resources:
         #   requests:
         #     ephemeral-storage: "200Mi"
         #   limits:
         #     ephemeral-storage: "500Mi"
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
         - name: etcd-snapshot
           mountPath: /tmp/etcd-snapshot
           readOnly: true
       containers:
       - command:
         - etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-temp
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
       volumes:
       - name: etcddir
         emptyDir: {}
         # Используйте фрагмент ниже вместо emptyDir: {}, чтобы задать лимиты для контейнера, если ресурсов узла недостаточно для его запуска.
         # emptyDir:
         #  sizeLimit: 500Mi
       - name: etcd-snapshot
         hostPath:
           path: $SNAPSHOT
           type: File
     EOF
     ```

   * Запустите под:

     ```shell
     d8 k create -f etcd.pod.yaml
     ```

1. Установите нужные переменные. В текущем примере:

   * `infra-production` - пространство имен, в котором мы будем искать ресурсы.

   * `/root/etcd-restore/output` - каталог для восстановленных манифестов.

   * `/root/auger` - путь до исполняемого файла утилиты `auger`:

     ```shell
     FILTER=infra-production
     BACKUP_OUTPUT_DIR=/root/etcd-restore/output
     mkdir -p $BACKUP_OUTPUT_DIR && cd $BACKUP_OUTPUT_DIR
     ```

1. Следующие команды отфильтруют список нужных ресурсов по переменной `$FILTER` и выгрузят их в каталог `$BACKUP_OUTPUT_DIR`:

   ```shell
   files=($(d8 k -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get / --prefix --keys-only | grep "$FILTER"))
   for file in "${files[@]}"
   do
     OBJECT=$(d8 k -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get "$file" --print-value-only | $AUGER_BIN decode)
     FILENAME=$(echo $file | sed -e "s#/registry/##g;s#/#_#g")
     echo "$OBJECT" > "$BACKUP_OUTPUT_DIR/$FILENAME.yaml"
     echo $BACKUP_OUTPUT_DIR/$FILENAME.yaml
   done
   ```

1. Удалите из полученных описаний объектов информацию о времени создания (`creationTimestamp`), `UID`, `status` и прочие оперативные данные, после чего восстановите объекты:

   ```bash
   d8 k create -f deployments_infra-production_supercronic.yaml
   ```

1. Удалите под с временным экземпляром etcd:

   ```bash
   d8 k -n default delete pod etcdrestore
   ```

## Как выбирается узел, на котором будет запущен под?

За распределение подов по узлам отвечает планировщик Kubernetes (компонент `scheduler`).
Он проходит через две основные фазы — `Filtering` и `Scoring` (на самом деле, фаз больше, например, `pre-filtering` и `post-filtering`, но в общем можно выделить две ключевые фазы).

### Общее устройство планировщика Kubernetes

Планировщик состоит из плагинов, которые работают в рамках какой-либо фазы (фаз).

Примеры плагинов:

* **ImageLocality** — отдает предпочтение узлам, на которых уже есть образы контейнеров, которые используются в запускаемом поде. Фаза: `Scoring`.
* **TaintToleration** — реализует механизм [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Фазы: `Filtering`, `Scoring`.
* **NodePorts** — проверяет, есть ли у узла свободные порты, необходимые для запуска пода. Фаза: `Filtering`.

С полным списком плагинов можно ознакомиться в [документации Kubernetes](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

### Логика работы

#### Профили планировщика

Есть два преднастроенных профиля планировщика:

* `default-scheduler` — профиль по умолчанию, который распределяет поды на узлы с наименьшей загрузкой;
* `high-node-utilization` — профиль, при котором поды размещаются на узлах с наибольшей загрузкой.

Чтобы задать профиль планировщика, укажите его параметре `spec.schedulerName` манифеста пода.

Пример использования профиля:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: scheduler-example
  labels:
    name: scheduler-example
spec:
  schedulerName: high-node-utilization
  containers:
  - name: example-pod
    image: registry.k8s.io/pause:2.0  
```

#### Этапы планирования подов

На первой фазе — `Filtering` — активируются плагины фильтрации (filter-плагины), которые из всех доступных узлов выбирают те, которые удовлетворяют определенным условиям фильтрации (например, `taints`, `nodePorts`, `nodeName`, `unschedulable` и другие). Если узлы расположены в разных зонах, планировщик чередует выбор зон, чтобы избежать размещения всех подов в одной зоне.

Предположим, что узлы распределяются по зонам следующим образом:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

В этом случае они будут выбираться в следующем порядке:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

Обратите внимание, что с целью оптимизации выбираются не все попадающие под условия узлы, а только их часть. По умолчанию функция выбора количества узлов линейная. Для кластера из ≤50 узлов будут выбраны 100% узлов, для кластера из 100 узлов — 50%, а для кластера из 5000 узлов — 10%. Минимальное значение — 5% при количестве узлов более 5000. Таким образом, при настройках по умолчанию узел может не попасть в список возможных узлов для запуска.

Эту логику можно изменить (см. подробнее про параметр `percentageOfNodesToScore` в [документации Kubernetes](https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/)), но Deckhouse не дает такой возможности.

После того как были выбраны узлы, соответствующие условиям фильтрации, запускается фаза `Scoring`. Каждый плагин анализирует список отфильтрованных узлов и назначает оценку (score) каждому узлу. Оценки от разных плагинов суммируются. На этой фазе оцениваются доступные ресурсы на узлах: `pod capacity`, `affinity`, `volume provisioning` и другие. По итогам этой фазы выбирается узел с наибольшей оценкой. Если сразу несколько узлов получили максимальную оценку, узел выбирается случайным образом.

В итоге под запускается на выбранном узле.

#### Документация

* [Общее описание scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).
* [Система плагинов](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).
* [Подробности фильтрации узлов](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/).
* [Исходный код scheduler](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler).

<div id='как-изменитьрасширить-логику-работы-планировщика'></div>

### Как изменить или расширить логику работы планировщика

Для изменения логики работы планировщика можно использовать [механизм плагинов расширения](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/624-scheduling-framework/README.md).

Каждый плагин представляет собой вебхук, отвечающий следующим требованиям:

* Использование TLS.
* Доступность через сервис внутри кластера.
* Поддержка стандартных `Verbs` (`filterVerb = filter`, `prioritizeVerb = prioritize`).
* Также, предполагается что все подключаемые плагины могут кэшировать информацию об узле (`nodeCacheCapable: true`).

Подключить `extender` можно при помощи ресурса [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration).

{% alert level="danger" %}
При использовании опции `failurePolicy: Fail`, в случае ошибки в работе вебхука планировщик Kubernetes прекратит свою работу, и новые поды не смогут быть запущены.
{% endalert %}

## Как происходит ротация сертификатов kubelet?

В Deckhouse Kubernetes Platform ротация сертификатов kubelet происходит автоматически.

Kubelet использует клиентский TLS-сертификат (`/var/lib/kubelet/pki/kubelet-client-current.pem`), при помощи которого может запросить у kube-apiserver новый клиентский сертификат или новый серверный сертификат (`/var/lib/kubelet/pki/kubelet-server-current.pem`).

Когда до истечения времени жизни сертификата остается 5-10% (случайное значение из диапазона) времени, kubelet запрашивает у kube-apiserver новый сертификат. С описанием алгоритма можно ознакомиться в официальной документации [Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/kubelet-tls-bootstrapping/#bootstrap-initialization).

### Время жизни сертификатов

По умолчанию время жизни сертификатов равно 1 году (8760 часов). При необходимости это значение можно изменить с помощью аргумента `--cluster-signing-duration` в манифесте `/etc/kubernetes/manifests/kube-controller-manager.yaml`. Но чтобы kubelet успел установить сертификат до его истечения, рекомендуем устанавливать время жизни сертификатов более, чем 1 час.

{% alert level="warning" %}
Если истекло время жизни клиентского сертификата, то kubelet не сможет делать запросы к kube-apiserver и не сможет обновить сертификаты. В данном случае узел (Node) будет помечен как `NotReady` и пересоздан.
{% endalert %}

### Особенности работы с серверными сертификатами kubelet в Deckhouse Kubernetes Platform

В Deckhouse Kubernetes Platform для запросов в kubelet API используются IP-адреса. Поэтому в конфигурации kubelet поля `tlsCertFile` и `tlsPrivateKeyFile` не указываются, а используется динамический сертификат, который kubelet генерирует самостоятельно. Также, из-за использования динамического сертификата, в Deckhouse Kubernetes Platform (в модуле `operator-trivy`) отключены проверки CIS benchmark `AVD-KCV-0088` и `AVD-KCV-0089`, которые отслеживают, были ли переданы аргументы `--tls-cert-file` и `--tls-private-key-file` для kubelet.

{% offtopic title="Информация о логике работы с серверными сертификатами в Kubernetes" %}

В kubelet реализована следующая логика работы с серверными сертификатами:

* Если `tlsCertFile` и `tlsPrivateKeyFile` не пустые, то kubelet будет использовать их как сертификат и ключ по умолчанию.
  * При запросе клиента в kubelet API с указанием IP-адреса (например `https://10.1.1.2:10250/`), для установления соединения по TLS-протоколу будет использован закрытый ключ по умолчанию (`tlsPrivateKeyFile`). В данном случае ротация сертификатов не будет работать.
  * При запросе клиента в kubelet API с указанием названия хоста (например `https://k8s-node:10250/`), для установления соединения по TLS-протоколу будет использован динамически сгенерированный закрытый ключ из директории `/var/lib/kubelet/pki/`. В данном случае ротация сертификатов будет работать.
* Если `tlsCertFile` и `tlsPrivateKeyFile` пустые, то для установления соединения по TLS-протоколу будет использован динамически сгенерированный закрытый ключ из директории `/var/lib/kubelet/pki/`. В данном случае ротация сертификатов будет работать.
{% endofftopic %}

## Как вручную обновить сертификаты компонентов управляющего слоя?

Может возникнуть ситуация, когда master-узлы кластера находятся в выключенном состоянии долгое время. За это время может истечь срок действия сертификатов компонентов управляющего слоя. После включения узлов сертификаты не обновятся автоматически, поэтому это необходимо сделать вручную.

Обновление сертификатов компонентов управляющего слоя происходит с помощью утилиты `kubeadm`.
Чтобы обновить сертификаты, выполните следующие действия на каждом master-узле:

1. Найдите утилиту `kubeadm` на master-узле и создайте символьную ссылку c помощью следующей команды:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

2. Обновите сертификаты:

   ```shell
   kubeadm certs renew all
   ```
