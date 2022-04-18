---
title: "Управление control plane: FAQ"
---

## Как добавить master-узел?

### Статичный или гибридный кластер
Добавление master-узла в статичный или гибридный кластер ничем не отличается от добавления обычного узла в кластер. Воспользуйтесь для этого соответствующей [инструкцией](../040-node-manager/faq.html#как-добавить-статичный-узел-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

### Облачный кластер

> Перед добавлением узлов убедитесь в наличии необходимых квот.

Чтобы добавить в облачный кластер один или несколько master-узлов, выполните следующие действия:
1. Определите версию и редакцию Deckhouse, используемую в кластере. Для этого выполните на master-узле или на хосте с настроенным доступом kubectl в кластер:
   ```shell
   kubectl -n d8-system get deployment deckhouse \
   -o jsonpath='version-{.metadata.annotations.core\.deckhouse\.io\/version}, edition-{.metadata.annotations.core\.deckhouse\.io\/edition}' \
   | tr '[:upper:]' '[:lower:]'
   ```
1. Запустите контейнер с инсталлятором, версия и редакция которого соответствуют версии и редакции Deckhouse в кластере:
   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
   registry.deckhouse.io/deckhouse/<DECKHOUSE_EDITION>/install:<DECKHOUSE_VERSION> bash
   ```
   Например, если версия Deckhouse в кластере — `v1.28.0`, редакция — `ee`, то команда запуска инсталлятора будет следующей:
   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ee/install:v1.28.0 bash
   ```
   > Измените адрес container registry при необходимости (например, если вы используете внутренний container registry).

1. В контейнере с инсталлятором выполните следующую команду (используйте ключи `--ssh-bastion-*` в случае доступа через bastion-хост):
   ```shell
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
   --ssh-host <SSH_HOST>
   ```
1. В открывшемся окне редактирования ресурса `<PROVIDERNAME>ClusterConfiguration` (например, `OpenStackClusterConfiguration` в случае OpenStack) укажите необходимое количество реплик master-узла в поле `masterNodeGroup.replicas` и сохраните изменения.
1. Запустите масштабирование, выполнив следующую команду (необходимо указать соответствующие параметры доступа в кластер, как на предыдущем шаге):
   ```shell
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <SSH_HOST>
   ```
1. Ответьте утвердительно на вопрос `Do you want to CHANGE objects state in the cloud?`.

Все остальные действия будут выполнены автоматически, дождитесь их завершения — появления необходимого количества master-узлов в статусе `Ready`.

## Как удалить master-узел?

1. Проверьте, нарушает ли удаление кворум:

   * Если удаление не нарушает кворум в etcd (в корректно функционирующем кластере это все ситуации, кроме перехода 2 -> 1):
     * Если виртуальную машину с master-узлом можно удалять (на ней нет никаких других нужных сервисов), то можно удалить виртуальную машину обычным способом.
     * Если удалять виртуальную машину с master-узлом нельзя, например, на ней настроены задания резервного копирования, выполняется деплой и т.п., то необходимо остановить используемый на узле Container Runtime:

       В случае использования Docker:
       ```shell
       systemctl stop docker
       systemctl disable docker
       ```
       В случае использования Containerd:
       ```shell
       systemctl stop containerd
       systemctl disable containerd
       kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
       ```
   * Если удаление нарушает кворум (переход 2 -> 1) - остановите kubelet на узле (не останавливая контейнер с etcd):
     ```shell
     systemctl stop kubelet
     systemctl stop bashible.timer
     systemctl stop bashible
     systemctl disable kubelet
     systemctl disable bashible.timer
     systemctl disable bashible
     ```
2. Удалите объект Node из Kubernetes.
3. [Дождитесь](#как-посмотреть-список-memberов-в-etcd), пока etcd member будет автоматически удален.

## Как убрать роль master-узла, сохранив узел?

1. Снимите лейблы `node.deckhouse.io/group: master` и `node-role.kubernetes.io/master: ""`, затем дождитесь, пока etcd member будет удален автоматически.
2. Зайдите на узел и выполните следующие действия:
   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd
   ```

## Как посмотреть список member'ов в etcd?

1. Зайдите в Pod с etcd:
   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
   ```
2. Выполните команду:
   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list
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
   - [В официальной документации Kubernetes](https://kubernetes.io/docs/tasks/debug-application-cluster/audit/#audit-policy).
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
