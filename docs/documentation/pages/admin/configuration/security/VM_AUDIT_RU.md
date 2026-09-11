---
title: "Аудит событий виртуализации"
permalink: ru/admin/configuration/security/events/virtualization-audit.html
description: "Аудит событий безопасности по ресурсам виртуализации: включение, состав событий и их просмотр в системе логирования кластера."
search: аудит виртуализации, события безопасности, virtualization-audit, аудит ВМ
lang: ru
---

Аудит фиксирует действия с виртуальными машинами (ВМ) и с самим модулем, чтобы вы могли разобрать инцидент и восстановить последовательность событий.

{% alert level="warning" %}
Доступно в редакциях DP EE и Ultimate.
{% endalert %}

## Включение аудита

Чтобы включить аудит событий безопасности, выполните следующие шаги:

1. Включите модули [`log-shipper`](/modules/log-shipper/) и [`runtime-audit-engine`](/modules/runtime-audit-engine/).
1. Включите аудит API Kubernetes, задав [`.spec.settings.apiserver.auditPolicyEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-auditpolicyenabled) в значение `true` в модуле [`control-plane-manager`](/modules/control-plane-manager/).
1. Задайте [`.spec.settings.audit.enabled`](/modules/virtualization/configuration.html#parameters-audit-enabled) в значение `true` в модуле `virtualization`:

   ```yaml
   spec:
     settings:
       audit:
         enabled: true
   ```

Пока все три условия не выполнены, компонент аудита в кластере не запускается. Остальные параметры описаны в [настройках модуля](/modules/virtualization/configuration.html).

## Состав событий

Тип события записан в поле `type`. Аудит различает следующие типы:

- `Access to VM` — подключение к ВМ по консоли, VNC или через проброс портов, фиксируются начало и завершение сеанса.
- `Manage VM` — создание, изменение или удаление ресурса [VirtualMachine](/modules/virtualization/cr.html#virtualmachine).
- `Control VM` — изменение состояния ВМ, в том числе запуск, остановка, перезапуск, миграция и вытеснение через ресурс [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation), а также остановка или перезапуск из гостевой ОС и аварийное завершение работы.
- `Module control` — создание, изменение, выключение или удаление [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig).
- `Virtualization control` — создание или удаление системного компонента модуля в неймспейсе `d8-virtualization`.
- `Integrity check` — несовпадение контрольной суммы конфигурации ВМ с эталонной.
- `Forbidden operation` — попытка выполнить запрещённую операцию.

Независимо от типа каждое событие содержит одни и те же поля:

- `name` — описание произошедшего;
- `datetime` — время события;
- `request_subject` — имя пользователя или ServiceAccount, от имени которого выполнено действие;
- `operation_result` — результат операции;
- `uid` — идентификатор записи в аудите Kubernetes.

К ним добавляются уточняющие поля, состав которых зависит от типа события. Например, события с ВМ содержат поля `virtual_machine_name` и `virtual_machine_namespace`, а запрещённые операции описывают источник запроса в поле `source_ip` и причину отказа в поле `forbid_reason`.

## Просмотр событий

События собирает системный компонент `virtualization-audit` в неймспейсе `d8-virtualization`. Чтобы перенаправить их в систему логирования кластера, например в [Loki](/modules/loki/), создайте [ClusterLoggingConfig](/modules/log-shipper/cr.html#clusterloggingconfig):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: virtualization-audit-logs
spec:
  destinationRefs:
    - d8-loki
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - d8-virtualization
    labelSelector:
      matchLabels:
        app: virtualization-audit
  type: KubernetesPods
```

Чтобы посмотреть события в [Grafana](/modules/prometheus/), используйте запрос к [Loki](/modules/loki/):

```logql
{namespace="d8-virtualization", pod=~"virtualization-audit-.*"}
```
