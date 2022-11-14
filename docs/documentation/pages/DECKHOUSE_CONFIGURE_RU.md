---
title: "Как настроить?"
permalink: ru/
lang: ru
---

Deckhouse состоит из оператора Deckhouse и модулей. Модуль это набор из Helm-чарта, хуков Addon-operator'а, других файлов и правил сборки компонентов модуля (компонентов Deckhouse).

Поведение Deckhouse настраивается с помощью:
- [Глобальных настроек](deckhouse-configure-global.html#параметры), хранящихся в ресурсе `ModuleConfig/global`.
- Настроек модулей, хранящихся в ресурсах `ModuleConfig` и, для некоторых модулей, в дополнительных custom resource'ах.

## Конфигурация Deckhouse

Конфигурация Deckhouse хранится в ресурсах `ModuleConfig` и может содержать следующие параметры:

- `metadata.name` — имя ресурса совпадает с именем модуля Deckhouse в виде kebab-case.
- `spec.version` — числовой параметр, версия настроек модуля больше 0.
- `spec.settings` — объект с настройками модуля.
- `spec.enabled` — опциональный флаг для явного [включения или отключения модуля](#включение-и-отключение-модуля). Если флаг не указан, модуль может быть включён по умолчанию в [наборе модулей](#наборы-модулей).

Если объект `spec.settings` не пустой, то в поле `spec.version` нужно обязательно указать версию настроек. Номер актуальной версия есть в описании настроек модуля.

С новыми релизами версия настроек может устареть. Чтобы управлять ресурсами ModuleConfig в стиле IaC, Deckhouse будет поддерживать настройки предыдущих версий. При редактировании ресурса и при просмотре списка ресурсов будет предупреждение о необходимости обновить `spec.settings` и `spec.version`.

В ресурсе `ModuleConfig/global` хранятся глобальные настройки. "global" нельзя выключить, поэтому значение в параметре `spec.enabled` игнорируется.

Пример ресурсов ModuleConfig:

```yaml
# Глобальные настройки.
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings:
    modules:
      publicDomainTemplate: "%s.kube.company.my"
---
# Настройки модуля monitoring-ping.
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  settings:
    externalTargets:
    - host: 8.8.8.8
---
# Модуль dashboard отключен.
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  enabled: false
```

В поле `status` добавляется состояние модуля, поэтому можно увидеть состояние после изменения настроек модуля простой командой kubectl:

```shell
kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
deckhouse           1         12h   Enabled              Ready
deckhouse-web       2         12h   Enabled              Ready
global              1         12h   Always On
prometheus          2         12h   Enabled              Ready
upmeter             2         12h   Disabled by config
```

Чтобы изменить конфигурацию Deckhouse, нужно создать или отредактировать ресурс ModuleConfig с именем модуля и указать нужные настройки. Например, чтобы настроить модуль `upmeter`, можно использовать такую команду:

```shell
kubectl -n d8-system edit moduleconfig/upmeter
```

После сохранения конфигурации Deckhouse изменения применяются автоматически.

Оператор Deckhouse не изменяет ресурсы `ModuleConfig`, поэтому ими можно управлять используя kubectl, Helm, Git и другие привычные инструменты IaC.

### Настройка модуля

> При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/VALUES.md). Будем признательны, если поставите проекту *звезду*.

Deckhouse работает только с включёнными модулями. В зависимости от используемого [набора модулей](#наборы-модулей) модули могут быть включены или выключены по умолчанию. Читайте подробнее про явное [включение или отключение модуля](#включение-и-отключение-модуля).

Модуль настраивается через ресурс `ModuleConfig` имя которого совпадает с именем модуля.

Пример настройки параметров модуля `kube-dns`:

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  settings:
    stubZones:
    - upstreamNameservers:
      - 192.168.121.55
      - 10.2.7.80
      zone: directory.company.my
    upstreamNameservers:
    - 10.2.100.55
    - 10.2.200.55
```

Некоторые модули дополнительно настраиваются с помощью custom resource'ов. Воспользуйтесь поиском (наверху страницы) или найдите модуль в меню слева, чтобы получить документацию по его настройкам и используемым custom resource'ам.

### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для включения или отключения модуля необходимо установить true или false в поле `.spec.enabled` в соответствующем ресурсе `ModuleConfig`. Если для модуля нет ресурса `ModuleConfig`, то нужно его создать.

Пример выключения модуля `user-authn`, включённого в наборе 'default':

```yaml
apiVersion: deckhouse.io/v1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

```shell
kubectl get moduleconfigs
NAME                VERSION   AGE   ENABLED              STATUS
user-authn          1         12h   Disabled by config
```

## Наборы модулей

В зависимости от используемого [набора модулей](./modules/002-deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

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

## Управление размещением компонентов Deckhouse

### Выделение узлов под определенный вид нагрузки

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
