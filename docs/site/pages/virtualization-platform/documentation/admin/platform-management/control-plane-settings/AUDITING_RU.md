---
title: "Аудит"
permalink: ru/virtualization-platform/documentation/admin/platform-management/contoler-plane-settings/auditing.html
lang: "ru"
---

## Дополнительные политики аудита

Модуль `control-plane-manager` автоматизирует настройку `apiserver` для добавления политик аудита. Чтобы дополнительные политики заработали, нужно удостовериться, что аудит включён в kube-apiserver и создать секрет с политикой аудита:

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

1. Создайте секрет `kube-system/audit-policy` с YAML-файлом политик, закодированным в Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <base64>
   ```

   Для примера `audit-policy.yaml` можно привести правило для логирования всех изменений в метаданных:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   С примерами и информацией о правилах политик аудита можно ознакомиться в:

- [Официальной документации Kubernetes](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy).
- [Нашей статье на Habr](https://habr.com/ru/company/flant/blog/468679/).
- [Коде скрипта-генератора, используемого в GCE](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

{% alert level="danger" %}
В текущей реализации не валидируется содержимое дополнительных политик.

Если в `audit-policy.yaml` в политике будут указаны неподдерживаемые опции или будет допущена опечатка, то `apiserver` не запустится, что приведёт к недоступности управляющего слоя.
{% endalert %}

В таком случае для восстановления потребуется вручную убрать параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить `apiserver` следующей командой:

```bash
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

После перезапуска будет достаточно времени, чтобы удалить ошибочный секрет:

```bash
kubectl -n kube-system delete secret audit-policy
```

## Отключение встроенных политик аудита

В составе Deckhouse есть встроенные политики аудита для системных компонентов. Чтобы отключить их, в настройках модуля установите значение `false` в параметре [apiserver.basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled).

Пример включения аудита без встроенных политик Deckhouse:

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

## Вывод аудит-лога в стандартный вывод вместо файла

В настройках модуля установите значение `Stdout` в параметре [apiserver.auditLog.output](configuration.html#parameters-apiserver-auditlog).

Пример включения аудита в выводом в stdout:

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

TODO добавить зачем это нужно и как этот лог потом ловить?

## Как работать с журналом аудита?

Предполагается наличие на master-узлах «скрейпера логов» *(например, [log-shipper](../460-log-shipper/cr.html#clusterloggingconfig), promtail, filebeat)*, который будет следить за файлом с логами:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации файла журнала предустановлены и их изменение не предусмотрено:

- Максимальное занимаемое место на диске `1000 МБ`.
- Максимальная глубина записи `7 дней`.

Учитывайте, что "максимальная глубина записи" не означает "гарантированная". Интенсивность записи в журнал зависит от настроек дополнительных политик и количества запросов к `apiserver`, поэтому фактическая глубина хранения может быть сильно меньше 7 дней, например, менее 30 минут. Это нужно принимать во внимание при настройке скрейпера и при написании политик аудита.
