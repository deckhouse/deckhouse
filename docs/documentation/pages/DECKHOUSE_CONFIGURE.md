---
title: "How to configure?"
permalink: en/
---

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — набор helm-чартов, хуков, файлов и правил сборки компонентов модуля (компонентов Deckhouse).

Поведение Deckhouse настраивается с помощью:
- [Глобальных настроек](deckhouse-configure-global.html#параметры), хранящихся в параметре `global` [конфигурации Deckhouse](#конфигурация-deckhouse);
- Настроек модулей, хранящихся в [конфигурации Deckhouse](#конфигурация-deckhouse) и custom resource'ах (для некоторых модулей Deckhouse).

## Deckhouse configration

Конфигурация Deckhouse хранится в ConfigMap `deckhouse` в пространстве имен `d8-system` и может содержать следующие параметры (ключи):
- `global` —  содержит [глобальные настройки](deckhouse-configure-global.html) Deckhouse в виде multi-line-строки в формате YAML;
- `<moduleName>` (где `<moduleName>` — название модуля Deckhouse в camelCase) — содержит [настройки модуля](#настройка-модуля) в виде multi-line-строки в формате YAML;
- `<moduleName>Enabled` (где `<moduleName>` — название модуля Deckhouse в camelCase) — параметр позволяет явно [включить или отключить модуль](#включение-и-отключение-модуля).

Use the following command to view the `deckhouse` ConfigMap:

```shell
kubectl -n d8-system get cm/deckhouse -o yaml
```

Example of the `deckhouse` ConfigMap:
```yaml
apiVersion: v1
metadata:
  name: deckhouse
  namespace: d8-system
data:
  global: |          # Note the vertical bar.
    # Section of the YAML file with global settings
    modules:
      publicDomainTemplate: "%s.kube.company.my"
  #  monitoring-ping related section of the YAML file.
  monitoringPing: |
    externalTargets:
    - host: 8.8.8.8
    config:
      hsts: true
  # Disabling the dashboard module.
  dashboardEnabled: "false"
```

Pay attention to the following:
- The `|` sign — vertical bar glyph that must be specified, because the parameter being passed is a multi-line string, not an object;
- A module name is in *camelCase* style.

Use the following command to edit the `deckhouse` ConfigMap:

```shell
kubectl -n d8-system edit cm/deckhouse
```

### Настройка модуля

> При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/VALUES.md). Будем признательны, если поставите проекту *звезду*.

Deckhouse only installs the modules that are enabled. Modules can be enabled or disabled by default, depending on the [bundle used](./modules/020-deckhouse/configuration.html#parameters-bundle). Читайте подробнее про явное [enabling and disabling the module](#enabling-and-disabling-the-module).

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

## Enabling and disabling the module

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#module-bundles).

Для включения или отключения модуля необходимо добавить в ConfigMap `deckhouse` параметр `<moduleName>Enabled`, который может принимать одно из двух значений: `"true"` или `"false"` (кавычки обязательны), где `<moduleName>` — название модуля в camelCase.

Here is an example of enabling the `user-authn` module:
```yaml
data:
  userAuthnEnabled: "true"
```

## Module bundles

Deckhouse работает только с включёнными модулями.

В зависимости от используемого [набора модулей](./modules/020-deckhouse/configuration.html#parameters-bundle) модули могут быть включены или выключены по умолчанию.

{%- assign bundles = site.data.bundles | sort %}
<table>
<thead>
<tr><th>Bundle name</th><th>List of modules, enabled by default</th></tr></thead>
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

## Advanced scheduling

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:
1. If the `nodeSelector` module parameter is not set, then Deckhouse will try to calculate the `nodeSelector` automatically. Deckhouse looks for nodes with the specific labels in the cluster  (see the list below). If there are any, then the corresponding `nodeSelectors` are automatically applied to module resources;
1. If the `tolerations` parameter is not set for the module, all the possible tolerations are automatically applied to the module's Pods (see the list below);
1. You can set both parameters to `false` to disable their automatic calculation.

You cannot set `nodeSelector` and `tolerations` for modules:
- that involve running a DaemonSet on all cluster nodes (e.g., `cni-flannel`, `monitoring-ping`);
- designed to run on master nodes (e.g., `prometheus-metrics-adapter` or some `vertical-pod-autoscaler` components).

### Особенности автоматики, зависящие от типа модуля
{% raw %}
* The *monitoring*-related modules (operator-prometheus, prometheus and vertical-pod-autoscaler):
  * Deckhouse examines nodes to determine a nodeSelector in the following order:
    * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster;
    * It checks if a node with the <code>node-role.deckhouse.io/monitoring</code> label is present in the cluster;
    * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster;
  * Tolerations to add (note that tolerations are added all at once):
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>

      (e.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}</code>);
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}</code>;
    * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>;
* The *frontend*-related modules (nginx-ingress only):
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> label is present in the cluster;
        * It checks if a node with the <code>node-role.deckhouse.io/frontend</code> label is present in the cluster;
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code>;
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}</code>;
* Other modules:
    * Deckhouse examines nodes to determine a nodeSelector in the following order:
        * It checks if a node with the <code>node-role.deckhouse.io/MODULE_NAME</code> 
        
          (e.g., <code>node-role.deckhouse.io/cert-manager</code>) label is present in the cluster;
        * It checks if a node with the <code>node-role.deckhouse.io/system</code> label is present in the cluster;
    * Tolerations to add (note that tolerations are added all at once):
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}</code> 
        
          (e.g., <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}</code>);
        * <code>{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}</code>;
{% endraw %}
