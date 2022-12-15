---
title: "Управление control plane: FAQ"
---

<div id="как-добавить-master-узел"></div>

## Как добавить master-узел в статичном или гибридном кластере?

Добавление master-узла в статичный или гибридный кластер ничем не отличается от добавления обычного узла в кластер. Воспользуйтесь для этого соответствующей [инструкцией](../040-node-manager/faq.html#как-добавить-статичный-узел-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

## Как добавить master-узлы в облачном кластере (single-master в multi-master)?

> Перед добавлением узлов убедитесь в наличии необходимых квот.

1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду и укажите требуемое количество реплик в параметре `masterNodeGroup.replicas`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Дождитесь появления необходимого количества master-узлов в статусе `Ready` и готовности всех экземпляров `control-plane-manager`:

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

<div id="как-удалить-master-узел"></div>

## Как уменьшить число master-узлов в облачном кластере (multi-master в single-master)?

1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
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
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Выполните drain для удаляемых узлов:

   ```bash
   kubectl drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальные машины, соответствующие удаляемым узлам, удалите инстансы соответствующих узлов из облака и подключенные к ним диски (`kubernetes-data-master-<N>`).

1. Удалите в кластере Pod'ы, оставшиеся на удаленных узлах:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. Удалите в кластере объекты Node удаленных узлов:

   ```bash
   kubectl delete node <MASTER-NODE-N-NAME>
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для запуска масштабирования:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

## Как убрать роль master-узла, сохранив узел?

1. Снимите лейблы `node.deckhouse.io/group: master` и `node-role.kubernetes.io/control-plane: ""`.
1. Убедитесь, что удаляемый master-узел пропал из списка членов кластера etcd:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
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

1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}') \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

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
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Выполните `drain` для узла:

   ```bash
   kubectl drain ${NODE} --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальную машину, соответствующую узлу, удалите инстанс узла из облака и подключенные к нему диски (kubernetes-data).

1. Удалите в кластере Pod'ы, оставшиеся на удаляемом узле:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

1. Удалите в кластере объект Node удаленного узла:

   ```bash
   kubectl delete node ${NODE}
   ```

1. **В контейнере с инсталлятором** выполните следующую команду для создания обновленного узла:

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
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- sh -c \
   "ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table"
   ```

1. Убедитесь, что `control-plane-manager` функционирует на узле.

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=${NODE}
   ```

1. Перейдите к обновлению следующего узла.

## Как изменить образ ОС в single-master-кластере?

1. Преобразуйте single-master-кластер в multi-master в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master).

   > Помимо увеличения числа реплик вы можете сразу указать образ с необходимой версией ОС в параметре `masterNode.instanceClass`.

1. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ос-в-multi-master-кластере).
1. Преобразуйте multi-master-кластер в single-master в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master)

## Как посмотреть список member'ов в etcd?

### Вариант 1

1. Зайдите в Pod с etcd:

   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
   ```

2. Выполните команду:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list
   ```

### Вариант 2

Используйте команду `etcdctl endpoint status`. Пятый параметр в таблице вывода будет`true` у лидера.

Пример:

```shell
$ ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status
https://10.2.1.101:2379, ade526d28b1f92f7, 3.5.3, 177 MB, false, false, 42007, 406566258, 406566258,
https://10.2.1.102:2379, d282ac2ce600c1ce, 3.5.3, 182 MB, true, false, 42007, 406566258, 406566258,
```

## Что делать, если что-то пошло не так?

В процессе работы control-plane-manager оставляет резервные копии в `/etc/kubernetes/deckhouse/backup`, они могут помочь.

## Что делать, если кластер etcd развалился?

1. Остановите (удалите `/etc/kubernetes/manifests/etcd.yaml`) etcd на всех узлах, кроме одного. С него начнётся восстановление multi-master'а.
2. На оставшемся узле в манифесте `/etc/kubernetes/manifests/etcd.yaml` укажите параметр `--force-new-cluster` в `spec.containers.command`.
3. После успешного подъёма кластера удалите параметр `--force-new-cluster`.

> **Внимание!** Операция деструктивна, она полностью уничтожает консенсус и запускает etcd кластер с состояния, которое сохранилось на узле. Любые pending записи пропадут.

## Как настроить дополнительные политики аудита?

1. Включите [флаг](configuration.html#parameters-apiserver-auditpolicyenabled) в `ConfigMap` `d8-system/deckhouse`:

   ```yaml
   controlPlaneManager: |
     apiserver:
       auditPolicyEnabled: true
   ```

2. Создайте Secret `kube-system/audit-policy` с файлом политик, закодированным в `base64`:

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
   - [В официальной документации Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
   - [В нашей статье на Habr](https://habr.com/ru/company/flant/blog/468679/).
   - [Опираясь на код скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

   Создать Secret вручную из файла можно командой:

   ```bash
   kubectl -n kube-system create secret generic audit-policy --from-file=./audit-policy.yaml
   ```

### Как исключить встроенные политики аудита?

Установите параметр `apiserver.basicAuditPolicyEnabled` в `false`.

Пример:

```yaml
controlPlaneManager: |
  apiserver:
    auditPolicyEnabled: true
    basicAuditPolicyEnabled: false
