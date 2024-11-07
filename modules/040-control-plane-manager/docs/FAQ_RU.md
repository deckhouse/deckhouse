---
title: "Управление control plane: FAQ"
---

<div id='как-добавить-master-узел'></div>

## Как добавить master-узел в статическом или гибридном кластере?

> Важно иметь нечетное количество master-узлов для обеспечения кворума.

Добавление master-узла в статический или гибридный кластер ничем не отличается от добавления обычного узла в кластер. Воспользуйтесь для этого соответствующими [примерами](../040-node-manager/examples.html#добавление-статического-узла-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

<div id='как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master'></div>

## Как добавить master-узлы в облачном кластере?

Далее описана конвертация кластера с одним master-узлом в мультимастерный кластер.

> Перед добавлением узлов убедитесь в наличии необходимых квот.
>
> Важно иметь нечетное количество master-узлов для обеспечения кворума.

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать созданию новых master-узлов.
1. Убедитесь, что [очередь Deckhouse пуста](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse).
1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
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

   > Для **Yandex Cloud**, при использовании внешних адресов на master-узлах, количество элементов массива в параметре [masterNodeGroup.instanceClass.externalIPAddresses](../030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalipaddresses) должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов), количество элементов в массиве все равно должно соответствовать количеству master-узлов.
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
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

<div id='как-удалить-master-узел'></div>

## Как уменьшить число master-узлов в облачном кластере?

Далее описана конвертация мультимастерного кластера в кластер с одним master-узлом.

{% alert level="warning" %}
Описанные ниже шаги необходимо выполнять с первого по порядку master-узла кластера (master-0). Это связано с тем, что кластер всегда масштабируется по порядку: например, невозможно удалить узлы master-0 и master-1, оставив master-2.
{% endalert %}

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что [очередь Deckhouse пуста](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse).
1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду и укажите `1` в параметре `masterNodeGroup.replicas`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   > Для **Yandex Cloud** при использовании внешних адресов на master-узлах количество элементов массива в параметре [masterNodeGroup.instanceClass.externalIPAddresses](../030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalipaddresses) должно равняться количеству master-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов) количество элементов в массиве все равно должно соответствовать количеству master-узлов.
   >
   > Например, при одном master-узле (`masterNodeGroup.replicas: 1`) и автоматическом заказе адресов параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. Снимите следующие лейблы с удаляемых master-узлов:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Команда для снятия лейблов:

   ```bash
   kubectl label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что удаляемые master-узлы пропали из списка узлов кластера etcd:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Выполните `drain` для удаляемых узлов:

   ```bash
   kubectl drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальные машины, соответствующие удаляемым узлам, удалите инстансы соответствующих узлов из облака и подключенные к ним диски (`kubernetes-data-master-<N>`).

1. Удалите в кластере поды, оставшиеся на удаленных узлах:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. Удалите в кластере объекты `Node` удаленных узлов:

   ```bash
   kubectl delete node <MASTER-NODE-N-NAME>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

## Как убрать роль master-узла, сохранив узел?

1. Сделайте [резервную копию etcd](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что [очередь Deckhouse пуста](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse).
1. Снимите лейблы `node.deckhouse.io/group: master` и `node-role.kubernetes.io/control-plane: ""`.
1. Убедитесь, что удаляемый master-узел пропал из списка узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
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
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что [очередь Deckhouse пуста](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse).
1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
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

Следующие действия **выполняйте поочередно на каждом** master-узле, начиная с узла с наивысшим номером (с суффиксом 2) и заканчивая узлом с наименьшим номером (с суффиксом 0).

1. Выберите master-узел для обновления (укажите его название):

   ```bash
   NODE="<MASTER-NODE-N-NAME>"
   ```

1. Выполните следующую команду для снятия лейблов `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, `node.deckhouse.io/group` с узла:

   ```bash
   kubectl label node ${NODE} \
     node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что узел пропал из списка узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Выполните `drain` для узла:

   ```bash
   kubectl drain ${NODE} --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальную машину, соответствующую узлу, удалите инстанс узла из облака и подключенные к нему диски (`kubernetes-data`).

1. Удалите в кластере поды, оставшиеся на удаляемом узле:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

1. Удалите в кластере объект `Node` удаленного узла:

   ```bash
   kubectl delete node ${NODE}
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы создать обновлённый узел:

   Внимательно изучите действия, которые планирует выполнить converge, когда запрашивает подтверждение.

   Если converge запрашивает подтверждение для другого master-узла, выберите `no`, чтобы пропустить его.

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **На созданном узле** откройте журнал systemd-юнита `bashible.service`. Дождитесь окончания настройки узла — в журнале должно появиться сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Проверьте, что узел отобразился в списке узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Убедитесь, что `control-plane-manager` функционирует на узле.

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=${NODE}
   ```

1. Перейдите к обновлению следующего узла.

<div id='как-изменить-образ-ос-в-single-master-кластере'></div>

## Как изменить образ ОС в кластере с одним master-узлом?

1. Преобразуйте кластер с одним master-узлом в мультимастерный в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master).
1. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ос-в-multi-master-кластере).
1. Преобразуйте мультимастерный кластер в кластер с одним master-узлом в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master)

<div id='как-посмотреть-список-memberов-в-etcd'></div>

## Как посмотреть список узлов кластера в etcd?

### Вариант 1

Используйте команду `etcdctl member list`.

Пример:

```shell
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ member list -w table
```

