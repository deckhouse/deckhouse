---
title: "Как настроить?"
permalink: ru/admin/configuration/
description: "Узнайте, как настроить платформу Deckhouse Kubernetes Platform с помощью глобальных настроек, конфигураций модулей и пользовательских ресурсов. Руководство по настройке DKP."
lang: ru
---

## Основы конфигурации Deckhouse

Deckhouse конфигурируется с помощью:

- **[Глобальных настроек](../../reference/api/global.html).** Глобальные настройки хранятся в ресурсе `ModuleConfig/global`. Эти настройки можно рассматривать как специальный модуль `global`, который нельзя отключить.
- **[Настроек модулей](#настройка-модуля).** Настройки каждого модуля хранятся в ресурсе `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case).
- **Кастомных ресурсов.** Некоторые модули настраиваются с помощью дополнительных кастомных ресурсов.

Пример набора кастомных ресурсов конфигурации Deckhouse:

```yaml
# Глобальные настройки.
apiVersion: deckhouse.io/v1alpha1
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
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  settings:
    externalTargets:
    - host: 8.8.8.8
---
# Отключить модуль dashboard.
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  enabled: false
```

Посмотреть список кастомных ресурсов `ModuleConfig`, состояние модулей (включен/выключен) и их статус можно с помощью следующей команды:

```shell
d8 k get moduleconfigs
```

{% offtopic title="Пример вывода..." %}

```console
$ d8 k get moduleconfigs
NAME            ENABLED   VERSION   AGE     MESSAGE
deckhouse       true      1         12h
documentation   true      1         12h
global                    1         12h
prometheus      true      2         12h
upmeter         false     2         12h
```

{% endofftopic %}

Чтобы изменить глобальную конфигурацию Deckhouse или конфигурацию модуля, нужно создать или отредактировать соответствующий ресурс `ModuleConfig`.

Например, чтобы отредактировать конфигурацию модуля `upmeter`, выполните следующую команду:

```shell
d8 k edit moduleconfig/upmeter
```

После завершения редактирования изменения применяются автоматически.

### Изменение конфигурации кластера

{% alert level="warning" %}
Для применения изменений конфигурации узлов необходимо выполнить команду `dhctl converge`, запустив инсталлятор DKP. Эта команда синхронизирует состояние узлов с указанным в конфигурации.
{% endalert %}

Общие параметры кластера хранятся в структуре [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
d8 platform edit cluster-configuration
```

После сохранения изменений DKP автоматически приведёт кластер в соответствие с новой конфигурацией. В зависимости от размеров кластера это может занять некоторое время.

### Просмотр текущих настроек

DKP управляется с помощью глобальных настроек, настроек модулей и различных кастомных ресурсов.

1. Для просмотра глобальных настроек выполните:

   ```shell
   d8 k get mc global -o yaml
   ```

1. Для просмотра состояния всех модулей (доступно для Deckhouse версии 1.47+):

   ```shell
   d8 k get modules
   ```

1. Для просмотра настроек модуля [`user-authn`](/modules/user-authn/):

   ```shell
   d8 k get moduleconfigs user-authn -o yaml
   ```

## Настройка модуля

Модуль настраивается с помощью ресурса [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig), имя которого совпадает с именем модуля (в kebab-case). ModuleConfig имеет следующие поля:

- `metadata.name` — название модуля Deckhouse в kebab-case (например, `prometheus`, `node-manager`);
- `spec.version` — версия схемы настроек модуля (целое число, больше нуля). Обязательное поле, если `spec.settings` не пустое. Номер актуальной версии можно увидеть в документации модуля в разделе *Настройки*:
  - Deckhouse поддерживает обратную совместимость версий схемы настроек модуля. Если используется схема настроек устаревшей версии, при редактировании или просмотре кастомного ресурса будет выведено предупреждение о необходимости обновить схему настроек модуля;
- `spec.settings` — настройки модуля. Необязательное поле, если используется поле `spec.enabled`. Описание возможных настроек можно найти в документации модуля в разделе *Настройки*;
- `spec.enabled` — необязательное поле для явного [включения или отключения модуля](#включение-и-отключение-модуля). Если не задано, модуль может быть включен по умолчанию в одном из [наборов модулей](#наборы-модулей).

> Deckhouse не изменяет кастомные ресурсы ModuleConfig. Это позволяет применять подход Infrastructure as Code (IaC) при хранении конфигурации. Другими словами, можно воспользоваться всеми преимуществами системы контроля версий для хранения настроек Deckhouse, использовать Helm, `d8 k` и другие привычные инструменты.

Пример кастомного ресурса для настройки [модуля `kube-dns`](/modules/kube-dns/):

```yaml
apiVersion: deckhouse.io/v1alpha1
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

Некоторые модули настраиваются с помощью дополнительных ресурсов. Воспользуйтесь поиском (вверху страницы) или выберите модуль в меню слева, чтобы просмотреть документацию по его настройкам и используемым кастомным ресурсам.

### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для явного включения или отключения модуля необходимо установить `true` или `false` в [поле `.spec.enabled`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig-v1alpha1-spec-enabled) в соответствующем кастомном ресурсе ModuleConfig. Если для модуля нет такого кастомного ресурса ModuleConfig, его нужно создать.

Пример явного выключения модуля [`user-authn`](/modules/user-authn/) (модуль будет выключен независимо от используемого набора модулей):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

Проверить состояние модуля можно с помощью команды `d8 k get moduleconfig <ИМЯ_МОДУЛЯ>`.

Пример:  

```shell
$ d8 k get moduleconfig user-authn
NAME         ENABLED   VERSION   AGE   MESSAGE
user-authn   false     1         12h
```

## Наборы модулей

В зависимости от используемого [набора модулей](/modules/deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

<table>
<thead>
<tr><th>Набор модулей (bundle)</th><th>Список включенных по умолчанию модулей</th></tr></thead>
<tbody>
{% for bundle in site.data.bundles.bundleNames %}
<tr>
<td><strong>{{ bundle }}</strong></td>
<td>
<ul style="columns: 3">
{%- for moduleName in site.data.bundles.bundleModules[bundle] %}
{%- if site.data.excludedModules contains moduleName %}{% continue %}{% endif %}
<li>{{ moduleName }}</li>
{%- endfor %}
</ul>
</td>
</tr>
{%- endfor %}
</tbody>
</table>

### Особенности работы с набором модулей Minimal

{% alert level="warning" %}
**Обратите внимание,** что в наборе модулей `Minimal` не включен ряд базовых модулей (например, модуль работы с CNI).

Deckhouse с набором модулей `Minimal` без включения базовых модулей сможет работать только в уже развернутом кластере.
{% endalert %}

Для установки Deckhouse с набором модулей `Minimal` включите как минимум следующие модули, указав их в конфигурационном файле установщика:

* [`cni-cilium`](/modules/cni-cilium/) или другой модуль управления CNI (при необходимости);
* [`control-plane-manager`](/modules/control-plane-manager/);
* [`kube-dns`](/modules/kube-dns/);
* [`terraform-manager`](/modules/terraform-manager/), в случае развертывания облачного кластера;
* [`node-manager`](/modules/node-manager/);
* `registry-packages-proxy`;
* модуль облачного провайдера (например, [`cloud-provider-aws`](/modules/cloud-provider-aws/) для AWS), в случае развертывания облачного кластера.

### Доступ к документации текущей версии

Документация запущенной в кластере версии Deckhouse доступна по адресу `documentation.<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из [параметра `modules.publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.

{% alert level="warning" %}
Документация доступна, если в кластере включен модуль [documentation](/modules/documentation/). Он включен по умолчанию, кроме [варианта поставки](/modules/deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

## Управление размещением компонентов Deckhouse

### Выделение узлов под определенный вид нагрузки

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:
1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с [лейблами из списка или лейблами определенного формата](#особенности-автоматики-зависящие-от-типа-модуля), Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля.
1. Если параметр `tolerations` модуля не указан, то подам модуля автоматически устанавливаются все возможные toleration'ы, ([подробнее](#особенности-автоматики-зависящие-от-типа-модуля)).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно, указав значение `false`.
1. При отсутствии в кластере [выделенных узлов](#особенности-автоматики-зависящие-от-типа-модуля) и автоматическом выборе `nodeSelector` (см. п. 1), `nodeSelector` в ресурсах модуля указан не будет. Модуль в таком случае будет использовать любой узел с не конфликтующими `taints`.

Возможность настройки `nodeSelector` и `tolerations` отключена для модулей:

- которые работают на всех узлах кластера (например, [`cni-flannel`](/modules/cni-flannel/), [`monitoring-ping`](/modules/monitoring-ping/));
- которые работают на всех master-узлах (например, [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/), [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/)).

### Особенности автоматики, зависящие от типа модуля

{% raw %}
* Модули *monitoring* ([`operator-prometheus`](/modules/operator-prometheus/), [`prometheus`](/modules/prometheus/) и [`vertical-pod-autoscaler`](/modules/vertical-pod-autoscaler/)):
  * Порядок поиска узлов (для определения [`nodeSelector`](/modules/prometheus/configuration.html#parameters-nodeselector)):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    1. Наличие узла с лейблом `node-role.deckhouse.io/monitoring`.
    1. Наличие узла с лейблом `node-role.deckhouse.io/system`.
  * Добавляемые toleration'ы (добавляются одновременно все):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (например, `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}`);
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}`;
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
* Модули *frontend* (исключительно [модуль `ingress-nginx`](/modules/ingress-nginx/)):
  * Порядок поиска узлов (для определения `nodeSelector`):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    1. Наличие узла с лейблом `node-role.deckhouse.io/frontend`.
  * Добавляемые toleration'ы (добавляются одновременно все):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}`;
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"frontend"}`.
* Все остальные модули:
  * Порядок поиска узлов (для определения `nodeSelector`):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME` (например, `node-role.deckhouse.io/cert-manager`).
    1. Наличие узла с лейблом `node-role.deckhouse.io/system`.
  * Добавляемые toleration'ы (добавляются одновременно все):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (например, `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"network-gateway"}`);
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
{% endraw %}