```

### Как вывести аудит-лог в стандартный вывод вместо файлов?

Установите параметр `apiserver.auditLog.output` в значение `stdout`.

Пример:

```yaml
controlPlaneManager: |
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
- Максимальное занимаемое место на диске `1000 Мб`.
- Максимальная глубина записи `7 дней`.

В зависимости от настроек `Policy` и количества запросов к **apiserver** логов может быть очень много, соответственно глубина хранения может быть менее 30 минут.

### Предостережение

> **Обратите внимание**, что текущая реализация функционала не является безопасной с точки зрения возможности временно сломать control plane.
>
> Если в Secret'е с конфигурационным файлом окажутся неподдерживаемые опции или опечатка, то apiserver не сможет запуститься.

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

## Как ускорить перезапуск Pod'ов при потере связи с узлом?

По умолчанию, если узел за 40 секунд не сообщает своё состояние — он помечается как недоступный. И еще через 5 минут Pod'ы узла начнут перезапускаться на других узлах. Итоговое время недоступности приложений около 6 минут.

В специфических случаях, когда приложение не может быть запущено в нескольких экземплярах, есть способ сократить период их недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой параметра `nodeMonitorGracePeriodSeconds`.
1. Установить меньший таймаут удаления Pod'ов с недоступного узла в параметре `failedNodePodEvictionTimeoutSeconds`.

### Пример

```yaml
controlPlaneManager: |
  nodeMonitorGracePeriodSeconds: 10
  failedNodePodEvictionTimeoutSeconds: 50
```

В этом случае при потере связи с узлом, приложения будут перезапущены через ~ 1 минуту.

### Предостережение

Оба описанных параметра оказывают непосредственное влияние на потребляемые control-plane'ом  ресурсы процессора и памяти. Уменьшая таймауты, мы заставляем системные компоненты чаще производить отправку статусов и сверки состояний ресурсов.

В процессе подбора подходящих вам значений обращайте внимание на графики потребления ресурсов управляющих узлов. Будьте готовы к тому, что чем меньшие значения параметров вы выбираете, тем больше ресурсов может потребоваться выделить на эти узлы.

## Как сделать бекап etcd?

Войдите на любой control-plane узел под пользователем `root` и используйте следующий bash-скрипт:

```bash
#!/usr/bin/env bash

for pod in $(kubectl get pod -n kube-system -l component=etcd,tier=control-plane -o name); do
  if kubectl -n kube-system exec "$pod" -- sh -c "ETCDCTL_API=3 /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /tmp/${pod##*/}.snapshot" && \
  kubectl -n kube-system exec "$pod" -- gzip -c /tmp/${pod##*/}.snapshot | zcat > "${pod##*/}.snapshot" && \
  kubectl -n kube-system exec "$pod" -- sh -c "cd /tmp && sha256sum ${pod##*/}.snapshot" | sha256sum -c && \
  kubectl -n kube-system exec "$pod" -- rm "/tmp/${pod##*/}.snapshot"; then
    mv "${pod##*/}.snapshot" etcd-backup.snapshot
    break
  fi
done
```

В текущей директории будет создан файл `etcd-backup.snapshot` со снимком базы etcd одного из членов etcd-кластера.
Из полученного снимка можно будет восстановить состояние кластера etcd.

Также рекомендуем сделать бекап директории `/etc/kubernetes` в которой находятся:
- манифесты и конфигурация компонентов [control-plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
- [PKI кластера Kubernetes](https://kubernetes.io/docs/setup/best-practices/certificates/).
Данная директория поможет быстро восстановить кластер при полной потери control-plane узлов без создания нового кластера
и без повторного присоединения узлов в новый кластер.

Мы рекомендуем хранить резервные копии снимков состояния кластера etcd, а также бекап директории `/etc/kubernetes/` в зашифрованном виде вне кластера Deckhouse.
Для этого вы можете использовать сторонние инструменты резервного копирования файлов, например: [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/) и т.д.

О возможных вариантах восстановления состояния кластера из снимка etcd вы можете узнать [здесь](https://github.com/deckhouse/deckhouse/blob/main/modules/040-control-plane-manager/docs/internal/ETCD_RECOVERY.md).
