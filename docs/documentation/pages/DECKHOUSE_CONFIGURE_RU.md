---
title: "Как настроить?"
permalink: ru/
lang: ru
---

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — набор helm-чартов, хуков, файлов и правил сборки компонентов модуля (компонентов Deckhouse).

Поведение Deckhouse настраивается с помощью:
- [Глобальных настроек](deckhouse-configure-global.html#параметры), хранящихся в параметре `global` [конфигурации Deckhouse](#конфигурация-deckhouse).
- Настроек модулей, хранящихся в [конфигурации Deckhouse](#конфигурация-deckhouse) и custom resource'ах (для некоторых модулей Deckhouse).

## Конфигурация Deckhouse

Конфигурация Deckhouse хранится в ConfigMap `deckhouse` в пространстве имен `d8-system` и может содержать следующие параметры (ключи):
- `global` —  содержит [глобальные настройки](deckhouse-configure-global.html) Deckhouse в виде multi-line-строки в формате YAML;
- `<moduleName>` (где `<moduleName>` — название модуля Deckhouse в camelCase) — содержит [настройки модуля](#настройка-модуля) в виде multi-line-строки в формате YAML;
- `<moduleName>Enabled` (где `<moduleName>` — название модуля Deckhouse в camelCase) — параметр позволяет явно [включить или отключить модуль](#включение-и-отключение-модуля).

Чтобы посмотреть конфигурацию Deckhouse выполните следующую команду:
```shell
kubectl -n d8-system get cm/deckhouse -o yaml
```

Пример ConfigMap `deckhouse`:
```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # Вертикальная черта.
    # Глобальные настройки в формате YAML.
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  # Настройки модуля monitoring-ping в формате YAML.
  monitoringPing: |
    externalTargets:
    - host: 8.8.8.8
  # Отключение модуля dashboard.
  dashboardEnabled: "false"
```

Обратите внимание на несколько важных нюансов в конфигурации:
* Символ `|` (вертикальная черта) обязательно должен быть указан в параметрах настройки, т.к. передаваемое значение — многострочная строка (multi-line string), а не объект.
* Наименование модулей пишется в стиле *camelCase*.

Чтобы изменить конфигурацию Deckhouse отредактируйте ConfigMap `deckhouse`, например, следующим способом:
```shell
kubectl -n d8-system edit cm/deckhouse
```

После сохранения конфигурации Deckhouse изменения применяются автоматически.

### Настройка модуля

> При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/VALUES.md). Будем признательны, если поставите проекту *звезду*.

Deckhouse работает только с включёнными модулями. В зависимости от используемого [набора модулей](#наборы-модулей) модули могут быть включены или выключены по умолчанию. Читайте подробнее про явное [включение или отключение модуля](#включение-и-отключение-модуля).

Модуль настраивается в конфигурации Deckhouse в параметре с названием модуля в camelCase. Значением параметра передается multi-line-строка в формате YAML с настройками модуля.

Некоторые модули дополнительно настраиваются с помощью custom resource'ов. Воспользуйтесь поиском (наверху страницы) или найдите модуль в меню слева, чтобы получить документацию по его настройкам и используемым custom resource'ам.

Пример настройки параметров модуля `kube-dns`:
```yaml
data:
  kubeDns: |
    stubZones:
    - upstreamNameservers:
      - 192.168.121.55
      - 10.2.7.80
      zone: directory.company.my
    upstreamNameservers:
    - 10.2.100.55
    - 10.2.200.55
```

### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для включения или отключения модуля необходимо добавить в ConfigMap `deckhouse` параметр `<moduleName>Enabled`, который может принимать одно из двух значений: `"true"` или `"false"` (кавычки обязательны), где `<moduleName>` — название модуля в camelCase.

Пример включения модуля `user-authn`:
```yaml
data:
  userAuthnEnabled: "true"
```

## Наборы модулей

В зависимости от используемого [набора модулей](./modules/020-deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

{%- assign bundles = site.data.bundles | sort %}
<table>
<thead>
<tr><th>Набор модулей (bundle)</th><th>Список включенных по умолчанию модулей</th></tr></thead>
<tbody>
{% for bundle in bundles %}
<tr>
<td><strong>{{ bundle[0] |  replace_first: "values-", "" | capitalize }}</strong></td>
<td>{% assign modules = bundle[1] | sort %}
<ul style="columns: 3">
{%- for module in modules %}
{%- assign moduleName = module[0] | regex_replace: "Enabled$", '' | camel_to_snake_case | replace: "_", '-' %}
{%- assign isExcluded = site.data.exclude.module_names | where: "name", moduleName %}
{%- if isExcluded.size > 0 %}{% continue %}{% endif %}
{%- if module[1] != true %}{% continue %}{% endif %}
<li>
{{ module[0] | regex_replace: "Enabled$", '' | camel_to_snake_case | replace: "_", '-' }}</li>
{%- endfor %}
</ul>
</td>
</tr>
{%- endfor %}
</tbody>
</table>

## Выделение узлов под определенный вид нагрузки

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:
1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с [лейблами из списка или лейблами определенного формата](#особенности-автоматики-зависящие-от-типа-модуля), Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля.
1. Если параметр `tolerations` модуля не указан, то Pod'ам модуля автоматически устанавливаются все возможные toleration'ы ([подробнее](#особенности-автоматики-зависящие-от-типа-модуля)).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно, указав значение `false`.

Возможность настройки `nodeSelector` и `tolerations` отключена для модулей:
- которые работают на всех узлах кластера (например, `cni-flannel`, `monitoring-ping`);
- которые работают на всех master-узлах (например, `prometheus-metrics-adapter`, `vertical-pod-autoscaler`).

### Особенности автоматики, зависящие от типа модуля
{% raw %}
* Модули *monitoring* (`operator-prometheus`, `prometheus` и `vertical-pod-autoscaler`):
  * Порядок поиска узлов (для определения `nodeSelector`):
    * Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    * Наличие узла с лейблом `node-role.deckhouse.io/monitoring`.
    * Наличие узла с лейблом `node-role.deckhouse.io/system`.
  * Добавляемые toleration'ы (добавляются одновременно все):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`;

      Например: `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}`;
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}`;
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`;
* Модули *frontend* (исключительно модуль `ingress-nginx`):
    * Порядок поиска узлов (для определения `nodeSelector`):
        * Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
        * Наличие узла с лейблом `node-role.deckhouse.io/frontend`.
    * Добавляемые toleration'ы (добавляются одновременно все):
        * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`;
        * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}`;
* Все остальные модули:
    * Порядок поиска узлов (для определения `nodeSelector`):
        * Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.

          Например: `node-role.deckhouse.io/cert-manager`.
        * Наличие узла с лейблом `node-role.deckhouse.io/system`.
    * Добавляемые toleration'ы (добавляются одновременно все):
        * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`;

          Например: `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}`;
        * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
{% endraw %}