**Внимание.** Последний параметр в таблице вывода показывает, что узел находится в состоянии [`learner`](https://etcd.io/docs/v3.5/learning/design-learner/), а не в состоянии `leader`.

### Вариант 2

Используйте команду `etcdctl endpoint status`. Для этой команды, после флага `--endpoints` нужно подставить адрес каждого узла control-plane. В пятом столбце таблицы вывода будет указано значение `true` для лидера.

Пример скрипта, который автоматически передает все адреса узлов control-plane:

```shell
MASTER_NODE_IPS=($(kubectl get nodes -l \
node-role.kubernetes.io/control-plane="" \
-o 'custom-columns=IP:.status.addresses[?(@.type=="InternalIP")].address' \
--no-headers))
unset ENDPOINTS_STRING
for master_node_ip in ${MASTER_NODE_IPS[@]}
do ENDPOINTS_STRING+="--endpoints https://${master_node_ip}:2379 "
done
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod \
-l component=etcd,tier=control-plane -o name | head -n1) \
-- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt  --cert /etc/kubernetes/pki/etcd/ca.crt \
--key /etc/kubernetes/pki/etcd/ca.key \
$(echo -n $ENDPOINTS_STRING) endpoint status -w table
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

Предполагается, что на master-узлах установлен «скрейпер логов»: [log-shipper](../460-log-shipper/cr.html#clusterloggingconfig), `promtail`, `filebeat`,  который будет мониторить файл с логами:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации логов в файле журнала предустановлены и их изменение не предусмотрено:

* Максимальное занимаемое место на диске `1000 МБ`.
* Максимальная глубина записи `7 дней`.

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
kubectl -n kube-system delete secret audit-policy
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
d8 backup etcd --kubeconfig $KUBECONFIG ./etcd-backup.snapshot
```

#### Используя bash (Deckhouse Kubernetes Platform v1.64 и старше)

Войдите на любой control-plane узел под пользователем `root` и используйте следующий bash-скрипт:

```bash
#!/usr/bin/env bash
set -e

pod=etcd-`hostname`
kubectl -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
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

1. Найдите утилиту `etcdctl` на мастер-узле и скопируйте исполняемый файл в `/usr/local/bin/`:

   ```shell
   cp $(find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/ \
   -name etcdctl -print | tail -n 1) /usr/local/bin/etcdctl
   etcdctl version
   ```

   Должен отобразиться корректный вывод `etcdctl version` без ошибок.

   Также вы можете загрузить исполняемый файл [etcdctl](https://github.com/etcd-io/etcd/releases) на сервер (желательно, чтобы версия `etcdctl` была такая же, как и версия etcd в кластере):

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.16/etcd-v3.5.16-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.16-linux-amd64.tar.gz && mv etcd-v3.5.16-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

   Проверить версию etcd в кластере можно выполнив следующую команду (команда может не сработать, если etcd и Kubernetes API недоступны):

   ```shell
   kubectl -n kube-system exec -ti etcd-$(hostname) -- etcdctl version
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
   rm -rf /var/lib/etcd/member/
   ```

1. Положите резервную копию etcd в файл `~/etcd-backup.snapshot`.

1. Восстановите базу данных etcd.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

1. Запустите etcd. Запуск может занять некоторое время.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

<div id='восстановление-кластерa-multi-master'></div>

#### Восстановление мултимастерного кластерa

Для корректного восстановления выполните следующие шаги:

1. Включите режим High Availability (HA) с помощью глобального параметра [highAvailability](../../deckhouse-configure-global.html#parameters-highavailability). Это необходимо для сохранения хотя бы одной реплики Prometheus и его PVC, поскольку в режиме кластера с одним master-узлом HA по умолчанию отключён.

1. Переведите кластер в режим с одним master-узлом в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master) для облачных кластеров, или самостоятельно выведите статические master-узлы из кластера.

1. На оставшемся единственном master-узле выполните шаги по восстановлению etcd из резервной копии в соответствии с [инструкцией](#восстановление-кластера-single-master) для кластера с одним master-узлом.

1. Когда работа etcd будет восстановлена, удалите из кластера информацию об уже удаленных в первом пункте master-узлах, воспользовавшись следующей командой (укажите название узла):

   ```shell
   kubectl delete node <MASTER_NODE_I>
   ```

1. Перезапустите все узлы кластера.

1. Дождитесь выполнения заданий из очереди Deckhouse:

   ```shell
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
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
         - etcdctl
         - snapshot
         - restore
         - "/tmp/etcd-snapshot"
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-snapshot-restore
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
       - name: etcd-snapshot
         hostPath:
           path: $SNAPSHOT
           type: File
     EOF
     ```

   * Запустите под:

     ```shell
     kubectl create -f etcd.pod.yaml
     ```

1. Устанавите нужные переменные. В текущем примере:

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
   files=($(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get / --prefix --keys-only | grep "$FILTER"))
   for file in "${files[@]}"
   do
     OBJECT=$(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get "$file" --print-value-only | $AUGER_BIN decode)
     FILENAME=$(echo $file | sed -e "s#/registry/##g;s#/#_#g")
     echo "$OBJECT" > "$BACKUP_OUTPUT_DIR/$FILENAME.yaml"
     echo $BACKUP_OUTPUT_DIR/$FILENAME.yaml
   done
   ```

1. Удалите из полученных описаний объектов информацию о времени создания (`creationTimestamp`), `UID`, `status` и прочие оперативные данные, после чего восстановите объекты:

   ```bash
   kubectl create -f deployments_infra-production_supercronic.yaml
   ```

1. Удалите под с временным экземпляром etcd:

   ```bash
   kubectl -n default delete pod etcdrestore
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
