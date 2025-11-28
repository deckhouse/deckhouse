---
title: Аудит событий API Kubernetes
permalink: ru/virtualization-platform/documentation/admin/platform-management/security/events/kubernetes-api-audit.html
lang: ru
---

Процедура аудита ([Kubernetes auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/))
позволяет отслеживать обращения к API-серверу и анализировать события, происходящие в кластере.
Аудит может быть полезен для отладки неожиданных сценариев поведения, а также для соблюдения требований безопасности.

Kubernetes поддерживает настройку аудита через механизм [Audit policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy),
который позволяет задавать правила логирования интересующих операций.
Результаты аудита по умолчанию записываются в лог-файл `/var/log/kube-audit/audit.log`.

## Встроенные политики аудита

В Deckhouse Virtualization Platform (DKP) по умолчанию разворачивается набор встроенных политик аудита. Они фиксируют следующие категории событий:

Общие события:

- Создание и удаление узлов кластера (объектов `Node`);
- Создание, изменение и удаление ресурсов во всех системных пространствах имён (`kube-system`, `d8-*`);
- Запросы от сервисных аккаунтов из системных пространств имён (`kube-system`, `d8-*`);
- Массовые запросы (`LIST`) ко всем ресурсам (для диагностики высокого потребления ресурсов API-сервером);
- Операции с ресурсами модуля [`virtualization`](/modules/virtualization/);
- Действия в пространстве имён `d8-virtualization`;
- Операции с объектами `ModuleConfig`;
- Запросы от пользователей, не прошедших аутентификацию (логируются только события уровня `Metadata`).

События, подлежащие регистрации в соответствии с требованиями ФСТЭК к средствам контейнеризации:

- Создание, изменение и удаление объектов `Pod`;
- Использование тегов образов контейнеров, отличных от формата `@sha256`;
- Создание и удаление объектов `ServiceAccount`, в том числе в системных пространствах имён;
- Создание, изменение и удаление объектов `Role` и `ClusterRole`;
- Создание, изменение и удаление объектов `ClusterRoleBinding`;
- Использование команд `attach` и `exec` к `Pod`, а также добавление ephemeral-контейнеров (запросы и модификация объектов `pods/exec`, `pods/attach`, `pods/ephemeralcontainers`).

События, которые намеренно не фиксируются политикой аудита (из-за высокой частоты обновления соответствующих объектов):

- Операции с объектами типов `Endpoints`, `EndpointSlice` и `Event`;
- Операции с объектами Lease (выбор лидера raft в компонентах платформы);
- Действия с ConfigMap, используемыми для выбора лидера raft (`cert-manager-cainjector-leader-election`, `cert-manager-controller`, `ingress-nginx` и др.);
- Операции с объектами `VerticalPodAutoscalerCheckpoints`;
- Операции `PATCH` для объектов `VerticalPodAutoscaler`, выполняемые сервисным аккаунтом `d8-vertical-pod-autoscaler-recommender`;
- Операции с объектами `UpmeterHookProbes`;
- Любые операции в пространстве имён `d8-upmeter`.

Эти политики активны по умолчанию.
Чтобы отключить их, установите [параметр `basicAuditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-basicauditpolicyenabled) в значение `false`.

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

## Настройка собственной политики аудита

Чтобы создать расширенную политику аудита API Kubernetes, выполните следующие шаги:

1. Включите [параметр `auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) в настройках модуля `control-plane-manager`:

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

1. Создайте Secret `kube-system/audit-policy`, содержащий YAML-файл политики в формате Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <Base64>
   ```

   Пример содержимого файла `audit-policy.yaml` с минимальной рабочей конфигурацией:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   Источники с подробной информацией по настройке возможного содержимого файла `audit-policy.yaml`:

   - [официальная документация Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy);
   - [статья из блога компании «Флант» на Habr](https://habr.com/ru/companies/flant/articles/468679/);
   - [код скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

## Работа с лог-файлом аудита

Предполагается, что на master-узлах кластера Deckhouse установлен инструмент
для отслеживания лог-файла `/var/log/kube-audit/audit.log`: `log-shipper`, `promtail` или `filebeat`.

Параметры ротации логов в файле предустановлены и не подлежат изменению:

- максимальный размер файла: 1000 МБ;
- максимальная глубина записи: 30 дней.

В зависимости от настроек политики и объема запросов к API-серверу количество записей может быть очень большим.
В таких условиях глубина хранения может составлять менее 30 минут.

{% alert level="warning" %}
Наличие неподдерживаемых опций или опечаток в конфигурационном файле может привести к ошибкам при запуске API-сервера.
{% endalert %}

В случае возникновения проблем с запуском API-сервера выполните следующие шаги:

1. Вручную удалите параметры `--audit-log-*` из манифеста `/etc/kubernetes/manifests/kube-apiserver.yaml`;
1. Перезапустите API-сервер с помощью следующей команды:

   ```shell
   docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
   
   # Альтернативный вариант (в зависимости от используемого CRI).
   crictl stopp $(crictl pods --name=kube-apiserver -q)
   ```

1. После перезапуска исправьте Secret или удалите его с помощью следующей команды:

   ```shell
   d8 k -n kube-system delete secret audit-policy
   ```

## Перенаправление лог-файла аудита в stdout

По умолчанию лог аудита сохраняется в файл `/var/log/kube-audit/audit.log` на master-узлах.
При необходимости вы можете перенаправить его вывод в stdout процесса `kube-apiserver` вместо файла,
установив [параметр `apiserver.auditLog.output`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditlog-output) модуля `control-plane-manager` в значение `Stdout`:

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

В этом случае лог будет доступен в stdout контейнера `kube-apiserver`.

Далее, используя [встроенный механизм логирования в DVP](/products/virtualization-platform/documentation/admin/platform-management/logging/delivery.html),
вы можете настроить сбор и отправку логов в собственную систему безопасности.
