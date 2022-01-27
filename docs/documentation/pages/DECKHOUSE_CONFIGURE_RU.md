---
title: "Как настроить?"
permalink: ru/
lang: ru
---

Конфигурация *Deckhouse* и его модулей находится в одном месте — в ConfigMap'е `deckhouse` в пространстве имен `d8-system`. Некоторые модули дополнительно настраиваются с использованием специальных custom resource'ов, описание которых, как и описание общей конфигурации, можно найти в документации модуля (воспользуйтесь поиском по названию модуля или custom resource на сайте).

Конфигурация Deckhouse (ConfigMap `deckhouse`) состоит из [глобальной секции](deckhouse-configure-global.html) и секции модулей.

Для редактирования конфигурации Deckhouse выполните следующую команду:

```shell
kubectl -n d8-system edit cm/deckhouse
```

## Пример ConfigMap `deckhouse`

При редактировании конфигурации обратите особое внимание на несколько важных нюансов:

* Символ `|`, вертикальная черта, которая обязательно должна быть указана, т.к. передаваемый параметр — многострочная строка (multi-line string), а не объект;
* Наименование модулей пишется в стиле *camelCase*, при котором несколько слов пишутся слитно без пробелов, при этом каждое слово внутри фразы пишется с прописной буквы.

```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # Вертикальная черта.
    # Тут кусок YAML-файла с глобальными настройками.
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  nginxIngress: |
    # Тут кусок YAML-файла, касающийся модуля nginx-ingress.
    config:
      hsts: true
  someModuleName: |  # Написание модуля в стиле camelCase.
    foo: bar
  dashboardEnabled: "false"   # Пример отключения модуля.
```

## Включение и отключение модуля

Deckhouse может установить только включённые [модули](https://github.com/flant/addon-operator/blob/master/MODULES.md). Подробнее про определение состояния модуля можно почитать [в документации](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md#modules-discovery).

В зависимости от используемого [варианта поставки](./modules/020-deckhouse/configuration.html) модули могут быть включены или выключены по умолчанию.

Для включения или отключения модуля необходимо добавить в ConfigMap `deckhouse` параметр `<moduleName>Enabled`, который может принимать одно из двух значений: `"true"` или `"false"`, где `<moduleName>` — название модуля в camelCase.

Пример включения модуля user-authn:

```yaml
data:
  userAuthnEnabled: "true"
```

## Выделение узлов под определенный вид нагрузки

Для всех модулей принята единая стратегия:

1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с лейблами из определенного списка или лейблами определенного формата (подробнее — ниже, под спойлером), то Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля;
1. Если параметр `tolerations` модуля не указан, то Pod'ам модуля автоматически устанавливаются все возможные toleration'ы (подробнее — ниже, под спойлером);
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно указав значение `false`.

>**Важно!** Если модуль предполагает работу DaemonSet'a на всех узлах кластера (например, `cni-flannel`, `monitoring-ping`) или он должен работать на master-узлах (например, `prometheus-metrics-adapter` или некоторые компоненты `vertical-pod-autoscaler`) — то у таких модулей возможность настройки `nodeSelector` и `tolerations` отключена.

{% offtopic title="Особенности автоматики, зависящие от **типа** модуля" %}{% raw %}
* Модули *monitoring* (operator-prometheus, prometheus и vertical-pod-autoscaler):
  * Порядок поиска узлов (для определения nodeSelector):
    * Наличие узла с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>;
    * Наличие узла с лейблом <code>node-role.deckhouse.io/monitoring</code>;
    * Наличие узла с лейблом <code>node-role.deckhouse.io/system</code>;
  * Добавляемые toleration'ы (добавляются одновременно все):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

      (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>);
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>;
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>;
* Модули *frontend* (исключительно nginx-ingress):
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие узла с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code>;
        * Наличие узла с лейблом <code>node-role.deckhouse.io/frontend</code>;
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>;
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>;
* Все остальные модули:
    * Порядок поиска узлов (для определения nodeSelector):
        * Наличие узла с лейблом <code>node-role.deckhouse.io/MODULE_NAME</code> (Например: <code>node-role.deckhouse.io/cert-manager</code>);
        * Наличие узла с лейблом <code>node-role.deckhouse.io/system</code>;
    * Добавляемые toleration'ы (добавляются одновременно все):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> 
        
        (Например: <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>);
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>.
{% endraw %}
{% endofftopic %}
