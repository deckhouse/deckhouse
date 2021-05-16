---
title: "Как настроить?"
permalink: ru/
lang: ru
---

Конфигурация самого *Deckhouse* и его модулей находится в одном месте - ConfigMap `deckhouse` в namespace `d8-system`. Некоторые модули помимо своей конфигурации настраиваются с учетом специальных custom resource в кластере. Описание параметров конфигурации модуля и используемых модулем custom resource'ов можно найти в описании модуля или функций подсистемы.

Конфигурация `deckhouse` (ConfigMap `deckhouse`) состоит из [глобальной секции](#глобальная-конфигурация) и секции модулей.

Редактирование конфигурации `deckhouse`:
```
kubectl -n d8-system edit cm/deckhouse
```

## Пример ConfigMap `deckhouse`

Обратите внимание на символ `|` и не запутайтесь.

```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # <--- очень важно, вертикальная черта!!!
    # Тут кусок Yaml-файла с глобальными настройками
    project: someproject
    clusterName: main
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  nginxIngress: |
    # Тут кусок Yaml-файла, касающийся модуля nginx-ingress
    config:
      hsts: true
  someModuleName: |  # <--- тут всегда camel case от названия модуля
    foo: bar
  dashboardEnabled: "false"   # <--- вот так можно отключить модуль
```

## Включение и отключение модуля

Deckhouse устанавливает только включённые [модули](https://github.com/flant/addon-operator/blob/master/MODULES.md). Смотри подробнее про алгоритм определения включённости модуля [тут](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md#modules-discovery).

Модули могут быть включены или выключены по умолчанию, исходя из используемого [варианта поставки]({{"/modules/020-deckhouse/configuration.html" | true_relative_url }} ).

Для включения/отключения модуля, необходимо добавить в configMap `deckhouse` параметр `<moduleName>Enabled` — `"true"` или `"false"`, где `<moduleName>` — название модуля в camelCase.

Пример включения модуля user-authn
```yaml
data:
  userAuthnEnabled: "true"
```

## Выделение узлов под определенный вид нагрузки

Для всех модулей принята единая стратегия:
1. Если параметр модуля `nodeSelector` не указан, то мы смотрим, есть ли в кластере узлы с определенными лейблами и если они есть – автоматически используем соответствующие nodeSelector'ы. Конкретные лейблы и порядок поиска узлов см. ниже.
1. Если параметр модуля `tolerations` не указан, то мы автоматически ставим pod'ам модуля все возможные toleration'ы (см. список ниже).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно значением `false`.

**Важно!** Если модуль предполагает работу DaemonSet'a на всех нодах кластера (например, `ping-exporter` и `node-problem-detector`) или модуль должен работать на master-узлах (например `prometheus-metrics-adapter` или некоторые компоненты `vertical-pod-autoscaler`) — то у таких модулей возможность настройки `nodeSelector` и `tolerations` отключена.

{% offtopic title="Особенности автоматики, зависящие от 'типа' модуля" %}{% raw %}
* Модули *monitoring* (operator-prometheus, prometheus и vertical-pod-autoscaler):
  * Порядок поиска узлов (для определения nodeSelector):
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/monitoring</code>
    * Наличие ноды с лейблом <code>node-role.deckhouse.io/system</code>
  * Добавляемые toleration'ы (добавляются одновременно все):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

      (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>)
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
* Модули *frontend* (исключительно nginx-ingress)
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/frontend</code>
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>
* Все остальные модули
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code> (Например: <code>node-role.deckhouse.io/cert-manager</code>)
        * Наличие ноды с лейблом <code>node-role.deckhouse.io/system</code>
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>)
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>
{% endraw %}
{% endofftopic %}
