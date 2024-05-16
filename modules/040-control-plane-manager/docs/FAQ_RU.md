---
title: "Управление control plane: FAQ"
---

<div id='как-добавить-master-узел'></div>

## Как добавить master-узел в статическом или гибридном кластере?

> Важно иметь нечетное количество master-узлов для обеспечения кворума.

Добавление master-узла в статический или гибридный кластер ничем не отличается от добавления обычного узла в кластер. Воспользуйтесь для этого соответствующими [примерами](../040-node-manager/examples.html#добавление-статического-узла-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

## Как добавить master-узлы в облачном кластере (single-master в multi-master)?

> Перед добавлением узлов убедитесь в наличии необходимых квот.
>
> Важно иметь нечетное количество master-узлов для обеспечения кворума.

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-etcd-и-восстановление) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать созданию новых master-узлов.
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

1. **В контейнере с инсталлятором** выполните следующую команду и укажите требуемое количество мастер-узлов в параметре `masterNodeGroup.replicas`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

   > Для **Yandex Cloud**, при использовании внешних адресов на мастер-узлах, количество элементов массива в параметре [masterNodeGroup.instanceClass.externalIPAddresses](../030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-externalipaddresses) должно равняться количеству мастер-узлов. При использовании значения `Auto` (автоматический заказ публичных IP-адресов), количество элементов в массиве все равно должно соответствовать количеству мастер-узлов.
   >
   > Например, при трех мастер-узлах (`masterNodeGroup.replicas: 3`) и автоматическом заказе адресов, параметр `masterNodeGroup.instanceClass.externalIPAddresses` будет выглядеть следующим образом:
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

## Как уменьшить число master-узлов в облачном кластере (multi-master в single-master)?

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-etcd-и-восстановление) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
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

1. Снимите следующие лейблы с удаляемых master-узлов:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Команда для снятия лейблов:

   ```bash
   kubectl label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что удаляемые master-узлы пропали из списка членов кластера etcd:

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

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-etcd-и-восстановление) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Снимите лейблы `node.deckhouse.io/group: master` и `node-role.kubernetes.io/control-plane: ""`.
1. Убедитесь, что удаляемый master-узел пропал из списка членов кластера etcd:

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

## Как изменить образ ОС в multi-master-кластере?

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-etcd-и-восстановление) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](../300-prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
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

   Ответ должен сообщить вам, что Terraform не хочет ничего менять.

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

1. Убедитесь, что узел пропал из списка членов кластера etcd:

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

1. Выключите виртуальную машину, соответствующую узлу, удалите инстанс узла из облака и подключенные к нему диски (kubernetes-data).

1. Удалите в кластере поды, оставшиеся на удаляемом узле:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

1. Удалите в кластере объект `Node` удаленного узла:

   ```bash
   kubectl delete node ${NODE}
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для создания обновленного узла:

    Вам нужно внимательно прочитать, что converge собирается делать, когда запрашивает одобрение.

    Если converge запрашивает одобрение для другого мастер-узла, его следует пропустить, выбрав `no`.

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **На созданном узле** посмотрите журнал systemd-unit'а `bashible.service`. Дождитесь окончания настройки узла — в журнале появится сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Убедитесь, что узел появился в списке членов кластера etcd:

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

## Как изменить образ ОС в single-master-кластере?

1. Преобразуйте single-master-кластер в multi-master в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master).
1. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ос-в-multi-master-кластере).
1. Преобразуйте multi-master-кластер в single-master в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master)

## Как посмотреть список member'ов в etcd?

### Вариант 1

Используйте команду `etcdctl member list`

Пример:

   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

