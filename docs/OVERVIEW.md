---
title: "Общие сведения"
permalink: /overview.html
---

## Конфигурация Deckhouse

Конфигурация самого *Deckhouse* и его модулей находится в одном месте - ConfigMap `deckhouse` в namespace `d8-system`. Некоторые модули помимо своей конфигурации настраиваются с учетом специальных custom resource в кластере. Описание параметров конфигурации модуля и используемых модулем custom resource'ов можно найти в описании модуля или функций подсистемы.

Конфигурация `deckhouse` (ConfigMap deckhouse`) состоит из [глобальной секции](#глобальная-конфигурация) и секции модулей.

Редактирование конфигурации `deckhouse`:
```
kubectl -n d8-system edit cm/deckhouse
```

### Пример ConfigMap `deckhouse`

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
      publicDomainTemplate: "%s.kube.domain.my"
  nginxIngress: |
    # Тут кусок Yaml-файла, касающийся модуля nginx-ingress
    config:
      hsts: true
  someModuleName: |  # <--- тут всегда camel case от названия модуля
    foo: bar
  dashboardEnabled: "false"   # <--- вот так можно отключить модуль
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

## Глобальная конфигурация

### Что нужно настроить?

Нужно обязательно настроить **project** и **clusterName** и желательно настроить **modules.publicDomainTemplate**.

```yaml
global: |
  project: someproject
  clusterName: main
  modules:
    publicDomainTemplate: "%s.kube.domain.my"
```

### Параметры

* `project` (обязательно) — имя проекта.
* `clusterName` (обязательно) — имя кластера (должно соответствовать имени кластера в `***REMOVED***_registry`).
* `modules` — параметры для служебных компонентов;
  * `publicDomainTemplate` (желательно) — шаблон c ключом "%s" в качестве динамической части строки. Будет использоваться для образования служебных доменов (например, `%s.kube.domain.my`). Если параметр не указан, то ingress-ресурсы создаваться не будут.
  * `ingressClass` — класс ingress контроллера, который используется для служебных компонентов.
    * По умолчанию `nginx`.
  * `placement` — настройки, определяющие расположение компонентов Deckhouse.
    * `customTolerationKeys` — список ключей пользовательских taint'ов, необходимо указывать, чтобы позволить выезжать на выделенные ноды критическим add-on'ам, таким как например cni и csi.
      * Пример:
        ```yaml
        customTolerationKeys:
        - dedicated.example.com
        - node-dedicated.example.com/master
        ```
  * `https` — способ реализации HTTPS, используемый служебными компонентами.
    * `mode` — режим работы HTTPS:
      * `Disabled` — в данном режиме все служебные компоненты будут работать только по http (некоторые модули могут не работать, например [user-authn](/modules/150-user-authn));
      * `CertManager` — все служебные компоненты будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
      * `CustomCertificate` — все служебные компоненты будут работать по https используя сертификат из namespace `d8-system`;
      * `OnlyInURI` — все служебные компоненты будут работать по http (подразумевая, что перед ними стоит внешний https-балансер, который терминирует https).
      * По умолчанию `CertManager`.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для служебных компонентов (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для системных компонентов (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По умолчанию `false`.
* `storageClass` — имя storage class, который будет использоваться для всех служебных компонентов (prometheus, grafana, openvpn, ...).
    * По умолчанию — null, а значит служебные будут использовать `cluster.defaultStorageClass` (который определяется автоматически), а если такого нет — `emptyDir`.
    * Этот параметр имеет смысл использовать только в исключительных ситуациях.
* `highAvailability` — глобальный включатель режима отказоустойчивости для модулей, которые это поддерживают. По умолчанию не определён и решение принимается на основе autodiscovery-параметра `global.discovery.clusterControlPlaneIsHighlyAvailable`.

## Включение и отключение модуля

Deckhouse устанавливает только включённые [модули](https://github.com/flant/addon-operator/blob/master/MODULES.md). Смотри подробнее про алгоритм определения включённости модуля [тут](https://github.com/flant/addon-operator/blob/master/LIFECYCLE.md#modules-discovery).

Модули могут быть включены или выключены по умолчанию, исходя из используемого [варианта поставки](/modules/020-deckhouse/configuration.html).

Для включения/отключения модуля, необходимо добавить в configMap `deckhouse` параметр `<moduleName>Enabled` — `"true"` или `"false"`, где `<moduleName>` — название модуля в camelCase.

Пример включения модуля user-authn
```yaml
data:
  userAuthnEnabled: "true"
```
