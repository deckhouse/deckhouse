---
title: "Аудит"
permalink: ru/stronghold/documentation/admin/platform-management/control-plane-settings/audit.html
lang: "ru"
---

## Аудит

Для диагностики операций с API, например, в случае неожиданного поведения компонентов управляющего слоя, в Kubernetes предусмотрен режим журналирования операций с API. Этот режим можно настроить путем создания правил [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy), а результатом работы аудита будет лог-файл `/var/log/kube-audit/audit.log` со всеми интересующими операциями. Более подробно можно почитать в разделе [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/) в документации Kubernetes.

## Базовые политики аудита

В кластерах Deckhouse по умолчанию созданы базовые политики аудита:
- логирование операций создания, удаления и изменения ресурсов;
  <!-- TODO здесь какие ресурсы имеются в виду? Надо бы уточнить. -->
- логирование действий, совершаемых от имени сервисных аккаунтов из системных пространств имён: `kube-system`, `d8-*`;
- логирование действий, совершаемых с ресурсами в системных пространствах имён: `kube-system`, `d8-*`.

### Отключение базовых политик

Отключить сбор логов по базовым политикам можно установив флаг [`basicAuditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-basicauditpolicyenabled) в `false`.

Пример включения возможности аудита в kube-apiserver, но без базовых политик Deckhouse:

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

Можно воспользоваться патчем:

```shell
d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true, "basicAuditPolicyEnabled": false}}}'
```

## Пользовательские политики аудита

Модуль control-plane manager автоматизирует настройку kube-apiserver для добавления пользовательских политик аудита. Чтобы такие дополнительные политики заработали, нужно удостовериться, что аудит включён в секции параметров `apiserver` и создать секрет с политикой аудита:

1. Включите параметр [`auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) в настройках модуля:

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

   Включить можно редактированием ресурса, либо с помощью патча:

   ```shell
   d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true}}}'
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

{% alert level="critical" %}
В текущей реализации не валидируется содержимое дополнительных политик.

Если в `audit-policy.yaml` в политике будут указаны неподдерживаемые опции или будет допущена опечатка, то `apiserver` не запустится, что приведёт к недоступности управляющего слоя.
{% endalert %}

В таком случае для восстановления потребуется вручную убрать параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить `apiserver` следующей командой:

```bash
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

После перезапуска будет достаточно времени, чтобы удалить ошибочный секрет:

```bash
d8 k -n kube-system delete secret audit-policy
```

## Как работать с журналом аудита?

Предполагается наличие на master-узлах сборщика логов *(например, [`log-shipper`](/modules/log-shipper/), promtail, filebeat)*, который будет отправлять записи из файла в централизованное хранилище:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации файла журнала предустановлены и их изменение не предусмотрено:

- Максимальное занимаемое место на диске `1000 МБ`.
- Максимальная глубина записи `30 дней`.

Учитывайте, что «максимальная глубина записи» не означает «гарантированная». Интенсивность записи в журнал зависит от настроек дополнительных политик и количества запросов к **apiserver**, поэтому фактическая глубина хранения может быть сильно меньше 7 дней, например, 30 минут. Это нужно принимать во внимание при настройке сборщика логов и при написании политик аудита.

## Вывод аудит-лога в стандартный вывод

Если в кластере настроен сборщик логов с подов, можно собирать аудит лог, выведя его в стандартный вывод. Для этого  в настройках модуля установите значение `Stdout` в параметре [`apiserver.auditLog.output`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditlog-output).

Пример включения аудита с выводом в stdout:

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

Можно воспользоваться патчем:

```shell
d8 k patch mc control-plane-manager --type=strategic -p '{"settings":{"apiserver":{"auditPolicyEnabled":true, "auditLog":{"output":"Stdout"}}}}'
```

После рестарта kube-apiserver, в его логе можно увидеть события аудита:

```shell
d8 k -n kube-system logs $(d8 k -n kube-system get po -l component=kube-apiserver -oname | head -n1)

{"kind":"Event","apiVersion":"audit.k8s.io/v1","level":"Metadata","auditID":"38a26239-7f3e-402f-8c56-2fb57a3fe49d","stage":"ResponseComplete","requestURI": ...
```