Внимание! Последний параметр в таблице вывода показывает, что член etcd находится в состоянии [**learner**](https://etcd.io/docs/v3.5/learning/design-learner/), а не в состоянии *leader*.

### Вариант 2

Используйте команду `etcdctl endpoint status`. Пятый параметр в таблице вывода будет `true` у лидера.

Пример:

```shell
$ kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl \ 
--cacert /etc/kubernetes/pki/etcd/ca.crt  --cert /etc/kubernetes/pki/etcd/ca.crt  \ 
--key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table

https://10.2.1.101:2379, ade526d28b1f92f7, 3.5.3, 177 MB, false, false, 42007, 406566258, 406566258,
https://10.2.1.102:2379, d282ac2ce600c1ce, 3.5.3, 182 MB, true, false, 42007, 406566258, 406566258,
```

## Что делать, если что-то пошло не так?

В процессе работы `control-plane-manager` оставляет резервные копии в `/etc/kubernetes/deckhouse/backup`, они могут помочь.

## Что делать, если кластер etcd развалился?

1. Остановите (удалите `/etc/kubernetes/manifests/etcd.yaml`) etcd на всех узлах, кроме одного. С него начнется восстановление multi-master'а.
2. На оставшемся узле в манифесте `/etc/kubernetes/manifests/etcd.yaml` укажите параметр `--force-new-cluster` в `spec.containers.command`.
3. После успешного подъема кластера удалите параметр `--force-new-cluster`.

> **Внимание!** Операция деструктивна, она полностью уничтожает консенсус и запускает etcd-кластер с состояния, которое сохранилось на узле. Любые pending-записи пропадут.

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

   Подробную информацию по настройке содержимого `audit-policy.yaml` можно получить по следующим ссылкам:
   - [Официальная документация Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
   - [Наша статья на Habr](https://habr.com/ru/company/flant/blog/468679/).
   - [Код скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

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

Предполагается наличие на master-узлах «скрейпера логов» *([log-shipper](../460-log-shipper/cr.html#clusterloggingconfig), promtail, filebeat)*, который будет следить за файлом с логами:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации файла журнала предустановлены и их изменение не предусмотрено:
- Максимальное занимаемое место на диске `1000 МБ`.
- Максимальная глубина записи `7 дней`.

В зависимости от настроек политики (`Policy`) и количества запросов к **apiserver** логов может быть очень много, соответственно глубина хранения может быть менее 30 минут.

### Предостережение

> **Внимание!** Текущая реализация функционала не является безопасной с точки зрения возможности временно сломать control plane.
>
> Если в Secret'е с конфигурационным файлом окажутся неподдерживаемые опции или опечатка, apiserver не сможет запуститься.

В случае возникновения проблем с запуском apiserver потребуется вручную отключить параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить apiserver следующей командой:

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

По умолчанию, если узел за 40 секунд не сообщает свое состояние, он помечается как недоступный. И еще через 5 минут поды узла начнут перезапускаться на других узлах. Итоговое время недоступности приложений около 6 минут.

В специфических случаях, когда приложение не может быть запущено в нескольких экземплярах, есть способ сократить период их недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой параметра `nodeMonitorGracePeriodSeconds`.
1. Установить меньший таймаут удаления подов с недоступного узла в параметре `failedNodePodEvictionTimeoutSeconds`.

### Пример

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

В этом случае при потере связи с узлом приложения будут перезапущены через ~ 1 минуту.

### Предостережение

Оба описанных параметра оказывают непосредственное влияние на потребляемые `control-plane'ом` ресурсы процессора и памяти. Уменьшая таймауты, мы заставляем системные компоненты чаще производить отправку статусов и сверку состояний ресурсов.

В процессе подбора подходящих вам значений обращайте внимание на графики потребления ресурсов управляющих узлов. Будьте готовы к тому, что чем меньшие значения параметров вы выбираете, тем больше ресурсов может потребоваться для выделения на эти узлы.

## Резервное копирование etcd и восстановление

### Как сделать бэкап etcd?

Войдите на любой control-plane-узел под пользователем `root` и используйте следующий bash-скрипт:

```bash
#!/usr/bin/env bash

pod=etcd-`hostname`
kubectl -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
mv /var/lib/etcd/"${pod##*/}.snapshot" etcd-backup.snapshot && \
cp -r /etc/kubernetes/ ./ && \
tar -cvzf kube-backup.tar.gz ./etcd-backup.snapshot ./kubernetes/
rm -r ./kubernetes ./etcd-backup.snapshot
```

В текущей директории будет создан файл `etcd-backup.snapshot` со снимком базы etcd одного из членов etcd-кластера.
Из полученного снимка можно будет восстановить состояние кластера etcd.

Также рекомендуем сделать бэкап директории `/etc/kubernetes`, в которой находятся:
- манифесты и конфигурация компонентов [control-plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
- [PKI кластера Kubernetes](https://kubernetes.io/docs/setup/best-practices/certificates/).
Данная директория поможет быстро восстановить кластер при полной потере control-plane-узлов без создания нового кластера
и без повторного присоединения узлов в новый кластер.

Мы рекомендуем хранить резервные копии снимков состояния кластера etcd, а также бэкап директории `/etc/kubernetes/` в зашифрованном виде вне кластера Deckhouse.
Для этого вы можете использовать сторонние инструменты резервного копирования файлов, например [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/) и т. д.

О возможных вариантах восстановления состояния кластера из снимка etcd вы можете узнать [здесь](https://github.com/deckhouse/deckhouse/blob/main/modules/040-control-plane-manager/docs/internal/ETCD_RECOVERY.md).

### Как выполнить полное восстановление состояния кластера из резервной копии etcd?

Далее будут описаны шаги по восстановлению до предыдущего состояния кластера из резервной копии при полной потере данных

#### Шаги по восстановлению single-master кластера

1. По необходимости скопируйте ключи доступа и сертификаты etcd-сервера в директорию `/etc/kubernetes`.
2. Загрузите утилиту [etcdctl](https://github.com/etcd-io/etcd/releases) на сервер (желательно чтобы её версия была такая же как и версия etcd в кластере).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

3. Остановите etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

4. Сохраните текущие данные etcd.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

5. Очистите директорию etcd.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

6. Перенесите и переименуйте бекап в `~/etcd-backup.snapshot`.

7. Восстановите базу данных etcd.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

8. Запустите etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

#### Шаги по восстановлению multi-master кластера

Для корректного восстановления multi-master:

1. Переведите кластер в single-master режим в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master) для облачных кластеров или самостоятельно выведите статические мастер ноды из кластера.

2. На единственной мастер ноде выполните шаги по восстановлению etcd из бекапа в соответствии с [инструкцией](#шаги-по-восстановлению-single-master-кластера) для single-master.

3. Когда работа etcd восстановлена, переведите кластер обратно в multi-master режим в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master) для облачных кластеров или [инструкцией](#как-добавить-master-узел-в-статическом-или-гибридном-кластере) для статических или гибридных кластеров.

### Как восстановить объект Kubernetes из резервной копии etcd?

Чтобы получить данные определенных объектов кластера из резервной копии etcd:
1. Запустите временный экземпляр etcd.
2. Наполните его данными из [резервной копии](#как-сделать-бэкап-etcd).
3. Получите описания нужных объектов с помощью `etcdhelper`.

#### Пример шагов по восстановлению объектов из резервной копии etcd

В примере далее `etcd-snapshot.bin` — [резервная копия](#как-сделать-бэкап-etcd) etcd (snapshot), `infra-production` — namespace, в котором нужно восстановить объекты.

1. Запустите под с временным экземпляром etcd.
   - Подготовьте файл `etcd.pod.yaml` шаблона пода, выполнив следующие команды:

     ```shell
     cat <<EOF >etcd.pod.yaml 
     apiVersion: v1
     kind: Pod
     metadata:
       name: etcdrestore
       namespace: default
     spec:
       containers:
       - command:
         - /bin/sh
         - -c
         - "sleep 96h"
         image: IMAGE
         imagePullPolicy: IfNotPresent
         name: etcd
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
       volumes:
       - name: etcddir
         emptyDir: {}
     EOF
     IMG=`kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' '`
     sed -i -e "s#IMAGE#$IMG#" etcd.pod.yaml
     ```

   - Создайте под:

     ```shell
     kubectl create -f etcd.pod.yaml
     ```

2. Скопируйте `etcdhelper` и снимок etcd в контейнер пода.

   `etcdhelper` можно собрать из [исходного кода](https://github.com/openshift/origin/tree/master/tools/etcdhelper) или скопировать из готового образа (например, из [образа `etcdhelper` на Docker Hub](https://hub.docker.com/r/webner/etcdhelper/tags)).

   Пример:

   ```shell
   kubectl cp etcd-snapshot.bin default/etcdrestore:/tmp/etcd-snapshot.bin
   kubectl cp etcdhelper default/etcdrestore:/usr/bin/etcdhelper
   ```

3. В контейнере установите права на запуск `etcdhelper`, восстановите данные из резервной копии и запустите etcd.

   Пример:

   ```console
   ~ # kubectl -n default exec -it etcdrestore -- sh
   / # chmod +x /usr/bin/etcdhelper
   / # etcdctl snapshot restore /tmp/etcd-snapshot.bin
   / # etcd &
   ```

4. Получите описания нужных объектов кластера, отфильтровав их с помощью `grep`.

   Пример:

   ```console
   ~ # kubectl -n default exec -it etcdrestore -- sh
   / # mkdir /tmp/restored_yaml
   / # cd /tmp/restored_yaml
   /tmp/restored_yaml # for o in `etcdhelper -endpoint 127.0.0.1:2379 ls /registry/ | grep infra-production` ; do etcdhelper -endpoint 127.0.0.1:2379 get $o > `echo $o | sed -e "s#/registry/##g;s#/#_#g"`.yaml ; done
   ```

   Замена символов с помощью `sed` в примере позволяет сохранить описания объектов в файлы, именованные подобно структуре реестра etcd. Например: `/registry/deployments/infra-production/supercronic.yaml` → `deployments_infra-production_supercronic.yaml`.

5. Скопируйте полученные описания объектов на master-узел:

   ```shell
   kubectl cp default/etcdrestore:/tmp/restored_yaml restored_yaml
   ```

6. Удалите из полученных описаний объектов информацию о времени создания, UID, status и прочие оперативные данные, после чего восстановите объекты:

   ```shell
   kubectl create -f restored_yaml/deployments_infra-production_supercronic.yaml
   ```

7. Удалите под с временным экземпляром etcd:

   ```shell
   kubectl -n default delete pod etcdrestore
   ```

## Как выбирается узел, на котором будет запущен под?

За распределение подов по узлам отвечает планировщик Kubernetes (компонент `scheduler`).
У него есть 2 фазы — Filtering и Scoring (на самом деле их больше, есть еще pre-filtering / post-filtering, но глобально можно свести к двум фазам).

### Общее устройство планировщика Kubernetes

Планировщик состоит из плагинов, которые работают в рамках какой-либо фазы (фаз).

Примеры плагинов:
- **ImageLocality** — отдает предпочтение узлам, на которых уже есть образы контейнеров, которые используются в запускаемом поде. Фаза: **Scoring**.
- **TaintToleration** — реализует механизм [taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Фазы: **Filtering, Scoring**.
- **NodePorts** — проверяет, есть ли у узла свободные порты, необходимые для запуска пода. Фаза: **Filtering**.

Полный список плагинов можно посмотреть в [документации Kubernetes](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins).

### Логика работы

Сначала идет фаза фильтрации (**Filtering**). В этот момент работают `filter`-плагины, которые из всего списка узлов выбирают те, которые попадают под условия фильтров (`taints`, `nodePorts`, `nodeName`, `unschedulable` и т. д.). Если узлы лежат в разных зонах, при выборе зоны чередуются, чтобы не размещать все поды в одной зоне.

Предположим, что узлы распределяются по зонам следующим образом:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

В этом случае они будут выбираться в следующем порядке:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

Обратите внимание, что с целью оптимизации выбираются не все попадающие под условия узлы, а только их часть. По умолчанию функция выбора количества узлов линейная. Для кластера из ≤50 узлов будут выбраны 100% узлов, для кластера из 100 узлов — 50%, а для кластера из 5000 узлов — 10%. Минимальное значение — 5% при количестве узлов более 5000. Таким образом, при настройках по умолчанию узел может не попасть в список возможных узлов для запуска. Эту логику можно изменить (см. подробнее про параметр `percentageOfNodesToScore` в [документации Kubernetes](https://kubernetes.io/docs/reference/config-api/kube-scheduler-config.v1/)), но Deckhouse не дает такой возможности.

После того как выбраны узлы, подходящие под условия, запускается фаза **Scoring**. Каждый плагин анализирует список отфильтрованных узлов и назначает оценку (score) каждому узлу. Оценки от разных плагинов суммируются. На этой фазе оцениваются доступные ресурсы на узлах, pod capacity, affinity, volume provisioning и так далее. По итогам этой фазы выбирается узел с наибольшей оценкой. Если сразу несколько узлов получили максимальную оценку, узел выбирается случайным образом.

В итоге под запускается на выбранном узле.

#### Документация

- [Общее описание scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)
- [Система плагинов](https://kubernetes.io/docs/reference/scheduling/config/#scheduling-plugins)
- [Подробности фильтрации узлов](https://kubernetes.io/docs/concepts/scheduling-eviction/scheduler-perf-tuning/)
- [Исходный код scheduler](https://github.com/kubernetes/kubernetes/tree/master/cmd/kube-scheduler)
