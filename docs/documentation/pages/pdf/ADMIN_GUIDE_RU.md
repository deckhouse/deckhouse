---
title: "Deckhouse Kubernetes Platform: Руководство администратора"
permalink: ru/deckhouse-admin-guide.html
lang: ru
sidebar: none
toc: true
layout: pdf
---

## Как настроить?

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — это набор из Helm-чарта, хуков Addon-operator'а, правил сборки компонентов модуля (компонентов Deckhouse) и других файлов.

<div markdown="0" style="height: 0;" id="конфигурация-deckhouse"></div>

Deckhouse конфигурируется с помощью:
- **[Глобальных настроек](deckhouse-configure-global.html).** Глобальные настройки хранятся в ресурсе `ModuleConfig/global`. Эти настройки можно рассматривать как специальный модуль `global`, который нельзя отключить.
- **[Настроек модулей](#настройка-модуля).** Настройки каждого модуля хранятся в ресурсе `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case).
- **Кастомных ресурсов.** Некоторые модули настраиваются с помощью дополнительных кастомных ресурсов.

Пример набора кастомных ресурсов конфигурации Deckhouse:

```yaml
### Глобальные настройки.
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
### Настройки модуля monitoring-ping.
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
### Отключить модуль dashboard.
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: dashboard
spec:
  enabled: false
```

Посмотреть список кастомных ресурсов `ModuleConfig`, состояние модулей (включен/выключен) и их статус можно с помощью команды `kubectl get moduleconfigs`:

```shell
$ kubectl get moduleconfigs
NAME            ENABLED   VERSION   AGE     MESSAGE
deckhouse       true      1         12h
documentation   true      1         12h
global                    1         12h
prometheus      true      2         12h
upmeter         false     2         12h
```

Чтобы изменить глобальную конфигурацию Deckhouse или конфигурацию модуля, нужно создать или отредактировать соответствующий ресурс `ModuleConfig`.

Например, чтобы отредактировать конфигурацию модуля `upmeter`, выполните следующую команду:

```shell
kubectl edit moduleconfig/upmeter
```

После завершения редактирования изменения применяются автоматически.

#### Настройка модуля

> При работе с модулями Deckhouse использует проект addon-operator. Ознакомьтесь с его документацией, если хотите понять, как Deckhouse работает с модулями, хуками модулей и параметрами модулей. Будем признательны, если поставите проекту *звезду*.

Модуль настраивается с помощью ресурса `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case). `ModuleConfig` имеет следующие поля:

- `metadata.name` — название модуля Deckhouse в kebab-case (например, `prometheus`, `node-manager`);
- `spec.version` — версия схемы настроек модуля (целое число, больше нуля). Обязательное поле, если `spec.settings` не пустое. Номер актуальной версии можно увидеть в документации модуля в разделе *Настройки*:
  - Deckhouse поддерживает обратную совместимость версий схемы настроек модуля. Если используется схема настроек устаревшей версии, при редактировании или просмотре кастомного ресурса будет выведено предупреждение о необходимости обновить схему настроек модуля;
- `spec.settings` — настройки модуля. Необязательное поле, если используется поле `spec.enabled`. Описание возможных настроек можно найти в документации модуля в разделе *Настройки*;
- `spec.enabled` — необязательное поле для явного [включения или отключения модуля](#включение-и-отключение-модуля). Если не задано, модуль может быть включен по умолчанию в одном из [наборов модулей](#наборы-модулей).

> Deckhouse не изменяет кастомные ресурсы `ModuleConfig`. Это позволяет применять подход Infrastructure as Code (IaC) при хранении конфигурации. Другими словами, можно воспользоваться всеми преимуществами системы контроля версий для хранения настроек Deckhouse, использовать Helm, kubectl и другие привычные инструменты.

Пример кастомного ресурса для настройки модуля `kube-dns`:

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

##### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для явного включения или отключения модуля необходимо установить `true` или `false` в поле `.spec.enabled` в соответствующем кастомном ресурсе `ModuleConfig`. Если для модуля нет такого кастомного ресурса `ModuleConfig`, его нужно создать.

Пример явного выключения модуля `user-authn` (модуль будет выключен независимо от используемого набора модулей):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  enabled: false
```

Проверить состояние модуля можно с помощью команды `kubectl get moduleconfig <ИМЯ_МОДУЛЯ>`.

Пример:  

```shell
$ kubectl get moduleconfig user-authn
NAME         ENABLED   VERSION   AGE   MESSAGE
user-authn   false     1         12h
```

#### Наборы модулей

В зависимости от используемого [набора модулей](./modules/deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

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


#### Управление размещением компонентов Deckhouse

##### Выделение узлов под определенный вид нагрузки

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:
1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с [лейблами из списка или лейблами определенного формата](#особенности-автоматики-зависящие-от-типа-модуля), Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля.
1. Если параметр `tolerations` модуля не указан, то Pod'ам модуля автоматически устанавливаются все возможные toleration'ы ([подробнее](#особенности-автоматики-зависящие-от-типа-модуля)).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно, указав значение `false`.
1. При отсутствии в кластере [выделенных узлов](#особенности-автоматики-зависящие-от-типа-модуля) и автоматическом выборе `nodeSelector` (см. п. 1), `nodeSelector` в ресурсах модуля указан не будет. Модуль в таком случае будет использовать любой узел с не конфликтующими `taints`.

Возможность настройки `nodeSelector` и `tolerations` отключена для модулей:
- которые работают на всех узлах кластера (например, `cni-flannel`, `monitoring-ping`);
- которые работают на всех master-узлах (например, `prometheus-metrics-adapter`, `vertical-pod-autoscaler`).

##### Особенности автоматики, зависящие от типа модуля

{% raw %}
* Модули *monitoring* (`operator-prometheus`, `prometheus` и `vertical-pod-autoscaler`):
  * Порядок поиска узлов (для определения [nodeSelector](modules/prometheus/configuration.html#parameters-nodeselector)):
    1. Наличие узла с лейблом `node-role.deckhouse.io/MODULE_NAME`.
    1. Наличие узла с лейблом `node-role.deckhouse.io/monitoring`.
    1. Наличие узла с лейблом `node-role.deckhouse.io/system`.
  * Добавляемые toleration'ы (добавляются одновременно все):
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"MODULE_NAME"}` (например, `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"operator-prometheus"}`);
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"monitoring"}`;
    * `{"key":"dedicated.deckhouse.io","operator":"Equal","value":"system"}`.
* Модули *frontend* (исключительно модуль `ingress-nginx`):
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

## Глобальные настройки

Глобальные настройки Deckhouse хранятся в ресурсе `ModuleConfig/global` (см. [конфигурация Deckhouse](./#конфигурация-deckhouse)).

{% alert %}
В параметре [publicDomainTemplate](#parameters-modules-publicdomaintemplate) указывается шаблон DNS-имен, с учетом которого некоторые модули Deckhouse создают Ingress-ресурсы.

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом sslip.io или его аналогами.

Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре [clusterDomain](installing/configuration.html#clusterconfiguration-clusterdomain). Например, если `clusterDomain` установлен в `cluster.local` (значение по умолчанию), то `publicDomainTemplate` не может быть `%s.cluster.local`.
{% endalert %}

Пример ресурса `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings: # <-- Параметры модуля из раздела "Параметры" ниже.
    defaultClusterStorageClass: 'default-fast'
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M
      placement:
        customTolerationKeys:
        - dedicated.example.com
      storageClass: 'default-fast'
```

#### Параметры

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
## Настройка ПО безопасности

### Общие настройки

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка для исключения ложноположительных срабатываний.

Deckhouse Kubernetes Platform (DKP) использует следующие директории при работе ([скачать в csv](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

### KESL

##### KESL

Далее приведены рекомендации по настройке Kaspersky Endpoint Security for Linux (KESL) для обеспечения корректной работы с платформой Deckhouse Kubernetes Platform, независимо от выбранной редакции.

Для обеспечения совместимости с DKP на стороне KESL необходимо отключить следующие задачи:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
Список задач может отличаться в будущих версиях KESL.
{% endalert %}

Убедитесь, что узлы Kubernetes соответствуют минимальным требованиям к ресурсам, указанным для DKP и KESL.

При совместной эксплуатации KESL и DKP может потребоваться оптимизация производительности согласно рекомендациям Kaspersky.

### KUMA

##### KUMA

Kaspersky Unified Monitoring and Analysis Platform (KUMA) объединяет продукты «Лаборатории Касперского» и сторонних поставщиков в единую систему информационной безопасности и является ключевым компонентом на пути реализации комплексного защитного подхода, способного обезопасить от актуальных киберугроз корпоративную и индустриальную среду, а также наиболее эксплуатируемый злоумышленниками стык IT/OT-систем.

###### Описание настроек

{% alert level="warning" %}
Для работы с KUMA должен быть **обязательно включён** модуль [log-shipper](./modules/log-shipper/).
{% endalert %}

Для отправки данных в KUMA необходимо настроить на стороне DKP следующие ресурсы:

- [`ClusterLogDestination`](./modules/log-shipper/cr.html#clusterlogdestination);
- [`ClusterLoggingConfig`](./modules/log-shipper/cr.html#clusterloggingconfig).

{% alert level="info" %}
На стороне KUMA должны быть настроены необходимые ресурсы для приёма событий.
{% endalert %}

Ниже приведены примеры конфигурации отправки файла аудита `/var/log/kube-audit/audit.log` в различных форматах.

###### Отправка логов в формате JSON по UDP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-udp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: UDP
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  destinationRefs:
    - kuma-udp-json
```

###### Отправка логов в формате JSON по TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-json
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "JSON"
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  destinationRefs:
    - kuma-tcp-json
```

###### Отправка логов в формате CEF по TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-cef
spec:
  type: Socket
  socket:
    extraLabels:
      cef.name: d8
      cef.severity: "1"
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "CEF"
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
    - field: userAgent
      operator: Regex
      values: [ "kubelet.*" ]
  destinationRefs:
    - kuma-tcp-cef
```

###### Отправка логов в формате Syslog по TCP

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-tcp-syslog
spec:
  type: Socket
  socket:
    address: IP_ADDRESS:PORT # Заменить при настройке
    mode: TCP
    tcp:
      verifyCertificate: false
      verifyHostname: false
    encoding:
      codec: "Syslog"
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
    - field: userAgent
      operator: Regex
      values: [ "kubelet.*" ]
  destinationRefs:
    - kuma-tcp-syslog
```

###### Отправка логов в Apache Kafka

{% alert level="info" %}
При условии, что Apache Kafka настроена на приём данных.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: kuma-kafka
spec:
  type: Kafka
  kafka:
    bootstrapServers:
      - kafka-address:9092 # Заменить при настройке на актуальное значение
    topic: k8s-logs
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
  spec:
  destinationRefs:
  - kuma-kafka
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
  - field: userAgent
    operator: Regex
    values:
    - kubelet.*
  type: File
```

## FAQ

#### Как узнать все параметры Deckhouse?

Deckhouse настраивается с помощью глобальных настроек, настроек модулей и различных custom resource’ов. Подробнее — [в документации](./).

Вывести глобальные настройки:

```shell
kubectl get mc global -o yaml
```

Вывести список состояния всех модулей (доступно для Deckhouse версии 1.47+):

```shell
kubectl get modules
```

Вывести настройки модуля `user-authn`:

```shell
kubectl get moduleconfigs user-authn -o yaml
```

#### Как найти документацию по установленной у меня версии?

Документация запущенной в кластере версии Deckhouse доступна по адресу `documentation.<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из параметра [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.

{% alert level="warning" %}
Документация доступна, если в кластере включен модуль [documentation](modules/documentation/). Он включен по умолчанию, кроме [варианта поставки](modules/deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

#### Закрытое окружение, работа через proxy и сторонние registry

##### Как установить Deckhouse из стороннего registry?

{% alert level="warning" %}
Deckhouse поддерживает работу только с Bearer token-схемой авторизации в container registry.

Протестирована и гарантируется работа со следующими container registry:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

При установке Deckhouse можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура).

Установите следующие параметры в ресурсе `InitConfiguration`:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа Deckhouse EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам Deckhouse в стороннем registry, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам Deckhouse в стороннем registry необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего registry в виде `<HOSTNAME>[:PORT]`;
* `<AUTH_BASE64>` — строка вида `<PROXY_USERNAME>:<PROXY_PASSWORD>`, закодированная в Base64.

Итоговое значение для `registryDockerCfg` должно быть также закодировано в Base64.

Вы можете использовать следующий скрипт для генерации `registryDockerCfg`:

```shell
declare MYUSER='<PROXY_USERNAME>'
declare MYPASSWORD='<PROXY_PASSWORD>'
declare MYREGISTRY='<PROXY_REGISTRY>'

MYAUTH=$(echo -n "$MYUSER:$MYPASSWORD" | base64 -w0)
MYRESULTSTRING=$(echo -n "{\"auths\":{\"$MYREGISTRY\":{\"username\":\"$MYUSER\",\"password\":\"$MYPASSWORD\",\"auth\":\"$MYAUTH\"}}}" | base64 -w0)

echo "$MYRESULTSTRING"
```

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

##### Особенности настройки Nexus

{% alert level="warning" %}
При взаимодействии с репозиторием типа `docker` расположенным в Nexus (например, при выполнении команд `docker pull`, `docker push`) требуется указывать адрес в формате `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Использование значения `URL` из параметров репозитория Nexus **недопустимо**
{% endalert %}

При использовании менеджера репозиториев Nexus должны быть выполнены следующие требования:

* Создан **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*):
  * Параметр `Maximum metadata age` для репозитория должен быть установлен в `0`.
* Должен быть настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

**Настройка**:

* Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на Deckhouse registry:
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания репозитория следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно `0`.
  * Флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Platform Certified Security Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль **Nexus** (*Administration* -> *Security* -> *Roles*) с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)

  * Создайте пользователя (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

##### Особенности настройки Harbor

Необходимо использовать такой функционал Harbor, как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Укажите `Access ID` и `Access Secret`.

  ![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

  ![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

##### Ручная загрузка образов в изолированный приватный registry

1. [Скачайте и установите утилиту Deckhouse CLI](deckhouse-cli/).

1. Скачайте образы Deckhouse в выделенную директорию, используя команду `d8 mirror pull`.

   По умолчанию `d8 mirror pull` скачивает только актуальные версии Deckhouse и официально поставляемых модулей.
   Например, для Deckhouse 1.59 будет скачана только версия `1.59.12`, т. к. этого достаточно для обновления Deckhouse с 1.58 до 1.59.

   Выполните следующую команду (укажите код редакции и лицензионный ключ), чтобы скачать образы актуальных версий:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' $(pwd)/d8.tar
   ```

   где:
   - `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `cse`);
   - `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform.

   > Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента ее остановки прошло не более суток.

   Вы также можете использовать следующие параметры команды:
   - `--no-pull-resume` — чтобы принудительно начать загрузку сначала;
   - `--no-modules` — для пропуска загрузки модулей;
   - `--min-version=X.Y` — чтобы скачать все версии Deckhouse, начиная с указанной минорной версии. Параметр будет проигнорирован, если указана версия выше чем версия находящаяся на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--release`;
   - `--release=X.Y.Z` — чтобы скачать только конкретную версию Deckhouse (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--min-version`;
   - `--gost-digest` — для расчета контрольной суммы итогового набора образов Deckhouse в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с tar-архивом, содержащим образы Deckhouse;
   - `--source` — чтобы указать адрес источника хранилища образов Deckhouse;
      - Для аутентификации в официальном хранилище образов Deckhouse нужно использовать лицензионный ключ и параметр `--license`;
      - Для аутентификации в стороннем хранилище образов нужно использовать параметры `--source-login` и `--source-password`;
   - `--images-bundle-chunk-size=N` — для указания максимального размера файла (в ГБ), на которые нужно разбить архив образов. В результате работы вместо одного файла архива образов будет создан набор `.chunk`-файлов (например, `d8.tar.NNNN.chunk`). Чтобы загрузить образы из такого набора файлов, укажите в команде `d8 mirror push` имя файла без суффикса `.NNNN.chunk` (например, `d8.tar` для файлов `d8.tar.NNNN.chunk`).

   Дополнительные параметры конфигурации для семейства команд `d8 mirror` доступны в виде переменных окружения:
   - `HTTP_PROXY`/`HTTPS_PROXY` — URL прокси-сервера для запросов к HTTP(S) хостам, которые не указаны в списке хостов в переменной `$NO_PROXY`;
   - `NO_PROXY` — список хостов, разделенных запятыми, которые следует исключить из проксирования. Каждое значение может быть представлено в виде IP-адреса (`1.2.3.4`), CIDR (`1.2.3.4/8`), домена или символа (`*`). IP-адреса и домены также могут включать номер порта (`1.2.3.4:80`). Доменное имя соответствует как самому себе, так и всем поддоменам. Доменное имя начинающееся с `.`, соответствует только поддоменам. Например, `foo.com` соответствует `foo.com` и `bar.foo.com`; `.y.com` соответствует `x.y.com`, но не соответствует `y.com`. Символ `*` отключает проксирование;
   - `SSL_CERT_FILE` — указывает путь до сертификата SSL. Если переменная установлена, системные сертификаты не используются;
   - `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы сертификатов SSL. Если переменная установлена, системные сертификаты не используются. Подробнее...;
   - `TMPDIR (*nix)`/`TMP (Windows)` — путь к директории для временных файлов, который будет использоваться во время операций загрузки и выгрузки образов. Вся обработка выполняется в этом каталоге. Он должен иметь достаточное количество свободного дискового пространства, чтобы вместить весь загружаемый пакет образов;
   - `MIRROR_BYPASS_ACCESS_CHECKS` — установите для этого параметра значение `1`, чтобы отключить проверку корректности переданных учетных данных для registry;

   Пример команды для загрузки всех версий Deckhouse EE начиная с версии 1.59 (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/ee' \
     --license='<LICENSE_KEY>' --min-version=1.59 $(pwd)/d8.tar
   ```

   Пример команды для загрузки образов Deckhouse из стороннего хранилища образов:

   ```shell
   d8 mirror pull \
     --source='corp.company.com:5000/sys/deckhouse' \
     --source-login='<USER>' --source-password='<PASSWORD>' $(pwd)/d8.tar
   ```

1. На хост с доступом к хранилищу, куда нужно загрузить образы Deckhouse, скопируйте загруженный пакет образов Deckhouse и установите [Deckhouse CLI](deckhouse-cli/).

1. Загрузите образы Deckhouse в хранилище с помощью команды `d8 mirror push`.

   Пример команды для загрузки образов из файла `/tmp/d8-images/d8.tar` (укажите данные для авторизации при необходимости):

   ```shell
   d8 mirror push /tmp/d8-images/d8.tar 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse`) и у используемой учетной записи есть права на запись.
   > Если вы используете Harbor, вы не сможете выгрузить образы в корень проекта, используйте выделенный репозиторий в проекте для размещения образов Deckhouse.

1. После загрузки образов в хранилище можно переходить к установке Deckhouse. Воспользуйтесь [руководством по быстрому старту](/products/kubernetes-platform/gs/bm-private/step2.html).

   При запуске установщика используйте не официальное публичное хранилище образов Deckhouse, а хранилище в которое ранее были загружены образы Deckhouse. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable`, вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе [InitConfiguration](installing/configuration.html#initconfiguration) при установке также используйте адрес вашего хранилища и данные авторизации (параметры [imagesRepo](installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) руководства по быстрому старту).

   После завершения установки примените сгенерированные во время загрузки манифесты [DeckhouseReleases](cr.html#deckhouserelease) к вашему кластеру, используя [Deckhouse CLI](deckhouse-cli/):

   ```shell
   d8 k apply -f ./deckhousereleases.yaml
   ```

##### Ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

Для ручной загрузки образов модулей, подключаемых из источника модулей (ресурс [ModuleSource](cr.html#modulesource)), выполните следующие шаги:

1. [Скачайте и установите утилиту Deckhouse CLI](deckhouse-cli/).

1. Создайте строку аутентификации для официального хранилища образов `registry.deckhouse.ru`, выполнив следующую команду (укажите лицензионный ключ):

   ```shell
   LICENSE_KEY='<LICENSE_KEY>'
   base64 -w0 <<EOF
     {
       "auths": {
         "registry.deckhouse.ru": {
           "auth": "$(echo -n license-token:${LICENSE_KEY} | base64 -w0)"
         }
       }
     }
   EOF
   ```

1. Скачайте образы модулей из их источника, описанного в виде ресурса `ModuleSource`, в выделенную директорию, используя команду `d8 mirror modules pull`.

   Если не передан параметр `--filter`, то `d8 mirror modules pull` скачивает только версии модулей, доступные в каналах обновлений модуля на момент копирования.

   - Создайте файл с описанием ресурса `ModuleSource` (например, `$HOME/module_source.yml`).

     Пример ресурса ModuleSource:

     ```yaml
     apiVersion: deckhouse.io/v1alpha1
     kind: ModuleSource
     metadata:
       name: deckhouse
     spec:
       registry:
         # Укажите строку аутентификации для официального хранилища образов, полученную в п. 2
         dockerCfg: <BASE64_REGISTRY_CREDENTIALS>
         repo: registry.deckhouse.ru/deckhouse/ee/modules
         scheme: HTTPS
       # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
       releaseChannel: "Stable"
     ```

   - Скачайте образы модулей из источника, описанного в ресурсе `ModuleSource`, в выделенную директорию, используя команду `d8 mirror modules pull`.

     Пример команды:

     ```shell
     d8 mirror modules pull -d ./d8-modules -m $HOME/module_source.yml
     ```

     Для загрузки только набора из определенных модулей конкретных версий используйте параметр `--filter`, передав набор необходимых модулей и их минимальных версий, разделенных символом `;`.

     Пример:

     ```shell
     d8 mirror modules pull -d /tmp/d8-modules -m $HOME/module_source.yml \
       --filter='deckhouse-admin@1.3.3; sds-drbd@0.0.1'
     ```

     Команда выше загрузит только модули `deckhouse-admin` и `sds-drbd`. Для `deckhouse-admin` будут загружены все доступные версии начиная с `1.3.3`, для `sds-drbd` — все доступные версии начиная с `0.0.1`.

1. На хост с доступом к хранилищу, куда нужно загрузить образы, скопируйте директорию с загруженными образами модулей Deckhouse и установите [Deckhouse CLI](deckhouse-cli/).

1. Загрузите образы модулей в хранилище с помощью команды `d8 mirror modules push`.

   Пример команды для загрузки образов из директории `/tmp/d8-modules`:

   ```shell
   d8 mirror modules push \
     -d /tmp/d8-modules --registry='corp.company.com:5000/sys/deckhouse/modules' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   > Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse/modules`) и у используемой учетной записи есть права на запись.

1. Отредактируйте YAML-манифест `ModuleSource`, подготовленный на шаге 3:

   * Измените поле `.spec.registry.repo` на адрес, который вы указали в параметре `--registry` при загрузке образов.
   * Измените поле `.spec.registry.dockerCfg` на Base64-строку с данными для авторизации в вашем хранилище образов в формате `dockercfg`. Обратитесь к документации вашего registry для получения информации о том, как сгенерировать этот токен.

   Пример `ModuleSource`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: deckhouse
   spec:
     registry:
       # Укажите строку аутентификации для вашего хранилища образов.
       dockerCfg: <BASE64_REGISTRY_CREDENTIALS>
       repo: 'corp.company.com:5000/sys/deckhouse/modules'
       scheme: HTTPS
     # Выберите подходящий канал обновлений: Alpha, Beta, EarlyAccess, Stable, RockSolid
     releaseChannel: "Stable"
   ```

1. Примените в кластере исправленный манифест `ModuleSource`:

   ```shell
   d8 k apply -f $HOME/module_source.yml
   ```

   После применения манифеста модули готовы к использованию. Обратитесь к [документации по разработке модуля](./module-development/) для получения дополнительной информации.

##### Как переключить работающий кластер Deckhouse на использование стороннего registry?

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
  * Пример запуска:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
    ```

  * Если registry использует самоподписанные сертификаты, положите корневой сертификат соответствующего сертификата registry в файл `/tmp/ca.crt` в поде Deckhouse и добавьте к вызову опцию `--ca-file /tmp/ca.crt` или вставьте содержимое CA в переменную, как в примере ниже:

    ```shell
    $ CA_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    CERTIFICATE
    -----END CERTIFICATE-----
    EOF
    )
    $ kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- bash -c "echo '$CA_CONTENT' > /tmp/ca.crt && deckhouse-controller helper change-registry --ca-file /tmp/ca.crt --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee"
    ```

  * Просмотреть список доступных ключей команды `deckhouse-controller helper change-registry` можно, выполнив следующую команду:

    ```shell
    kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --help
    ```

    Пример вывода:

    ```shell
    usage: deckhouse-controller helper change-registry [<flags>] <new-registry>

    Change registry for deckhouse images.

    Flags:
      --help               Show context-sensitive help (also try --help-long and --help-man).
      --user=USER          User with pull access to registry.
      --password=PASSWORD  Password/token for registry user.
      --ca-file=CA-FILE    Path to registry CA.
      --scheme=SCHEME      Used scheme while connecting to registry, http or https.
      --dry-run            Don't change deckhouse resources, only print them.
      --new-deckhouse-tag=NEW-DECKHOUSE-TAG
                          New tag that will be used for deckhouse deployment image (by default
                          current tag from deckhouse deployment will be used).

    Args:
      <new-registry>  Registry that will be used for deckhouse images (example:
                      registry.deckhouse.io/deckhouse/ce). By default, https will be used, if you need
                      http - provide '--scheme' flag with http value
    ```

* Дождитесь перехода пода Deckhouse в статус `Ready`. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его.
* Дождитесь применения bashible новых настроек на master-узле. В журнале bashible на master-узле (`journalctl -u bashible`) должно появится сообщение `Configuration is in sync, nothing to do`.
* Если необходимо отключить автоматическое обновление Deckhouse через сторонний registry, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
* Проверьте, не осталось ли в кластере подов с оригинальным адресом registry:

  ```shell
  kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
    | select(.image | startswith("registry.deckhouse"))) | .metadata.namespace + "	" + .metadata.name' | sort | uniq
  ```

##### Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.60.5`, используйте образ `your.private.registry.com/deckhouse/install:v1.60.5`.
  * **Не указывайте** параметр [deckhouse.releaseChannel](modules/deckhouse/configuration.html#parameters-releasechannel).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse, ознакомьтесь с документацией [по закреплению релиза](modules/deckhouse/#закрепление-релиза].

##### Использование proxy-сервера

{% offtopic title="Пример шагов по настройке proxy-сервера на базе Squid..." %}
* Подготовьте сервер (или виртуальную машину). Сервер должен быть доступен с необходимых узлов кластера, и у него должен быть выход в интернет.
* Установите Squid (здесь и далее примеры для Ubuntu):

  ```shell
  apt-get install squid
  ```

* Создайте файл конфигурации Squid:

  ```shell
  cat <<EOF > /etc/squid/squid.conf
  auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
  auth_param basic realm proxy
  acl authenticated proxy_auth REQUIRED
  http_access allow authenticated

  # Choose the port you want. Below we set it to default 3128.
  http_port 3128
  ```

* Создайте пользователя и пароль для аутентификации на proxy-сервере:

  Пример для пользователя `test` с паролем `test` (обязательно измените):

  ```shell
  echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
  ```

* Запустите Squid и включите его автоматический запуск при загрузке сервера:

  ```shell
  systemctl restart squid
  systemctl enable squid
  ```

{% endofftopic %}

Для настройки Deckhouse на использование proxy используйте параметр [proxy](installing/configuration.html#clusterconfiguration-proxy) ресурса `ClusterConfiguration`.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: OpenStack
  prefix: main
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
cri: "Containerd"
clusterDomain: "cluster.local"
proxy:
  httpProxy: "http://user:password@proxy.company.my:3128"
  httpsProxy: "https://user:password@proxy.company.my:8443"
```

#### Изменение конфигурации

{% alert level="warning" %}
Для применения изменений конфигурации узлов необходимо выполнить команду  `dhctl converge`, запустив инсталлятор Deckhouse. Эта команда синхронизирует состояние узлов с указанным в конфигурации.
{% endalert %}

##### Как изменить конфигурацию кластера?

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

##### Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit static-cluster-configuration
```

##### Как получить доступ к контроллеру Deckhouse в multi-master-кластере?

В кластерах с несколькими master-узлами Deckhouse запускается в режиме высокой доступности (в нескольких экземплярах). Для доступа к активному контроллеру Deckhouse можно использовать следующую команду (на примере команды `deckhouse-controller queue list`):

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

##### Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration), выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader \
     -c deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

##### Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

{% alert level="warning" %}
Используйте для запуска Deckhouse только узлы с типом **CloudStatic** или **Static**. Также избегайте использования для запуска Deckhouse группы узлов (`NodeGroup`), содержащей только один узел.
{% endalert %}

Пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    nodeSelector:
      node-role.deckhouse.io/deckhouse: ""
```
## Подсистема Кластер Kubernetes

### Модуль chrony

Обеспечивает синхронизацию времени на всех узлах кластера с помощью модуля chrony.

### Модуль chrony: настройки

 
<!-- SCHEMA -->

#### Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: chrony
spec:
  enabled: true
  settings:
    ntpServers:
      - pool.ntp.org
      - ntp.ubuntu.com
      - time.google.com
  version: 1
```
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['chrony'].config-values | format_module_configuration: moduleKebabName }}

### Модуль chrony: FAQ

#### Как запретить использование chrony и использовать NTP-демоны на узлах?

1. [Выключите](configuration.html) модуль chrony.

1. Создайте `NodeGroupConfiguration` custom step, чтобы включить NTP-демоны на узлах (пример для `systemd-timesyncd`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: enable-ntp-on-node.sh
   spec:
     weight: 100
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   ```

### Модуль cni-cilium

Модуль `cni-cilium` обеспечивает работу сети в кластере. Основан на проекте Cilium.

#### Ограничения

1. Сервисы с типом `NodePort` и `LoadBalancer` несовместимы с hostNetwork-эндпойнтами в LB-режиме `DSR`. Переключитесь на режим `SNAT`, если это требуется.
2. `HostPort` поды связываются только с одним IP-адресом. Если в ОС есть несколько интерфейсов/IP, Cilium выберет один, предпочитая «серые» «белым».
3. Требования к ядру:
   * ядро Linux версии не ниже `5.7` для работы модуля `cni-cilium` и его совместной работы с модулями [istio](./istio/), [openvpn](./openvpn/), [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/node-local-dns/).
4. Совместимость с ОС:
   * Ubuntu:
     * несовместим с версией 18.04;
     * для работы с версией 20.04 необходима установка ядра HWE.
   * Astra Linux:
     * несовместим с изданием «Смоленск».
   * CentOS:
     * для версий 7 и 8 необходимо новое ядро из репозитория.

#### Обработка внешнего трафика в разных режимах работы `bpfLB` (замена kube-proxy от Cilium)

В Kubernetes обычно используются схемы, где трафик приходит на балансировщик, который распределяет его между многими серверами. Через балансировщик проходят и входящий, и исходящий трафики. Таким образом, общая пропускная способность ограничена ресурсами и шириной канала балансировщика. Для оптимизации трафика и разгрузки балансировщика и был придуман механизм `DSR`, в котором входящие пакеты проходят через балансировщик, а исходящие идут напрямую с терминирующих серверов. Так как обычно ответы имеют много больший размер чем запросы, то такой подход позволяет значительно увеличить общую пропускную способность схемы.

В модуле возможен [выбор режима работы](configuration.html#parameters-bpflbmode), влияющий на поведение `Service` с типом `NodePort` и `LoadBalancer`:

* `SNAT` (Source Network Address Translation) — один из подвидов NAT, при котором для каждого исходящего пакета происходит трансляция IP-адреса источника в IP-адрес шлюза из целевой подсети, а входящие пакеты, проходящие через шлюз, транслируются обратно на основе таблицы трансляций. В этом режиме `bpfLB` полностью повторяет логику работы `kube-proxy`:
  * если в `Service` указан `externalTrafficPolicy: Local`, то трафик будет передаваться и балансироваться только в те целевые поды, которые запущены на том же узле, на который этот трафик пришел. Если целевой под не запущен на этом узле, то трафик будет отброшен.
  * если в `Service` указан `externalTrafficPolicy: Cluster`, то трафик будет передаваться и балансироваться во все целевые поды в кластере. При этом, если целевые поды находятся на других узлах, то при передаче трафика на них будет произведен SNAT (IP-адрес источника будет заменен на InternalIP узла).

   ![Схема потоков данных SNAT](./images/cni-cilium/snat.png)

* `DSR` - (Direct Server Return) — метод, при котором весь входящий трафик проходит через балансировщик нагрузки, а весь исходящий трафик обходит его. Такой метод используется вместо `SNAT`. Часто ответы имеют много больший размер чем запросы и `DSR` позволяет значительно увеличить общую пропускную способность схемы:
  * если в `Service` указан `externalTrafficPolicy: Local`, то поведение абсолютно аналогично `kube-proxy` и `bpfLB` в режиме `SNAT`.
  * если в `Service` указан `externalTrafficPolicy: Cluster`, то трафик так же будет передаваться и балансироваться во все целевые поды в кластере.  
  При этом важно учитывать следующие особенности:
    * если целевые поды находятся на других узлах, то при передаче на них входящего трафика будет сохранен IP-адрес источника;
    * исходящий трафик пойдет прямо с узла, на котором был запущен целевой под;
    * IP-адрес источника будет заменен на внешний IP-адрес узла, на которую изначально пришел входящий запрос.

   ![Схема потоков данных DSR](./images/cni-cilium/dsr.png)

{% alert level="warning" %}
В случае использования режима `DSR` и `Service` с `externalTrafficPolicy: Cluster` требуются дополнительные настройки сетевого окружения.
Сетевое оборудование должно быть готово к ассиметричному прохождению трафика: отключены или настроены соответствующим образом средства фильтрации IP адресов на входе в сеть (`uRPF`, `sourceGuard` и т.п.).
{% endalert %}

* `Hybrid` — в данном режиме TCP-трафик обрабатывается в режиме `DSR`, а UDP — в режиме `SNAT`.

#### Использование CiliumClusterwideNetworkPolicies

Для использования CiliumClusterwideNetworkPolicies следует применить:

1. Первичный набор объектов `CiliumClusterwideNetworkPolicy`, поставив конфигурационную опцию `policyAuditMode` в `true`. Отсутствие опции может привести к некорректной работе Control plane или потере доступа ко всем узлам кластера по SSH. Опция может быть удалена после применения всех `CniliumClusterwideNetworkPolicy`-объектов и проверки корректности их работы в Hubble UI.
2. Правило политики сетевой безопасности:

   ```yaml
   apiVersion: "cilium.io/v2"
   kind: CiliumClusterwideNetworkPolicy
   metadata:
     name: "allow-control-plane-connectivity"
   spec:
     ingress:
     - fromEntities:
       - kube-apiserver
     nodeSelector:
       matchLabels:
         node-role.kubernetes.io/control-plane: ""
   ```

В случае, если CiliumClusterwideNetworkPolicies не будут использованы, Control plane может некорректно работать до одной минуты во время перезагрузки `cilium-agent`-подов. Это происходит из-за сброса Conntrack-таблицы. Привязка к entity `kube-apiserver` позволяет обойти баг.

#### Смена режима работы Cilium

При смене режима работы Cilium (параметр [tunnelMode](configuration.html#parameters-tunnelmode)) c `Disabled` на `VXLAN` или обратно, необходимо перезагрузить все узлы, иначе возможны проблемы с доступностью подов.

#### Выключение модуля kube-proxy

Cilium полностью заменяет собой функционал модуля `kube-proxy`, поэтому `kube-proxy` автоматически отключается при включении модуля `cni-cilium`.

#### Использование Egress Gateway

##### Базовый режим

Используются предварительно настроенные IP-адреса на egress-узлах.

<div data-presentation="./presentations/cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

##### Режим с Virtual IP

Реализована возможность динамически назначать дополнительные IP-адреса узлам.

<div data-presentation="./presentations/cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->

### Модуль cni-cilium: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['cni-cilium'].config-values | format_module_configuration: moduleKebabName }}

### Модуль cni-cilium: Custom Resources
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumbgppeeringpolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumcidrgroups | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumclusterwideenvoyconfigs | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumclusterwidenetworkpolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumegressgatewaypolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumendpoints | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumendpointslices | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumenvoyconfigs | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumexternalworkloads | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumidentities | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliuml2announcementpolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumloadbalancerippools | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumlocalredirectpolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumnetworkpolicies | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumnodeconfigs | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumnodes | format_crd: "cni-cilium" }}
{{ site.data.schemas.cni-cilium.crds.cilium.ciliumpodippools | format_crd: "cni-cilium" }}

### Модуль cni-cilium: примеры

#### Egress Gateway

##### Принцип работы

Для настройки egress-шлюза необходимо настроить два ресурса:

* `EgressGateway` — описывает группу узлов, которые осуществляют функцию egress-шлюза в режиме горячего резерва:
  * Среди группы узлов, попадающих под `spec.nodeSelector`, будут выявлены пригодные к работе и один из них будет назначен активным. Признаки пригодного узла:
    * Узел в состоянии Ready.
    * Узел не находится в состоянии технического обслуживания (cordon).
    * cilium-agent на узле в состоянии Ready.
  * При использовании `EgressGateway` в режиме `VirtualIP` на активном узле запускается агент, который эмулирует "виртуальный" IP средствами протокола ARP. При определении пригодности узла также учитывается состояние пода данного агента.
  * Разные EgressGateway могут использовать для работы общие узлы, при этом активные узлы будут выбираться независимо, тем самым распределяя нагрузку между ними.
* `EgressGatewayPolicy` — описывает политику перенаправления сетевых запросов от подов в кластере на определённый egress-шлюз, описанный с помощью `EgressGateway`.

##### Сравнение с CiliumEgressGatewayPolicy

`CiliumEgressGatewayPolicy` подразумевает настройку лишь одного узла в качестве egress-шлюза. При выходе его из строя не предусмотрено failover-механизмов и сетевая связь будет нарушена.

##### Пример настройки

###### EgressGateway в режиме PrimaryIPFromEgressGatewayNodeInterface

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myegressgw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # На всех узлах, попадающих под nodeSelector, "публичный" интерфейс должен иметь одинаковое имя.
      # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
      # IP-адрес отправителя у сетевых пакетов поменяется.
      interfaceName: eth1
```

###### EgressGateway в режиме VirtualIPAddress

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: myeg
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      # На каждом узле должны быть настроены все необходимые маршруты для доступа на все внешние публичные сервисы,
      # "публичный" интерфейс должен быть подготовлен к автоматической настройке "виртуального" IP в качестве secondary IP-адреса.
      # При выходе из строя активного узла, трафик будет перенаправлен через резервный и
      # IP-адрес отправителя у сетевых пакетов не поменяется.
      ip: 172.18.18.242
      # Список сетевых интерфейсов для _виртуального_ IP.
      interfaces:
      - eth1
```

###### EgressGatewayPolicy

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: my-egressgw-policy
spec:
  destinationCIDRs:
  - 0.0.0.0/0
  egressGatewayName: my-egressgw
  selectors:
  - podSelector:
      matchLabels:
        app: backend
        io.kubernetes.pod.namespace: my-ns
```

### Модуль cilium-hubble

Модуль может обеспечивать визуализацию сетевого стека кластера, если включен Cilium CNI.

### Модуль cilium-hubble: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль останется отключенным вне зависимости от параметра `ciliumHubbleEnabled:`, если не включен модуль `cni-cilium`.

{% include module-settings.liquid %}

#### Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values cilium-hubble -o json | jq '.ciliumHubble.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-cni-cilium delete secret/hubble-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['cilium-hubble'].config-values | format_module_configuration: moduleKebabName }}

### Управление control plane

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, который запускается на всех master-узлах кластера (узлы с лейблом `node-role.kubernetes.io/control-plane: ""`).

Функционал управления control plane:
- **Управление сертификатами**, необходимыми для работы control-plane, в том числе продление, выпуск при изменении конфигурации и т. п. Позволяет автоматически поддерживать безопасную конфигурацию control plane и быстро добавлять дополнительные SAN для организации защищенного доступа к API Kubernetes.
- **Настройка компонентов**. Автоматически создает необходимые конфигурации и манифесты компонентов `control-plane`.
- **Upgrade/downgrade компонентов**. Поддерживает в кластере одинаковые версии компонентов.
- **Управление конфигурацией etcd-кластера** и его членов. Масштабирует master-узлы, выполняет миграцию из single-master в multi-master и обратно.
- **Настройка kubeconfig**. Обеспечивает всегда актуальную конфигурацию для работы kubectl. Генерирует, продлевает, обновляет kubeconfig с правами cluster-admin и создает symlink пользователю root, чтобы kubeconfig использовался по умолчанию.
- **Расширение работы планировщика**, за счет подключения внешних плагинов через вебхуки. Управляется ресурсом [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration). Позволяет использовать более сложную логику при решении задач планирования нагрузки в кластере. Например:
  - размещение подов приложений организации хранилища данных ближе к самим данным,
  - приоритизация узлов в зависимости от их состояния (сетевой нагрузки, состояния подсистемы хранения и т. д.),
  - разделение узлов на зоны, и т. п.

#### Управление сертификатами

Управляет SSL-сертификатами компонентов `control-plane`:
- Серверными сертификатами для `kube-apiserver` и `etcd`. Они хранятся в Secret'е `d8-pki` пространства имен `kube-system`:
  - корневой CA kubernetes (`ca.crt` и `ca.key`);
  - корневой CA etcd (`etcd/ca.crt` и `etcd/ca.key`);
  - RSA-сертификат и ключ для подписи Service Account'ов (`sa.pub` и `sa.key`);
  - корневой CA для extension API-серверов (`front-proxy-ca.key` и `front-proxy-ca.crt`).
- Клиентскими сертификатами для подключения компонентов `control-plane` друг к другу. Выписывает, продлевает и перевыписывает, если что-то изменилось (например, список SAN). Следующие сертификаты хранятся только на узлах:
  - серверный сертификат API-сервера (`apiserver.crt` и `apiserver.key`);
  - клиентский сертификат для подключения `kube-apiserver` к `kubelet` (`apiserver-kubelet-client.crt` и `apiserver-kubelet-client.key`);
  - клиентский сертификат для подключения `kube-apiserver` к `etcd` (`apiserver-etcd-client.crt` и `apiserver-etcd-client.key`);
  - клиентский сертификат для подключения `kube-apiserver` к extension API-серверам (`front-proxy-client.crt` и `front-proxy-client.key`);
  - серверный сертификат `etcd` (`etcd/server.crt` и `etcd/server.key`);
  - клиентский сертификат для подключения `etcd` к другим членам кластера (`etcd/peer.crt` и `etcd/peer.key`);
  - клиентский сертификат для подключения `kubelet` к `etcd` для helthcheck'ов (`etcd/healthcheck-client.crt` и `etcd/healthcheck-client.key`).

Также позволяет добавить дополнительные SAN в сертификаты, это дает возможность быстро и просто добавлять дополнительные «точки входа» в API Kubernetes.

При изменении сертификатов также автоматически обновляется соответствующая конфигурация kubeconfig.

#### Масштабирование

Поддерживается работа `control-plane` в конфигурации как *single-master*, так и *multi-master*.

В конфигурации *single-master*:
- `kube-apiserver` использует только тот экземпляр `etcd`, который размещен с ним на одном узле;
- На узле настраивается прокси-сервер, отвечающий на localhost,`kube-apiserver` отвечает на IP-адрес master-узла.

В конфигурации *multi-master* компоненты `control-plane` автоматически разворачиваются в отказоустойчивом режиме:
- `kube-apiserver` настраивается для работы со всеми экземплярами `etcd`.
- На каждом master-узле настраивается дополнительный прокси-сервер, отвечающий на localhost. Прокси-сервер по умолчанию обращается к локальному экземпляру `kube-apiserver`, но в случае его недоступности последовательно опрашивает остальные экземпляры `kube-apiserver`.

##### Масштабирование master-узлов

Масштабирование узлов `control-plane` осуществляется автоматически, с помощью лейбла `node-role.kubernetes.io/control-plane=""`:
- Установка лейбла `node-role.kubernetes.io/control-plane=""` на узле приводит к развертыванию на нем компонентов `control-plane`, подключению нового узла `etcd` в etcd-кластер, а также перегенерации необходимых сертификатов и конфигурационных файлов.
- Удаление лейбла `node-role.kubernetes.io/control-plane=""` с узла приводит к удалению всех компонентов `control-plane`, перегенерации необходимых конфигурационных файлов и сертификатов, а также корректному исключению узла из etcd-кластера.

> **Важно!** При масштабировании узлов с 2 до 1 требуются [ручные действия](./faq.html#что-делать-если-кластер-etcd-развалился) с `etcd`. В остальных случаях все необходимые действия происходят автоматически. Обратите внимание, что при масштабировании с любого количества master-узлов до 1 рано или поздно на последнем шаге возникнет ситуация масштабирования узлов с 2 до 1.

#### Управление версиями

Обновление **patch-версии** компонентов control plane (то есть в рамках минорной версии, например с `1.27.3` на `1.27.5`) происходит автоматически вместе с обновлением версии Deckhouse. Управлять обновлением patch-версий нельзя.

Обновлением **минорной-версии** компонентов control plane (например, с `1.26.*` на `1.28.*`) можно управлять с помощью параметра [kubernetesVersion](./installing/configuration.html#clusterconfiguration-kubernetesversion), в котором можно выбрать автоматический режим обновления (значение `Automatic`) или указать желаемую минорную версию control plane. Версию control plane, которая используется по умолчанию (при `kubernetesVersion: Automatic`), а также список поддерживаемых версий Kubernetes можно найти в [документации](./supported_versions.html#kubernetes).

Обновление control plane выполняется безопасно и для single-master-, и для multi-master-кластеров. Во время обновления может быть кратковременная недоступность API-сервера. На работу приложений в кластере обновление не влияет и может выполняться без выделения окна для регламентных работ.

Если указанная для обновления версия (параметр [kubernetesVersion](./installing/configuration.html#clusterconfiguration-kubernetesversion)) не соответствует текущей версии control plane в кластере, запускается умная стратегия изменения версий компонентов:
- Общие замечания:
  - Обновление в разных NodeGroup выполняется параллельно. Внутри каждой NogeGroup узлы обновляются последовательно, по одному.
- При upgrade:
  - Обновление происходит **последовательными этапами**, по одной минорной версии: 1.26 -> 1.27, 1.27 -> 1.28, 1.28 -> 1.29.
  - На каждом этапе сначала обновляется версия control plane, затем происходит обновление kubelet на узлах кластера.  
- При downgrade:
  - Успешный downgrade гарантируется только на одну версию вниз от максимальной минорной версии control plane, когда-либо использовавшейся в кластере.
  - Сначала происходит downgrade kubelet'a на узлах кластера, затем — downgrade компонентов control plane.

#### Аудит

Если требуется журналировать операции с API или отдебажить неожиданное поведение, для этого в Kubernetes предусмотрен Auditing. Его можно настроить путем создания правил Audit Policy, а результатом работы аудита будет лог-файл `/var/log/kube-audit/audit.log` со всеми интересующими операциями.

В установках Deckhouse по умолчанию созданы базовые политики, которые отвечают за логирование событий:
- связанных с операциями создания, удаления и изменения ресурсов;
- совершаемых от имен сервисных аккаунтов из системных Namespace `kube-system`, `d8-*`;
- совершаемых с ресурсами в системных пространствах имен `kube-system`, `d8-*`.

Для выключения базовых политик установите флаг [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) в `false`.

Настройка политик аудита подробно рассмотрена в [одноименной секции FAQ](faq.html#как-настроить-дополнительные-политики-аудита).

### Управление control plane: настройки

Некоторые параметры кластера, влияющие на управление control plane, также берутся из ресурса [ClusterConfiguration](./installing/configuration.html#clusterconfiguration).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['control-plane-manager'].config-values | format_module_configuration: moduleKebabName }}

### Управление control plane: Custom Resources
{{ site.data.schemas.control-plane-manager.crds.kube_scheduler_webhook_configuration | format_crd: "control-plane-manager" }}

### Управление control plane: примеры

#### Подключение внешнего плагина планировщика

Пример подключения внешнего плагина планировщика через вебхук.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KubeSchedulerWebhookConfiguration
metadata:
  name: sds-replicated-volume
webhooks:
- weight: 5
  failurePolicy: Ignore
  clientConfig:
    service:
      name: scheduler
      namespace: d8-sds-replicated-volume
      port: 8080
      path: /scheduler
    caBundle: ABCD=
  timeoutSeconds: 5
```

### Управление control plane: FAQ

<div id='как-добавить-master-узел'></div>

#### Как добавить master-узел в статическом или гибридном кластере?

> Важно иметь нечетное количество master-узлов для обеспечения кворума.

Добавление master-узла в статический или гибридный кластер ничем не отличается от добавления обычного узла в кластер. Воспользуйтесь для этого соответствующими [примерами](./node-manager/examples.html#добавление-статического-узла-в-кластер). Все необходимые действия по настройке компонентов control plane кластера на новом узле будут выполнены автоматически, дождитесь их завершения — появления master-узлов в статусе `Ready`.

<div id='как-изменить-образ-ос-в-multi-master-кластере'></div>

#### Как изменить образ ОС в мультимастерном кластере?

1. Сделайте [резервную копию `etcd`](faq.html#резервное-копирование-и-восстановление-etcd) и папки `/etc/kubernetes`.
1. Скопируйте полученный архив за пределы кластера (например, на локальную машину).
1. Убедитесь, что в кластере нет [алертов](./prometheus/faq.html#как-получить-информацию-об-алертах-в-кластере), которые могут помешать обновлению master-узлов.
1. Убедитесь, что [очередь Deckhouse пуста](./deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse).
1. **На локальной машине** запустите контейнер установщика Deckhouse соответствующей редакции и версии (измените адрес container registry при необходимости):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы проверить состояние перед началом работы:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Ответ должен сообщить, что Terraform не нашел расхождений и изменений не требуется.

1. **В контейнере с инсталлятором** выполните следующую команду и укажите необходимый образ ОС в параметре `masterNodeGroup.instanceClass` (укажите адреса всех master-узлов в параметре `--ssh-host`):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

Следующие действия **выполняйте поочередно на каждом** master-узле, начиная с узла с наивысшим номером (с суффиксом 2) и заканчивая узлом с наименьшим номером (с суффиксом 0).

1. Выберите master-узел для обновления (укажите его название):

   ```bash
   NODE="<MASTER-NODE-N-NAME>"
   ```

1. Выполните следующую команду для снятия лейблов `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, `node.deckhouse.io/group` с узла:

   ```bash
   kubectl label node ${NODE} \
     node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Убедитесь, что узел пропал из списка узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Выполните `drain` для узла:

   ```bash
   kubectl drain ${NODE} --ignore-daemonsets --delete-emptydir-data
   ```

1. Выключите виртуальную машину, соответствующую узлу, удалите инстанс узла и подключенные к нему диски (`kubernetes-data`).

1. Удалите в кластере поды, оставшиеся на удаляемом узле:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=${NODE} --force
   ```

1. Удалите в кластере объект `Node` удаленного узла:

   ```bash
   kubectl delete node ${NODE}
   ```

1. **В контейнере с инсталлятором** выполните следующую команду, чтобы создать обновлённый узел:

   Внимательно изучите действия, которые планирует выполнить converge, когда запрашивает подтверждение.

   Если converge запрашивает подтверждение для другого master-узла, выберите `no`, чтобы пропустить его.

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **На созданном узле** откройте журнал systemd-юнита `bashible.service`. Дождитесь окончания настройки узла — в журнале должно появиться сообщение `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Проверьте, что узел отобразился в списке узлов кластера:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Убедитесь, что `control-plane-manager` функционирует на узле.

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=${NODE}
   ```

1. Перейдите к обновлению следующего узла.

<div id='как-изменить-образ-ос-в-single-master-кластере'></div>

#### Как изменить образ ОС в кластере с одним master-узлом?

1. Преобразуйте кластер с одним master-узлом в мультимастерный.
1. Обновите master-узлы в соответствии с [инструкцией](#как-изменить-образ-ос-в-multi-master-кластере).
1. Преобразуйте мультимастерный кластер в кластер с одним master-узлом.

<div id='как-посмотреть-список-memberов-в-etcd'></div>

#### Как посмотреть список узлов кластера в etcd?

##### Вариант 1

Используйте команду `etcdctl member list`.

Пример:

```shell
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ member list -w table
```

**Внимание.** Последний параметр в таблице вывода показывает, что узел находится в состоянии `learner`, а не в состоянии `leader`.

##### Вариант 2

Используйте команду `etcdctl endpoint status`. Для этой команды, после флага `--endpoints` нужно подставить адрес каждого узла control-plane. В пятом столбце таблицы вывода будет указано значение `true` для лидера.

Пример скрипта, который автоматически передает все адреса узлов control-plane:

```shell
MASTER_NODE_IPS=($(kubectl get nodes -l \
node-role.kubernetes.io/control-plane="" \
-o 'custom-columns=IP:.status.addresses[?(@.type=="InternalIP")].address' \
--no-headers))
unset ENDPOINTS_STRING
for master_node_ip in ${MASTER_NODE_IPS[@]}
do ENDPOINTS_STRING+="--endpoints https://${master_node_ip}:2379 "
done
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod \
-l component=etcd,tier=control-plane -o name | head -n1) \
-- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt  --cert /etc/kubernetes/pki/etcd/ca.crt \
--key /etc/kubernetes/pki/etcd/ca.key \
$(echo -n $ENDPOINTS_STRING) endpoint status -w table
```

#### Что делать, если что-то пошло не так?

В процессе работы `control-plane-manager` автоматически создает резервные копии конфигурации и данных, которые могут пригодиться в случае возникновения проблем. Эти резервные копии сохраняются в директории `/etc/kubernetes/deckhouse/backup`. Если в процессе работы возникли ошибки или непредвиденные ситуации, вы можете использовать эти резервные копии для восстановления до предыдущего исправного состояния.

<div id='что-делать-если-кластер-etcd-развалился'></div>

#### Что делать, если кластер etcd не функционирует?

Если кластер etcd не функционирует и не удается восстановить его из резервной копии, вы можете попытаться восстановить его с нуля, следуя шагам ниже.

1. Сначала на всех узлах, которые являются частью вашего кластера etcd, кроме одного, удалите манифест `etcd.yaml`, который находится в директории `/etc/kubernetes/manifests/`. После этого только один узел останется активным, и с него будет происходить восстановление состояния мультимастерного кластера.
1. На оставшемся узле откройте файл манифеста `etcd.yaml` и укажите параметр `--force-new-cluster` в `spec.containers.command`.
1. После успешного восстановления кластера, удалите параметр `--force-new-cluster`.

 {% alert level="warning" %}
 Эта операция является деструктивной, так как она полностью уничтожает текущие данные и инициализирует кластер с состоянием, которое сохранено на узле. Все pending-записи будут утеряны.
 {% endalert %}

##### Что делать, если etcd постоянно перезапускается с ошибкой?

Этот способ может понадобиться, если использование параметра `--force-new-cluster` не восстанавливает работу etcd. Это может произойти, если converge master-узлов прошел неудачно, в результате чего новый master-узел был создан на старом диске etcd, изменил свой адрес в локальной сети, а другие master-узлы отсутствуют. Этот метод стоит использовать если контейнер etcd находится в бесконечном цикле перезапуска, а в его логах появляется ошибка: `panic: unexpected removal of unknown remote peer`.

1. Установите утилиту etcdutl.
1. С текущего локального снапшота базы etcd (`/var/lib/etcd/member/snap/db`) выполните создание нового снапшота:

   ```shell
   ./etcdutl snapshot restore /var/lib/etcd/member/snap/db --name <HOSTNAME> \
   --initial-cluster=HOSTNAME=https://<ADDRESS>:2380 --initial-advertise-peer-urls=https://ADDRESS:2380 \
   --skip-hash-check=true --data-dir /var/lib/etcdtest
   ```

   * `<HOSTNAME>` — название master-узла;
   * `<ADDRESS>` — адрес master-узла.

1. Выполните следующие команды для использования нового снапшота:

   ```shell
   cp -r /var/lib/etcd /tmp/etcd-backup
   rm -rf /var/lib/etcd
   mv /var/lib/etcdtest /var/lib/etcd
   ```

1. Найдите контейнеры `etcd` и `api-server`:

   ```shell
   crictl ps -a | egrep "etcd|apiserver"
   ```

1. Удалите найденные контейнеры `etcd` и `api-server`:

   ```shell
   crictl rm <CONTAINER-ID>
   ```

1. Перезапустите master-узел.

#### Как настроить дополнительные политики аудита?

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

   Минимальный рабочий пример `audit-policy.yaml` выглядит так:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   С подробной информацией по настройке содержимого файла `audit-policy.yaml` можно ознакомиться:
   * В официальной документации Kubernetes;
   * В статье на Habr;
   * В коде скрипта-генератора, используемого в GCE.

##### Как исключить встроенные политики аудита?

Установите параметр [apiserver.basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) модуля в `false`.

Пример:

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

##### Как вывести аудит-лог в стандартный вывод вместо файлов?

Установите параметр [apiserver.auditLog.output](configuration.html#parameters-apiserver-auditlog) модуля в значение `Stdout`.

Пример:

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

##### Как работать с журналом аудита?

Предполагается, что на master-узлах установлен «скрейпер логов»: [log-shipper](./log-shipper/cr.html#clusterloggingconfig), `promtail`, `filebeat`,  который будет мониторить файл с логами:

```bash
/var/log/kube-audit/audit.log
```

Параметры ротации логов в файле журнала предустановлены и их изменение не предусмотрено:

* Максимальное занимаемое место на диске `1000 МБ`.
* Максимальная глубина записи `7 дней`.

В зависимости от настроек политики (`Policy`) и количества запросов к `apiserver` логов может быть очень много, соответственно глубина хранения может быть менее 30 минут.

{% alert level="warning" %}
Текущая реализация функционала не гарантирует безопасность, так как существует риск временного нарушения работы control plane.

Если в Secret'е с конфигурационным файлом окажутся неподдерживаемые опции или опечатка, `apiserver` не сможет запуститься.
{% endalert %}

В случае возникновения проблем с запуском `apiserver`, потребуется вручную отключить параметры `--audit-log-*` в манифесте `/etc/kubernetes/manifests/kube-apiserver.yaml` и перезапустить `apiserver` следующей командой:

```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
### Или (в зависимости используемого вами CRI).
crictl stopp $(crictl pods --name=kube-apiserver -q)
```

После перезапуска будет достаточно времени исправить Secret или удалить его:

```bash
kubectl -n kube-system delete secret audit-policy
```

#### Как ускорить перезапуск подов при потере связи с узлом?

По умолчанию, если узел в течении 40 секунд не сообщает свое состояние, он помечается как недоступный. И еще через 5 минут поды узла начнут перезапускаться на других узлах.  В итоге общее время недоступности приложений составляет около 6 минут.

В специфических случаях, когда приложение не может быть запущено в нескольких экземплярах, есть способ сократить период их недоступности:

1. Уменьшить время перехода узла в состояние `Unreachable` при потере с ним связи настройкой параметра `nodeMonitorGracePeriodSeconds`.
1. Установить меньший таймаут удаления подов с недоступного узла в параметре `failedNodePodEvictionTimeoutSeconds`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

В этом случае при потере связи с узлом приложения будут перезапущены примерно через 1 минуту.

Оба упомянутых параметра напрямую влияют на использование процессора и памяти control-plane'ом. Снижая таймауты, системные компоненты чаще отправляют статусы и проверяют состояние ресурсов.

При выборе оптимальных значений учитывайте графики использования ресурсов управляющих узлов. Чем меньше значения параметров, тем больше ресурсов может понадобиться для их обработки на этих узлах.

#### Резервное копирование и восстановление etcd

##### Что выполняется автоматически

Автоматически запускаются CronJob `kube-system/d8-etcd-backup-*` в 00:00 по UTC+0. Результат сохраняется в `/var/lib/etcd/etcd-backup.tar.gz` на всех узлах с `control-plane` в кластере (master-узлы).

<div id='как-сделать-бэкап-etcd-вручную'></div>

##### Как сделать резервную копию etcd вручную

###### Используя Deckhouse CLI (Deckhouse Kubernetes Platform v1.65+)

Начиная с релиза Deckhouse Kubernetes Platform v1.65, стала доступна утилита `d8 backup etcd`, которая предназначена для быстрого создания снимков состояния etcd.

```bash
d8 backup etcd --kubeconfig $KUBECONFIG ./etcd-backup.snapshot
```

###### Используя bash (Deckhouse Kubernetes Platform v1.64 и старше)

Войдите на любой control-plane узел под пользователем `root` и используйте следующий bash-скрипт:

```bash
###!/usr/bin/env bash
set -e

pod=etcd-`hostname`
kubectl -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
mv /var/lib/etcd/"${pod##*/}.snapshot" etcd-backup.snapshot && \
cp -r /etc/kubernetes/ ./ && \
tar -cvzf kube-backup.tar.gz ./etcd-backup.snapshot ./kubernetes/
rm -r ./kubernetes ./etcd-backup.snapshot
```

В текущей директории будет создан файл `kube-backup.tar.gz` со снимком базы etcd одного из узлов кластера.
Из полученного снимка можно будет восстановить состояние кластера.

Рекомендуем сделать резервную копию директории `/etc/kubernetes`, в которой находятся:

* манифесты и конфигурация компонентов control-plane;
* PKI кластера Kubernetes.

Данная директория поможет быстро восстановить кластер при полной потере control-plane узлов без создания нового кластера и без повторного присоединения узлов в новый кластер.

Рекомендуем хранить резервные копии снимков состояния кластера etcd, а также резервную копию директории `/etc/kubernetes/` в зашифрованном виде вне кластера Deckhouse.
Для этого вы можете использовать сторонние инструменты резервного копирования файлов, например Restic, Borg, Duplicity и т.д.

О возможных вариантах восстановления состояния кластера из снимка etcd вы можете узнать в документации.

##### Как выполнить полное восстановление состояния кластера из резервной копии etcd?

Далее описаны шаги по восстановлению кластера до предыдущего состояния из резервной копии при полной потере данных.

<div id='восстановление-кластера-single-master'></div>

###### Восстановление кластера с одним master-узлом

Для корректного восстановления выполните следующие шаги на master-узле:

1. Найдите утилиту `etcdctl` на master-узле и скопируйте исполняемый файл в `/usr/local/bin/`:

   ```shell
   cp $(find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/ \
   -name etcdctl -print | tail -n 1) /usr/local/bin/etcdctl
   etcdctl version
   ```

   Должен отобразиться корректный вывод `etcdctl version` без ошибок.

   Также вы можете загрузить исполняемый файл etcdctl на сервер (желательно, чтобы версия `etcdctl` была такая же, как и версия etcd в кластере):

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.16/etcd-v3.5.16-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.16-linux-amd64.tar.gz && mv etcd-v3.5.16-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

   Проверить версию etcd в кластере можно выполнив следующую команду (команда может не сработать, если etcd и Kubernetes API недоступны):

   ```shell
   kubectl -n kube-system exec -ti etcd-$(hostname) -- etcdctl version
   ```

1. Остановите etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Сохраните текущие данные etcd.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Очистите директорию etcd.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

1. Положите резервную копию etcd в файл `~/etcd-backup.snapshot`.

1. Восстановите базу данных etcd.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

1. Запустите etcd. Запуск может занять некоторое время.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

<div id='восстановление-кластерa-multi-master'></div>

###### Восстановление мультимастерного кластера

Для корректного восстановления выполните следующие шаги:

1. Включите режим High Availability (HA) с помощью глобального параметра [highAvailability](./deckhouse-configure-global.html#parameters-highavailability). Это необходимо для сохранения хотя бы одной реплики Prometheus и его PVC, поскольку в режиме кластера с одним master-узлом HA по умолчанию отключён.

1. Переведите кластер в режим с одним master-узлом или самостоятельно выведите статические master-узлы из кластера.

1. На оставшемся единственном master-узле выполните шаги по восстановлению etcd из резервной копии в соответствии с [инструкцией](#восстановление-кластера-single-master) для кластера с одним master-узлом.

1. Когда работа etcd будет восстановлена, удалите из кластера информацию об уже удаленных в первом пункте master-узлах, воспользовавшись следующей командой (укажите название узла):

   ```shell
   kubectl delete node <MASTER_NODE_I>
   ```

1. Перезапустите все узлы кластера.

1. Дождитесь выполнения заданий из очереди Deckhouse:

   ```shell
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
   ```

1. Переведите кластер обратно в режим мультимастерного кластера в соответствии с [инструкцией](#как-добавить-master-узел-в-статическом-или-гибридном-кластере) для статических кластеров.

##### Как восстановить объект Kubernetes из резервной копии etcd?

Чтобы получить данные определенных объектов кластера из резервной копии etcd:

1. Запустите временный экземпляр etcd.
1. Заполните его данными из [резервной копии](#как-сделать-бэкап-etcd-вручную).
1. Получите описания нужных объектов с помощью `auger`.

###### Пример шагов по восстановлению объектов из резервной копии etcd

В следующем примере `etcd-backup.snapshot` — [резервная копия](#как-сделать-бэкап-etcd-вручную) etcd (snapshot), `infra-production` — пространство имен, в котором нужно восстановить объекты.

* Для выгрузки бинарных данных из etcd потребуется утилита auger. Ее можно собрать из исходного кода на любой машине с Docker (на узлах кластера это сделать невозможно).

  ```shell
  git clone -b v1.0.1 --depth 1 https://github.com/etcd-io/auger
  cd auger
  make release
  build/auger -h
  ```
  
* Получившийся исполняемый файл `build/auger`, а также `snapshot` из резервной копии etcd нужно загрузить на master-узел, с которого будет выполняться дальнейшие действия.

Данные действия выполняются на master-узле в кластере, на который предварительно был загружен файл `snapshot` и утилита `auger`:

1. Установите полный путь до `snapshot` и до утилиты в переменных окружения:

   ```shell
   SNAPSHOT=/root/etcd-restore/etcd-backup.snapshot
   AUGER_BIN=/root/auger 
   chmod +x $AUGER_BIN
   ```

1. Запустите под с временным экземпляром etcd:

   * Создайте манифест пода. Он будет запускаться именно на текущем master-узле, выбрав его по переменной `$HOSTNAME`, и смонтирует `snapshot` по пути `$SNAPSHOT` для загрузки во временный экземпляр etcd:

     ```shell
     cat <<EOF >etcd.pod.yaml 
     apiVersion: v1
     kind: Pod
     metadata:
       name: etcdrestore
       namespace: default
     spec:
       nodeName: $HOSTNAME
       tolerations:
       - operator: Exists
       initContainers:
       - command:
         - etcdctl
         - snapshot
         - restore
         - "/tmp/etcd-snapshot"
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-snapshot-restore
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
         - name: etcd-snapshot
           mountPath: /tmp/etcd-snapshot
           readOnly: true
       containers:
       - command:
         - etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-temp
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
       volumes:
       - name: etcddir
         emptyDir: {}
       - name: etcd-snapshot
         hostPath:
           path: $SNAPSHOT
           type: File
     EOF
     ```

   * Запустите под:

     ```shell
     kubectl create -f etcd.pod.yaml
     ```

1. Установите нужные переменные. В текущем примере:

   * `infra-production` - пространство имен, в котором мы будем искать ресурсы.

   * `/root/etcd-restore/output` - каталог для восстановленных манифестов.

   * `/root/auger` - путь до исполняемого файла утилиты `auger`:

     ```shell
     FILTER=infra-production
     BACKUP_OUTPUT_DIR=/root/etcd-restore/output
     mkdir -p $BACKUP_OUTPUT_DIR && cd $BACKUP_OUTPUT_DIR
     ```

1. Следующие команды отфильтруют список нужных ресурсов по переменной `$FILTER` и выгрузят их в каталог `$BACKUP_OUTPUT_DIR`:

   ```shell
   files=($(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get / --prefix --keys-only | grep "$FILTER"))
   for file in "${files[@]}"
   do
     OBJECT=$(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get "$file" --print-value-only | $AUGER_BIN decode)
     FILENAME=$(echo $file | sed -e "s#/registry/##g;s#/#_#g")
     echo "$OBJECT" > "$BACKUP_OUTPUT_DIR/$FILENAME.yaml"
     echo $BACKUP_OUTPUT_DIR/$FILENAME.yaml
   done
   ```

1. Удалите из полученных описаний объектов информацию о времени создания (`creationTimestamp`), `UID`, `status` и прочие оперативные данные, после чего восстановите объекты:

   ```bash
   kubectl create -f deployments_infra-production_supercronic.yaml
   ```

1. Удалите под с временным экземпляром etcd:

   ```bash
   kubectl -n default delete pod etcdrestore
   ```

#### Как выбирается узел, на котором будет запущен под?

За распределение подов по узлам отвечает планировщик Kubernetes (компонент `scheduler`).
Он проходит через две основные фазы — `Filtering` и `Scoring` (на самом деле, фаз больше, например, `pre-filtering` и `post-filtering`, но в общем можно выделить две ключевые фазы).

##### Общее устройство планировщика Kubernetes

Планировщик состоит из плагинов, которые работают в рамках какой-либо фазы (фаз).

Примеры плагинов:

* **ImageLocality** — отдает предпочтение узлам, на которых уже есть образы контейнеров, которые используются в запускаемом поде. Фаза: `Scoring`.
* **TaintToleration** — реализует механизм taints and tolerations. Фазы: `Filtering`, `Scoring`.
* **NodePorts** — проверяет, есть ли у узла свободные порты, необходимые для запуска пода. Фаза: `Filtering`.

С полным списком плагинов можно ознакомиться в документации Kubernetes.

##### Логика работы

###### Профили планировщика

Есть два преднастроенных профиля планировщика:

* `default-scheduler` — профиль по умолчанию, который распределяет поды на узлы с наименьшей загрузкой;
* `high-node-utilization` — профиль, при котором поды размещаются на узлах с наибольшей загрузкой.

Чтобы задать профиль планировщика, укажите его параметре `spec.schedulerName` манифеста пода.

Пример использования профиля:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: scheduler-example
  labels:
    name: scheduler-example
spec:
  schedulerName: high-node-utilization
  containers:
  - name: example-pod
    image: registry.k8s.io/pause:2.0  
```

###### Этапы планирования подов

На первой фазе — `Filtering` — активируются плагины фильтрации (filter-плагины), которые из всех доступных узлов выбирают те, которые удовлетворяют определенным условиям фильтрации (например, `taints`, `nodePorts`, `nodeName`, `unschedulable` и другие). Если узлы расположены в разных зонах, планировщик чередует выбор зон, чтобы избежать размещения всех подов в одной зоне.

Предположим, что узлы распределяются по зонам следующим образом:

```text
Zone 1: Node 1, Node 2, Node 3, Node 4
Zone 2: Node 5, Node 6
```

В этом случае они будут выбираться в следующем порядке:

```text
Node 1, Node 5, Node 2, Node 6, Node 3, Node 4
```

Обратите внимание, что с целью оптимизации выбираются не все попадающие под условия узлы, а только их часть. По умолчанию функция выбора количества узлов линейная. Для кластера из ≤50 узлов будут выбраны 100% узлов, для кластера из 100 узлов — 50%, а для кластера из 5000 узлов — 10%. Минимальное значение — 5% при количестве узлов более 5000. Таким образом, при настройках по умолчанию узел может не попасть в список возможных узлов для запуска.

Эту логику можно изменить (см. подробнее про параметр `percentageOfNodesToScore` в документации Kubernetes), но Deckhouse не дает такой возможности.

После того как были выбраны узлы, соответствующие условиям фильтрации, запускается фаза `Scoring`. Каждый плагин анализирует список отфильтрованных узлов и назначает оценку (score) каждому узлу. Оценки от разных плагинов суммируются. На этой фазе оцениваются доступные ресурсы на узлах: `pod capacity`, `affinity`, `volume provisioning` и другие. По итогам этой фазы выбирается узел с наибольшей оценкой. Если сразу несколько узлов получили максимальную оценку, узел выбирается случайным образом.

В итоге под запускается на выбранном узле.

###### Документация

* Общее описание scheduler.
* Система плагинов.
* Подробности фильтрации узлов.
* Исходный код scheduler.

<div id='как-изменитьрасширить-логику-работы-планировщика'></div>

##### Как изменить или расширить логику работы планировщика

Для изменения логики работы планировщика можно использовать механизм плагинов расширения.

Каждый плагин представляет собой вебхук, отвечающий следующим требованиям:

* Использование TLS.
* Доступность через сервис внутри кластера.
* Поддержка стандартных `Verbs` (`filterVerb = filter`, `prioritizeVerb = prioritize`).
* Также, предполагается что все подключаемые плагины могут кэшировать информацию об узле (`nodeCacheCapable: true`).

Подключить `extender` можно при помощи ресурса [KubeSchedulerWebhookConfiguration](cr.html#kubeschedulerwebhookconfiguration).

{% alert level="danger" %}
При использовании опции `failurePolicy: Fail`, в случае ошибки в работе вебхука планировщик Kubernetes прекратит свою работу, и новые поды не смогут быть запущены.
{% endalert %}

#### Как происходит ротация сертификатов kubelet?

С настройкой и включением ротации сертификатов kubelet вы можете ознакомиться в официальной документации Kubernetes.

В файле `/var/lib/kubelet/config.yaml` хранится конфигурация kubelet и указывается путь к сертификату (`tlsCertFile`) и закрытому ключу (`tlsPrivateKeyFile`).

В kubelet реализована следующая логика работы с серверными сертификатами:

* Если `tlsCertFile` и `tlsPrivateKeyFile` не пустые, то kubelet будет использовать их как сертификат и ключ по умолчанию.
  * При запросе клиента в kubelet API с указанием IP-адреса (например https://10.1.1.2:10250/), для установления соединения по TLS-протоколу будет использован закрытый ключ по умолчанию (`tlsPrivateKeyFile`). В данном случае ротация сертификатов не будет работать.
  * При запросе клиента в kubelet API с указанием названия хоста (например https://k8s-node:10250/), для установления соединения по TLS-протоколу будет использован динамически сгенерированный закрытый ключ из директории `/var/lib/kubelet/pki/`. В данном случае ротация сертификатов будет работать.
* Если `tlsCertFile` и `tlsPrivateKeyFile` пустые, то для установления соединения по TLS-протоколу будет использован динамически сгенерированный закрытый ключ из директории `/var/lib/kubelet/pki/`. В данном случае ротация сертификатов будет работать.

Поскольку в Deckhouse Kubernetes Platform для запросов в kubelet API используются IP-адреса, то в конфигурации kubelet поля `tlsCertFile` и `tlsPrivateKeyFile` не используются, а используется динамический сертификат, который kubelet генерирует самостоятельно. Также в модуле `operator-trivy` отключены проверки CIS benchmark `AVD-KCV-0088` и `AVD-KCV-0089`, которые отслеживают, были ли переданы аргументы `--tls-cert-file` и `--tls-private-key-file` для kubelet.

Kubelet использует клиентский TLS сертификат(`/var/lib/kubelet/pki/kubelet-client-current.pem`), при помощи которого может запросить у kube-apiserver новый клиентский сертификат или новый серверный сертификат(`/var/lib/kubelet/pki/kubelet-server-current.pem`).

Когда до истечения времени жизни сертификата остается 5-10% (случайное значение из диапазона) времени, kubelet запрашивает у kube-apiserver новый сертификат. С описанием алгоритма ознакомьтесь в официальной документации Kubernetes.

Чтобы kubelet успел установить сертификат до его истечения, рекомендуем устанавливать время жизни сертификатов более, чем 1 час. Время устанавливается с помощью аргумента `--cluster-signing-duration` в манифесте `/etc/kubernetes/manifests/kube-controller-manager.yaml`. По умолчанию это значение равно 1 году (8760 часов).

Если истекло время жизни клиентского сертификата, то kubelet не сможет делать запросы к kube-apiserver и не сможет обновить сертификаты. В данном случае узел (Node) будет помечен как `NotReady` и пересоздан.

#### Как вручную обновить сертификаты компонентов управляющего слоя?

Может возникнуть ситуация, когда master-узлы кластера находятся в выключенном состоянии долгое время. За это время может истечь срок действия сертификатов компонентов управляющего слоя. После включения узлов сертификаты не обновятся автоматически, поэтому это необходимо сделать вручную.

Обновление сертификатов компонентов управляющего слоя происходит с помощью утилиты `kubeadm`.
Чтобы обновить сертификаты, выполните следующие действия на каждом master-узле:

1. Найдите утилиту `kubeadm` на master-узле и создайте символьную ссылку c помощью следующей команды:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

2. Обновите сертификаты:

   ```shell
   kubeadm certs renew all
   ```

### Модуль flow-schema

Модуль применяет FlowSchema and PriorityLevelConfiguration для предотвращения перегрузки API.

`FlowSchema` устанавливает `PriorityLevel` для `list`-запросов от всех сервис-аккаунтов в пространствах имен Deckhouse (у которых установлен label `heritage: deckhouse`) к следующим apiGroup:
* `v1` (Pod, Secret, ConfigMap, Node и т. д.). Это помогает в случае большого количества основных ресурсов в кластере (например, Secret'ов или подов).
* `apps/v1` (DaemonSet, Deployment, StatefulSet, ReplicaSet и т. д.). Это помогает в случае развертывания большого количества приложений в кластере (например, Deployment'ов).
* `deckhouse.io` (custom resource'ы Deckhouse). Это помогает в случае большого количества различных кастомных ресурсов Deckhouse в кластере.
* `cilium.io` (custom resource'ы cilium). Это помогает в случае большого количества политик cilium в кластере.

Все запросы к API, соответствующие `FlowSchema`, помещаются в одну очередь.

### Модуль flow-schema: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль не имеет настроек.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['flow-schema'].config-values | format_module_configuration: moduleKebabName }}

### Модуль flow-schema: FAQ

#### Как проверить состояние priority level'ов?

Выполните:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_priority_levels
```

#### Как проверить состояние очередей priority level'ов?

Выполните:

```shell
kubectl get --raw /debug/api_priority_and_fairness/dump_queues
```

#### Полезные метрики

- `apiserver_flowcontrol_rejected_requests_total` — общее число отброшенных запросов.
- `apiserver_flowcontrol_dispatched_requests_total` — общее число обработанных запросов.
- `apiserver_flowcontrol_current_inqueue_requests` — количество запросов в очередях.
- `apiserver_flowcontrol_current_executing_requests` — количество запросов в обработке.

### Модуль ingress-nginx

Устанавливает и управляет NGINX Ingress controller с помощью Custom Resources. Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме и учитывает все особенности реализации инфраструктуры bare metal, а также кластеров Kubernetes различных типов.

Поддерживает запуск и раздельное конфигурирование одновременно нескольких NGINX Ingress controller'ов — один **основной** и сколько угодно **дополнительных**. Например, это позволяет отделять внешние и intranet Ingress-ресурсы приложений.

#### Варианты терминирования трафика

Трафик к nginx-ingress может быть отправлен несколькими способами:
- напрямую без внешнего балансировщика;
- через внешний балансировщик.

#### Терминация HTTPS

Модуль позволяет управлять для каждого из NGINX Ingress controller'а политиками безопасности HTTPS, в частности:
- параметрами HSTS;
- набором доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем [cert-manager](./modules/cert-manager/), при взаимодействии с которым возможны автоматический заказ SSL-сертификатов и их дальнейшее использование NGINX Ingress controller'ами.

#### Мониторинг и статистика

В нашей реализации `ingress-nginx` добавлена система сбора статистики в Prometheus с множеством метрик:
- по длительности времени всего ответа и апстрима отдельно;
- кодам ответа;
- количеству повторов запросов (retry);
- размерам запроса и ответа;
- методам запросов;
- типам `content-type`;
- географии распределения запросов и т. д.

Данные доступны в нескольких разрезах:
- по `namespace`;
- `vhost`;
- `ingress`-ресурсу;
- `location` (в nginx).

Все графики собраны в виде удобных досок в Grafana, при этом есть возможность drill-down'а по графикам: при просмотре, например, статистики в разрезе namespace есть возможность, нажав на ссылку на dashboard в Grafana, углубиться в статистику по `vhosts` в этом `namespace` и т. д.

#### Статистика

##### Основные принципы сбора статистики

1. На каждый запрос на стадии `log_by_lua_block` вызывается наш модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого nginx worker'а свой буфер).
2. На стадии `init_by_lua_block` для каждого nginx worker'а запускается процесс, который раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (наша собственная разработка).
3. `protobuf_exporter` запущен sidecar-контейнером в поде с ingress-controller'ом, принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и protobuf_exporter, на основании этих данных все и работает!

##### Какая статистика собирается и как она представлена

У всех собираемых метрик есть служебные лейблы, позволяющие идентифицировать экземпляр контроллера: `controller`, `app`, `instance` и `endpoint` (они видны в `/prometheus/targets`).

* Все метрики (кроме geo), экспортируемые protobuf_exporter'ом, представлены в трех уровнях детализации:
  * `ingress_nginx_overall_*` — «вид с вертолета», у всех метрик есть лейблы `namespace`, `vhost` и `content_kind`;
  * `ingress_nginx_detail_*` — кроме лейблов уровня overall, добавляются `ingress`, `service`, `service_port` и `location`;
  * `ingress_nginx_detail_backend_*` — ограниченная часть данных, собирается в разрезе по бэкендам. У этих метрик, кроме лейблов уровня detail, добавляется лейбл `pod_ip`.

* Для уровней overall и detail собираются следующие метрики:
  * `*_requests_total` — counter количества запросов (дополнительные лейблы — `scheme`, `method`);
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status`);
  * `*_request_seconds_{sum,count,bucket}` — histogram времени ответа;
  * `*_bytes_received_{sum,count,bucket}` — histogram размера запроса;
  * `*_bytes_sent_{sum,count,bucket}` — histogram размера ответа;
  * `*_upstream_response_seconds_{sum,count,bucket}` — histogram времени ответа upstream'а (используется сумма времен ответов всех upstream'ов, если их было несколько);
  * `*_lowres_upstream_response_seconds_{sum,count,bucket}` — то же самое, что и предыдущая метрика, только с меньшей детализацией (подходит для визуализации, но не подходит для расчета quantile);
  * `*_upstream_retries_{count,sum}` — количество запросов, при обработке которых были retry бэкендов, и сумма retry'ев.

* Для уровня overall собираются следующие метрики:
  * `*_geohash_total` — counter количества запросов с определенным geohash (дополнительные лейблы — `geohash`, `place`).

* Для уровня detail_backend собираются следующие метрики:
  * `*_lowres_upstream_response_seconds` — то же самое, что аналогичная метрика для overall и detail;
  * `*_responses_total` — counter количества ответов (дополнительный лейбл — `status_class`, а не просто `status`);
  * `*_upstream_bytes_received_sum` — counter суммы размеров ответов бэкенда.

### Модуль ingress-nginx: настройки

> Если модуль был выключен и вы его включаете, обратите внимание на глобальный параметр [publicDomainTemplate](./deckhouse-configure-global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонентов Deckhouse (dashboard, user-auth, grafana, upmeter  и т. п.) создаваться не будут.

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['ingress-nginx'].config-values | format_module_configuration: moduleKebabName }}

### Модуль ingress-nginx: Custom Resources
{{ site.data.schemas.ingress-nginx.crds.ingress-nginx | format_crd: "ingress-nginx" }}
{{ site.data.schemas.ingress-nginx.crds.kruise.crd_daemonsets | format_crd: "ingress-nginx" }}

### Модуль ingress-nginx: пример

{% raw %}

#### Пример для AWS (Network Load Balancer)

При создании балансировщика будут использованы все доступные в кластере зоны.

В каждой зоне балансировщик получает публичный IP. Если в зоне есть инстанс с Ingress-контроллером, A-запись с IP-адресом балансировщика из этой зоны автоматически добавляется к доменному имени балансировщика.

Если в зоне не остается инстансов с Ingress-контроллером, тогда IP автоматически убирается из DNS.

В том случае, если в зоне всего один инстанс с Ingress-контроллером, при перезапуске пода IP-адрес балансировщика этой зоны будет временно исключен из DNS.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

#### Пример для GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
```

{% endraw %}

{% alert level="warning" %}
В GCP на узлах должна присутствовать аннотация, разрешающая принимать подключения на внешние адреса для сервисов с типом NodePort.
{% endalert %}

{% raw %}

#### Пример для OpenStack

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

#### Пример для bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

#### Пример для bare metal (при использовании внешнего балансировщика, например Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и др.)

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

{% endraw %}

#### Пример для bare metal (балансировщик MetalLB в режиме BGP LoadBalancer)


```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

В случае использования MetalLB его speaker-поды должны быть запущены на тех же узлах, что и поды Ingress–контроллера.

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создается с параметром `externalTrafficPolicy: Local` (запрещая межузловой SNAT), и для удовлетворения данного параметра MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые поды.

Таким образом, для данного примера [конфигурация модуля `metallb`](./metallb/configuration.html) должна быть такой:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

#### Пример для bare metal (балансировщик MetalLB в режиме L2 LoadBalancer)


1. Включите модуль `metallb`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Создайте ресурс _MetalLoadBalancerClass_:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # селектор узлов-балансировщиков
     type: L2
   ```

1. Создайте ресурс _IngressNginxController_:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: IngressNginxController
   metadata:
     name: main
   spec:
     ingressClass: nginx
     inlet: LoadBalancer
     loadBalancer:
       loadBalancerClass: ingress
       annotations:
         # Количество адресов, которые будут выделены из пула, описанного в _MetalLoadBalancerClass_.
         network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
   ```

1. Платформа создаст сервис с типом `LoadBalancer`, которому будет присвоено заданное количество адресов:

   ```shell
   $ kubectl -n d8-ingress-nginx get svc
   NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
   main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
   ```

### Модуль ingress-nginx: FAQ

#### Как разрешить доступ к приложению внутри кластера только от ingress controller'ов?

Если вы хотите ограничить доступ к вашему приложению внутри кластера ТОЛЬКО от подов ingress'а, необходимо в под с приложением добавить контейнер с kube-rbac-proxy:

##### Пример Deployment для защищенного приложения

{% raw %}

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-app
  replicas: 1
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-app:v0.5.3
        args:
        - "--listen=127.0.0.1:8080"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 443
            scheme: HTTPS
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # Рекомендуется использовать прокси из нашего репозитория.
        args:
        - "--secure-listen-address=0.0.0.0:443"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        - "--v=2"
        - "--logtostderr=true"
        # Если kube-apiserver недоступен, мы не сможем аутентифицировать и авторизовывать пользователей.
        # Stale Cache хранит только результаты успешной авторизации и используется, только если apiserver недоступен.
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 443
          name: https
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```

{% endraw %}

Приложение принимает запросы на адресе 127.0.0.1, это означает, что по незащищенному соединению к нему можно подключиться только изнутри пода.
Прокси же слушает на адресе 0.0.0.0 и перехватывает весь внешний трафик к поду.

##### Как дать минимальные права для Service Account?

Чтобы аутентифицировать и авторизовывать пользователей с помощью kube-apiserver, у прокси должны быть права на создание `TokenReview` и `SubjectAccessReview`.

В наших кластерах уже есть готовая ClusterRole — **d8-rbac-proxy**.
Создавать ее самостоятельно не нужно! Нужно только прикрепить ее к Service Account'у вашего Deployment'а.
{% raw %}

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-namespace:my-sa:d8-rbac-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:rbac-proxy
subjects:
- kind: ServiceAccount
  name: my-sa
  namespace: my-namespace
```

##### Конфигурация Kube-RBAC-Proxy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy
data:
  config-file.yaml: |+
    excludePaths:
    - /healthz # Не требуем авторизацию для liveness пробы.
    upstreams:
    - upstream: http://127.0.0.1:8081/ # Куда проксируем.
      path: / # Location прокси, с которого запросы будут проксированы на upstream.
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: http
          name: my-app
```

{% endraw %}
Согласно конфигурации, у пользователя должны быть права на доступ к Deployment с именем `my-app`
и его дополнительному ресурсу `http` в namespace `my-namespace`.

Выглядят такие права в виде RBAC так:
{% raw %}

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/http"]
  resourceNames: ["my-app"]
  verbs: ["get", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-app
subjects:
### Все пользовательские сертификаты ingress-контроллеров выписаны для одной конкретной группы.
- kind: Group
  name: ingress-nginx:auth
```

Для ingress'а ресурса необходимо добавить параметры:

```yaml
nginx.ingress.kubernetes.io/backend-protocol: HTTPS
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_ssl_certificate /etc/nginx/ssl/client.crt;
  proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
  proxy_ssl_protocols TLSv1.2;
  proxy_ssl_session_reuse on;
```

{% endraw %}
Подробнее о том, как работает аутентификация по сертификатам, можно прочитать в документации Kubernetes.

#### Как сконфигурировать балансировщик нагрузки для проверки доступности IngressNginxController?

В ситуации, когда `IngressNginxController` размещен за балансировщиком нагрузки, рекомендуется сконфигурировать балансировщик для проверки доступности
узлов `IngressNginxController` с помощью HTTP-запросов или TCP-подключений. В то время как тестирование с помощью TCP-подключений представляет собой простой и универсальный механизм проверки доступности, мы рекомендуем использовать проверку на основе HTTP-запросов со следующими параметрами:
- протокол: `HTTP`;
- путь: `/healthz`;
- порт: `80` (в случае использования inlet'а `HostPort` нужно указать номер порта, соответствующий параметру [httpPort](cr.html#ingressnginxcontroller-v1-spec-hostport-httpport).

#### Как настроить работу через MetalLB с доступом только из внутренней сети?

Пример MetalLB с доступом только из внутренней сети.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    sourceRanges:
    - 192.168.0.0/24
```

{% alert level="warning" %}
Для работы необходимо включить параметр [`svcSourceRangeCheck`](./cni-cilium/configuration.html#parameters-svcsourcerangecheck) в модуле cni-cilium.
{% endalert %}

#### Как добавить дополнительные поля для логирования в nginx-controller?

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  additionalLogFields:
    my-cookie: "$cookie_MY_COOKIE"
```

#### Как включить HorizontalPodAutoscaling для IngressNginxController?

> **Важно.** Режим HPA возможен только для контроллеров с inlet'ом `LoadBalancer` или `LoadBalancerWithProxyProtocol`.
>
> **Важно.** Режим HPA возможен только при `minReplicas` != `maxReplicas`, в противном случае deployment `hpa-scaler` не создается.

HPA выставляется с помощью аттрибутов `minReplicas` и `maxReplicas` в [IngressNginxController CR](cr.html#ingressnginxcontroller).

IngressNginxController разворачивается с помощью DaemonSet. DaemonSet не предоставляет возможности горизонтального масштабирования, поэтому создается дополнительный deployment `hpa-scaler` и HPA resource, который следит за предварительно созданной метрикой `prometheus-metrics-adapter-d8-ingress-nginx-cpu-utilization-for-hpa`. Если CPU utilization превысит 50%, HPA закажет новую реплику для `hpa-scaler` (с учетом minReplicas и maxReplicas).

`hpa-scaler` deployment обладает HardPodAntiAffinity, поэтому он попытается заказать себе новый узел (если это возможно
в рамках своей NodeGroup), куда автоматически будет размещен еще один Ingress-контроллер.

Примечания:
* Минимальное реальное количество реплик IngressNginxController не может быть меньше минимального количества узлов в NodeGroup, в которую разворачивается IngressNginxController.
* Максимальное реальное количество реплик IngressNginxController не может быть больше максимального количества узлов в NodeGroup, в которую разворачивается IngressNginxController.

#### Как использовать IngressClass с установленными IngressClassParameters?

Начиная с версии 1.1 IngressNginxController, Deckhouse создает объект IngressClass самостоятельно. Если вы хотите использовать свой IngressClass,
например с установленными IngressClassParameters, достаточно добавить к нему label `ingress-class.deckhouse.io/external: "true"`

#### Как отключить сборку детализированной статистики Ingress-ресурсов?

По умолчанию Deckhouse собирает подробную статистику со всех Ingress-ресурсов в кластере, что может генерировать высокую нагрузку на систему мониторинга.

Для отключения сбора статистики добавьте label `ingress.deckhouse.io/discard-metrics: "true"` к соответствующему namespace или Ingress-ресурсу.

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов в пространстве имен `review-1`:

```shell
kubectl label ns review-1 ingress.deckhouse.io/discard-metrics=true
```

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов `test-site` в пространстве имен `development`:

```shell
kubectl label ingress test-site -n development ingress.deckhouse.io/discard-metrics=true
```

#### Как корректно вывести из эксплуатации (drain) узел с запущенным IngressNginxController?

Доступно два способа вывода такого узла из эксплуатации - или с помощью аннотации узла (аннотация будет удалена после завершения операции):

```shell
kubectl annotate node <node_name> update.node.deckhouse.io/draining=user
```

или с помощью базового функционала kubectl drain (тут стоит отметить, что необходимо указать флаг --force, несмотря на то, что указан флаг --ignore-daemonsets, так как IngressNginxController
разворачивается с помощью Advanced DaemonSet):

```shell
kubectl drain <node_name> --delete-emptydir-data --ignore-daemonsets --force
```

### Модуль istio

#### Таблица совместимости поддерживаемых версий

| Версия Istio | Версии K8S, поддерживаемые Istio | Статус в текущем релизе D8 |
|:------------:|:------------------------------------------------------------------------------------------------------------------------------:|:--------------------------:|
|     1.21     |                                           1.26, 1.27, 1.28, 1.29, 1.30, 1.31                                          |  Поддерживается  |
|     1.19     |                                                     1.25<sup>*</sup>, 1.26, 1.27, 1.28, 1.28, 1.29, 1.30                                                     |       Устарела и будет удалена       |

<sup>*</sup> — версия Kubernetes **НЕ поддерживается** в текущем релизе Deckhouse Kubernetes Platform.

{::options parse_block_html="false" /}

#### Задачи, которые решает Istio

Istio — фреймворк централизованного управления сетевым трафиком, реализующий подход Service Mesh.

В частности, Istio прозрачно решает для приложений следующие задачи:

* [Использование Mutual TLS:](#mutual-tls)
  * Взаимная достоверная аутентификация сервисов.
  * Шифрование трафика.
* [Авторизация доступа между сервисами.](#авторизация)
* [Маршрутизация запросов:](#маршрутизация-запросов)
  * Canary-deployment и A/B-тестирование — позволяют отправлять часть запросов на новую версию приложения.
* [Управление балансировкой запросов между endpoint'ами сервиса:](#управление-балансировкой-запросов-между-endpointами-сервиса)
  * Circuit Breaker:
    * временное исключение endpoint'а из балансировки, если превышен лимит ошибок;
    * настройка лимитов на количество TCP-соединений и количество запросов в сторону одного endpoint'а;
    * выявление зависших запросов и обрывание их с кодом ошибки (HTTP request timeout).
  * Sticky Sessions:
    * привязка запросов от конечных пользователей к endpoint'у сервиса.
  * Locality Failover — позволяет отдавать предпочтение endpoint'ам в локальной зоне доступности.
  * Балансировка gRPC-сервисов.
* [Повышение Observability:](#observability)
  * Сбор и визуализация данных для трассировки прикладных запросов с помощью Jaeger.
  * Сбор метрик о трафике между сервисами в Prometheus и визуализация их в Grafana.
  * Визуализация состояния связей между сервисами и состояния служебных компонентов Istio с помощью Kiali.
* [Организация мульти-ЦОД кластера за счет объединения кластеров в единый Service Mesh (мультикластер).](#мультикластер)
* [Объединение разрозненных кластеров в федерацию с возможностью предоставлять стандартный (в понимании Service Mesh) доступ к избранным сервисам.](#федерация)

> Рекомендуем ознакомиться с видео, где мы обсуждаем архитектуру Istio и оцениваем накладные расходы.

#### Mutual TLS

Данный механизм — это главный метод взаимной аутентификации сервисов. Принцип основывается на том, что при всех исходящих запросах проверяется серверный сертификат, а при входящих — клиентский. После проверок sidecar-proxy получает возможность идентифицировать удаленный узел и использовать эти данные для авторизации либо в прикладных целях.

Каждый сервис получает собственный идентификатор в формате `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`, где `TrustDomain` в нашем случае — это домен кластера. Каждому сервису можно выделять собственный ServiceAccount или использовать стандартный «default». Полученный идентификатор сервиса можно использовать как в правилах авторизации, так и в прикладных целях. Именно этот идентификатор используется в качестве удостоверяемого имени в TLS-сертификатах.

Данные настройки можно переопределить на уровне namespace.

#### Авторизация

Управление авторизацией осуществляется с помощью ресурса [AuthorizationPolicy](istio-cr.html#authorizationpolicy). В момент, когда для сервиса создается этот ресурс, начинает работать следующий алгоритм принятия решения о судьбе запроса:

* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного сервиса нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если явно что-то запретить, работает только запрет. Если же что-то явно разрешить, будут разрешены только явно одобренные запросы (запреты при этом имеют приоритет).

Для написания правил авторизации можно использовать следующие аргументы:

* идентификаторы сервисов и wildcard на их основе (`mycluster.local/ns/myns/sa/myapp` или `mycluster.local/*`);
* namespace;
* диапазоны IP;
* HTTP-заголовки;
* JWT-токены из прикладных запросов.

#### Маршрутизация запросов

Основной ресурс для управления маршрутизацией — [VirtualService](istio-cr.html#virtualservice), он позволяет переопределять судьбу HTTP- или TCP-запроса. Доступные аргументы для принятия решения о маршрутизации:

* Host и любые другие заголовки;
* URI;
* метод (GET, POST и пр.);
* лейблы пода или namespace источника запросов;
* dst-IP или dst-порт для не-HTTP-запросов.

#### Управление балансировкой запросов между endpoint'ами сервиса

Основной ресурс для управления балансировкой запросов — [DestinationRule](istio-cr.html#destinationrule), он позволяет настроить нюансы исходящих из подов запросов:

* лимиты/таймауты для TCP;
* алгоритмы балансировки между endpoint'ами;
* правила определения проблем на стороне endpoint'а для выведения его из балансировки;
* нюансы шифрования.

> **Важно!** Все настраиваемые лимиты работают для каждого пода клиента по отдельности! Если настроить для сервиса ограничение на одно TCP-соединение, а клиентских подов — три, то сервис получит три входящих соединения.

#### Observability

##### Трассировка

Istio позволяет осуществлять сбор трейсов с приложений и инъекцию трассировочных заголовков, если таковых нет. При этом важно понимать следующее:

* Если запрос инициирует на сервисе вторичные запросы, для них необходимо наследовать трассировочные заголовки средствами приложения.
* Jaeger для сбора и отображения трейсов потребуется устанавливать самостоятельно.

##### Grafana

В стандартной комплектации с модулем предоставлены дополнительные доски:

* доска для оценки производительности и успешности запросов/ответов между приложениями;
* доска для оценки работоспособности и нагрузки на control plane.

##### Kiali

Инструмент для визуализации дерева сервисов вашего приложения. Позволяет быстро оценить обстановку в сетевой связности благодаря визуализации запросов и их количественных характеристик непосредственно на схеме.

#### Архитектура кластера с включенным Istio

Компоненты кластера делятся на две категории:

* control plane — управляющие и обслуживающие сервисы. Под control plane обычно подразумевают поды istiod.
* data plane — прикладная часть Istio. Представляет собой контейнеры sidecar-proxy.

![Архитектура кластера с включенным Istio](./images/istio/istio-architecture.svg)
<!--- Исходник: https://docs.google.com/drawings/d/1wXwtPwC4BM9_INjVVoo1WXj5Cc7Wbov2BjxKp84qjkY/edit --->

Все сервисы из data plane группируются в mesh. Его характеристики:

* Общее пространство имен для генерации идентификатора сервиса в формате `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`. Каждый mesh имеет идентификатор TrustDomain, который в нашем случае совпадает с доменом кластера. Например: `mycluster.local/ns/myns/sa/myapp`.
* Сервисы в рамках одного mesh имеют возможность аутентифицировать друг друга с помощью доверенных корневых сертификатов.

Элементы control plane:

* `istiod` — ключевой сервис, обеспечивающий решение следующих задач:
  * Непрерывная связь с API Kubernetes и сбор информации о прикладных сервисах.
  * Обработка и валидация с помощью механизма Kubernetes Validating Webhook всех Custom Resources, которые связаны с Istio.
  * Компоновка конфигурации для каждого sidecar-proxy индивидуально:
    * генерация правил авторизации, маршрутизации, балансировки и пр.;
    * распространение информации о других прикладных сервисах в кластере;
    * выпуск индивидуальных клиентских сертификатов для организации схемы Mutual TLS. Эти сертификаты не связаны с сертификатами, которые использует и контролирует сам Kubernetes для своих служебных нужд.
  * Автоматическая подстройка манифестов, определяющих прикладные поды через механизм Kubernetes Mutating Webhook:
    * внедрение дополнительного служебного контейнера sidecar-proxy;
    * внедрение дополнительного init-контейнера для адаптации сетевой подсистемы (настройка DNAT для перехвата прикладного трафика);
    * перенаправление readiness- и liveness-проб через sidecar-proxy.
* `operator` — компонент, отвечающий за установку всех ресурсов, необходимых для работы control plane определенной версии.
* `kiali` — панель управления и наблюдения за ресурсами Istio и пользовательскими сервисами под управлением Istio, позволяющая следующее:
  * Визуализировать связи между сервисами.
  * Диагностировать проблемные связи между сервисами.
  * Диагностировать состояние control plane.

Для приема пользовательского трафика требуется доработка Ingress-контроллера:

* К подам контроллера добавляется sidecar-proxy, который обслуживает только трафик от контроллера в сторону прикладных сервисов (параметр IngressNginxController [`enableIstioSidecar`](./ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у ресурса IngressNginxController).
* Сервисы не под управлением Istio продолжают работать как раньше, запросы в их сторону не перехватываются сайдкаром контроллера.
* Запросы в сторону сервисов под управлением Istio перехватываются сайдкаром и обрабатываются в соответствии с правилами Istio (подробнее о том, [как активировать Istio для приложения](#как-активировать-istio-для-приложения)).

Контроллер istiod и каждый контейнер sidecar-proxy экспортируют собственные метрики, которые собирает кластерный Prometheus.

#### Архитектура прикладного сервиса с включенным Istio

##### Особенности

* Каждый под сервиса получает дополнительный контейнер — sidecar-proxy. Технически этот контейнер содержит два приложения:
  * **Envoy** — проксирует прикладной трафик и реализует весь функционал, который предоставляет Istio, включая маршрутизацию, аутентификацию, авторизацию и пр.
  * **pilot-agent** — часть Istio, отвечает за поддержание конфигурации Envoy в актуальном состоянии, а также содержит в себе кэширующий DNS-сервер.
* В каждом поде настраивается DNAT входящих и исходящих прикладных запросов в sidecar-proxy. Делается это с помощью дополнительного init-контейнера. Таким образом, трафик будет перехватываться прозрачно для приложений.
* Так как входящий прикладной трафик перенаправляется в sidecar-proxy, readiness/liveness-трафика это тоже касается. Подсистема Kubernetes, которая за это отвечает, не рассчитана на формирование проб в формате Mutual TLS. Для адаптации все существующие пробы автоматически перенастраиваются на специальный порт в sidecar-proxy, который перенаправляет трафик на приложение в неизменном виде.
* Для приема запросов извне кластера необходимо использовать подготовленный Ingress-контроллер:
  * Поды контроллера аналогично имеют дополнительный контейнер sidecar-proxy.
  * В отличие от подов приложения, sidecar-proxy Ingress-контроллера перехватывает только трафик от контроллера к сервисам. Входящий трафик от пользователей обрабатывает непосредственно сам контроллер.
* Ресурсы типа Ingress требуют минимальной доработки в виде добавления аннотаций:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — Ingress-контроллер в качестве upstream будет использовать ClusterIP сервиса вместо адресов подов. Балансировкой трафика между подами теперь занимается sidecar-proxy. Используйте эту опцию, только если у вашего сервиса есть ClusterIP.
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — sidecar-proxy Ingress-контроллера принимает решения о маршрутизации на основе заголовка Host. Без данной аннотации контроллер оставит заголовок с адресом сайта, например `Host: example.com`.
* Ресурсы типа Service не требуют адаптации и продолжают выполнять свою функцию. Приложениям все так же доступны адреса сервисов вида servicename, servicename.myns.svc и пр.
* DNS-запросы изнутри подов прозрачно перенаправляются на обработку в sidecar-proxy:
  * Требуется для разыменования DNS-имен сервисов из соседних кластеров.

##### Жизненный цикл пользовательского запроса

###### Приложение с выключенным Istio

<div data-presentation="./presentations/istio/request_lifecycle_istio_disabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1_lw3EyDNTFTYNirqEfrRANnEAVjGhrOCdFJc-zCOuvs/ --->

###### Приложение с включенным Istio

<div data-presentation="./presentations/istio/request_lifecycle_istio_enabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1gQfX9ge2vhp74yF5LOfpdK2nY47l_4DIvk6px_tAMPU/ --->

#### Как активировать Istio для приложения

Основная цель активации — добавить sidecar-контейнер к подам приложения, после чего Istio сможет управлять трафиком.

Рекомендованный способ добавления sidecar-ов — использовать sidecar-injector. Istio умеет «подселять» к вашим подам sidecar-контейнер с помощью механизма Admission Webhook. Настраивается с помощью лейблов и аннотаций:

* Лейбл к namespace — обращает внимание компонента sidecar-injector на ваш namespace. После применения лейбла к новым подам будут добавлены sidecar-контейнеры:
  * `istio-injection=enabled` — использует глобальную версию Istio (`spec.settings.globalVersion` в `ModuleConfig`);
  * `istio.io/rev=v1x16` — использует конкретную версию Istio для этого namespace.
* Аннотация к **поду** `sidecar.istio.io/inject` (`"true"` или `"false"`) позволяет локально переопределить политику `sidecarInjectorPolicy`. Эти аннотации работают только в namespace, обозначенных лейблами из списка выше.

Также существует возможность добавить sidecar к индивидуальному поду в namespace без установленных лейблов `istio-injection=enabled` или `istio.io/rev=vXxYZ` путем установки лейбла `sidecar.istio.io/inject=true`.

**Важно!** Istio-proxy, который работает в качестве sidecar-контейнера, тоже потребляет ресурсы и добавляет накладные расходы:

* Каждый запрос DNAT'ится в Envoy, который обрабатывает это запрос и создает еще один. На принимающей стороне — аналогично.
* Каждый Envoy хранит информацию обо всех сервисах в кластере, что требует памяти. Больше кластер — больше памяти потребляет Envoy. Решение — CustomResource [Sidecar](istio-cr.html#sidecar).

Также важно подготовить Ingress-контроллер и Ingress-ресурсы приложения:

* Включить [`enableIstioSidecar`](./ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у ресурса IngressNginxController.
* Добавить аннотации на Ingress-ресурсы приложения:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — Ingress-контроллер в качестве upstream будет использовать ClusterIP сервиса вместо адресов подов. Балансировкой трафика между подами теперь занимается sidecar-proxy. Используйте эту опцию, только если у вашего сервиса есть ClusterIP;
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — sidecar-proxy Ingress-контроллера принимает решения о маршрутизации на основе заголовка Host. Без данной аннотации контроллер оставит заголовок с адресом сайта, например `Host: example.com`.

#### Федерация и мультикластер

Поддерживаются две схемы межкластерного взаимодействия:

* [федерация](#федерация);
* [мультикластер](#мультикластер).

Принципиальные отличия:

* Федерация объединяет суверенные кластеры:
  * у каждого кластера собственное пространство имен (для namespace, Service и пр.);
  * доступ к отдельным сервисам между кластерами явно обозначен.
* Мультикластер объединяет созависимые кластеры:
  * пространство имен у кластеров общее — каждый сервис доступен для соседних кластеров так, словно он работает на локальном кластере (если это не запрещают правила авторизации).

##### Федерация

###### Требования к кластерам

* У каждого кластера должен быть уникальный домен в параметре [`clusterDomain`](./installing/configuration.html#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](./installing/configuration.html#clusterconfiguration). По умолчанию значение параметра — `cluster.local`.
* Подсети подов и сервисов в параметрах [`podSubnetCIDR`](./installing/configuration.html#clusterconfiguration-podsubnetcidr) и [`serviceSubnetCIDR`](./installing/configuration.html#clusterconfiguration-servicesubnetcidr) ресурса [*ClusterConfiguration*](./installing/configuration.html#clusterconfiguration) не должны быть уникальными.

###### Общие принципы федерации

* Федерация требует установления взаимного доверия между кластерами. Соответственно, для установления федерации нужно в кластере A сделать кластер Б доверенным и аналогично в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для прикладной эксплуатации федерации необходимо также обменяться информацией о публичных сервисах. Чтобы опубликовать сервис bar из кластера Б в кластере А, необходимо в кластере А создать ресурс ServiceEntry, который описывает публичный адрес ingress-gateway кластера Б.

<div data-presentation="./presentations/istio/federation_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1EI2MQMuVCGACnLNBXMGVDNJVhwU3vJYtVcHhrWfjLDc/ --->

###### Включение федерации

При включении федерации (параметр модуля `istio.federation.enabled = true`) происходит следующее:

* В кластер добавляется сервис `ingressgateway`, чья задача — проксировать mTLS-трафик извне кластера на прикладные сервисы.
* В кластер добавляется сервис, который экспортит метаданные кластера наружу:
  * корневой сертификат Istio (доступен без аутентификации);
  * список публичных сервисов в кластере (доступен только для аутентифицированных запросов из соседних кластеров);
  * список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).

###### Управление федерацией

<div data-presentation="./presentations/istio/federation_istio_federation_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1MpmtwJwvSL32EdwOUNpJ6GjgWt0gplzjqL8OOprNqvc/ --->

Для построения федерации необходимо сделать следующее:

* В каждом кластере создать набор ресурсов `IstioFederation`, которые описывают все остальные кластеры.
  * После успешного автосогласования между кластерами, в ресурсе `IstioFederation` заполнятся разделы `status.metadataCache.public` и `status.metadataCache.private` служебными данными, необходимыми для работы федерации.
* Каждый ресурс(`service`), который считается публичным в рамках федерации, пометить лейблом `federation.istio.deckhouse.io/public-service: ""`.
  * В кластерах из состава федерации, для каждого `service` создадутся соответствующие `ServiceEntry`, ведущие на `ingressgateway` оригинального кластера.

> Важно чтобы в этих `service`, в разделе `.spec.ports` у каждого порта обязательно было заполнено поле `name`.

##### Мультикластер

###### Требования к кластерам

* Домены кластеров в параметре [`clusterDomain`](./installing/configuration.html#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](./installing/configuration.html#clusterconfiguration) должны быть одинаковыми для всех членов мультикластера. По умолчанию значение параметра — `cluster.local`.
* Подсети подов и сервисов в параметрах [`podSubnetCIDR`](./installing/configuration.html#clusterconfiguration-podsubnetcidr) и [`serviceSubnetCIDR`](./installing/configuration.html#clusterconfiguration-servicesubnetcidr) ресурса [*ClusterConfiguration*](./installing/configuration.html#clusterconfiguration) должны быть уникальными для каждого члена мультикластера.

###### Общие принципы

<div data-presentation="./presentations/istio/multicluster_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1WeNrp0Ni2Tz3_Az0f45rkWRUZxZUDx93Om5MB3sEod8/ --->

* Мультикластер требует установления взаимного доверия между кластерами. Соответственно, для построения мультикластера нужно в кластере A сделать кластер Б доверенным и в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для сбора информации о соседних сервисах Istio подключается напрямую к API-серверу соседнего кластера. Данный модуль Deckhouse берет на себя организацию соответствующего канала связи.

###### Включение мультикластера

При включении мультикластера (параметр модуля `istio.multicluster.enabled = true`) происходит следующее:

* В кластер добавляется прокси для публикации доступа к API-серверу посредством стандартного Ingress-ресурса:
  * Доступ через данный публичный адрес ограничен авторизацией на основе Bearer-токенов, подписанных доверенными ключами. Обмен доверенными публичными ключами происходит автоматически средствами Deckhouse при взаимной настройке мультикластера.
  * Непосредственно прокси имеет read-only-доступ к ограниченному набору ресурсов.
* В кластер добавляется сервис, который экспортит метаданные кластера наружу:
  * Корневой сертификат Istio (доступен без аутентификации).
  * Публичный адрес, через который доступен API-сервер (доступен только для аутентифицированных запросов из соседних кластеров).
  * Список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).
  * Публичные ключи сервера для аутентификации запросов к API-серверу и закрытым метаданным (см. выше).

###### Управление мультикластером

<div data-presentation="./presentations/istio/multicluster_istio_multicluster_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1D3nuoC0okJQRCOY4teJ6p598Bd4JwPXZT5cdG0hW8Hc/ --->

Для сборки мультикластера необходимо в каждом кластере создать набор ресурсов `IstioMulticluster`, которые описывают все остальные кластеры.

#### Накладные расходы

Внедрение Istio повлечёт за собой дополнительные расходы ресурсов, как для **control-plane** (контроллер istiod), так и для **data-plane** (istio-сайдкары приложений).

##### control-plane

Контроллер istiod непрерывно наблюдает за конфигурацией кластера, компонует настройки для istio-сайдкаров data-plane и рассылает их по сети. Соответственно, чем больше приложений и их экземпляров, чем больше сервисов и чем чаще эта конфигурация меняется, тем больше требуется вычислительных ресурсов и больше нагрузка на сеть. При этом, поддерживается два подхода для снижения нагрузки на экземпляры контроллеров:
* горизонтальное масштабирование (настройка модуля [`controlPlane.replicasManagement`](configuration.html#parameters-controlplane-replicasmanagement)) — чем больше экземпляров контроллеров, тем меньше экземпляров istio-сайдкаров обслуживать каждому из них и тем меньше нагрузка на CPU и на сеть.
* сегментация data-plane с помощью ресурса [*Sidecar*](istio-cr.html#sidecar) (рекомендуемый подход) — чем меньше область видимости у отдельного istio-сайдкара, тем меньше требуется обновлять данных в data-plane и тем меньше нагрузка на CPU и на сеть.

Примерная оценка накладных расходов для экземпляра control-plane, который обслуживает 1000 сервисов и 2000 istio-сайдкаров — 1 vCPU и 1.5GB RAM.

##### data-plane

На потребление ресурсов data-plane (istio-сайдкары) влияет множество факторов:

* количество соединений,
* интенсивность запросов,
* размер запросов и ответов,
* протокол (HTTP/TCP),
* количество ядер CPU,
* сложность конфигурации Service Mesh.

Примерная оценка накладных расходов для экземпляра istio-сайдкара — 0.5 vCPU на 1000 запросов/сек и 50MB RAM.

istio-сайдкары также вносят задержку в сетевые запросы — примерно 2.5мс на запрос.

### Модуль istio: настройки

 
<!-- SCHEMA -->

#### Аутентификация

По умолчанию используется модуль [user-authn](./user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values istio -o json | jq '.istio.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-istio delete secret/kiali-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['istio'].config-values | format_module_configuration: moduleKebabName }}

### Модуль istio: Custom Resources
{{ site.data.schemas.istio.crds.ingress-istio | format_crd: "istio" }}
{{ site.data.schemas.istio.crds.istio.121.crd-allgen | format_crd: "istio" }}
{{ site.data.schemas.istio.crds.istio.121.crd-operator | format_crd: "istio" }}

### Модуль istio: Custom Resources (от istio.io)
{{ site.data.schemas.istio.crds.ingress-istio | format_crd: "istio" }}
{{ site.data.schemas.istio.crds.istio.121.crd-allgen | format_crd: "istio" }}
{{ site.data.schemas.istio.crds.istio.121.crd-operator | format_crd: "istio" }}

### Модуль istio: примеры

#### Circuit Breaker

Для выявления проблемных эндпоинтов используются настройки `outlierDetection` в custom resource [DestinationRule](istio-cr.html#destinationrule).
Более подробно алгоритм Outlier Detection описан в документации Envoy.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: reviews-cb-policy
spec:
  host: reviews.prod.svc.cluster.local
  trafficPolicy:
    connectionPool:
      tcp:
        maxConnections: 100 # Максимальное число коннектов в сторону host, суммарно для всех эндпоинтов.
      http:
        maxRequestsPerConnection: 10 # Каждые 10 запросов коннект будет пересоздаваться.
    outlierDetection:
      consecutive5xxErrors: 7 # Допустимо 7 ошибок (включая пятисотые, TCP-таймауты и HTTP-таймауты)
      interval: 5m            # в течение пяти минут,
      baseEjectionTime: 15m   # после которых эндпоинт будет исключен из балансировки на 15 минут.
```

А также для настройки HTTP-таймаутов используется ресурс [VirtualService](istio-cr.html#virtualservice). Эти таймауты также учитываются при подсчете статистики ошибок на эндпоинтах.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: my-productpage-rule
  namespace: myns
spec:
  hosts:
  - productpage
  http:
  - timeout: 5s
    route:
    - destination:
        host: productpage
```

#### Балансировка gRPC

**Важно!** Чтобы балансировка gRPC-сервисов заработала автоматически, присвойте name с префиксом или значением `grpc` для порта в соответствующем Service.

#### Locality Failover

> При необходимости ознакомьтесь с основной документацией.

Istio позволяет настроить приоритетный географический фейловер между эндпоинтами. Для определения зоны Istio использует лейблы узлов с соответствующей иерархией:

* `topology.istio.io/subzone`;
* `topology.kubernetes.io/zone`;
* `topology.kubernetes.io/region`.

Это полезно для межкластерного фейловера при использовании совместно с [мультикластером](#устройство-мультикластера-из-двух-кластеров-с-помощью-ресурса-istiomulticluster).

> **Важно!** Для включения Locality Failover используется ресурс DestinationRule, в котором также необходимо настроить `outlierDetection`.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: helloworld
spec:
  host: helloworld
  trafficPolicy:
    loadBalancer:
      localityLbSetting:
        enabled: true # Включили LF.
    outlierDetection: # outlierDetection включить обязательно.
      consecutive5xxErrors: 1
      interval: 1s
      baseEjectionTime: 1m
```

#### Retry

С помощью ресурса [VirtualService](istio-cr.html#virtualservice) можно настроить Retry для запросов.

**Внимание!** По умолчанию при возникновении ошибок все запросы (включая POST-запросы) выполняются повторно до трех раз.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: ratings-route
spec:
  hosts:
  - ratings.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: ratings.prod.svc.cluster.local
    retries:
      attempts: 3
      perTryTimeout: 2s
      retryOn: gateway-error,connect-failure,refused-stream
```

#### Canary

**Важно!** Istio отвечает лишь за гибкую маршрутизацию запросов, которая опирается на спецзаголовки запросов (например, cookie) или просто на случайность. За настройку этой маршрутизации и «переключение» между канареечными версиями отвечает CI/CD-система.

Подразумевается, что в одном namespace выкачено два Deployment с разными версиями приложения. У подов разных версий разные лейблы (`version: v1` и `version: v2`).

Требуется настроить два custom resource:
* [DestinationRule](istio-cr.html#destinationrule) с описанием, как идентифицировать разные версии вашего приложения (subset'ы);
* [VirtualService](istio-cr.html#virtualservice) с описанием, как распределять трафик между разными версиями приложения.

Пример:

```yaml
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: productpage-canary
spec:
  host: productpage
  # subset'ы доступны только при обращении к хосту через VirtualService из пода под управлением Istio.
  # Эти subset'ы должны быть указаны в маршрутах.
  subsets:
  - name: v1
    labels:
      version: v1
  - name: v2
    labels:
      version: v2
```

##### Распределение по наличию cookie

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - match:
    - headers:
       cookie:
         regex: "^(.*;?)?(canary=yes)(;.*)?"
    route:
    - destination:
        host: productpage
        subset: v2 # Ссылка на subset из DestinationRule.
  - route:
    - destination:
        host: productpage
        subset: v1
```

##### Распределение по вероятности

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: productpage-canary
spec:
  hosts:
  - productpage
  http:
  - route:
    - destination:
        host: productpage
        subset: v1 # Ссылка на subset из DestinationRule.
      weight: 90 # Процент трафика, который получат поды с лейблом version: v1.
  - route:
    - destination:
        host: productpage
        subset: v2
      weight: 10
```

#### Ingress для публикации приложений

##### Istio Ingress Gateway

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressIstioController
metadata:
 name: main
spec:
  # ingressGatewayClass содержит значение селектора меток, используемое при создании ресурса Gateway.
  ingressGatewayClass: istio-hp
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  nodeSelector:
    node-role/frontend: ''
  tolerations:
    - effect: NoExecute
      key: dedicated
      operator: Equal
      value: frontend
  resourcesRequests:
    mode: VPA
```

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: app-tls-secert
  namespace: d8-ingress-istio # Обратите внимание, что namespace не является app-ns.
type: kubernetes.io/tls
data:
  tls.crt: |
    <tls.crt data>
  tls.key: |
    <tls.key data>
```

```yaml
apiVersion: networking.istio.io/v1beta1
kind: Gateway
metadata:
  name: gateway-app
  namespace: app-ns
spec:
  selector:
    # Селектор меток для использования Istio Ingress Gateway main-hp.
    istio.deckhouse.io/ingress-gateway-class: istio-hp
  servers:
    - port:
        # Стандартный шаблон для использования протокола HTTP.
        number: 80
        name: http
        protocol: HTTP
      hosts:
        - app.example.com
    - port:
        # Стандартный шаблон для использования протокола HTTPS.
        number: 443
        name: https
        protocol: HTTPS
      tls:
        mode: SIMPLE
        # Secret с сертификатом и ключем, который должен быть создан в d8-ingress-istio namespace.
        # Поддерживаемые форматы Secret'ов можно посмотреть по ссылке https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#key-formats.
        credentialName: app-tls-secrets
      hosts:
        - app.example.com
```

```yaml
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: vs-app
  namespace: app-ns
spec:
  gateways:
    - gateway-app
  hosts:
    - app.example.com
  http:
    - route:
        - destination:
            host: app-svc
```

##### NGINX Ingress

Для работы с NGINX Ingress требуется подготовить:
* Ingress-контроллер, добавив к нему sidecar от Istio. В нашем случае включить параметр `enableIstioSidecar` у custom resource [IngressNginxController](./modules/ingress-nginx/cr.html#ingressnginxcontroller) модуля [ingress-nginx](./modules/ingress-nginx/).
* Ingress-ресурс, который ссылается на Service. Обязательные аннотации для Ingress-ресурса:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — с этой аннотацией Ingress-контроллер будет отправлять запросы на ClusterIP сервиса (из диапазона Service CIDR) вместо того, чтобы слать их напрямую в поды приложения. Sidecar-контейнер `istio-proxy` перехватывает трафик только в сторону диапазона Service CIDR, остальные запросы отправляются напрямую;
  * `nginx.ingress.kubernetes.io/upstream-vhost: myservice.myns.svc` — с данной аннотацией sidecar сможет идентифицировать прикладной сервис, для которого предназначен запрос.

Примеры:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: productpage
  namespace: bookinfo
  annotations:
    # Просим nginx проксировать трафик на ClusterIP вместо собственных IP подов.
    nginx.ingress.kubernetes.io/service-upstream: "true"
    # В Istio вся маршрутизация осуществляется на основе `Host:` заголовка запросов.
    # Чтобы не сообщать Istio о существовании внешнего домена `productpage.example.com`,
    # мы просто используем внутренний домен, о котором Istio осведомлен.
    nginx.ingress.kubernetes.io/upstream-vhost: productpage.bookinfo.svc
spec:
  rules:
    - host: productpage.example.com
      http:
        paths:
        - path: /
          pathType: Prefix
          backend:
            service:
              name: productpage
              port:
                number: 9080
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: productpage
  namespace: bookinfo
spec:
  ports:
  - name: http
    port: 9080
  selector:
    app: productpage
  type: ClusterIP
```

#### Примеры настройки авторизации

##### Алгоритм принятия решения

**Важно!** Как только для приложения создается `AuthorizationPolicy`, начинает работать следующий алгоритм принятия решения о судьбе запроса:
* Если запрос попадает под политику DENY — запретить запрос.
* Если для данного приложения нет политик ALLOW — разрешить запрос.
* Если запрос попадает под политику ALLOW — разрешить запрос.
* Все остальные запросы — запретить.

Иными словами, если вы явно что-то запретили, работает только ваш запрет. Если же вы что-то явно разрешили, теперь разрешены только явно одобренные запросы (запреты никуда не исчезают и имеют приоритет).

**Важно!** Для работы политик, основанных на высокоуровневых параметрах, таких как namespace или principal, необходимо, чтобы все вовлеченные сервисы работали под управлением Istio. Также между приложениями должен быть организован Mutual TLS.

Примеры:
* Запретим POST-запросы для приложения myapp. Отныне, так как для приложения появилась политика, согласно алгоритму выше будут запрещены только POST-запросы к приложению.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-post-requests
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: DENY
    rules:
    - to:
      - operation:
          methods: ["POST"]
  ```

* Здесь для приложения создана политика ALLOW. При ней будут разрешены только запросы из NS `bar`, остальные запрещены.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # default, можно не указывать.
    rules:
    - from:
      - source:
          namespaces: ["bar"]
  ```

* Здесь для приложения создана политика ALLOW. При этом она не имеет ни одного правила, и поэтому ни один запрос под нее не попадет, но она таки есть. Поэтому, согласно алгоритму, раз что-то разрешено, то все остальное запрещено. В данном случае все остальное — это вообще все запросы.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    action: ALLOW # default, можно не указывать.
    rules: []
  ```

* Здесь для приложения созданы политика ALLOW (это default) и одно пустое правило. Под это правило попадает любой запрос и автоматически получает добро.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: allow-all
    namespace: foo
  spec:
    selector:
      matchLabels:
        app: myapp
    rules:
    - {}
  ```

##### Запретить вообще все в рамках namespace foo

Два способа:

* Запретить явно. Здесь мы создаем политику DENY с единственным универсальным фильтром `{}`, под который попадают все запросы:

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec:
    action: DENY
    rules:
    - {}
  ```

* Неявно. Здесь мы создаем политику ALLOW (по умолчанию), но не создаем ни одного фильтра, так что ни один запрос под нее не попадет и будет автоматически запрещен.

  ```yaml
  apiVersion: security.istio.io/v1beta1
  kind: AuthorizationPolicy
  metadata:
    name: deny-all
    namespace: foo
  spec: {}
  ```

##### Запретить доступ только из namespace foo

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: deny-from-ns-foo
 namespace: myns
spec:
 action: DENY
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

##### Разрешить запросы только в рамках нашего namespace foo

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-intra-namespace-only
 namespace: foo
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       namespaces: ["foo"]
```

##### Разрешить из любого места в нашем кластере

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-my-cluster
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["mycluster.local/*"]
```

##### Разрешить любые запросы только кластеров foo или bar

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["foo.local/*", "bar.local/*"]
```

##### Разрешить любые запросы только кластеров foo или bar, при этом из namespace baz

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-foo-or-bar-clusters-to-ns-baz
 namespace: baz
spec:
 action: ALLOW
 rules:
 - from:
   - source: # Правила ниже логически перемножаются.
       namespaces: ["baz"]
       principals: ["foo.local/*", "bar.local/*"]
```

##### Разрешить из любого кластера (по mTLS)

**Важно!** Если есть запрещающие правила, у них будет приоритет. Смотри [алгоритм](#алгоритм-принятия-решения).

Пример:

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any-cluster-with-mtls
 namespace: myns
spec:
 action: ALLOW
 rules:
 - from:
   - source:
       principals: ["*"] # To set mTLS mandatory.
```

##### Разрешить вообще откуда угодно (в том числе без mTLS)

```yaml
apiVersion: security.istio.io/v1beta1
kind: AuthorizationPolicy
metadata:
 name: allow-all-from-any
 namespace: myns
spec:
 action: ALLOW
 rules: [{}]
```

#### Устройство федерации из двух кластеров с помощью custom resource IstioFederation

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
  trustDomain: cluster-b.local
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioFederation
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
  trustDomain: cluster-a.local
```

#### Устройство мультикластера из двух кластеров с помощью ресурса IstioMulticluster

Cluster A:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-b
spec:
  metadataEndpoint: https://istio.k8s-b.example.com/metadata/
```

Cluster B:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IstioMulticluster
metadata:
  name: cluster-a
spec:
  metadataEndpoint: https://istio.k8s-a.example.com/metadata/
```

#### Управление поведением data plane

##### Предотвратить завершение работы istio-proxy до завершения соединений основного приложения

По умолчанию в процессе остановки пода все контейнеры, включая istio-proxy, получают сигнал SIGTERM одновременно. Но некоторым приложениям для правильного завершения работы необходимо время и иногда дополнительная сетевая активность. Это невозможно, если istio-proxy завершился раньше.

Решение — добавить в istio-proxy preStop-хук для оценки активности прикладных контейнеров, а единственный доступный метод — это выявление сетевых сокетов приложения, и если таковых нет, тогда можно останавливать контейнер.

Аннотация ниже добавляет описанный выше preStop-хук в контейнер istio-proxy прикладного пода:

```yaml
annotations:
  inject.istio.io/templates: "sidecar,d8-hold-istio-proxy-termination-until-application-stops"
```

#### Ограничения режима перенаправления прикладного трафика `CNIPlugin`

В отличие от режима `InitContainer`, настройка перенаправления осуществляется в момент создании пода, а не в момент срабатывания init-контейнера `istio-init`. Это значит, что прикладные init-контейнеры не смогут взаимодействовать с остальными сервисами так как весь трафик будет перенаправлен на обработку в sidecar-контейнер `istio-proxy`, который ещё не запущен. Обходные пути:

* Запустить прикладной init-контейнер от пользователя с uid `1337`. Запросы данного пользователя не перехватываются под управление Istio.
* Исключить IP-адрес или порт сервиса из-под контроля Istio с помощью аннотаций `traffic.sidecar.istio.io/excludeOutboundIPRanges` или `traffic.sidecar.istio.io/excludeOutboundPorts`.

#### Обновление Istio

#### Обновление control plane Istio

* Deckhouse позволяет инсталлировать несколько версий control plane одновременно:
  * Одна глобальная, обслуживает namespace'ы или поды без явного указания версии (label у namespace `istio-injection: enabled`). Настраивается параметром [globalVersion](configuration.html#parameters-globalversion).
  * Остальные — дополнительные, обслуживают namespace'ы или поды с явным указанием версии (label у namespace или пода `istio.io/rev: v1x21`). Настраиваются параметром [additionalVersions](configuration.html#parameters-additionalversions).
* Istio заявляет обратную совместимость между data plane и control plane в диапазоне двух минорных версий:

* Алгоритм обновления (для примера, на версию `1.21`):
  * Добавить желаемую версию в параметр модуля [additionalVersions](configuration.html#parameters-additionalversions) (`additionalVersions: ["1.21"]`).
  * Дождаться появления соответствующего пода `istiod-v1x21-xxx-yyy` в namespace `d8-istio`.
  * Для каждого прикладного namespace, где включен istio:
    * поменять label `istio-injection: enabled` на `istio.io/rev: v1x21`;
    * по очереди пересоздать поды в namespace, параллельно контролируя работоспособность приложения.
  * Поменять настройку `globalVersion` на `1.21` и удалить `additionalVersions`.
  * Убедиться, что старый под `istiod` удалился.
  * Поменять лейблы прикладных namespace на `istio-injection: enabled`.

Чтобы найти все поды под управлением старой ревизии Istio, выполните:

```shell
kubectl get pods -A -o json | jq --arg revision "v1x21" \
  '.items[] | select(.metadata.annotations."sidecar.istio.io/status" // "{}" | fromjson |
   .revision == $revision) | .metadata.namespace + "/" + .metadata.name'
```

##### Автоматическое обновление data plane Istio

Для автоматизации обновления istio-sidecar'ов установите лейбл `istio.deckhouse.io/auto-upgrade="true"` на `Namespace` либо на отдельный ресурс — `Deployment`, `DaemonSet` или `StatefulSet`.

### Управление узлами

#### Основные функции

Управление узлами осуществляется с помощью модуля `node-manager`, основные функции которого:
1. Управление несколькими узлами как связанной группой (**NodeGroup**):
    * Возможность определить метаданные, которые наследуются всеми узлами группы.
    * Мониторинг группы узлов как единой сущности (группировка узлов на графиках по группам, группировка алертов о недоступности узлов, алерты о недоступности N узлов или N% узлов группы).
2. Систематическое прерывание работы узлов — **Chaos Monkey**. Предназначено для верификации отказоустойчивости элементов кластера и запущенных приложений.
3. Установка/обновление и настройка ПО узла (containerd, kubelet и др.), подключение узла в кластер:
    * Установка операционной системы (смотри [список поддерживаемых ОС](./supported_versions.html#linux)) вне зависимости от типа используемой инфраструктуры (на любом железе).
    * Базовая настройка операционной системы (отключение автообновления, установка необходимых пакетов, настройка параметров журналирования и т. д.).
    * Настройка nginx (и системы автоматического обновления перечня upstream’ов) для балансировки запросов от узла (kubelet) по API-серверам.
    * Установка и настройка CRI containerd и Kubernetes, включение узла в кластер.
    * Управление обновлениями узлов и их простоем (disruptions):
        * Автоматическое определение допустимой минорной версии Kubernetes группы узлов на основании ее
          настроек (указанной для группы kubernetesVersion), версии по умолчанию для всего кластера и текущей
          действительной версии control plane (не допускается обновление узлов в опережение обновления control plane).
        * Из группы одновременно производится обновление только одного узла и только если все узлы группы доступны.
        * Два варианта обновлений узлов:
            * обычные — всегда происходят автоматически;
            * требующие disruption (например, обновление ядра, смена версии containerd, значительная смена версии kubelet и пр.) — можно выбрать ручной или автоматический режим. В случае, если разрешены автоматические disruptive-обновления, перед обновлением производится drain узла (можно отключить).
    * Мониторинг состояния и прогресса обновления.
4. Масштабирование кластера.

   * Поддержание желаемого количества узлов в группе.

     Доступно при использовании [Cluster API Provider Static](#работа-со-статическими-узлами).
5. Управление Linux-пользователями на узлах.

Управление узлами осуществляется через управление группой узлов (ресурс [NodeGroup](cr.html#nodegroup)), где каждая такая группа выполняет определенные для нее задачи. Примеры групп узлов по выполняемым задачам:
- группы master-узлов;
- группа узлов маршрутизации HTTP(S)-трафика (front-узлы);
- группа узлов мониторинга;
- группа узлов приложений (worker-узлы) и т. п.

Узлы в группе имеют общие параметры и настраиваются автоматически в соответствии с параметрами группы. Deckhouse масштабирует группы, добавляя, исключая и обновляя ее узлы.

Работа со [статическими узлами](#работа-со-статическими-узлами) (например, серверами bare metal) выполняется с помощью в провайдера CAPS (Cluster API Provider Static).

Поддерживается работа со следующими сервисами Managed Kubernetes (может быть доступен не весь функционал сервиса):
- Google Kubernetes Engine (GKE);
- Elastic Kubernetes Service (EKS).

#### Типы узлов

Типы узлов, с которыми возможна работа в рамках группы узлов (ресурс [NodeGroup](cr.html#nodegroup)):
- `Static` — статический узел, размещенный на сервере bare metal или виртуальной машине.

#### Группировка узлов и управление группами

Группировка и управление узлами как связанной группой означает, что все узлы группы будут иметь одинаковые метаданные, взятые из custom resource'а [`NodeGroup`](cr.html#nodegroup).

Для групп узлов доступен мониторинг:
- с группировкой параметров узлов на графиках группы;
- с группировкой алертов о недоступности узлов;
- с алертами о недоступности N узлов или N% узлов группы и т. п.

#### Автоматическое развертывание, настройка и обновление узлов Kubernetes

Автоматическое развертывание (в *static/hybrid* — частично), настройка и дальнейшее обновление ПО работают на любых кластерах, независимо от его размещения.

##### Развертывание узлов Kubernetes

Deckhouse автоматически разворачивает узлы кластера, выполняя следующие **идемпотентные** операции:
- Настройку и оптимизацию операционной системы для работы с containerd и Kubernetes:
  - устанавливаются требуемые пакеты из репозиториев дистрибутива;
  - настраиваются параметры работы ядра, параметры журналирования, ротация журналов и другие параметры системы.
- Установку требуемых версий containerd и kubelet, включение узла в кластер Kubernetes.
- Настройку Nginx и обновление списка upstream для балансировки запросов от узла к Kubernetes API.

##### Поддержка актуального состояния узлов

Для поддержания узлов кластера в актуальном состоянии могут применяться два типа обновлений:
- **Обычные**. Такие обновления всегда применяются автоматически, и не приводят к остановке или перезагрузке узла.
- **Требующие прерывания** (disruption). Пример таких обновлений — обновление версии ядра или containerd, значительная смена версии kubelet и т. д. Для этого типа обновлений можно выбрать ручной или автоматический режим (секция параметров [disruptions](cr.html#nodegroup-v1-spec-disruptions)). В автоматическом режиме перед обновлением выполняется корректная приостановка работы узла (drain) и только после этого производится обновление.

В один момент времени производится обновление только одного узла из группы и только в том случае, когда все узлы группы доступны.

Модуль `node-manager` имеет набор встроенных метрик мониторинга, которые позволяют контролировать прогресс обновления, получать уведомления о возникающих во время обновления проблемах или о необходимости получения разрешения на обновление (ручное подтверждение обновления).

#### Работа со статическими узлами

При работе со статическими узлами функции модуля `node-manager` выполняются со следующими ограничениями:
- **Отсутствует заказ узлов.** Непосредственное выделение ресурсов (серверов bare metal, виртуальных машин, связанных ресурсов) выполняется вручную. Дальнейшая настройка ресурсов  (подключение узла к кластеру, настройка мониторинга и т.п.) выполняются полностью автоматически или частично.
- **Отсутствует автоматическое масштабирование узлов.** Доступно поддержание в группе указанного количества узлов при использовании [Cluster API Provider Static](#cluster-api-provider-static) (параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count)). Т.е. Deckhouse будет пытаться поддерживать указанное количество узлов в группе, очищая лишние узлы и настраивая новые при необходимости (выбирая их из ресурсов [StaticInstance](cr.html#staticinstance), находящихся в состоянии *Pending*).

Настройка/очистка узла, его подключение к кластеру и отключение могут выполняться следующими способами:
- **Вручную,** с помощью подготовленных скриптов.

  Для настройки сервера (ВМ) и ввода узла в кластер нужно загрузить и выполнить специальный bootstrap-скрипт. Такой скрипт генерируется для каждой группы статических узлов (каждого ресурса `NodeGroup`). Он находится в секрете `d8-cloud-instance-manager/manual-bootstrap-for-<ИМЯ-NODEGROUP>`. Пример добавления статического узла в кластер можно найти в [FAQ](examples.html#вручную).

  Для отключения узла кластера и очистки сервера (виртуальной машины) нужно выполнить скрипт `/var/lib/bashible/cleanup_static_node.sh`, который уже находится на каждом статическом узле. Пример отключения узла кластера и очистки сервера можно найти в [FAQ](faq.html#как-вручную-очистить-статический-узел).

- **Автоматически,** с помощью [Cluster API Provider Static](#cluster-api-provider-static).

  Cluster API Provider Static (CAPS) подключается к серверу (ВМ) используя ресурсы [StaticInstance](cr.html#staticinstance) и [SSHCredentials](cr.html#sshcredentials), выполняет настройку, и вводит узел в кластер.

  При необходимости (например, если удален соответствующий серверу ресурс [StaticInstance](cr.html#staticinstance) или уменьшено [количество узлов группы](cr.html#nodegroup-v1-spec-staticinstances-count)), Cluster API Provider Static подключается к узлу кластера, очищает его и отключает от кластера.

- **Вручную с последующей передачей узла под автоматическое управление** [Cluster API Provider Static](#cluster-api-provider-static).

  > Функциональность доступна начиная с версии Deckhouse 1.63.

  Для передачи существующего узла кластера под управление CAPS необходимо подготовить для этого узла ресурсы [StaticInstance](cr.html#staticinstance) и [SSHCredentials](cr.html#sshcredentials), как при автоматическом управлении в пункте выше, однако ресурс [StaticInstance](cr.html#staticinstance) должен дополнительно быть помечен аннотацией `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

##### Cluster API Provider Static

Cluster API Provider Static (CAPS), это реализация провайдера декларативного управления статическими узлами (серверами bare metal или виртуальными машинами) для проекта Cluster API Kubernetes. По сути, CAPS это дополнительный слой абстракции к уже существующему функционалу Deckhouse по автоматической настройке и очистке статических узлов с помощью скриптов, генерируемых для каждой группы узлов (см. раздел [Работа со статическими узлами](#работа-со-статическими-узлами)).

CAPS выполняет следующие функции:
- настройка сервера bare metal (или виртуальной машины) для подключения к кластеру Kubernetes;
- подключение узла в кластер Kubernetes;
- отключение узла от кластера Kubernetes;
- очистка сервера bare metal (или виртуальной машины) после отключения узла из кластера Kubernetes.

CAPS использует следующие ресурсы (CustomResource) при работе:
- **[StaticInstance](cr.html#staticinstance).** Каждый ресурс `StaticInstance` описывает конкретный хост (сервер, ВМ), который управляется с помощью CAPS.
- **[SSHCredentials](cr.html#sshcredentials)**. Содержит данные SSH, необходимые для подключения к хосту (`SSHCredentials` указывается в параметре [credentialsRef](cr.html#staticinstance-v1alpha1-spec-credentialsref) ресурса `StaticInstance`).
- **[NodeGroup](cr.html#nodegroup)**. Секция параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances) определяет необходимое количество узлов в группе и фильтр множества ресурсов `StaticInstance` которые могут использоваться в группе.

CAPS включается автоматически, если в NodeGroup заполнена секция параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances). Если в `NodeGroup` секция параметров `staticInstances` не заполнена, то настройка и очистка узлов для работы в этой группе выполняется *вручную* (см. примеры [добавления статического узла в кластер](examples.html#вручную) и [очистки узла](faq.html#как-вручную-очистить-статический-узел)), а не с помощью CAPS.

Схема работы со статичными узлами при использовании Cluster API Provider Static (CAPS) ([практический пример добавления узла](examples.html#с-помощью-cluster-api-provider-static)):
1. **Подготовка ресурсов.**

   Перед тем, как отдать сервер bare metal или виртуальную машину под управление CAPS, может быть необходима предварительная подготовка, например:
   - Подготовка системы хранения, добавление точек монтирования и т. п.;
   - Установка специфических пакетов ОС. Например, установка пакета `ceph-common`, если на сервере используется тома CEPH;
   - Настройка необходимой сетевой связанности. Например, между сервером и узлами кластера;
   - Настройка доступа по SSH на сервер, создание пользователя для управления с root-доступом через `sudo`. Хорошей практикой является создание отдельного пользователя и уникальных ключей для каждого сервера.

1. **Создание ресурса [SSHCredentials](cr.html#sshcredentials).**

   В ресурсе `SSHCredentials` указываются параметры, необходимые CAPS для подключения к серверу по SSH. Один ресурс `SSHCredentials` может использоваться для подключения к нескольким серверам, но хорошей практикой является создание уникальных пользователей и ключей доступа для подключения к каждому серверу. В этом случае ресурс `SSHCredentials` также будет отдельный на каждый сервер.

1. **Создание ресурса [StaticInstance](cr.html#staticinstance).**

   На каждый сервер (ВМ) в кластере создается отдельный ресурс `StaticInstance`. В нем указан IP-адрес для подключения и ссылка на ресурс `SSHCredentials`, данные которого нужно использовать при подключении.

   Возможные состояния `StaticInstances` и связанных с ним серверов (ВМ) и узлов кластера:
   - `Pending`. Сервер не настроен, и в кластере нет соответствующего узла.
   - `Bootstrapping`. Выполняется процедура настройки сервера (ВМ) и подключения узла в кластер.
   - `Running`. Сервер настроен, и в кластер добавлен соответствующий узел.
   - `Cleaning`. Выполняется процедура очистки сервера и отключение узла из кластера.

   > Можно отдать существующий узел кластера, заранее введенный в кластер вручную, под управление CAPS, пометив его StaticInstance аннотацией `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

1. **Создание ресурса [NodeGroup](cr.html#nodegroup).**

   В контексте CAPS в ресурсе `NodeGroup` нужно обратить внимание на параметр [nodeType](cr.html#nodegroup-v1-spec-nodetype) (должен быть `Static`) и секцию параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances).

   Секция параметров [staticInstances.labelSelector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) определяет фильтр, по которому CAPS выбирает ресурсы `StaticInstance`, которые нужно использовать в группе. Фильтр позволяет использовать для разных групп узлов только определенные `StaticInstance`, а также позволяет использовать один `StaticInstance` в разных группах узлов. Фильтр можно не определять, чтобы использовать в группе узлов любой доступный `StaticInstance`.

   Параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count) определяет желаемое количество узлов в группе.  При изменении параметра, CAPS начинает добавлять или удалять необходимое количество узлов, запуская этот процесс параллельно.

В соответствии с данными секции параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances), CAPS будет пытаться поддерживать указанное (параметр [count](cr.html#nodegroup-v1-spec-staticinstances-count)) количество узлов в группе. При необходимости добавить узел в группу, CAPS выбирает соответствующий [фильтру](cr.html#nodegroup-v1-spec-staticinstances-labelselector) ресурс [StaticInstance](cr.html#staticinstance) находящийся в статусе `Pending`, настраивает сервер (ВМ) и добавляет узел в кластер. При необходимости удалить узел из группы, CAPS выбирает [StaticInstance](cr.html#staticinstance) находящийся в статусе `Running`, очищает сервер (ВМ) и удаляет узел из кластера (после чего, соответствующий `StaticInstance` переходит в состояние `Pending` и снова может быть использован).

#### Пользовательские настройки на узлах

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](cr.html#nodegroupconfiguration). Ресурс позволяет выполнять на узлах bash-скрипты, в которых можно пользоваться набором команд bashbooster, а также позволяет использовать шаблонизатор Go Template. Это удобно для автоматизации таких операций, как:
- Установка и настройки дополнительных пакетов ОС.  

  Примеры:  
  - [установка kubectl-плагина](examples.html#установка-плагина-cert-manager-для-kubectl-на-master-узлах);
  - [настройка containerd с поддержкой Nvidia GPU](faq.html#как-использовать-containerd-с-поддержкой-nvidia-gpu).

- Обновление ядра ОС на конкретную версию.

  Примеры:
  - [обновление ядра Debian](faq.html#для-дистрибутивов-основанных-на-debian);
  - [обновление ядра CentOS](faq.html#для-дистрибутивов-основанных-на-centos).

- Изменение параметров ОС.

  Примеры:  
  - [настройка параметра sysctl](examples.html#задание-параметра-sysctl);
  - [добавление корневого сертификата](examples.html#добавление-корневого-сертификата-в-хост).

- Сбор информации на узле и выполнение других подобных действий.

Ресурс `NodeGroupConfiguration` позволяет указывать [приоритет](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) выполняемым скриптам, ограничивать их выполнение определенными [группами узлов](cr.html#nodegroupconfiguration-v1alpha1-spec-nodegroups) и [типами ОС](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).

Код скрипта указывается в параметре [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор Go Template, который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

Переменные, которые доступны для использования в шаблонизаторе:
<ul>
<li>
{% offtopic title="Пример данных..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}</li>
<li><code>.cri</code> — используемый CRI (с версии Deckhouse 1.49 используется только <code>Containerd</code>).</li>
<li><code>.kubernetesVersion</code> — используемая версия Kubernetes.</li>
<li><code>.nodeUsers</code> — массив данных о пользователях узла, добавленных через ресурс <a href="cr.html#nodeuser">NodeUser</a>.
{% offtopic title="Пример данных..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — массив данных группы узлов.
{% offtopic title="Пример данных..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.27"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Пример использования переменных в шаблонизаторе:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
### Some code for tuning user environment
{{- end }}
```

Пример использования команд bashbooster:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

Ход выполнения скриптов можно увидеть на узле в журнале сервиса bashible c помощью команды:

```bash
journalctl -u bashible.service
```  

Сами скрипты находятся на узле в директории `/var/lib/bashible/bundle_steps/`.  

Сервис принимает решение о повторном запуске скриптов путем сравнения единой контрольной суммы всех файлов, расположенной по пути `/var/lib/bashible/configuration_checksum` с контрольной суммой размещенной в кластере `kubernetes` в секрете `configuration-checksums` namespace `d8-cloud-instance-manager`.
Проверить контрольную сумму можно следующей командой:  

```bash
kubectl -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```  

Сравнение контрольных суммы сервис совершает каждую минуту.  

Контрольная сумма в кластере изменяется раз в 4 часа, тем самым повторно запуская скрипты на всех узлах.  
Принудительный вызов исполнения bashible на узле можно произвести путем удаления файла с контрольной суммой скриптов с помощью следующей команды:  

```bash
rm /var/lib/bashible/configuration_checksum
```  

##### Особенности написания скриптов

При написании скриптов важно учитывать следующие особенности их использования в Deckhouse:

1. Скрипты в deckhouse выполняются раз в 4 часа или на основании внешних триггеров. Поэтому важно писать скрипты таким образом, чтобы они производили проверку необходимости своих изменений в системе перед выполнением действий, а не производили изменения каждый раз при запуске.
2. Существуют предзаготовленные скрипты которые производят различные действия в т.ч. установку и настройку сервисов. Важно учитывать это при выборе [приоритета](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) пользовательских скриптов. Например, если в скрипте планируется произвести перезапуск сервиса, то данный скрипт должен вызываться после скрипта установки сервиса. В противном случае он не сможет выполниться при развертывании нового узла.

Полезные особенности некоторых скриптов:

* `032_configure_containerd.sh` - производит объединение всех конфигурационных файлов сервиса `containerd` расположенных по пути `/etc/containerd/conf.d/*.toml`, а также **перезапуск** сервиса. Следует учитывать что директория `/etc/containerd/conf.d/` не создается автоматически, а также что создание файлов в этой директории следует производить в скриптах с приоритетом менее `32`

#### Chaos Monkey

Инструмент (включается у каждой из `NodeGroup` отдельно), позволяющий систематически вызывать случайные прерывания работы узлов. Предназначен для проверки элементов кластера, приложений и инфраструктурных компонентов на реальную работу отказоустойчивости.

#### Мониторинг

Для групп узлов (ресурс `NodeGroup`) DKP экспортирует метрики доступности группы.

##### Какую информацию собирает Prometheus?

Все метрики групп узлов имеют префикс `d8_node_group_` в названии, и метку с именем группы `node_group_name`.

Следующие метрики собираются для каждой группы узлов:
- `d8_node_group_ready` — количество узлов группы, находящихся в статусе `Ready`;
- `d8_node_group_nodes` — количество узлов в группе (в любом статусе);
- `d8_node_group_instances` — количество инстансов в группе (в любом статусе);
- `d8_node_group_desired` — желаемое (целевое) количество объектов `Machines` в группе;
- `d8_node_group_min` — минимальное количество инстансов в группе;
- `d8_node_group_max` — максимальное количество инстансов в группе;
- `d8_node_group_up_to_date` — количество узлов в группе в состоянии up-to-date;
- `d8_node_group_standby` — количество резервных узлов (см. параметр [standby](cr.html#nodegroup-v1-spec-cloudinstances-standby)) в группе;
- `d8_node_group_has_errors` — единица, если в группе узлов есть какие-либо ошибки.

### Управление узлами: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['node-manager'].config-values | format_module_configuration: moduleKebabName }}

### Управление узлами: custom resources
{{ site.data.schemas.node-manager.crds.cluster | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.deckhousecontrolplane | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.extension-config | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.instance | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.instancetypescatalogs | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.machine-deployment | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.machine-health-check | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.machine-pools | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.machine-sets | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.machine | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.mcm | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.node_group | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.nodegroupconfiguration | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.nodeuser | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.sshcredentials | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.staticcluster | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.staticinstance | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.staticmachine | format_crd: "node-manager" }}
{{ site.data.schemas.node-manager.crds.staticmachinetemplate | format_crd: "node-manager" }}

### Управление узлами: примеры

Ниже представлены несколько примеров описания NodeGroup, а также установки плагина cert-manager для `kubectl` и задания параметра `sysctl`.

#### Примеры описания NodeGroup

<span id="пример-описания-nodegroup"></span>

##### Статические узлы

<span id="пример-описания-статической-nodegroup"></span>

Для виртуальных машин на гипервизорах или физических серверов используйте статические узлы, указав `nodeType: Static` в NodeGroup.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Узлы в такую группу добавляются [вручную](#вручную) с помощью подготовленных скриптов.

Также можно использовать способ [добавления статических узлов с помощью Cluster API Provider Static](#с-помощью-cluster-api-provider-static).

##### Системные узлы

<span id="пример-описания-статичной-nodegroup-для-системных-узлов"></span>

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
```

#### Добавление статического узла в кластер

<span id="пример-описания-статичной-nodegroup"></span>

Добавление статического узла можно выполнить вручную или с помощью Cluster API Provider Static.

##### С помощью Cluster API Provider Static

Простой пример добавления статического узла в кластер с помощью [Cluster API Provider Static (CAPS)](./#cluster-api-provider-static):

1. Подготовьте необходимые ресурсы.

   * Выделите сервер (или виртуальную машину), настройте сетевую связанность и т. п., при необходимости установите специфические пакеты ОС и добавьте точки монтирования которые потребуются на узле.

   * Создайте пользователя (в примере — `caps`) с возможностью выполнять `sudo`, выполнив **на сервере** следующую команду:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Разрешите пользователю выполнять команды через sudo без пароля. Для этого **на сервере** внесите следующую строку в конфигурацию sudo (отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   * Сгенерируйте **на сервере** пару SSH-ключей с пустой парольной фразой:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     Публичный и приватный ключи пользователя `caps` будут сохранены в файлах `caps-id.pub` и `caps-id` в текущей директории на сервере.

   * Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами **на сервере** следующие команды:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

   В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

     ```shell
     pdpl-user -i 63 caps
     ```

1. Создайте в кластере ресурс [SSHCredentials](cr.html#sshcredentials).

   В директории с ключами пользователя **на сервере** выполните следующую команду для получения закрытого ключа в формате Base64:

   ```shell
   base64 -w0 caps-id
   ```

   На любом компьютере с `kubectl`, настроенным на управление кластером, создайте переменную окружения со значением закрытого ключа созданного пользователя в Base64, полученным на предыдущем шаге:

   ```shell
    CAPS_PRIVATE_KEY_BASE64=<ЗАКРЫТЫЙ_КЛЮЧ_В_BASE64>
   ```

   Выполните следующую команду, для создания в кластере ресурса `SSHCredentials` (здесь и далее также используйте `kubectl`, настроенный на управление кластером):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: credentials
   spec:
     user: caps
     privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
   EOF
   ```

1. Создайте в кластере ресурс [StaticInstance](cr.html#staticinstance), указав IP-адрес сервера статического узла:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
   spec:
     # Укажите IP-адрес сервера статического узла.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

   > Поле `labelSelector` в ресурсе `NodeGroup` является неизменным. Чтобы обновить labelSelector, нужно создать новую NodeGroup и перенести в неё статические узлы, изменив их лейблы (labels).

1. Создайте в кластере ресурс [NodeGroup](cr.html#nodegroup):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
   EOF
   ```

##### С помощью Cluster API Provider Static и фильтрами в label selector

Пример использования фильтров в [label selector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) StaticInstance, для группировки статических узлов и использования их в разных NodeGroup. В примере используются две группы узлов (`front` и `worker`), предназначенные для разных задач, которые должны содержать разные по характеристикам узлы — два сервера для группы `front` и один для группы `worker`.

1. Подготовьте необходимые ресурсы (3 сервера или виртуальные машины) и создайте ресурс `SSHCredentials`, аналогично п.1 и п.2 [примера](#с-помощью-cluster-api-provider-static).

1. Создайте в кластере два ресурса [NodeGroup](cr.html#nodegroup) (здесь и далее используйте `kubectl`, настроенный на управление кластером):

   > Поле `labelSelector` в ресурсе `NodeGroup` является неизменным. Чтобы обновить labelSelector, нужно создать новую NodeGroup и перенести в неё статические узлы, изменив их лейблы (labels).

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 2
       labelSelector:
         matchLabels:
           role: front
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

1. Создайте в кластере ресурсы [StaticInstance](cr.html#staticinstance), указав актуальные IP-адреса серверов:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-1
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP1>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-2
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP2>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     address: "<SERVER-WORKER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

#### Пример описания `NodeUser`

```yaml
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: testuser
spec:
  uid: 1100
  sshPublicKeys:
  - "<SSH_PUBLIC_KEY>"
  passwordHash: <PASSWORD_HASH>
  isSudoer: true
```

#### Пример описания `NodeGroupConfiguration`

##### Установка плагина cert-manager для kubectl на master-узлах

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-cert-manager-plugin.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "master"
  content: |
    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/kubectl-cert_manager-linux-amd64.tar.gz -o - | tar -zxvf - kubectl-cert_manager
    mv kubectl-cert_manager /usr/local/bin
```

##### Задание параметра sysctl

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "*"
  content: |
    sysctl -w vm.max_map_count=262144
```

##### Добавление корневого сертификата в хост

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
  
При адаптации скрипта под другую ОС измените параметр [bundles](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).
{% endalert %}

{% alert level="warning" %}
Для использования сертификата в `containerd` (в т.ч. pull контейнеров из приватного репозитория) после добавления сертификата требуется произвести рестарт сервиса.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca.sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    CERT_FILE_NAME=example_ca
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates
    }

    # bb-tmp-file - Creating temp file function. More information: http://www.bashbooster.net/#tmp
    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
```

##### Добавление сертификата в ОС и containerd

{% alert level="warning" %}
Данный пример приведен для ОС Ubuntu.  
Способ добавления сертификатов в хранилище может отличаться в зависимости от ОС.
  
При адаптации скрипта под другую ОС измените параметр [bundles](cr.html#nodegroupconfiguration-v1alpha1-spec-bundles).
{% endalert %}

{% alert level="info" %}
Пример NodeGroupConfiguration основан на функциях, заложенных в скрипте [032_configure_containerd.sh](./#особенности-написания-скриптов).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-custom-ca-containerd..sh
spec:
  weight: 31
  nodeGroups:
  - '*'  
  bundles:
  - 'ubuntu-lts'
  content: |-
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/usr/local/share/ca-certificates"
    CERT_CONTENT=$(cat <<EOF
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )
    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )
    
    mkdir -p /etc/containerd/conf.d

    # bb-tmp-file - Create temp file function. More information: http://www.bashbooster.net/#tmp

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # bb-event           - Creating subscription for event function. More information: http://www.bashbooster.net/#event
    ## ca-file-updated   - Event name
    ## update-certs      - The function name that the event will call
    
    bb-event-on "ca-file-updated" "update-certs"
    
    update-certs() {          # Function with commands for adding a certificate to the store
      update-ca-certificates  # Restarting the containerd service is not required as this is done automatically in the script 032_configure_containerd.sh
    }

    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file
    ##  ca-file-updated                           - Name of event that will be called if the file changes.

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} \
      ca-file-updated   
      
    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
```

### Управление узлами: FAQ

#### Статические узлы

<span id='как-добавить-статический-узел-в-кластер'></span>
<span id='как-добавить-статичный-узел-в-кластер'></span>

Добавить статический узел в кластер можно вручную ([пример](examples.html#вручную)) или с помощью [Cluster API Provider Static](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static).

##### Как добавить статический узел в кластер (Cluster API Provider Static)?

Чтобы добавить статический узел в кластер (сервер bare-metal или виртуальную машину), выполните следующие шаги:

1. Подготовьте необходимые ресурсы:

   - Выделите сервер или виртуальную машину и убедитесь, что узел имеет необходимую сетевую связанность с кластером.

   - При необходимости установите дополнительные пакеты ОС и настройте точки монтирования, которые будут использоваться на узле.

1. Создайте пользователя с правами `sudo`:

   - Добавьте нового пользователя (в данном примере — `caps`) с правами выполнения команд через `sudo`:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   - Разрешите пользователю выполнять команды через `sudo` без ввода пароля. Для этого отредактируйте конфигурацию `sudo` (отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```shell
     caps ALL=(ALL) NOPASSWD: ALL
     ```

1. На сервере откройте файл `/etc/ssh/sshd_config` и убедитесь, что параметр `UsePAM` установлен в значение `yes`. Затем перезапустите службу `sshd`:

   ```shell
   sudo systemctl restart sshd
   ```

1. Сгенерируйте на сервере пару SSH-ключей с пустой парольной фразой:

   ```shell
   ssh-keygen -t rsa -f caps-id -C "" -N ""
   ```

   Приватный и публичный ключи будут сохранены в файлах `caps-id` и `caps-id.pub` соответственно в текущей директории.

1. Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами на сервере следующие команды:

   ```shell
   mkdir -p /home/caps/.ssh 
   cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
   chmod 700 /home/caps/.ssh 
   chmod 600 /home/caps/.ssh/authorized_keys
   chown -R caps:caps /home/caps/
   ```

1. Создайте ресурс [SSHCredentials](cr.html#sshcredentials).
1. Создайте ресурс [StaticInstance](cr.html#staticinstance).
1. Создайте ресурс [NodeGroup](cr.html#nodegroup) с [nodeType](cr.html#nodegroup-v1-spec-nodetype) `Static`, указав [желаемое количество узлов](cr.html#nodegroup-v1-spec-staticinstances-count) в группе и, при необходимости, [фильтр](cr.html#nodegroup-v1-spec-staticinstances-labelselector) выбора `StaticInstance`.

[Пример](examples.html#с-помощью-cluster-api-provider-static) добавления статического узла.

##### Как добавить несколько статических узлов в кластер вручную?

Используйте существующий или создайте новый кастомный ресурс (Custom Resource) [NodeGroup](cr.html#nodegroup) ([пример](examples.html#пример-описания-статической-nodegroup) `NodeGroup` с именем `worker`).

Автоматизировать процесс добавления узлов можно с помощью любой платформы автоматизации. Далее приведен пример для Ansible.

1. Получите один из адресов Kubernetes API-сервера. Обратите внимание, что IP-адрес должен быть доступен с узлов, которые добавляются в кластер:

   ```shell
   kubectl -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Проверьте версию K8s. Если версия >= 1.25, создайте токен `node-group`:

   ```shell
   kubectl create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Сохраните полученный токен, и добавьте в поле `token:` playbook'а Ansible на дальнейших шагах.

1. Если версия Kubernetes меньше 1.25, получите Kubernetes API-токен для специального ServiceAccount'а, которым управляет Deckhouse:

   ```shell
   kubectl -n d8-cloud-instance-manager get $(kubectl -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

1. Создайте Ansible playbook с `vars`, которые заменены на полученные на предыдущих шагах значения:

{% raw %}

   ```yaml
   - hosts: all
     become: yes
     gather_facts: no
     vars:
       kube_apiserver: <KUBE_APISERVER>
       token: <TOKEN>
     tasks:
       - name: Check if node is already bootsrapped
         stat:
           path: /var/lib/bashible
         register: bootstrapped
       - name: Get bootstrap secret
         uri:
           url: "https://{{ kube_apiserver }}/api/v1/namespaces/d8-cloud-instance-manager/secrets/manual-bootstrap-for-{{ node_group }}"
           return_content: yes
           method: GET
           status_code: 200
           body_format: json
           headers:
             Authorization: "Bearer {{ token }}"
           validate_certs: no
         register: bootstrap_secret
         when: bootstrapped.stat.exists == False
       - name: Run bootstrap.sh
         shell: "{{ bootstrap_secret.json.data['bootstrap.sh'] | b64decode }}"
         args:
           executable: /bin/bash
         ignore_errors: yes
         when: bootstrapped.stat.exists == False
       - name: wait
         wait_for_connection:
           delay: 30
         when: bootstrapped.stat.exists == False
   ```

{% endraw %}

1. Определите дополнительную переменную `node_group`. Значение переменной должно совпадать с именем `NodeGroup`, которой будет принадлежать узел. Переменную можно передать различными способами, например с использованием inventory-файла:

   ```text
   [system]
   system-0
   system-1

   [system:vars]
   node_group=system

   [worker]
   worker-0
   worker-1

   [worker:vars]
   node_group=worker
   ```

1. Запустите выполнение playbook'а с использованием inventory-файла.

##### Как вручную очистить статический узел?

<span id='как-вывести-узел-из-под-управления-node-manager'></span>

{% alert level="info" %}
Инструкция справедлива как для узла, настроенного вручную (с помощью бутстрап-скрипта), так и для узла, настроенного с помощью CAPS.
{% endalert %}

Чтобы вывести из кластера узел и очистить сервер (ВМ), выполните следующую команду на узле:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

##### Можно ли удалить StaticInstance?

`StaticInstance`, находящийся в состоянии `Pending` можно удалять без каких-либо проблем.

Чтобы удалить `StaticInstance` находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstrapping`), выполните следующие шаги:

1. Добавьте метку `"node.deckhouse.io/allow-bootstrap": "false"` в `StaticInstance`.
1. Дождитесь, пока `StaticInstance` перейдет в статус `Pending`.
1. Удалите `StaticInstance`.
1. Уменьшите значение параметра `NodeGroup.spec.staticInstances.count` на 1.

##### Как изменить IP-адрес StaticInstance?

Изменить IP-адрес в ресурсе `StaticInstance` нельзя. Если в `StaticInstance` указан ошибочный адрес, то нужно [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

##### Как мигрировать статический узел настроенный вручную под управление CAPS?

Необходимо выполнить [очистку узла](#как-вручную-очистить-статический-узел), затем [добавить](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static) узел под управление CAPS.

#### Как изменить NodeGroup у статического узла?

<span id='как-изменить-nodegroup-у-статичного-узла'><span>

Если узел находится под управлением [CAPS](./#cluster-api-provider-static), то изменить принадлежность к `NodeGroup` у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

Чтобы перенести существующий статический узел созданный [вручную](./#работа-со-статическими-узлами) из одной `NodeGroup` в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.

#### Как зачистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить `NodeGroup`, следуйте [этой инструкции](#как-изменить-nodegroup-у-статического-узла).

{% alert level="warning" %}
Если на зачищаемом узле есть пулы хранения LINSTOR/DRBD, то предварительно перенесите ресурсы с узла и удалите узел LINSTOR/DRBD, следуя [инструкции](/modules/sds-replicated-volume/stable/faq.html#как-выгнать-ресурсы-с-узла).
{% endalert %}

1. Удалите узел из кластера Kubernetes:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Запустите на узле скрипт очистки:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. После перезагрузки узла [запустите](#как-добавить-статический-узел-в-кластер) скрипт `bootstrap.sh`.

#### Как понять, что что-то пошло не так?

Если узел в NodeGroup не обновляется (значение `UPTODATE` при выполнении команды `kubectl get nodegroup` меньше значения `NODES`) или вы предполагаете какие-то другие проблемы, которые могут быть связаны с модулем `node-manager`, нужно проверить логи сервиса `bashible`. Сервис `bashible` запускается на каждом узле, управляемом модулем `node-manager`.

Чтобы проверить логи сервиса `bashible`, выполните на узле следующую команду:

```shell
journalctl -fu bashible
```

Пример вывода, когда все необходимые действия выполнены:

```console
May 25 04:39:16 kube-master-0 systemd[1]: Started Bashible service.
May 25 04:39:16 kube-master-0 bashible.sh[1976339]: Configuration is in sync, nothing to do.
May 25 04:39:16 kube-master-0 systemd[1]: bashible.service: Succeeded.
```

#### Как посмотреть, что в данный момент выполняется на узле при его создании?

Если необходимо узнать, что происходит на узле (например, узел долго создается), можно проверить логи `cloud-init`. Для этого выполните следующие шаги:

1. Найдите узел, который находится в стадии бутстрапа:

   ```shell
   kubectl get instances | grep Pending
   ```

   Пример:

   ```shell
   $ kubectl get instances | grep Pending
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Получите информацию о параметрах подключения для просмотра логов:

   ```shell
   kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   Пример:

   ```shell
   $ kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   bootstrapStatus:
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. Выполните полученную команду (в примере выше — `nc 192.168.199.178 8000`), чтобы просмотреть логи `cloud-init` и определить, на каком этапе остановилась настройка узла.

Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.

#### Как обновить ядро на узлах?

##### Для дистрибутивов, основанных на Debian

Создайте ресурс `NodeGroupConfiguration`, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

##### Для дистрибутивов, основанных на CentOS

Создайте ресурс `NodeGroupConfiguration`, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-yum-install "kernel-${desired_version}"
```

#### Какие параметры NodeGroup к чему приводят?

| Параметр NG                           | Disruption update          | Перезаказ узлов   | Рестарт kubelet |
|---------------------------------------|----------------------------|-------------------|-----------------|
| chaos                                 | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |
| cloudInstances.maxSurgePerZone        | -                          | -                 | -               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| disruptions                           | -                          | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

Подробно о всех параметрах можно прочитать в описании кастомного ресурса [NodeGroup](cr.html#nodegroup).

В случае изменения параметров `InstanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит. Количество заказываемых одновременно `MachineDeployment` определяется параметром `cloudInstances.maxSurgePerZone`.

При обновлении, которое требует прерывания работы узла (disruption update), выполняется процесс вытеснения подов с узла. Если какой-либо под не может быть вытеснен, попытка повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После истечения этого времени, поды, которые не удалось вытеснить, удаляются принудительно.

#### Как выделить узлы под специфические нагрузки?

{% alert level="warning" %}
Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов Deckhouse. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.
{% endalert %}

Для решений данной задачи существуют два механизма:

1. Установка меток в `NodeGroup` `spec.nodeTemplate.labels` для последующего использования их в `Pod` spec.nodeSelector или spec.affinity.nodeAffinity. Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
1. Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` spec.tolerations. Запрещает исполнение не разрешенных явно приложений на этих узлах.

{% alert level="info" %}
Deckhouse по умолчанию поддерживает использование taint'а с ключом `dedicated`, поэтому рекомендуется применять этот ключ с любым значением для taints на ваших выделенных узлах.

Если требуется использовать другие ключи для taints (например, `dedicated.client.com`), необходимо добавить соответствующее значение ключа в параметр [modules.placement.customTolerationKeys](./deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys). Это обеспечит разрешение системным компонентам, таким как `cni-flannel`, использовать эти узлы.
{% endalert %}

Подробности в статье на Habr.

#### Как выделить узлы под системные компоненты?

##### Фронтенд

Для Ingress-контроллеров используйте `NodeGroup` со следующей конфигурацией:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/frontend: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

##### Системные

Для компонентов подсистем Deckhouse параметр `NodeGroup` будет настроен с параметрами:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```


#### Как восстановить master-узел, если kubelet не может загрузить компоненты control plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы компонентов control plane (например, удалена директория `/var/lib/containerd`).
В этом случае kubelet при рестарте не сможет скачать образы компонентов `control plane`, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Далее приведена инструкция по восстановлению master-узла.

##### containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать и присвоить переменной `AUTH` на поврежденном master-узле.

Далее на поврежденном master-узле нужно загрузить образы компонентов `control-plane`:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить `kubelet`.

#### Как изменить CRI для NodeGroup?

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для NodeGroup, установите параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type) в `Containerd` или в `NotManaged`.

Пример YAML-манифеста NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
```

Также эту операцию можно выполнить с помощью патча:

* Для `Containerd`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Для `NotManaged`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
 При изменении `cri.type` для NodeGroup, созданных с помощью `dhctl`, необходимо обновить это значение в `dhctl config edit provider-cluster-configuration` и настройках объекта NodeGroup.
{% endalert %}

После изменения CRI для NodeGroup модуль `node-manager` будет поочередно перезагружать узлы, применяя новый CRI.  Обновление узла сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup, модуль `node-manager` либо автоматически выполнит обновление узлов, либо потребует подтверждения вручную.

#### Как изменить CRI для всего кластера?

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для всего кластера, необходимо с помощью утилиты `dhctl` отредактировать параметр `defaultCRI` в конфигурационном файле `cluster-configuration`.

Также возможно выполнить эту операцию с помощью `kubectl patch`.

* Для `Containerd`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* Для `NotManaged`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

Если необходимо, чтобы отдельные NodeGroup использовали другой CRI, перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [в документации](#как-изменить-cri-для-nodegroup).

{% alert level="danger" %}
Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера.
Рекомендуется использовать multimaster-конфигурацию и менять тип CRI только после этого.
{% endalert %}

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Чтобы определить, какой узел в текущий момент обновляется в master NodeGroup, используйте следующую команду:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердите остановку (disruption) для master-узла, полученного на предыдущем шаге:

   ```shell
   kubectl annotate node <имя master-узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождаитесь перехода обновленного master-узла в `Ready`. Выполните итерацию для следующего master-узла.

#### Как добавить шаг для конфигурации узлов?

Дополнительные шаги для конфигурации узлов задаются с помощью кастомного ресурса [NodeGroupConfiguration](cr.html#nodegroupconfiguration).

#### Как использовать containerd с поддержкой Nvidia GPU?

Необходимо создать отдельную NodeGroup для GPU-узлов:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

Далее создайте NodeGroupConfiguration для NodeGroup `gpu` для конфигурации containerd:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
  - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/nvidia_gpu.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".containerd]
          default_runtime_name = "nvidia"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = false
    EOF
  nodeGroups:
  - gpu
  weight: 31
```

Добавьте NodeGroupConfiguration для установки драйверов Nvidia для NodeGroup `gpu`.

##### Ubuntu

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - ubuntu-lts
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/apt/sources.list.d/nvidia-container-toolkit.list" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
      curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    fi
    bb-apt-install nvidia-container-toolkit nvidia-driver-535-server
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

##### Centos

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - centos
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/yum.repos.d/nvidia-container-toolkit.repo" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    fi
    bb-yum-install nvidia-container-toolkit nvidia-driver
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

После того как конфигурации будут применены, необходимо провести бутстрап и перезагрузить узлы, чтобы применить настройки и установить драйвера.

##### Как проверить, что все прошло успешно?

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: nvidia-cuda-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: nvidia-cuda-test
          image: nvidia/cuda:11.6.2-base-ubuntu20.04
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

Проверьте логи командой:

```shell
$ kubectl logs job/nvidia-cuda-test
Tue Jan 24 11:36:18 2023
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.60.13    Driver Version: 525.60.13    CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla T4            Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   45C    P0    25W /  70W |      0MiB / 15360MiB |      0%      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+

+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gpu-operator-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: gpu-operator-test
          image: nvidia/samples:vectoradd-cuda10.2
          imagePullPolicy: "IfNotPresent"
```

Проверьте логи командой:

```shell
$ kubectl logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

#### Как развернуть кастомный конфигурационный файл containerd?

{% alert level="info" %}
Пример `NodeGroupConfiguration` основан на функциях, заложенных в скрипте [032_configure_containerd.sh](./#особенности-написания-скриптов).
{% endalert %}

{% alert level="danger" %}
Добавление кастомных настроек вызывает перезапуск сервиса `containerd`.
{% endalert %}

Bashible на узлах объединяет конфигурацию containerd для Deckhouse с конфигурацией из файла `/etc/containerd/conf.d/*.toml`.

{% alert level="warning" %}
Вы можете переопределять значения параметров, которые заданы в файле `/etc/containerd/deckhouse.toml`, но их работу придётся обеспечивать самостоятельно. Также, лучше изменением конфигурации не затрагивать master-узлы (nodeGroup `master`).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_option.toml - << EOF
    oom_score = 500
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

##### Как добавить авторизацию в дополнительный registry?

Разверните скрипт `NodeGroupConfiguration`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
              endpoint = ["https://${REGISTRY_URL}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
              auth = "AAAABBBCCCDDD=="
    EOF
  nodeGroups:
    - "*"
  weight: 31
```

##### Как настроить сертификат для дополнительного registry?

{% alert level="info" %}
Помимо containerd, сертификат можно [одновременно добавить](examples.html#добавление-сертификата-в-ос-и-containerd) и в операционной системе.
{% endalert %}

Пример `NodeGroupConfiguration` для настройки сертификата для дополнительного registry:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-cert-containerd.sh
spec:
  bundles:
  - '*'
  content: |-
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"
    CERT_CONTENT=$(cat <<"EOF"
    -----BEGIN CERTIFICATE-----
    MIIDSjCCAjKgAwIBAgIRAJ4RR/WDuAym7M11JA8W7D0wDQYJKoZIhvcNAQELBQAw
    JTEjMCEGA1UEAxMabmV4dXMuNTEuMjUwLjQxLjIuc3NsaXAuaW8wHhcNMjQwODAx
    MTAzMjA4WhcNMjQxMDMwMTAzMjA4WjAlMSMwIQYDVQQDExpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAL1p
    WLPr2c4SZX/i4IS59Ly1USPjRE21G4pMYewUjkSXnYv7hUkHvbNL/P9dmGBm2Jsl
    WFlRZbzCv7+5/J+9mPVL2TdTbWuAcTUyaG5GZ/1w64AmAWxqGMFx4eyD1zo9eSmN
    G2jis8VofL9dWDfUYhRzJ90qKxgK6k7tfhL0pv7IHDbqf28fCEnkvxsA98lGkq3H
    fUfvHV6Oi8pcyPZ/c8ayIf4+JOnf7oW/TgWqI7x6R1CkdzwepJ8oU7PGc0ySUWaP
    G5bH3ofBavL0bNEsyScz4TFCJ9b4aO5GFAOmgjFMMUi9qXDH72sBSrgi08Dxmimg
    Hfs198SZr3br5GTJoAkCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB
    /wQCMAAwUwYDVR0RBEwwSoIPbmV4dXMuc3ZjLmxvY2FsghpuZXh1cy41MS4yNTAu
    NDEuMi5zc2xpcC5pb4IbZG9ja2VyLjUxLjI1MC40MS4yLnNzbGlwLmlvMA0GCSqG
    SIb3DQEBCwUAA4IBAQBvTjTTXWeWtfaUDrcp1YW1pKgZ7lTb27f3QCxukXpbC+wL
    dcb4EP/vDf+UqCogKl6rCEA0i23Dtn85KAE9PQZFfI5hLulptdOgUhO3Udluoy36
    D4WvUoCfgPgx12FrdanQBBja+oDsT1QeOpKwQJuwjpZcGfB2YZqhO0UcJpC8kxtU
    by3uoxJoveHPRlbM2+ACPBPlHu/yH7st24sr1CodJHNt6P8ugIBAZxi3/Hq0wj4K
    aaQzdGXeFckWaxIny7F1M3cIWEXWzhAFnoTgrwlklf7N7VWHPIvlIh1EYASsVYKn
    iATq8C7qhUOGsknDh3QSpOJeJmpcBwln11/9BGRP
    -----END CERTIFICATE-----
    EOF
    )

    CONFIG_CONTENT=$(cat <<EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
        ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
    )

    mkdir -p ${CERTS_FOLDER}
    mkdir -p /etc/containerd/conf.d

    # bb-tmp-file - Create temp file function. More information: http://www.bashbooster.net/#tmp

    CERT_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CERT_CONTENT}" > "${CERT_TMP_FILE}"  
    
    CONFIG_TMP_FILE="$( bb-tmp-file )"
    echo -e "${CONFIG_CONTENT}" > "${CONFIG_TMP_FILE}"  

    # bb-sync-file                                - File synchronization function. More information: http://www.bashbooster.net/#sync
    ## "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"    - Destination file
    ##  ${CERT_TMP_FILE}                          - Source file

    bb-sync-file \
      "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" \
      ${CERT_TMP_FILE} 

    bb-sync-file \
      "/etc/containerd/conf.d/${REGISTRY_URL}.toml" \
      ${CONFIG_TMP_FILE} 
  nodeGroups:
  - '*'  
  weight: 31
```

#### Как использовать NodeGroup с приоритетом?

С помощью параметра [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) кастомного ресурса `NodeGroup` можно задавать порядок заказа узлов в кластере.
Например, можно сделать так, чтобы сначала заказывались узлы типа *spot-node*, а если они закончились — обычные узлы.

Пример создания двух `NodeGroup` с использованием узлов типа spot-node:

```yaml
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-spot
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-spot
    maxPerZone: 5
    minPerZone: 0
    priority: 50
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 0
    priority: 30
  nodeType: CloudEphemeral
```

В приведенном выше примере, `cluster-autoscaler` сначала попытается заказать узел типа *_spot-node*. Если в течение 15 минут его не получится добавить в кластер, NodeGroup `worker-spot` будет поставлен на паузу (на 20 минут) и `cluster-autoscaler` начнет заказывать узлы из NodeGroup `worker`.
Если через 30 минут в кластере возникнет необходимость развернуть еще один узел, `cluster-autoscaler` сначала попытается заказать узел из NodeGroup `worker-spot` и только потом — из NodeGroup `worker`.

После того как NodeGroup `worker-spot` достигнет своего максимума (5 узлов в примере выше), узлы будут заказываться из NodeGroup `worker`.

Шаблоны узлов (labels/taints) для NodeGroup `worker` и `worker-spot` должны быть одинаковыми, или как минимум подходить для той нагрузки, которая запускает процесс увеличения кластера.

#### Как интерпретировать состояние группы узлов?

**Ready** — группа узлов содержит минимально необходимое число запланированных узлов с состоянием `Ready` для всех зон.

Пример 1. Группа узлов в состоянии `Ready`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Пример 2. Группа узлов в состоянии `Not Ready`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating** — группа узлов содержит как минимум один узел, в котором присутствует аннотация с префиксом `update.node.deckhouse.io` (например, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval** — группа узлов содержит как минимум один узел, в котором присутствует аннотация `update.node.deckhouse.io/disruption-required` и
отсутствует аннотация `update.node.deckhouse.io/disruption-approved`.

**Scaling** — рассчитывается только для групп узлов с типом `CloudEphemeral`. Состояние `True` может быть в двух случаях:

1. Когда число узлов меньше *желаемого числа узлов в группе, то есть когда нужно увеличить число узлов в группе*.
1. Когда какой-то узел помечается к удалению или число узлов больше *желаемого числа узлов*, то есть когда нужно уменьшить число узлов в группе.

*Желаемое число узлов* — это сумма всех реплик, входящих в группу узлов.

Пример. Желаемое число узлов равно 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error** — содержит последнюю ошибку, возникшую при создании узла в группе узлов.

#### Как заставить werf игнорировать состояние Ready в группе узлов?

werf проверяет состояние `Ready` у ресурсов и в случае его наличия дожидается, пока значение станет `True`.

Создание (обновление) ресурса [nodeGroup](cr.html#nodegroup) в кластере может потребовать значительного времени на развертывание необходимого количества узлов. При развертывании такого ресурса в кластере с помощью werf (например, в рамках процесса CI/CD) развертывание может завершиться по превышении времени ожидания готовности ресурса. Чтобы заставить werf игнорировать состояние `nodeGroup`, необходимо добавить к `nodeGroup` следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

#### Что такое ресурс Instance?

Ресурс `Instance` в Kubernetes представляет собой описание объекта эфемерной виртуальной машины, но без конкретной реализации. Это абстракция, которая используется для управления машинами, созданными с помощью таких инструментов, как MachineControllerManager или Cluster API Provider Static.

Объект не содержит спецификации. Статус содержит:

1. Ссылку на `InstanceClass`, если он существует для данной реализации.
1. Ссылку на объект Node Kubernetes.
1. Текущий статус машины.
1. Информацию о том, как проверить [логи создания машины](#как-посмотреть-что-в-данный-момент-выполняется-на-узле-при-его-создании) (появляется на этапе создания машины).

При создании или удалении машины создается или удаляется соответствующий объект Instance.
Самостоятельно ресурс `Instance` создать нельзя, но можно удалить. В таком случае машина будет удалена из кластера (процесс удаления зависит от деталей реализации).

#### Когда требуется перезагрузка узлов?

Некоторые операции по изменению конфигурации узлов могут потребовать перезагрузки.

Перезагрузка узла может потребоваться при изменении некоторых настроек sysctl, например, при изменении параметра `kernel.yama.ptrace_scope` (изменяется при использовании команды `astra-ptrace-lock enable/disable` в Astra Linux).

### Модуль kube-dns

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

> **Внимание!** Модуль удаляет ранее установленные kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS.

### Модуль kube-dns: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['kube-dns'].config-values | format_module_configuration: moduleKebabName }}

### Модуль kube-dns: примеры

#### Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    upstreamNameservers:
    - 8.8.8.8
    - 8.8.4.4
    hosts:
    - domain: one.example.com
      ip: 192.168.0.1
    - domain: two.another.example.com
      ip: 10.10.0.128
    stubZones:
    - zone: consul.local
      upstreamNameservers:
      - 10.150.0.1
    enableLogs: true
    clusterDomainAliases:
    - foo.bar
    - baz.qux
```

### Модуль kube-dns: FAQ

#### Как поменять домен кластера с минимальным простоем?

Добавьте новый домен и сохраните предыдущий. Для этого измените конфигурацию параметров:

1. В [controlPlaneManager.apiserver](./control-plane-manager/configuration.html):

   - [controlPlaneManager.apiserver.certSANs](./control-plane-manager/configuration.html#parameters-apiserver-certsans),
   - [apiserver.serviceAccount.additionalAPIAudiences](./control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiaudiences),
   - [apiserver.serviceAccount.additionalAPIIssuers](./control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiissuers).

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     enabled: true
     settings:
       apiserver:
         certSANs:
          - kubernetes.default.svc.<старый clusterDomain>
          - kubernetes.default.svc.<новый clusterDomain>
         serviceAccount:
           additionalAPIAudiences:
           - https://kubernetes.default.svc.<старый clusterDomain>
           - https://kubernetes.default.svc.<новый clusterDomain>
           additionalAPIIssuers:
           - https://kubernetes.default.svc.<старый clusterDomain>
           - https://kubernetes.default.svc.<новый clusterDomain>
   ```

1. В [kubeDns.clusterDomainAliases](configuration.html#параметры):

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-dns
   spec:
     version: 1
     enabled: true
     settings:
       clusterDomainAliases:
         - <старый clusterDomain>
         - <новый clusterDomain>
   ```

1. Дождитесь перезапуска `kube-apiserver`.
1. Поменяйте `clusterDomain` на новый в `dhctl config edit cluster-configuration`.

**Важно!** Если версия вашего Kubernetes 1.20 и выше, контроллеры для работы с API-server гарантированно используют расширенные токены для ServiceAccount'ов. Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`).
При смене `clusterDomain` API-server начнет выдавать токены с новым `service-account-issuer`, но благодаря произведенной конфигурации `additionalAPIAudiences` и `additionalAPIIssuers` по-прежнему будет принимать старые токены. По истечении 48 минут (80% от 3607 секунд) Kubernetes начнет обновлять выпущенные токены, при обновлении будет использован новый `service-account-issuer`. Через 90 минут (3607 секунд и немного больше) после перезагрузки kube-apiserver можете удалить конфигурацию `serviceAccount` из конфигурации `control-plane-manager`.

**Важно!** Если вы используете модуль [istio](./modules/istio/), после смены `clusterDomain` обязательно потребуется рестарт всех прикладных подов под управлением Istio.

### Модуль local-path-provisioner

Позволяет пользователям Kubernetes использовать локальное хранилище на узлах.

#### Как это работает?

Для каждого custom resource [LocalPathProvisioner](cr.html) создается соответствующий `StorageClass`.

Допустимая топология для `StorageClass` вычисляется на основе списка `nodeGroup` из custom resource. Топология используется при шедулинге подов.

Когда под заказывает диск, то:
- создается `HostPath` PV;
- `Provisioner` создает на нужном узле локальную папку по пути, состоящем из параметра `path` custom resource, имени PV и имени PVC.
  
  Пример пути:

  ```shell
  /opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
  ```

#### Ограничения

- Ограничение на размер диска не поддерживается для локальных томов.

### Модуль local-path-provisioner: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль не требует конфигурации.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['local-path-provisioner'].config-values | format_module_configuration: moduleKebabName }}

### Модуль local-path-provisioner: custom resources
{{ site.data.schemas.local-path-provisioner.crds.local_path_provisioner | format_crd: "local-path-provisioner" }}

### Модуль local-path-provisioner: примеры

#### Пример custom resource `LocalPathProvisioner`

Reclaim policy устанавливается по умолчанию в `Retain`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

#### Пример custom resource `LocalPathProvisioner` с установленным `reclaimPolicy`

Reclaim policy устанавливается в `Delete`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
  reclaimPolicy: "Delete"
```

### Модуль local-path-provisioner: FAQ

#### Как настроить Prometheus на использование локального хранилища?

Применить custom resource `LocalPathProvisioner`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

- `spec.nodeGroups` должен совпадать с NodeGroup, где запущен под Prometheus’а.
- `spec.path` - путь на узле, где будут лежать данные.

Добавить в конфигурацию модуля `prometheus` следующие параметры:

```yaml
longtermStorageClass: localpath-system
storageClass: localpath-system
```

Дождаться переката подов Prometheus.

### Модуль namespace-configurator

Позволяет автоматически управлять аннотациями и label'ами на namespace'ах.

Модуль полезен тем, что помогает автоматически включать новые namespace'ы в мониторинг посредством добавления лейбла `extended-monitoring.deckhouse.io/enabled=true`.

##### Как работает

Модуль следит за изменениями namespace и своей конфигурации:
* Всем namespace'ам, попадающим под шаблон `includeNames` и не попадающим под шаблон `excludeNames`, будут назначены соответствующие label'ы и аннотации из конфигурации.
* При изменении конфигурации модуля соответствующие label'ы и аннотации на namespace'ах будут переназначены согласно конфигурациии.

##### Что нужно настроить?

Необходимо перечислить список желаемых label'ов и аннотаций, а также список шаблонов поиска namespace в конфигурации модуля.

### Модуль namespace-configurator: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['namespace-configurator'].config-values | format_module_configuration: moduleKebabName }}

### Модуль namespace-configurator: примеры

#### Пример

Этот пример добавит лейбл `extended-monitoring.deckhouse.io/enabled=true` и аннотацию `foo=bar` к каждому namespace, начинающемуся с `prod-` или `infra-`, за исключением `infra-test`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: namespace-configurator
spec:
  version: 1
  enabled: true
  settings:
    configurations:
    - annotations:
        foo: bar
      labels:
        extended-monitoring.deckhouse.io/enabled: "true"
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```

### Модуль priority-class

Модуль создает в кластере набор классов приоритета (PriorityClass) и назначает их компонентам, установленным Deckhouse, и приложениям в кластере.

Функциональность классов приоритета реализуется планировщиком (scheduler), который позволяет учитывать приоритет пода (определяемый его принадлежностью к классу) при планировании.

Например, при развертывании в кластере подов с `priorityClassName: production-low`, если в кластере не будет доступных ресурсов для данного пода, Kubernetes начнет вытеснять поды с наименьшим приоритетом.
То есть сначала будут вытеснены все поды с `priorityClassName: develop`, затем — с `cluster-low` и так далее.

При указании класса приоритета очень важно понимать тип приложения и окружение, в котором оно будет работать. Указание любого класса приоритета не уменьшит его фактический приоритет, так как если у пода не установлен приоритет, то планировщик считает его самым низким.

{% alert level="warning" %}
Нельзя использовать классы приоритета `system-node-critical`, `system-cluster-critical`, `cluster-medium`, `cluster-low`.
{% endalert %}

Устанавливаемые модулем классы приоритета (в порядке приоритета от высшего к низшему):

| Класс приоритета          | Описание                                                                                                                                                                                                                                                                                                                                                              | Значение   |
|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| `system-node-critical`    | Компоненты кластера, которые обязаны присутствовать на узле. Также полностью защищает от вытеснения kubelet'ом.<br>Примеры: `node-exporter`, `csi` и другие.                                                                                                                                  | 2000001000 |
| `system-cluster-critical` | Компоненты кластера, без которых его корректная работа невозможна. Этим PriorityClass'ом обязательно помечаются MutatingWebhooks и Extension API servers. Также полностью защищает от вытеснения kubelet'ом.<br>Примеры: `kube-dns`, `kube-proxy`, `cni-flannel`, `cni-cillium` и другие.     | 2000000000 |
| `production-high`         | Stateful-приложения, отсутствие которых в production-окружении приводит к полной недоступности сервиса или потере данных.<br>Примеры: `PostgreSQL`, `Memcached`, `Redis`, `MongoDB` и другие.                                                                                                                                                                         | 9000       |
| `cluster-medium`          | Компоненты кластера, влияющие на мониторинг (алерты, диагностика) и автомасштабирование. Без мониторинга невозможно оценить масштабы происшествия, без автомасштабирования — предоставить приложениям необходимые ресурсы.<br>Примеры: `deckhouse`, `node-local-dns`, `grafana`, `upmeter` и другие.                                                                  | 7000       |
| `production-medium`       | Основные stateless-приложения в production-окружении, которые отвечают за работу сервиса для посетителей.                                                                                                                                                                                                                                                             | 6000       |
| `deployment-machinery`    | Компоненты кластера, используемые для сборки и деплоя в кластер.                                                                                                                                                                                                                                                                                                      | 5000       |
| `production-low`          | Приложения в production-окружении (cron-задания, административные панели, batch-процессы), без которых можно обойтись некоторое время. Если batch или cron-задачи нельзя прерывать, их следует отнести к `production-medium`.                                                                                                                                         | 4000       |
| `staging`                 | Staging-окружения для приложений.                                                                                                                                                                                                                                                                                                                                     | 3000       |
| `cluster-low`             | Компоненты кластера, без которых эксплуатация возможна, но которые желательны. <br>Примеры: `dashboard`, `cert-manager`, `prometheus` и другие.                                                                                                                                                                                                                       | 2000       |
| `develop` (по умолчанию)  | Develop-окружения для приложений. Класс по умолчанию, если не указан иной класс.                                                                                                                                                                                                                                                                                      | 1000       |
| `standby`                 | Класс не предназначен для приложений. Используется в системных целях для резервирования узлов.                                                                                                                                                                                                                                                                        | -1         |

### Модуль priority-class: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

Модуль не имеет настроек.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['priority-class'].config-values | format_module_configuration: moduleKebabName }}
## Подсистема Deckhouse

### Модуль console

Консоль (модуль «console») упрощает управление кластером Deckhouse Kubernetes Platform и делает
состояние системы наглядным.

Если шаблон публичных доменов `%s.example.com`, то в веб-приложение можно зайти по адресу
`https://console.example.com`. Доступ к интерфейсу будет у администраторов, а для не-администраторов
доступ запрещен.

#### Основные возможности

- Обзор кластера, актуальной версии, состояния системы и обновлений
- Управление модулями и их настройками
- Управление узлами: конфигурация узлов, масштабирование, параметры обновления
- Управление тенантами: проекты, созданные на основании шаблонов
- Управление доступом: провайдеры аутентификации, права групп и пользователей
- Ингресс-контроллеры: заведение трафика в кластер
- Журналирование: сбор логов с узлов и подов, отправка в различные типы хранилищ
- Мониторинг: обработка и отправка метрик, создание алертов и recording rule, дашборды и источники данных для Grafana, настройки Prometheus и список горящих алертов
- Поддержка GitOps: специально отмечены ресурсы Kubernetes, созданные автоматикой (werf, Argo CD, Helm)
- Метрики и мониторинг в узлах, группах узлов и в ингресс-контроллерах
- Состояние подов Prometheus, ингресс-контроллеров и поды на узлах
- И многое другое!

#### Как включить

Чтобы включить модуль, создайте ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: console
spec:
  enabled: true
```

#### Требования к ресурсам

Потребление ресурсов подами серверной части в зависимости от количества одновременных пользователей
отображено в таблице ниже

| Пользователей | ЦП, ядра | Память, МиБ |
| ------------: | -------: | ----------: |
|             0 |   0.0005 |          18 |
|             1 |   0.0500 |          25 |
|            10 |   0.4000 |          53 |
|           100 |   0.6500 |         130 |

Ограничение на вертикальное масштабирование подов: минимальные значения CPU/памяти в 100m/100MiB и максимальные значения в 1/512MiB. 
Две реплики серверной части включаются автоматически для DKP в режиме высокой доступности.

### Конфигурация

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['console'].config-values | format_module_configuration: moduleKebabName }}

### Модуль deckhouse

Этот модуль настраивает в Deckhouse:
- **[Уровень логирования](configuration.html#parameters-loglevel)**
- **[Набор модулей](configuration.html#parameters-bundle), включенных по умолчанию**

  Обычно используется набор модулей `Default`, который подходит в большинстве случаев.

  Независимо от используемого набора включенных по умолчанию модулей любой модуль может быть явно включен или выключен в конфигурации Deckhouse (подробнее [про включение и отключение модуля](./#включение-и-отключение-модуля)).
- **[Канал обновлений](configuration.html#parameters-releasechannel)**

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](./deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](./deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](./deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

  Deckhouse может использовать **ручной** или **автоматический** режим обновлений.

  В ручном режиме обновлений автоматически применяются только важные исправления (patch-релизы), и для перехода на более свежий релиз Deckhouse требуется [ручное подтверждение](./cr.html#deckhouserelease-v1alpha1-approved).

  В автоматическом режиме обновлений, если в кластере **не установлены** [окна обновлений](configuration.html#parameters-update-windows), переход на более свежий релиз Deckhouse осуществляется сразу после его появления на соответствующем канале обновлений. Если же в кластере **установлены** окна обновлений, переход на более свежий релиз Deckhouse начнется в ближайшее доступное окно обновлений после появления свежего релиза на соответствующем канале обновлений.
  
- **Сервис валидирования custom resource'ов**

  Сервис валидирования предотвращает создание custom resource'ов с некорректными данными или внесение таких данных в уже существующие custom resource'ы. Отслеживаются только custom resource'ы, находящиеся под управлением модулей Deckhouse.

#### Обновление релизов Deckhouse

##### Просмотр статуса релизов Deckhouse

Список последних релизов в кластере можно получить командной `kubectl get deckhousereleases`. По умолчанию хранятся 10 последних релизов и все будущие.
Каждый релиз может иметь один из следующих статусов:
* `Pending` — релиз находится в ожидании, ждет окна обновления, настроек канареечного развертывания и т. д. Подробности можно увидеть с помощью команды `kubectl describe deckhouserelease $name`.
* `Deployed` — релиз применен. Это значит, что образ пода Deckhouse уже поменялся на новую версию,
 но при этом процесс обновления всех компонентов кластера идет асинхронно, так как зависит от многих настроек.
* `Superseded` — релиз устарел и больше не используется.
* `Suspended` — релиз был отменен (например, в нем обнаружилась ошибка). Релиз переходит в этот статус, если его отменили и при этом он еще был применен в кластере.

##### Процесс обновления

В момент перехода в статус `Deployed` релиз меняет версию (tag) образа Deckhouse. После запуска Deckhouse начнет проверку и обновление всех модулей, которые поменялись с предыдущего релиза. Длительность обновления сильно зависит от настроек и размера кластера.
Например, если у вас много `NodeGroup`, они будут обновляться продолжительное время, если много `IngressNginxController` — они будут
обновляться по одному и это тоже займет некоторое время.

##### Ручное применение релизов

Если у вас стоит [ручной режим обновления](usage.html#ручное-подтверждение-обновлений) и скопилось несколько релизов,
вы можете отметить их одобренными к применению все сразу. В таком случае Deckhouse будет обновляться последовательно, сохраняя порядок релизов и меняя статус каждого примененного релиза.

##### *Закрепление* релиза

Под *закреплением* релиза подразумевается полное или частичное отключение автоматического обновления версий Deckhouse.

Есть три варианта ограничения автоматического обновления Deckhouse:
- Установить ручной режим обновления.

  В этом случае вы остановитесь на текущей версии, сможете получать обновления в кластер, но для применения обновления необходимо будет выполнить [ручное действие](usage.html#ручное-подтверждение-обновлений). Это носится и к patch-версиям, и к минорным версиям.
  
  Для установки ручного режима обновления необходимо в ModuleConfig `deckhouse` установить параметр [settings.update.mode](configuration.html#parameters-update-mode) в `Manual`:

  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"Manual"}}}}'
  ```
  
- Установить режим автоматического обновления для патч-версий.

  В этом случае вы остановитесь на текущем релизе, но будете получать patch-версии текущего релиза. Для применения обновления минорной версии релиза необходимо будет выполнить [ручное действие](usage.html#ручное-подтверждение-обновлений).
  
  Например: текущая версия DKP `v1.65.2`, после установки режима автоматического обновления для патч-версий, Deckhouse сможет обновиться до версии `v1.65.6`, но не будет обновляться до версии `v1.66.*` и выше.

  Для установки режима автоматического обновления для патч-версий необходимо в ModuleConfig `deckhouse` установить параметр [settings.update.mode](configuration.html#parameters-update-mode) в `AutoPatch`:

  ```shell
  kubectl patch mc deckhouse --type=merge -p='{"spec":{"settings":{"update":{"mode":"AutoPatch"}}}}'
  ```

- Установить конкретный тег для Deployment `deckhouse` и удалить параметр [releaseChannel](configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

  В таком случае DKP останется на конкретной версии, никакой информации о новых доступных версиях (объекты DeckhouseRelease) в кластере появляться не будет.

  Пример установки версии `v1.66.3` для DKP EE и удаления параметра `releaseChannel` из конфигурации модуля `deckhouse`:

  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- kubectl set image deployment/deckhouse deckhouse=registry.deckhouse.ru/deckhouse/ee:v1.66.3
  kubectl patch mc deckhouse --type=json -p='[{"op": "remove", "path": "/spec/settings/releaseChannel"}]'
  ```

### Модуль deckhouse: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['deckhouse'].config-values | format_module_configuration: moduleKebabName }}

### Модуль deckhouse: FAQ

#### Как запустить kube-bench в кластере?

Вначале необходимо зайти внутрь пода Deckhouse:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- bash
```

Далее необходимо выбрать, на каком узле запустить kube-bench.

* Запуск на случайном узле:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl create -f -
  ```

* Запуск на конкретном узле, например на control-plane:

  ```shell
  curl -s https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml | kubectl apply -f - --dry-run=client -o json | jq '.spec.template.spec.tolerations=[{"operator": "Exists"}] | .spec.template.spec.nodeSelector={"node-role.kubernetes.io/control-plane": ""}' | kubectl create -f -
  ```

Далее можно проверить результат выполнения:

```shell
kubectl logs job.batch/kube-bench
```

{% alert level="warning" %}
В Deckhouse установлен срок хранения логов — 7 дней. Однако, в соответствии с требованиями безопасности указанными в kube-bench, логи должны храниться не менее 30 дней. Используйте отдельное хранилище для логов, если вам необходимо хранить логи более 7 дней.
{% endalert %}

#### Как собрать информацию для отладки?

Мы всегда рады помочь пользователям с расследованием сложных проблем. Пожалуйста, выполните следующие шаги, чтобы мы смогли вам помочь:

1. Выполните следующую команду, чтобы собрать необходимые данные:

   ```sh
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse \
     -- deckhouse-controller collect-debug-info \
     > deckhouse-debug-$(date +"%Y_%m_%d").tar.gz
   ```

2. Отправьте получившийся архив команде Deckhouse для дальнейшего расследования.

Данные, которые будут собраны:
* состояние очереди Deckhouse;
* Deckhouse values. За исключением значений `kubeRBACProxyCA` и `registry.dockercfg`;
* список включенных модулей;
* `events` из всех пространств имен;
* манифесты controller'ов и подов из всех пространств имен Deckhouse;
* все объекты `nodegroups`;
* все объекты `nodes`;
* все объекты `machines`;
* все объекты `instances`;
* все объекты `staticinstances`;
* данные о текущей версии пода deckhouse;
* все объекты `deckhousereleases`;
* логи Deckhouse;
* логи machine controller manager;
* логи cloud controller manager;
* логи cluster autoscaler;
* логи Vertical Pod Autoscaler admission controller;
* логи Vertical Pod Autoscaler recommender;
* логи Vertical Pod Autoscaler updater;
* логи Prometheus;
* метрики terraform-state-exporter. За исключением значений в `provider` из `providerClusterConfiguration`;
* все горящие уведомления в Prometheus.

#### Как отлаживать проблемы в подах с помощью ephemeral containers?

Выполните следующую команду:

```shell
kubectl -n <namespace_name> debug -it <pod_name> --image=ubuntu <container_name>
```

Подробнее можно почитать в официальной документации.

#### Как отлаживать проблемы на узлах с помощью ephemeral containers?

Выполните следующую команду:

```shell
kubectl debug node/mynode -it --image=ubuntu
```

Подробнее можно почитать в официальной документации.

### Модуль deckhouse-tools

Этот модуль создает веб-интерфейс со ссылками на скачивание утилит Deckhouse (в настоящее время – [Deckhouse CLI](./deckhouse-cli/) под различные операционные системы).

Адрес веб-интерфейса формируется в соответствии с шаблоном [publicDomainTemplate](./deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse (ключ `%s` заменяется на `tools`).

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс будет доступен по адресу `tools-kube.company.my`.

### Модуль deckhouse-tools: настройки

У модуля нет обязательных настроек.

#### Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-tools
spec:
  enabled: true
  version: 1
```

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['deckhouse-tools'].config-values | format_module_configuration: moduleKebabName }}

### Модуль deckhouse-tools: примеры


### Модуль documentation

Этот модуль создает веб-интерфейс с документацией, соответствующей запущенной версии Deckhouse.

Это может быть полезно, например, когда Deckhouse работает в сети с ограничением доступа в интернет.

Адрес веб-интерфейса формируется следующим образом: в шаблоне [publicDomainTemplate](./deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `documentation`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс документации будет доступен по адресу `documentation-kube.company.my`.

### Модуль documentation: настройки

У модуля нет обязательных настроек.

 
<!-- SCHEMA -->

#### Аутентификация

По умолчанию используется модуль [user-authn](./user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values documentation -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-system delete secret/documentation-basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['documentation'].config-values | format_module_configuration: moduleKebabName }}

### Модуль documentation: примеры

#### Пример конфигурации модуля

Ниже представлен простой пример конфигурации модуля:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: documentation
spec:
  version: 1
  enabled: true
  settings:
    nodeSelector:
      node-role/example: ""
    tolerations:
    - key: dedicated
      operator: Equal
      value: example
    externalAuthentication:
      authURL: "https://<applicationDomain>/auth"
      authSignInURL: "https://<applicationDomain>/sign-in"
      authResponseHeaders: "Authorization"
```
## Подсистема Мониторинг

### Модуль extended-monitoring

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — включает расширенный сбор метрик и отправку алертов по свободному месту и inode на узлах, плюс включает «расширенный мониторинг» объектов в namespace, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=""`;
- `image-availability-exporter` — добавляет метрики и включает отправку алертов, позволяющих узнать о проблемах с доступностью образа контейнера в registry, прописанному в поле `image` из spec пода в `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`;
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик;
- `cert-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них.

### Модуль extended-monitoring: настройки

 
<!-- SCHEMA -->

#### Как использовать `extended-monitoring-exporter`

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на namespace лейбл `extended-monitoring.deckhouse.io/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый);
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create);
- поставить руками (`kubectl label namespace my-app-production extended-monitoring.deckhouse.io/enabled=""`);
- настроить через [namespace-configurator](./namespace-configurator/) модуль.

Сразу же после этого для всех поддерживаемых Kubernetes-объектов в данном namespace в Prometheus появятся default-метрики + любые кастомные с префиксом `threshold.extended-monitoring.deckhouse.io/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-объекты) Kubernetes-объектов, описанных ниже, мониторинг включается автоматически.

К Kubernetes-объектам `threshold.extended-monitoring.deckhouse.io/что-то свое` можно добавить любые другие лейблы с указанным значением. Пример: `kubectl label pod test threshold.extended-monitoring.deckhouse.io/disk-inodes-warning=30`.
В таком случае значение из лейбла заменит значение по умолчанию.

Слежение за объектом можно отключить индивидуально, поставив на него лейбл `extended-monitoring.deckhouse.io/enabled=false`. Соответственно, отключатся и лейблы по умолчанию, а также все алерты, привязанные к лейблам.

##### Стандартные лейблы и поддерживаемые Kubernetes-объекты

Далее приведен список используемых в Prometheus Rules лейблов, а также их стандартные значения.

**Обратите внимание,** что все лейблы начинаются с префикса `threshold.extended-monitoring.deckhouse.io/`. Указанное в лейбле значение — число, которое устанавливает порог срабатывания алерта.

Например, лейбл `threshold.extended-monitoring.deckhouse.io/5xx-warning: "5"` на Ingress-ресурсе изменяет порог срабатывания алерта с 10% (по умолчанию) на 5%.

###### Non-namespaced Kubernetes-объекты

Non-namespaced Kubernetes-объекты не нуждаются в лейблах на namespace и мониторинг на них включается по умолчанию при включении модуля.

####### Node

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 70             |
| disk-bytes-critical                     | int (percent) | 80             |
| disk-inodes-warning                     | int (percent) | 90             |
| disk-inodes-critical                    | int (percent) | 95             |
| load-average-per-core-warning           | int           | 3              |
| load-average-per-core-critical          | int           | 10             |

> **Важно!** Эти лейблы **не** действуют для тех разделов, в которых расположены `imagefs` (по умолчанию — `/var/lib/docker`) и `nodefs` (по умолчанию — `/var/lib/kubelet`).
Для этих разделов пороги настраиваются полностью автоматически согласно eviction thresholds в kubelet.
Значения по умолчанию см. тут, подробнее см. экспортер.

###### Namespaced Kubernetes-объекты

####### Под

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 85             |
| disk-bytes-critical                     | int (percent) | 95             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |

####### Ingress

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning            | int (percent) | 10            |
| 5xx-critical           | int (percent) | 20            |

####### Deployment

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable. Сработает, если недоступно реплик больше на указанное значение, чем разрешено в `maxUnavailable`. То есть при нуле сработает, если недоступно больше, чем указано в `maxUnavailable`, а при единице сработает, если недоступно больше, чем указано в `maxUnavailable`, плюс 1. Таким образом, у конкретных Deployment, которые находятся в namespace со включенным расширенным мониторингом и которым допустимо быть недоступными, можно подкрутить этот параметр, чтобы не получать ненужные алерты.

####### StatefulSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable (см. комментарии к [Deployment](#deployment)).

####### DaemonSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable (см. комментарии к [Deployment](#deployment)).

####### CronJob

Работает только выключение через лейбл `extended-monitoring.deckhouse.io/enabled=false`.

##### Как работает

Модуль экспортирует в Prometheus специальные лейблы Kubernetes-объектов. Позволяет улучшить Prometheus-правила путем добавления порога срабатывания для алертов.
Использование метрик, экспортируемых данным модулем, позволяет, например, заменить «магические» константы в правилах.

До:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> 1
```

После:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> on (namespace, statefulset)
(
  max by (namespace, statefulset) (extended_monitoring_statefulset_threshold{threshold="replicas-not-ready"})
)
```
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['extended-monitoring'].config-values | format_module_configuration: moduleKebabName }}

### Модуль loki

Модуль предназначен для организации хранилища логов.

Модуль использует проект Grafana Loki.

Модуль разворачивает хранилище логов на базе Grafana Loki, при необходимости настраивает модуль [log-shipper](./log-shipper/) на использование модуля loki и добавляет в Grafana соответствующий datasource.

{% alert level="warning" %}
Модуль не поддерживает работу в режиме высокой доступности, что ограничивает его использование. Для хранения важных журналов рекомендуется использовать другое хранилище.
{% endalert %}

### Модуль loki: настройки

 
<!-- SCHEMA -->

#### Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 10
    retentionPeriodHours: 48
  enabled: true
  version: 1
```
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['loki'].config-values | format_module_configuration: moduleKebabName }}

### Модуль loki: примеры

{% raw %}

#### Чтение логов из всех подов из указанного namespace и направление их в Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 30
    retentionPeriodHours: 168
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: development-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - development
  destinationRefs:
    - d8-loki
```

Больше примеров в описании модуля [log-shipper](./log-shipper/examples.html).

{% endraw %}

### Модуль log-shipper

Модуль разворачивает агенты `log-shipper` для сборки логов на узлы кластера.
Предназначение этих агентов — с минимальными изменениями отправить логи дальше из кластера.
Каждый агент — это отдельный vector, конфигурацию для которого сгенерировал Deckhouse.

![log-shipper architecture](./images/log-shipper/log_shipper_architecture.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1cOm5emdfPqWp9NT1UrB__TTL31lw7oCgh0VicQH-ouc/edit -->

1. Deckhouse следит за ресурсами [ClusterLoggingConfig](cr.html#clusterloggingconfig), [ClusterLogDestination](cr.html#clusterlogdestination) и [PodLoggingConfig](cr.html#podloggingconfig).
   Комбинация конфигурации для сбора логов и направления для отправки называется `pipeline`.
2. Deckhouse генерирует конфигурационный файл и сохраняет его в `Secret` в Kubernetes.
3. `Secret` монтируется всем подам агентов `log-shipper`, конфигурация обновляется при ее изменении с помощью sidecar-контейнера `reloader`.

#### Топологии отправки

Этот модуль отвечает за агентов на каждом узле. Однако подразумевается, что логи из кластера отправляются согласно одной из описанных ниже топологий.

##### Распределенная

Агенты шлют логи напрямую в хранилище, например в Loki или Elasticsearch.

![log-shipper distributed](./images/log-shipper/log_shipper_distributed.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1FFuPgpDHUGRdkMgpVWXxUXvfZTsasUhEh8XNz7JuCTQ/edit -->

* Менее сложная схема для использования.
* Доступна из коробки без лишних зависимостей, кроме хранилища.
* Сложные трансформации потребляют больше ресурсов на узлах для приложений.

##### Централизованная

Все логи отсылаются в один из доступных агрегаторов, например, Logstash, Vector.
Агенты на узлах стараются отправить логи с узла максимально быстро с минимальным потреблением ресурсов.
Сложные преобразования применяются на стороне агрегатора.

![log-shipper centralized](./images/log-shipper/log_shipper_centralized.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1TL-YUBk0CKSJuKtRVV44M9bnYMq6G8FpNRjxGxfeAhQ/edit -->

* Меньше потребление ресурсов для приложений на узлах.
* Пользователи могут настроить в агрегаторе любые трансформации и слать логи в гораздо большее количество хранилищ.
* Количество выделенных узлов под агрегаторы может увеличиваться вверх и вниз в зависимости от нагрузки.

##### Потоковая

Главная задача данной архитектуры — как можно быстрее отправить логи в очередь сообщений, из которой они в служебном порядке будут переданы в долгосрочное хранилище для дальнейшего анализа.

![log-shipper stream](./images/log-shipper/log_shipper_stream.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1R7vbJPl93DZPdrkSWNGfUOh0sWEAKnCfGkXOvRvK3mQ/edit -->

* Те же плюсы и минусы, что и у централизованной архитектуры, но добавляется еще одно промежуточное хранилище.
* Повышенная надежность. Подходит тем, для кого доставка логов является наиболее критичной.

#### Метаданные

При сборе логов сообщения будут обогащены метаданными в зависимости от способа их сбора. Обогащение происходит на этапе `Source`.

##### Kubernetes

Следующие поля будут экспортированы:

| Label        | Pod spec path           |
|--------------|-------------------------|
| `pod`        | metadata.name           |
| `namespace`  | metadata.namespace      |
| `pod_labels` | metadata.labels         |
| `pod_ip`     | status.podIP            |
| `image`      | spec.containers[].image |
| `container`  | spec.containers[].name  |
| `node`       | spec.nodeName           |
| `pod_owner`  | metadata.ownerRef[0]    |

| Label        | Node spec path                            |
|--------------|-------------------------------------------|
| `node_group` | metadata.labels[].node.deckhouse.io/group |

{% alert -%}
Для Splunk поля `pod_labels` не экспортируются, потому что это вложенный объект, который не поддерживается самим Splunk.
{%- endalert %}

##### File

Единственный лейбл — это `host`, в котором записан hostname сервера.

#### Фильтры сообщений

Существуют два фильтра, чтобы снизить количество отправляемых сообщений в хранилище, — `log filter` и `label filter`.

![log-shipper pipeline](./images/log-shipper/log_shipper_pipeline.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1SnC29zf4Tse4vlW_wfzhggAeTDY2o9wx9nWAZa_A6RM/edit -->

Они запускаются сразу после объединения строк с помощью multiline parser'а.

1. `label filter` — правила запускаются для метаданных сообщения. Поля для метаданных (или лейблов) наполняются на основании источника логов, так что для разных источников будет разный набор полей. Эти правила полезны, например, чтобы отбросить сообщения от определенного контейнера или пода с/без какой-то метки.
2. `log filter` — правила запускаются для исходного сообщения. Есть возможность отбросить сообщение на основании JSON-поля или, если сообщение не в формате JSON, использовать регулярное выражение для поиска по строке.

Оба фильтра имеют одинаковую структурированную конфигурацию:
* `field` — источник данных для запуска фильтрации (чаще всего это значение label'а или поля из JSON-документа).
* `operator` — действие для сравнения, доступные варианты — In, NotIn, Regex, NotRegex, Exists, DoesNotExist.
* `values` — эта опция имеет разные значения для разных операторов:
  * DoesNotExist, Exists — не поддерживается;
  * In, NotIn — значение поля должно равняться или не равняться одному из значений в списке values;
  * Regex, NotRegex — значение должно подходить хотя бы под одно или не подходить ни под одно регулярное выражение из списка values.

Вы можете найти больше примеров в разделе [Примеры](examples.html) документации.

{% alert -%}
Extra labels добавляются на этапе `Destination`, поэтому невозможно фильтровать логи на их основании.
{%- endalert %}

### Модуль log-shipper: настройки

Модуль начинает чтение логов, только если создан pipeline в виде связанных между собой [ClusterLoggingConfig](cr.html#clusterloggingconfig)/[PodLoggingConfig](cr.html#podloggingconfig) и [ClusterLogDestination](cr.html#clusterlogdestination).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['log-shipper'].config-values | format_module_configuration: moduleKebabName }}

### Модуль log-shipper: Custom Resources
{{ site.data.schemas.log-shipper.crds.cluster-log-destination | format_crd: "log-shipper" }}
{{ site.data.schemas.log-shipper.crds.cluster-logging-config | format_crd: "log-shipper" }}
{{ site.data.schemas.log-shipper.crds.pod-logging-config | format_crd: "log-shipper" }}

### Модуль log-shipper: примеры

{% raw %}

#### Чтение логов из всех подов кластера и направление их в Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Чтение логов подов из указанного namespace с указанным label и перенаправление одновременно в Loki и Elasticsearch

Чтение логов подов из namespace `whispers` только с label `app=booking` и перенаправление одновременно в Loki и Elasticsearch:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: whispers-booking-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - whispers
    labelSelector:
      matchLabels:
        app: booking
  destinationRefs:
  - loki-storage
  - es-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: logs-%F
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

#### Создание source в namespace и чтение логов всех подов в этом NS с направлением их в Loki

Следующий pipeline создает source в namespace `test-whispers`, читает логи всех подов в этом NS и пишет их в Loki:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Чтение только подов в указанном namespace и с определенным label

Пример чтения только подов, имеющих label `app=booking`, в namespace `test-whispers`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: booking
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Переход с Promtail на Log-Shipper

В ранее используемом URL Loki требуется убрать путь `/loki/api/v1/push`.

**Vector** сам добавит этот путь при работе с Loki.

#### Добавление Loki в Deckhouse Grafana

Вы можете работать с Loki из встроенной в Deckhouse Grafana. Достаточно добавить [**GrafanaAdditionalDatasource**](./modules/prometheus/cr.html#grafanaadditionaldatasource).

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: loki
spec:
  access: Proxy
  basicAuth: false
  jsonData:
    maxLines: 5000
    timeInterval: 30s
  type: loki
  url: http://loki.loki:3100
```

#### Поддержка Elasticsearch < 6.X

Для Elasticsearch < 6.0 нужно включить поддержку doc_type индексов.
Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Укажите значение здесь. Оно не должно начинаться с '_'.
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

#### Шаблон индекса для Elasticsearch

Существует возможность отправлять сообщения в определенные индексы на основе метаданных с помощью шаблонов индексов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ namespace }}-%F"
```

В приведенном выше примере для каждого пространства имен Kubernetes будет создан свой индекс в Elasticsearch.

Эта функция также хорошо работает в комбинации с `extraLabels`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ service }}-{{ namespace }}-%F"
  extraLabels:
    service: "{{ service_name }}"
```

1. Если сообщение имеет формат JSON, поле `service_name` этого документа JSON перемещается на уровень метаданных.
2. Новое поле метаданных `service` используется в шаблоне индекса.

#### Пример интеграции со Splunk

Существует возможность отсылать события из Deckhouse в Splunk.

1. Endpoint должен быть таким же, как имя вашего экземпляра Splunk с портом `8088` и без указания пути, например `https://prd-p-xxxxxx.splunkcloud.com:8088`.
2. Чтобы добавить token для доступа, откройте пункт меню `Setting` -> `Data inputs`, добавьте новый `HTTP Event Collector` и скопируйте token.
3. Укажите индекс Splunk для хранения логов, например `logs`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: splunk
spec:
  type: Splunk
  splunk:
    endpoint: https://prd-p-xxxxxx.splunkcloud.com:8088
    token: xxxx-xxxx-xxxx
    index: logs
    tls:
      verifyCertificate: false
      verifyHostname: false
```

{% endraw %}
{% alert -%}
`destination` не поддерживает метки пода для индексирования. Рассмотрите возможность добавления нужных меток с помощью опции `extraLabels`.
{%- endalert %}
{% raw %}

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

#### Простой пример Logstash

Чтобы отправлять логи в Logstash, на стороне Logstash должен быть настроен входящий поток `tcp` и его кодек должен быть `json`.

Пример минимальной конфигурации Logstash:

```hcl
input {
  tcp {
    port => 12345
    codec => json
  }
}
output {
  stdout { codec => json }
}
```

Пример манифеста `ClusterLogDestination`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: logstash
spec:
  type: Logstash
  logstash:
    endpoint: logstash.default:12345
```

#### Syslog

Следующий пример показывает, как отправлять сообщения через сокет по протоколу TCP в формате syslog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: rsyslog
spec:
  type: Socket
  socket:
    mode: TCP
    address: 192.168.0.1:3000
    encoding: 
      codec: Syslog
  extraLabels:
    syslog.severity: "alert"
    # поле request_id должно присутствовать в сообщении
    syslog.message_id: "{{ request_id }}"
```

#### Пример интеграции с Graylog

Убедитесь, что в Graylog настроен входящий поток для приема сообщений по протоколу TCP на указанном порту. Пример манифеста для интеграции с Graylog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-socket2-dest
spec:
  type: Socket
  socket:
    address: graylog.svc.cluster.local:9200
    mode: TCP
    encoding:
      codec: GELF
```

#### Логи в CEF формате

Существует способ формировать логи в формате CEF, используя `codec: CEF`, с переопределением `cef.name` и `cef.severity` по значениям из поля `message` (лога приложения) в формате JSON.

В примере ниже `app` и `log_level` это ключи содержащие значения для переопределения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: siem-kafka
spec:
  extraLabels:
    cef.name: '{{ app }}'
    cef.severity: '{{ log_level }}'
  type: Kafka
  kafka:
    bootstrapServers:
      - my-cluster-kafka-brokers.kafka:9092
    encoding:
      codec: CEF
    tls:
      verifyCertificate: false
      verifyHostname: true
    topic: logs
```

Так же можно вручную задать свои значения:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

#### Сбор событий Kubernetes

События Kubernetes могут быть собраны log-shipper'ом, если `events-exporter` включен в настройках модуля [extended-monitoring](./extended-monitoring/).

Включите events-exporter, изменив параметры модуля `extended-monitoring`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: extended-monitoring
spec:
  version: 1
  settings:
    events:
      exporterEnabled: true
```

Выложите в кластер следующий `ClusterLoggingConfig`, чтобы собирать сообщения с пода `events-exporter`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubernetes-events
spec:
  type: KubernetesPods
  kubernetesPods:
    labelSelector:
      matchLabels:
        app: events-exporter
    namespaceSelector:
      matchNames:
      - d8-monitoring
  destinationRefs:
  - loki-storage
```

#### Фильтрация логов

Пользователи могут фильтровать логи, используя следующие фильтры:

* `labelFilter` — применяется к метаданным, например имени контейнера (`container`), пространству имен (`namespace`) или имени пода (`pod_name`);
* `logFilter` — применяется к полям самого сообщения, если оно в JSON-формате.

##### Сборка логов только для контейнера `nginx`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: nginx-logs
spec:
  type: KubernetesPods
  labelFilter:
  - field: container
    operator: In
    values: [nginx]
  destinationRefs:
  - loki-storage
```

##### Сборка логов без строки, содержащей `GET /status" 200`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
  labelFilter:
  - field: message
    operator: NotRegex
    values:
    - .*GET /status" 200$
```

##### Аудит событий kubelet'а

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
  - field: userAgent  
    operator: Regex
    values: ["kubelet.*"]
  destinationRefs:
  - loki-storage
```

##### Системные логи Deckhouse

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: system-logs
spec:
  type: File
  file:
    include:
    - /var/log/syslog
  labelFilter:
  - field: message
    operator: Regex
    values:
    - .*d8-kubelet-forker.*
    - .*containerd.*
    - .*bashible.*
    - .*kernel.*
  destinationRefs:
  - loki-storage
```

{% endraw %}
{% alert -%}
Если вам нужны только логи одного пода или малой группы подов, постарайтесь использовать настройки `kubernetesPods`, чтобы сузить количество читаемых файлов. Фильтры необходимы только для высокогранулярной настройки.
{%- endalert %}
{% raw %}

#### Настройка сборки логов с продуктовых namespace'ов, используя опцию namespace label selector

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: production-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchLabels:
          environment: production
  destinationRefs:
  - loki-storage
```

#### Исключение подов и пространств имён, используя label

Существует преднастроенный label для исключения определенных подов и пространств имён: `log-shipper.deckhouse.io/exclude=true`.
Он помогает остановить сбор логов с подов и пространств имён без изменения глобальной конфигурации.

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
  labels:
    log-shipper.deckhouse.io/exclude: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  ...
  template:
    metadata:
      labels:
        log-shipper.deckhouse.io/exclude: "true"
```

#### Включение буферизации

Настройка буферизации логов необходима для улучшения надежности и производительности системы сбора логов. Буферизация может быть полезна в следующих случаях:

1. Временные перебои с подключением. Если есть временные перебои или нестабильность соединения с системой хранения логов (например, с Elasticsearch), буфер позволяет временно сохранять логи и отправить их, когда соединение восстановится.

1. Сглаживание пиков нагрузки. При внезапных всплесках объема логов буфер позволяет сгладить пиковую нагрузку на систему хранения логов, предотвращая её перегрузку и потенциальную потерю данных.

1. Оптимизация производительности. Буферизация помогает оптимизировать производительность системы сбора логов за счёт накопления логов и отправки их группами, что снижает количество сетевых запросов и улучшает общую пропускную способность.

##### Пример включения буферизации в оперативной памяти

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    memory:
      maxEvents: 4096
    type: Memory
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

##### Пример включения буферизации на диске

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

##### Пример определения поведения при переполнении буфера

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
    whenFull: DropNewest
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

Более подробное описание параметров доступно [в ресурсе ClusterLogDestination](cr.html#clusterlogdestination).

{% endraw %}

### The log-shipper module: FAQ

#### Как добавить авторизацию в ресурс _ClusterLogDestination_?

Чтобы добавить параметры авторизации в ресурс [ClusterLogDestination](cr.html#clusterlogdestination), необходимо:
- изменить [протокол](cr.html#clusterlogdestination-v1alpha1-spec-loki-endpoint) подключения к Loki на HTTPS;
- добавить секцию [auth](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth), в которой:
  - параметр [strategy](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-strategy) установить в `Bearer`;
  - в параметре [token](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-token) указать токен `log-shipper-token` из пространства имен `d8-log-shipper`.

Пример:

- Ресурс _ClusterLogDestination_ без авторизации:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "http://loki.d8-monitoring:3100"
  ```

- Получите токен `log-shipper-token` из пространства имен `d8-log-shipper`:

  ```bash
  kubectl -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
  ```

- Ресурс _ClusterLogDestination_ с авторизацией:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "https://loki.d8-monitoring:3100"
      auth:
        strategy: "Bearer"
        token: <log-shipper-token>
      tls:
        verifyHostname: false
        verifyCertificate: false
  ```

### Модуль monitoring-custom

Модуль расширяет возможности модуля [prometheus](./modules/prometheus/) по мониторингу приложений пользователей.

Чтобы организовать сбор метрик с приложений модулем `monitoring-custom`, необходимо:

- Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или под. Значение лейбла определит имя в списке target'ов Prometheus.
  - В качестве значения label'а prometheus.deckhouse.io/custom-target стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере.

     При этом, если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или даже ставится несколько раз в один namespace, достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
- Порту, с которого нужно собирать метрики, указать имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

  Если это невозможно (например, порт уже определен и назван другим именем), необходимо воспользоваться аннотациями: `prometheus.deckhouse.io/port: номер_порта` — для указания порта и `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

  > **Важно!** При указании аннотации на Service в качестве значения порта необходимо использовать `targetPort`. То есть тот порт, что открыт и слушается приложением, а не порт Service'а.

  - Пример 1:

    ```yaml
    ports:
    - name: https-metrics
      containerPort: 443
    ```

  - Пример 2:

    ```yaml
    annotations:
      prometheus.deckhouse.io/port: "443"
      prometheus.deckhouse.io/tls: "true"  # Если метрики отдаются по HTTP, эту аннотацию указывать не нужно.
    ```

- При использовании service mesh [Istio](./istio/) в режиме STRICT mTLS указать для сбора метрик следующую аннотацию у Service или Pod: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Не обязательно)* Указать дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: ''):
    - возможно указать несколько таких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Эту аннотацию надо вешать на тот же ресурс, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

##### Пример: Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                      # По умолчанию будет использоваться порт сервиса с именем http-metrics или https-metrics.
    prometheus.deckhouse.io/path: "/my_app/metrics"           # По умолчанию /metrics.
    prometheus.deckhouse.io/query-param-format: "prometheus"  # По умолчанию ''.
    prometheus.deckhouse.io/allow-unready-pod: "true"         # По умолчанию поды НЕ в Ready игнорируются.
    prometheus.deckhouse.io/sample-limit: "5000"              # По умолчанию принимается не больше 5000 метрик от одного пода.
spec:
  ports:
  - name: my-app
    port: 8060
  - name: http-metrics
    port: 8061
    targetPort: 8061
  selector:
    app: my-app
```

##### Пример: Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        prometheus.deckhouse.io/custom-target: my-app
      annotations:
        prometheus.deckhouse.io/sample-limit: "5000"  # По умолчанию принимается не больше 5000 метрик от одного пода.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

### Модуль monitoring-custom: настройки

Модуль работает, если включен модуль `prometheus`, и не имеет параметров для настройки.

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-custom'].config-values | format_module_configuration: moduleKebabName }}

### Модуль monitoring-kubernetes

Модуль предназначен для базового мониторинга узлов кластера.

Обеспечивает безопасный сбор метрик и предоставляет базовый набор правил для мониторинга:
- текущей версии container runtime (docker, containerd) на узле и ее соответствия версиям, разрешенным для использования;
- общей работоспособности подсистемы мониторинга кластера (Dead man's switch);
- доступных файловых дескрипторов, сокетов, свободного места и inode;
- работы `kube-state-metrics`, `node-exporter`, `kube-dns`;
- состояния узлов кластера (NotReady, drain, cordon);
- состояния синхронизации времени на узлах;
- случаев продолжительного превышения CPU steal;
- состояния таблицы Conntrack на узлах;
- подов с некорректным состоянием (как возможное следствие проблем с kubelet) и др.

### Модуль monitoring-kubernetes: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-kubernetes'].config-values | format_module_configuration: moduleKebabName }}

### Мониторинг control plane

Мониторинг control plane осуществляется с помощью модуля `monitoring-kubernetes-control-plane`, который организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:
* kube-apiserver;
* kube-controller-manager;
* kube-scheduler;
* kube-etcd.

### Мониторинг control plane: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-kubernetes-control-plane'].config-values | format_module_configuration: moduleKebabName }}

### Модуль monitoring-ping

#### Описание

Данный модуль предназначен для мониторинга сетевого взаимодействия между всеми узлами кластера, а также — опционально — до дополнительных внешних узлов.

Каждый узел два раза в секунду отправляет ICMP-пакеты на все другие узлы кластера (и на опциональные внешние узлы) и экспортирует данные в `Prometheus`.
В комплекте идет dashboard для `Grafana`, на котором отражаются соответствующие графики.

#### Как работает

Модуль следит за любыми изменениями поля `.status.addresses` узла. В случае выявления таковых
запускается хук, который собирает полный список имен узлов и их адресов и передает в daemonSet, что в свою очередь пересоздает поды.
Таким образом, `ping` проверяет всегда актуальный список узлов.

### Модуль monitoring-ping: настройки

У модуля нет обязательных настроек.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-ping'].config-values | format_module_configuration: moduleKebabName }}

### Модуль operator-prometheus

Модуль устанавливает prometheus operator, который позволяет создавать и автоматизированно управлять инсталляциями Prometheus.

<!-- Исходник картинок: https://docs.google.com/drawings/d/1KMgawZD4q7jEYP-_g6FvUeJUaT3edro_u6_RsI3ZVvQ/edit -->

Функционал устанавливаемого оператора:
- определяет следующие custom resource'ы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
- следит за этими ресурсами и:
  - генерирует `StatefulSet` с самим *Prometheus* и необходимые для его работы конфигурационные файлы, сохраняя их в `Secret`;
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в `Secret`.

#### Prometheus

##### Что делает Prometheus?

В целом, сервер Prometheus делает две ключевых вещи — **собирает метрики** и **выполняет правила**:
* Для каждого *target'а* (цель для мониторинга), каждый `scrape_interval`, делает HTTP запрос на этот *target*, получает в ответ метрики в своем формате, которые сохраняет к себе в базу
* Каждый `evaluation_interval` обрабатывает *rules*, на основании чего:
  * или шлет алерты
  * или записывает (себе же в базу) новые метрики (результат выполнения *rule'а*)

##### Как настраивается Prometheus?

* У сервера Prometheus есть *config* и есть *rule files* (файлы с правилами)
* В `config` имеются следующие секции:
  * `scrape_configs` — настройки поиска *target'ов* (целей для мониторинга, см. подробней следующий раздел).
  * `rule_files` — список директорий, в которых лежат *rule'ы*, которые необходимо загружать:

    ```yaml
    rule_files:
    - /etc/prometheus/rules/rules-0/*
    - /etc/prometheus/rules/rules-1/*
    ```

  * `alerting` — настройки поиска *Alert Manager'ов*, в которые слать алерты. Секция очень похожа на `scrape_configs`, только результатом ее работы является список *endpoint'ов*, в которые Prometheus будет слать алерты.

##### Где Prometheus берет список *target'ов*?

* В целом Prometheus работает следующим образом:

  ![Работа Prometheus](./images/operator-prometheus/targets.png)

  * **(1)** Prometheus читает секцию конфига `scrape_configs`, согласно которой настраивает свой внутренний механизм Service Discovery
  * **(2)** Механизм Service Discovery взаимодействует с API Kubernetes (в основном — получает endpoint`ы)
  * **(3)** На основании происходящего в Kubernetes механизм Service Discovery обновляет Targets (список *target'ов*)
* В `scrape_configs` указан список *scrape job'ов* (внутреннее понятие Prometheus), каждый из которых определяется следующим образом:

  ```yaml
  scrape_configs:
    # Общие настройки
  - job_name: d8-monitoring/custom/0    # просто название scrape job'а, показывается в разделе Service Discovery
    scrape_interval: 30s                  # как часто собирать данные
    scrape_timeout: 10s                   # таймаут на запрос
    metrics_path: /metrics                # path, который запрашивать
    scheme: http                          # http или https
    # Настройки service discovery
    kubernetes_sd_configs:                # означает, что target'ы мы получаем из Kubernetes
    - api_server: null                    # означает, что адрес API-сервера использовать из переменных окружения (которые есть в каждом Pod'е)
      role: endpoints                     # target'ы брать из endpoint'ов
      namespaces:
        names:                            # искать endpoint'ы только в этих namespace'ах
        - foo
        - baz
    # Настройки "фильтрации" (какие enpoint'ы брать, а какие нет) и "релейблинга" (какие лейблы добавить или удалить, на все получаемые метрики)
    relabel_configs:
    # Фильтр по значению label'а prometheus_custom_target (полученного из связанного с endpoint'ом service'а)
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: .+                           # подходит любой НЕ пустой лейбл
      action: keep
    # Фильтр по имени порта
    - source_labels: [__meta_kubernetes_endpointslice_port_name]
      regex: http-metrics                 # подходит, только если порт называется http-metrics
      action: keep
    # Добавляем label job, используем значение label'а prometheus_custom_target у service'а, к которому добавляем префикс "custom-"
    #
    # Лейбл job это служебный лейбл Prometheus:
    #    * он определяет название группы, в которой будет показываться target на странице targets
    #    * и конечно же он будет у каждой метрики, полученной у этих target'ов, чтобы можно было удобно фильтровать в rule'ах и dashboard'ах
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: (.*)
      target_label: job
      replacement: custom-$1
      action: replace
    # Добавляем label namespace
    - source_labels: [__meta_kubernetes_namespace]
      regex: (.*)
      target_label: namespace
      replacement: $1
      action: replace
    # Добавляем label service
    - source_labels: [__meta_kubernetes_service_name]
      regex: (.*)
      target_label: service
      replacement: $1
      action: replace
    # Добавляем label instance (в котором будет имя Pod'а)
    - source_labels: [__meta_kubernetes_pod_name]
      regex: (.*)
      target_label: instance
      replacement: $1
      action: replace
  ```

* Таким образом, Prometheus сам отслеживает:
  * добавление и удаление Pod'ов (при добавлении/удалении Pod'ов Kubernetes изменяет endpoint'ы, а Prometheus это видит и добавляет/удаляет *target'ы*)
  * добавление и удаление сервисов (точнее endpoint'ов) в указанных namespace'ах
* Изменение конфига требуется в следующих случаях:
  * нужно добавить новый scrape config (обычно — новый вид сервисов, которые надо мониторить)
  * нужно изменить список namespace'ов

#### Prometheus Operator

##### Что делает Prometheus Operator?

* С помощью механизма CRD (Custom Resource Definitions) определяет четыре custom ресурса:
  * prometheus — определяет инсталляцию (кластер) Prometheus
  * servicemonitor — определяет, как "мониторить" (собирать метрики) набор сервисов
  * alertmanager — определяет кластер Alertmanager'ов
  * prometheusrule — определяет список Prometheus rules
* Следит за ресурсами `prometheus` и генерирует для каждого:
  * StatefulSet (с самим Prometheus'ом)
  * Secret с `prometheus.yaml` (конфиг Prometheus'а) и `configmaps.json` (конфиг для `prometheus-config-reloader`)
* Следит за ресурсами `servicemonitor` и `prometheusrule` и на их основании обновляет конфиги (`prometheus.yaml` и `configmaps.json`, которые лежат в секрете).

##### Что в Pod'е с Prometheus'ом?

![Что в Pod Prometheus](./images/operator-prometheus/pod.png)

* Два контейнера:
  * `prometheus` — сам Prometheus
  * `prometheus-config-reloader` — обвязка, которая:
    * следит за изменениями `prometheus.yaml` и, при необходимости, вызывает reload конфигурации Prometheus'у (специальным HTTP-запросом, см. [подробнее ниже](#как-обрабатываются-service-monitorы))
    * следит за PrometheusRule'ами (см. [подробнее ниже](#как-обрабатываются-custome-resources-с-ruleами)) и по необходимости скачивает их и перезапускает Prometheus
* Pod использует три volume:
  * config — примонтированный secret (два файла: `prometheus.yaml` и `configmaps.json`). Подключен в оба контейнера.
  * rules — `emptyDir`, который наполняет `prometheus-config-reloader`, а читает `prometheus`. Подключен в оба контейнера, но в `prometheus` в режиме read only.
  * data — данные Prometheus. Подмонтирован только в `prometheus`.

##### Как обрабатываются Service Monitor'ы?

![Как обрабатываются Service Monitor'ы](./images/operator-prometheus/servicemonitors.png)

* **(1)** Prometheus Operator читает (а также следит за добавлением/удалением/изменением) Service Monitor'ы (какие именно Service Monitor'ы — указано в самом ресурсе `prometheus`, см. подробней официальную документацию).
* **(2)** Для каждого Service Monitor'а, если в нем НЕ указан конкретный список namespace'ов (указано `any: true`), Prometheus Operator вычисляет (обращаясь к API Kubernetes) список namespace'ов, в которых есть Service'ы (подходящие под указанные в Service Monitor'е label'ы).
* **(3)** На основании прочитанных ресурсов `servicemonitor` (см. официальную документацию) и на основании вычисленных namespace'ов Prometheus Operator генерирует часть конфига (секцию `scrape_configs`) и сохраняет конфиг в соответствующий Secret.
* **(4)** Штатными средствами самого Kubernetes данные из секрета прилетают в Pod (файл `prometheus.yaml` обновляется).
* **(5)** Изменение файла замечает `prometheus-config-reloader`, который по HTTP отправляет запрос Prometheus'у на перезагрузку.
* **(6)** Prometheus перечитывает конфиг и видит изменения в scrape_configs, которые обрабатывает уже согласно своей логике работы (см. подробнее выше).

##### Как обрабатываются Custome Resources с *rule'ами*?

![Как обрабатываются Custome Resources с rule'ами](./images/operator-prometheus/rules.png)

* **(1)** Prometheus Operator следит за PrometheusRule'ами (подходящими под указанный в ресурсе `prometheus` `ruleSelector`).
* **(2)** Если появился новый (или был удален существующий) PrometheusRule — Prometheus Operator обновляет `prometheus.yaml` (а дальше срабатывает логика в точности соответствующая обработке Service Monitor'ов, которая описана выше).
* **(3)** Как в случае добавления/удаления PrometheusRule'а, так и при изменении содержимого PrometheusRule'а, Prometheus Operator обновляет ConfigMap `prometheus-main-rulefiles-0`.
* **(4)** Штатными средствами самого Kubernetes данные из ConfigMap прилетают в Pod
* Изменение файла замечает `prometheus-config-reloader`, который:
  * **(5)** скачивает изменившиеся ConfigMap'ы в директорию rules (это `emptyDir`)
  * **(6)** по HTTP отправляет запрос Prometheus'у на перезагрузку
* **(7)** Prometheus перечитывает конфиг и видит изменившиеся *rule'ы*.

### Модуль operator-prometheus: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['operator-prometheus'].config-values | format_module_configuration: moduleKebabName }}

### Prometheus-operator: примеры конфигурации

#### Установка еще одного prometheus-operator в кластер

Пользователю может понадобится установить в кластер еще один prometheus-operator,
чтобы добавить Prometheus'ы или alertmanager'ы в кластер.

1. Чтобы не пересекаться с prometheus-operator из Deckhouse, необходимо указать флаг
   `--deny-namespaces=d8-monitoring` для пользовательской инсталляции prometheus-operator.

2. Prometheus-operator из Deckhouse следит за ресурсами правил и мониторов только в пространствах имен
   с меткой `heritage: deckhouse`. Не устанавливайте эту метку на пользовательские пространства имен.

### Prometheus-мониторинг

Устанавливает и полностью настраивает Prometheus, настраивает сбор метрик со многих распространенных приложений, а также предоставляет необходимый минимальный набор alert'ов для Prometheus и dashboard Grafana.

Если используется StorageClass с поддержкой автоматического расширения (`allowVolumeExpansion: true`), при нехватке места на диске для данных Prometheus его емкость будет увеличена.

Ресурсы CPU и memory автоматически выставляются при пересоздании пода на основе истории потребления, благодаря модулю [Vertical Pod Autoscaler](./modules/vertical-pod-autoscaler/). Также, благодаря кэшированию запросов к Prometheus с помощью Trickster, потребление памяти Prometheus сильно сокращается.

Поддерживается как pull-, так и push-модель получения метрик.

#### Мониторинг аппаратных ресурсов

Реализовано отслеживание нагрузки на аппаратные ресурсы кластера с графиками по утилизации:
- процессора;
- памяти;
- диска;
- сети.

Графики доступны с агрегацией в разрезе:
- по подам;
- контроллерам;
- пространствам имен;
- узлам.

#### Мониторинг Kubernetes

Deckhouse настраивает мониторинг широкого набора параметров «здоровья» Kubernetes и его компонентов, в частности:
- общей утилизации кластера;
- связанности узлов Kubernetes между собой (измеряется rtt между всеми узлами);
- доступности и работоспособности компонентов control plane:
  - `etcd`;
  - `coredns` и `kube-dns`;
  - `kube-apiserver` и др.
- синхронизации времени на узлах и др.

#### Мониторинг Ingress

Подробно описан [здесь](./modules/ingress-nginx/#мониторинг-и-статистика)

#### Режим расширенного мониторинга

В Deckhouse возможно использование [режима расширенного мониторинга](./extended-monitoring/), который предоставляет возможности алертов по дополнительным метрикам: свободному месту и inode на дисках узлов, утилизации узлов, доступности подов и образов контейнеров, истечении действия сертификатов, другим событиям кластера.

##### Алертинг в режиме расширенного мониторинга

Deckhouse позволяет гибко настроить алертинг на каждый из namespace'ов и указывать разную критичность в зависимости от порогового значения. Есть возможность указать множество пороговых значений отправки алертов в различные namespace'ы, например, для таких параметров, как:
- значения свободного места и inodes на диске;
- утилизация CPU узлов и контейнера;
- процент 5xx ошибок на `nginx-ingress`;
- количество возможных недоступных подов в `Deployment`, `StatefulSet`, `DaemonSet`.

#### Алерты

Мониторинг в составе Deckhouse включает также и возможности уведомления о событиях. В стандартной поставке уже идет большой набор только необходимых алертов, покрывающих состояние кластера и его компонентов. При этом всегда остается возможность добавления кастомных алертов.

##### Отправка алертов во внешние системы

Deckhouse поддерживает отправку алертов с помощью `Alertmanager`:
- по протоколу SMTP;
- в Telegram;
- посредством Webhook.

#### Включенные модули

![Схема взаимодействия](./images/prometheus/prometheus_monitoring_new.svg)

##### Компоненты, устанавливаемые Deckhouse

| Компонент                   | Описание                                                                                                                                                                                                                                                                                        |
|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **prometheus-main**         | Основной Prometheus, который выполняет scrape каждые 30 секунд (с помощью параметра `scrapeInterval` можно изменить это значение). Именно он обрабатывает все правила, отправляет алерты и является основным источником данных.                                                                 |
| **prometheus-longterm**     | Дополнительный Prometheus, который выполняет scrape данных из основного Prometheus (`prometheus-main`) каждые 5 минут (с помощью параметра `longtermScrapeInterval` можно изменить это значение). Используется для продолжительного хранения истории и отображения больших промежутков времени. |
| **trickster**               | Кэширующий прокси, снижающий нагрузку на Prometheus.                                                                                                                                                                                                                                            |
| **aggregating-proxy**       | Агрегирующий и кеширующий прокси, снижающий нагрузку на Prometheus и объединяющий main и longterm в один источник.                                                                                                                                                                             |
| **memcached**               | Сервис кэширования данных в оперативной памяти.                                                                                                                                                                                                                                                 |
| **grafana**                 | Управляемая платформа визуализации данных. Включает подготовленные dashboard'ы для всех модулей Deckhouse и некоторых популярных приложений. Grafana умеет работать в режиме высокой доступности, не хранит состояние и настраивается с помощью CRD.                                            |
| **metrics-adapter**         | Компонент, соединяющий Prometheus и Kubernetes metrics API. Включает поддержку HPA в кластере Kubernetes.                                                                                                                                                                                       |
| **vertical-pod-autoscaler** | Компонент, позволяющий автоматически изменять размер запрошенных ресурсов для подов с целью оптимальной утилизации CPU и памяти.                                                                                                                                                                |
| **Различные exporter'ы**    | Подготовленные и подключенные к Prometheus exporter'ы. Список включает множество exporter'ов для всех необходимых метрик: `kube-state-metrics`, `node-exporter`, `oomkill-exporter`, `image-availability-exporter` и многие другие.                                                             |

##### Внешние компоненты

Deckhouse может интегрироваться с большим количеством разнообразных решений следующими способами:

| Название                       | Описание|
|--------------------------------|--------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanager'ы могут быть подключены к Prometheus и Grafana и находиться как в кластере Deckhouse, так и за его пределами.|
| **Long-term metrics storages** | Используя протокол `remote write`, возможно отсылать метрики из Deckhouse в большое количество хранилищ, включающее Cortex, Thanos, VictoriaMetrics.|

### Prometheus-мониторинг: настройки

Модуль не требует обязательной конфигурации (все работает из коробки).

 
<!-- SCHEMA -->

#### Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем и пользователем `admin`.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values prometheus -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-monitoring delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

#### Примечание

* `retentionSize` для `main` и `longterm` **рассчитывается автоматически, возможности задать значение нет!**
  * Алгоритм расчета:
    * `pvc_size * 0.85` — если PVC существует;
    * `10 GiB` — если PVC нет и StorageClass поддерживает ресайз;
    * `25 GiB` — если PVC нет и StorageClass не поддерживает ресайз.
  * Если используется `local-storage` и требуется изменить `retentionSize`, необходимо вручную изменить размер PV и PVC в нужную сторону. **Внимание!** Для расчета берется значение из `.status.capacity.storage` PVC, поскольку оно отражает реальный размер PV в случае ручного ресайза.
* `40 GiB` — размер PersistentVolumeClaim создаваемого по умолчанию.
* Размер дисков Prometheus можно изменить стандартным для Kubernetes способом (если в StorageClass это разрешено), отредактировав в PersistentVolumeClaim поле `.spec.resources.requests.storage`.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['prometheus'].config-values | format_module_configuration: moduleKebabName }}

### Prometheus-мониторинг: custom resources
{{ site.data.schemas.prometheus.crds.clusteralerts | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.customalertmanager | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.customprometheusrules | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanaadditionaldatasources | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanaalertschannel | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanadashboarddefinition | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.prometheusremotewrite | format_crd: "prometheus" }}

### Prometheus-мониторинг: FAQ

{% raw %}

#### Как собирать метрики с приложений, расположенных вне кластера?

1. Сконфигурировать Service по аналогии с сервисом для [сбора метрик с вашего приложения](./monitoring-custom/#пример-service), но без указания параметра `spec.selector`.
1. Создать Endpoints для этого Service, явно указав в них `IP:PORT`, по которым ваши приложения отдают метрики.
> Важный момент: имена портов в Endpoints должны совпадать с именами этих портов в Service.

##### Пример

Метрики приложения доступны без TLS, по адресу `http://10.182.10.5:9114/metrics`.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
spec:
  ports:
  - name: http-metrics
    port: 9114
---
apiVersion: v1
kind: Endpoints
metadata:
  name: my-app
  namespace: my-namespace
subsets:
  - addresses:
    - ip: 10.182.10.5
    ports:
    - name: http-metrics
      port: 9114
```

#### Как добавить дополнительные dashboard'ы в вашем проекте?

Добавление пользовательских dashboard'ов для Grafana в Deckhouse реализовано с помощью подхода Infrastructure as a Code.
Чтобы ваш dashboard появился в Grafana, необходимо создать в кластере специальный ресурс — [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш dashboard.
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```

**Важно!** Системные и добавленные через [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) dashboard'ы нельзя изменить через интерфейс Grafana.

#### Как добавить алерты и/или recording-правила для вашего проекта?

Для добавления алертов существует специальный ресурс — `CustomPrometheusRules`.

Параметры:
- `groups` — единственный параметр, в котором необходимо описать группы алертов. Структура групп полностью совпадает с аналогичной в prometheus-operator.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: my-rules
spec:
  groups:
  - name: cluster-state-alert.rules
    rules:
    - alert: CephClusterErrorState
      annotations:
        description: Storage cluster is in error state for more than 10m.
        summary: Storage cluster is in error state
        plk_markup_format: markdown
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```

##### Как подключить дополнительные data source для Grafana?

Для подключения дополнительных data source к Grafana существует специальный ресурс — `GrafanaAdditionalDatasource`.

Параметры ресурса подробно описаны в документации к Grafana. Тип ресурса смотрите в документации по конкретному datasource.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: another-prometheus
spec:
  type: prometheus
  access: Proxy
  url: https://another-prometheus.example.com/prometheus
  basicAuth: true
  basicAuthUser: foo
  jsonData:
    timeInterval: 30s
    httpMethod: POST
  secureJsonData:
    basicAuthPassword: bar
```

#### Как обеспечить безопасный доступ к метрикам?

Для обеспечения безопасности настоятельно рекомендуем использовать `kube-rbac-proxy`.

##### Пример безопасного сбора метрик с приложения, расположенного в кластере

Для настройки защиты метрик приложения с использованием `kube-rbac-proxy` и последующей сборки метрик с него средствами Prometheus выполните следующие шаги:

1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: rbac-proxy-test
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: d8:rbac-proxy
   subjects:
   - kind: ServiceAccount
     name: rbac-proxy-test
     namespace: default
   ```

   > Обратите внимание, что используется встроенная в Deckhouse ClusterRole `d8:rbac-proxy`.

2. Создайте конфигурацию для `kube-rbac-proxy`:

   ```yaml
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: rbac-proxy-config-test
     namespace: rbac-proxy-test
   data:
     config-file.yaml: |+
       authorization:
         resourceAttributes:
           namespace: default
           apiVersion: v1
           resource: services
           subresource: proxy
           name: rbac-proxy-test
   ```

   > Более подробную информацию по атрибутам можно найти в документации Kubernetes.

3. Создайте `Service` и `Deployment` для вашего приложения, где `kube-rbac-proxy` займет позицию sidecar-контейнера:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: rbac-proxy-test
     labels:
       prometheus.deckhouse.io/custom-target: rbac-proxy-test
   spec:
     ports:
     - name: https-metrics
       port: 8443
       targetPort: https-metrics
     selector:
       app: rbac-proxy-test
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: rbac-proxy-test
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: rbac-proxy-test
     template:
       metadata:
         labels:
           app: rbac-proxy-test
       spec:
         securityContext:
           runAsUser: 65532
         serviceAccountName: rbac-proxy-test
         containers:
         - name: kube-rbac-proxy
           image: quay.io/brancz/kube-rbac-proxy:v0.14.0
           args:
           - "--secure-listen-address=0.0.0.0:8443"
           - "--upstream=http://127.0.0.1:8081/"
           - "--config-file=/kube-rbac-proxy/config-file.yaml"
           - "--logtostderr=true"
           - "--v=10"
           ports:
           - containerPort: 8443
             name: https-metrics
           volumeMounts:
           - name: config
             mountPath: /kube-rbac-proxy
         - name: prometheus-example-app
           image: quay.io/brancz/prometheus-example-app:v0.1.0
           args:
           - "--bind=127.0.0.1:8081"
         volumes:
         - name: config
           configMap:
             name: rbac-proxy-config-test
   ```

4. Назначьте необходимые права на ресурс для Prometheus:

   ```yaml
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: rbac-proxy-test-client
   rules:
   - apiGroups: [""]
     resources: ["services/proxy"]
     verbs: ["get"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test-client
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: rbac-proxy-test-client
   subjects:
   - kind: ServiceAccount
     name: prometheus
     namespace: d8-monitoring
   ```

После шага 4 метрики вашего приложения должны появиться в Prometheus.

##### Пример безопасного сбора метрик с приложения, расположенного вне кластера

Предположим, что есть доступный через интернет сервер, на котором работает `node-exporter`. По умолчанию `node-exporter` слушает на порту `9100` и доступен на всех интерфейсах. Необходимо обеспечить контроль доступа к `node-exporter` для безопасного сбора метрик. Ниже приведен пример такой настройки.

Требования:
- Из кластера должен быть доступ до сервиса `kube-rbac-proxy`, запущенного на *удаленном сервере*.
- От *удаленного сервера* должен быть доступ до API-сервера кластера.

Выполните следующие шаги:
1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: prometheus-external-endpoint
   rules:
   - apiGroups: ["authentication.k8s.io"]
     resources:
     - tokenreviews
     verbs: ["create"]
   - apiGroups: ["authorization.k8s.io"]
     resources:
     - subjectaccessreviews
     verbs: ["create"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: prometheus-external-endpoint-server-01
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: prometheus-external-endpoint
   subjects:
   - kind: ServiceAccount
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ```

2. Сгенерируйте `kubeconfig` для созданного `ServiceAccount` ([пример генерации kubeconfig для `ServiceAccount`](./user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа)).

3. Положите получившийся `kubeconfig` на *удаленный сервер*. В дальнейшем понадобится указать путь к этому `kubeconfig` в настройках `kube-rbac-proxy` (в примере используется путь `${PWD}/.kube/config`).

4. Настройте `node-exporter` на *удаленном сервере*, чтобы он был доступен только на локальном интерфейсе (слушал `127.0.0.1:9100`).
5. Запустите `kube-rbac-proxy` на *удаленном сервере*:

   ```shell
   docker run --network host -d -v ${PWD}/.kube/config:/config quay.io/brancz/kube-rbac-proxy:v0.14.0 --secure-listen-address=0.0.0.0:8443 \
     --upstream=http://127.0.0.1:9100 --kubeconfig=/config --logtostderr=true --v=10
   ```

6. Проверьте, что порт `8443` доступен по внешнему адресу *удаленного сервера*.

7. Создайте в кластере `Service` и `Endpoint`, указав в качестве `<server_ip_address>` внешний адрес *удаленного сервера*:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: prometheus-external-endpoint-server-01
     labels:
       prometheus.deckhouse.io/custom-target: prometheus-external-endpoint-server-01
   spec:
     ports:
     - name: https-metrics
       port: 8443
   ---
   apiVersion: v1
   kind: Endpoints
   metadata:
     name: prometheus-external-endpoint-server-01
   subsets:
     - addresses:
       - ip: <server_ip_address>
       ports:
       - name: https-metrics
         port: 8443
   ```

#### Как добавить Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `Internal`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: webhook
spec:
  type: Internal
  internal:
    route:
      groupBy: ['job']
      groupWait: 30s
      groupInterval: 5m
      repeatInterval: 12h
      receiver: 'webhook'
    receivers:
    - name: 'webhook'
      webhookConfigs:
      - url: 'http://webhookserver:8080/'
```

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

#### Как добавить внешний дополнительный Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `External`, который может указывать на Alertmanager по FQDN или через сервис в Kubernetes-кластере.

Пример FQDN Alertmanager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-fqdn-alertmanager
spec:
  external:
    address: https://alertmanager.mycompany.com/myprefix
  type: External
```

Пример Alertmanager с Kubernetes service:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-service-alertmanager
spec:
  external:
    service:
      namespace: myns
      name: my-alertmanager
      path: /myprefix/
  type: External
```

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

#### Как в Alertmanager игнорировать лишние алерты?

Решение сводится к настройке маршрутизации алертов в вашем Alertmanager.

Потребуется:

1. Завести получателя без параметров.
1. Смаршрутизировать лишние алерты в этого получателя.

Ниже приведены примеры настройки `CustomAlertmanager`.

Чтобы получать только алерты с лейблами `service: foo|bar|baz`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель  
  - name: some-other-receiver
    # ...
route:
  # receiver по умолчанию.
  receiver: blackhole
  routes:
    # Дочерний маршрут
    - matchers:
        - matchType: =~
          name: service
          value: ^(foo|bar|baz)$
      receiver: some-other-receiver
```

Чтобы получать все алерты, кроме `DeadMansSwitch`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель.
  - name: some-other-receiver
  # ...
route:
  # receiver по умолчанию.
  receiver: some-other-receiver
  routes:
    # Дочерний маршрут.
    - matchers:
        - matchType: =
          name: alertname
          value: DeadMansSwitch
      receiver: blackhole
```

С подробным описанием всех параметров можно ознакомиться в официальной документации.

#### Почему нельзя установить разный scrapeInterval для отдельных таргетов?

Наиболее полный ответ на этот вопрос дает разработчик Prometheus Brian Brazil.
Если коротко, разные scrapeInterval'ы принесут следующие проблемы:
* увеличение сложности конфигурации;
* проблемы при написании запросов и создании графиков;
* короткие интервалы больше похожи на профилирование приложения, и, скорее всего, Prometheus — не самый подходящий инструмент для этого.

Наиболее разумное значение для scrapeInterval находится в диапазоне 10–60 секунд.

#### Как ограничить потребление ресурсов Prometheus?

Чтобы избежать ситуаций, когда VPA запрашивает для Prometheus или Longterm Prometheus ресурсов больше, чем есть на выделенном для этого узле, можно явно ограничить VPA с помощью [параметров модуля](configuration.html):
- `vpa.longtermMaxCPU`;
- `vpa.longtermMaxMemory`;
- `vpa.maxCPU`;
- `vpa.maxMemory`.

#### Как настроить ServiceMonitor или PodMonitor для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Pod/Service Monitor.
Добавьте в namespace, в котором находится Pod/Service Monitor, лейбл `prometheus.deckhouse.io/monitor-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/monitor-watcher-enabled: "true"
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: example-app
  namespace: frontend
  labels:
    prometheus: main
spec:
  selector:
    matchLabels:
      app: example-app
  endpoints:
    - port: web
```

#### Как настроить Probe для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Probe.
Добавьте в namespace, в котором находится Probe, лейбл `prometheus.deckhouse.io/probe-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/probe-watcher-enabled: "true"
---
apiVersion: monitoring.coreos.com/v1
kind: Probe
metadata:
  labels:
    app: prometheus
    component: probes
    prometheus: main
  name: cdn-is-up
  namespace: frontend
spec:
  interval: 30s
  jobName: httpGet
  module: http_2xx
  prober:
    path: /probe
    scheme: http
    url: blackbox-exporter.blackbox-exporter.svc.cluster.local:9115
  targets:
    staticConfig:
      static:
      - https://example.com/status
```

#### Как настроить PrometheusRules для работы с Prometheus?

Добавьте в namespace, в котором находятся PrometheusRules, лейбл `prometheus.deckhouse.io/rules-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
```

#### Как увеличить размер диска

1. Для увеличения размера отредактируйте PersistentVolumeClaim, указав новый размер в поле `spec.resources.requests.storage`.
   * Увеличение размера возможно, если в StorageClass поле `allowVolumeExpansion` установлено в `true`.
2. Если используемое хранилище не поддерживает изменение диска на лету, в статусе PersistentVolumeClaim появится сообщение `Waiting for user to (re-)start a pod to finish file system resize of volume on node.`.
3. Перезапустите под для завершения изменения размера файловой системы.

#### Как получить информацию об алертах в кластере?

Информацию об активных алертах можно получить не только в веб-интерфейсе Grafana/Prometheus, но и в CLI. Это может быть полезным, если у вас есть только доступ к API-серверу кластера и нет возможности открыть веб-интерфейс Grafana/Prometheus.

Выполните следующую команду для получения списка алертов в кластере:

```shell
kubectl get clusteralerts
```

Пример:

```shell
### kubectl get clusteralerts
NAME               ALERT                                      SEVERITY   AGE     LAST RECEIVED   STATUS
086551aeee5b5b24   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
226d35c886464d6e   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
235d4efba7df6af4   D8SnapshotControllerPodIsNotReady          8          5d4h    44s             firing
27464763f0aa857c   D8PrometheusOperatorPodIsNotReady          7          5d4h    43s             firing
ab17837fffa5e440   DeadMansSwitch                             4          5d4h    41s             firing
```

Выполните следующую команду для просмотра конкретного алерта:

```shell
kubectl get clusteralerts <ALERT_NAME> -o yaml
```

Пример:

```shell
### kubectl get clusteralerts 235d4efba7df6af4 -o yaml
alert:
  description: |
    The recommended course of action:
    1. Retrieve details of the Deployment: `kubectl -n d8-snapshot-controller describe deploy snapshot-controller`
    2. View the status of the Pod and try to figure out why it is not running: `kubectl -n d8-snapshot-controller describe pod -l app=snapshot-controller`
  labels:
    pod: snapshot-controller-75bd776d76-xhb2c
    prometheus: deckhouse
    tier: cluster
  name: D8SnapshotControllerPodIsNotReady
  severityLevel: "8"
  summary: The snapshot-controller Pod is NOT Ready.
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAlert
metadata:
  creationTimestamp: "2023-05-15T14:24:08Z"
  generation: 1
  labels:
    app: prometheus
    heritage: deckhouse
  name: 235d4efba7df6af4
  resourceVersion: "36262598"
  uid: 817f83e4-d01a-4572-8659-0c0a7b6ca9e7
status:
  alertStatus: firing
  lastUpdateTime: "2023-05-15T18:10:09Z"
  startsAt: "2023-05-10T13:43:09Z"
```

Помните о специальном алерте `DeadMansSwitch` — его присутствие в кластере говорит о работоспособности Prometheus.

#### Как добавить дополнительные эндпоинты в scrape config?

Добавьте в namespace, в котором находится ScrapeConfig, лейбл `prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"
```

Добавьте ScrapeConfig, который имеет обязательный лейбл `prometheus: main`:

```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: ScrapeConfig
metadata:
  name: example-scrape-config
  namespace: frontend
  labels:
    prometheus: main
spec:
  honorLabels: true
  staticConfigs:
    - targets: ['example-app.frontend.svc.{{ .Values.global.discovery.clusterDomain }}.:8080']
  relabelings:
    - regex: endpoint|namespace|pod|service
      action: labeldrop
    - targetLabel: scrape_endpoint
      replacement: main
    - targetLabel: job
      replacement: kube-state-metrics
  metricsPath: '/metrics'
```

{% endraw %}
## Подсистема Масштабирование и управление ресурсами

### Модуль extended-monitoring

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — включает расширенный сбор метрик и отправку алертов по свободному месту и inode на узлах, плюс включает «расширенный мониторинг» объектов в namespace, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=""`;
- `image-availability-exporter` — добавляет метрики и включает отправку алертов, позволяющих узнать о проблемах с доступностью образа контейнера в registry, прописанному в поле `image` из spec пода в `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`;
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик;
- `cert-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них.

### Модуль extended-monitoring: настройки

 
<!-- SCHEMA -->

#### Как использовать `extended-monitoring-exporter`

Чтобы включить экспортирование extended-monitoring метрик, нужно навесить на namespace лейбл `extended-monitoring.deckhouse.io/enabled` любым удобным способом, например:
- добавить в проект соответствующий helm-чарт (рекомендуемый);
- добавить в описание `.gitlab-ci.yml` (kubectl patch/create);
- поставить руками (`kubectl label namespace my-app-production extended-monitoring.deckhouse.io/enabled=""`);
- настроить через [namespace-configurator](./namespace-configurator/) модуль.

Сразу же после этого для всех поддерживаемых Kubernetes-объектов в данном namespace в Prometheus появятся default-метрики + любые кастомные с префиксом `threshold.extended-monitoring.deckhouse.io/`. Для ряда [non-namespaced](#non-namespaced-kubernetes-объекты) Kubernetes-объектов, описанных ниже, мониторинг включается автоматически.

К Kubernetes-объектам `threshold.extended-monitoring.deckhouse.io/что-то свое` можно добавить любые другие лейблы с указанным значением. Пример: `kubectl label pod test threshold.extended-monitoring.deckhouse.io/disk-inodes-warning=30`.
В таком случае значение из лейбла заменит значение по умолчанию.

Слежение за объектом можно отключить индивидуально, поставив на него лейбл `extended-monitoring.deckhouse.io/enabled=false`. Соответственно, отключатся и лейблы по умолчанию, а также все алерты, привязанные к лейблам.

##### Стандартные лейблы и поддерживаемые Kubernetes-объекты

Далее приведен список используемых в Prometheus Rules лейблов, а также их стандартные значения.

**Обратите внимание,** что все лейблы начинаются с префикса `threshold.extended-monitoring.deckhouse.io/`. Указанное в лейбле значение — число, которое устанавливает порог срабатывания алерта.

Например, лейбл `threshold.extended-monitoring.deckhouse.io/5xx-warning: "5"` на Ingress-ресурсе изменяет порог срабатывания алерта с 10% (по умолчанию) на 5%.

###### Non-namespaced Kubernetes-объекты

Non-namespaced Kubernetes-объекты не нуждаются в лейблах на namespace и мониторинг на них включается по умолчанию при включении модуля.

####### Node

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 70             |
| disk-bytes-critical                     | int (percent) | 80             |
| disk-inodes-warning                     | int (percent) | 90             |
| disk-inodes-critical                    | int (percent) | 95             |
| load-average-per-core-warning           | int           | 3              |
| load-average-per-core-critical          | int           | 10             |

> **Важно!** Эти лейблы **не** действуют для тех разделов, в которых расположены `imagefs` (по умолчанию — `/var/lib/docker`) и `nodefs` (по умолчанию — `/var/lib/kubelet`).
Для этих разделов пороги настраиваются полностью автоматически согласно eviction thresholds в kubelet.
Значения по умолчанию см. тут, подробнее см. экспортер.

###### Namespaced Kubernetes-объекты

####### Под

| Label                                   | Type          | Default value  |
|-----------------------------------------|---------------|----------------|
| disk-bytes-warning                      | int (percent) | 85             |
| disk-bytes-critical                     | int (percent) | 95             |
| disk-inodes-warning                     | int (percent) | 85             |
| disk-inodes-critical                    | int (percent) | 90             |

####### Ingress

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| 5xx-warning            | int (percent) | 10            |
| 5xx-critical           | int (percent) | 20            |

####### Deployment

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable. Сработает, если недоступно реплик больше на указанное значение, чем разрешено в `maxUnavailable`. То есть при нуле сработает, если недоступно больше, чем указано в `maxUnavailable`, а при единице сработает, если недоступно больше, чем указано в `maxUnavailable`, плюс 1. Таким образом, у конкретных Deployment, которые находятся в namespace со включенным расширенным мониторингом и которым допустимо быть недоступными, можно подкрутить этот параметр, чтобы не получать ненужные алерты.

####### StatefulSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable (см. комментарии к [Deployment](#deployment)).

####### DaemonSet

| Label                  | Type          | Default value |
|------------------------|---------------|---------------|
| replicas-not-ready     | int (count)   | 0             |

Порог подразумевает количество недоступных реплик **сверх** maxUnavailable (см. комментарии к [Deployment](#deployment)).

####### CronJob

Работает только выключение через лейбл `extended-monitoring.deckhouse.io/enabled=false`.

##### Как работает

Модуль экспортирует в Prometheus специальные лейблы Kubernetes-объектов. Позволяет улучшить Prometheus-правила путем добавления порога срабатывания для алертов.
Использование метрик, экспортируемых данным модулем, позволяет, например, заменить «магические» константы в правилах.

До:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> 1
```

После:

```text
(
  kube_statefulset_status_replicas - kube_statefulset_status_replicas_ready
)
> on (namespace, statefulset)
(
  max by (namespace, statefulset) (extended_monitoring_statefulset_threshold{threshold="replicas-not-ready"})
)
```
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['extended-monitoring'].config-values | format_module_configuration: moduleKebabName }}

### Модуль loki

Модуль предназначен для организации хранилища логов.

Модуль использует проект Grafana Loki.

Модуль разворачивает хранилище логов на базе Grafana Loki, при необходимости настраивает модуль [log-shipper](./log-shipper/) на использование модуля loki и добавляет в Grafana соответствующий datasource.

{% alert level="warning" %}
Модуль не поддерживает работу в режиме высокой доступности, что ограничивает его использование. Для хранения важных журналов рекомендуется использовать другое хранилище.
{% endalert %}

### Модуль loki: настройки

 
<!-- SCHEMA -->

#### Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 10
    retentionPeriodHours: 48
  enabled: true
  version: 1
```
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['loki'].config-values | format_module_configuration: moduleKebabName }}

### Модуль loki: примеры

{% raw %}

#### Чтение логов из всех подов из указанного namespace и направление их в Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: loki
spec:
  settings:
    storageClass: ceph-csi-rbd
    diskSizeGigabytes: 30
    retentionPeriodHours: 168
  enabled: true
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: development-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - development
  destinationRefs:
    - d8-loki
```

Больше примеров в описании модуля [log-shipper](./log-shipper/examples.html).

{% endraw %}

### Модуль log-shipper

Модуль разворачивает агенты `log-shipper` для сборки логов на узлы кластера.
Предназначение этих агентов — с минимальными изменениями отправить логи дальше из кластера.
Каждый агент — это отдельный vector, конфигурацию для которого сгенерировал Deckhouse.

![log-shipper architecture](./images/log-shipper/log_shipper_architecture.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1cOm5emdfPqWp9NT1UrB__TTL31lw7oCgh0VicQH-ouc/edit -->

1. Deckhouse следит за ресурсами [ClusterLoggingConfig](cr.html#clusterloggingconfig), [ClusterLogDestination](cr.html#clusterlogdestination) и [PodLoggingConfig](cr.html#podloggingconfig).
   Комбинация конфигурации для сбора логов и направления для отправки называется `pipeline`.
2. Deckhouse генерирует конфигурационный файл и сохраняет его в `Secret` в Kubernetes.
3. `Secret` монтируется всем подам агентов `log-shipper`, конфигурация обновляется при ее изменении с помощью sidecar-контейнера `reloader`.

#### Топологии отправки

Этот модуль отвечает за агентов на каждом узле. Однако подразумевается, что логи из кластера отправляются согласно одной из описанных ниже топологий.

##### Распределенная

Агенты шлют логи напрямую в хранилище, например в Loki или Elasticsearch.

![log-shipper distributed](./images/log-shipper/log_shipper_distributed.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1FFuPgpDHUGRdkMgpVWXxUXvfZTsasUhEh8XNz7JuCTQ/edit -->

* Менее сложная схема для использования.
* Доступна из коробки без лишних зависимостей, кроме хранилища.
* Сложные трансформации потребляют больше ресурсов на узлах для приложений.

##### Централизованная

Все логи отсылаются в один из доступных агрегаторов, например, Logstash, Vector.
Агенты на узлах стараются отправить логи с узла максимально быстро с минимальным потреблением ресурсов.
Сложные преобразования применяются на стороне агрегатора.

![log-shipper centralized](./images/log-shipper/log_shipper_centralized.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1TL-YUBk0CKSJuKtRVV44M9bnYMq6G8FpNRjxGxfeAhQ/edit -->

* Меньше потребление ресурсов для приложений на узлах.
* Пользователи могут настроить в агрегаторе любые трансформации и слать логи в гораздо большее количество хранилищ.
* Количество выделенных узлов под агрегаторы может увеличиваться вверх и вниз в зависимости от нагрузки.

##### Потоковая

Главная задача данной архитектуры — как можно быстрее отправить логи в очередь сообщений, из которой они в служебном порядке будут переданы в долгосрочное хранилище для дальнейшего анализа.

![log-shipper stream](./images/log-shipper/log_shipper_stream.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1R7vbJPl93DZPdrkSWNGfUOh0sWEAKnCfGkXOvRvK3mQ/edit -->

* Те же плюсы и минусы, что и у централизованной архитектуры, но добавляется еще одно промежуточное хранилище.
* Повышенная надежность. Подходит тем, для кого доставка логов является наиболее критичной.

#### Метаданные

При сборе логов сообщения будут обогащены метаданными в зависимости от способа их сбора. Обогащение происходит на этапе `Source`.

##### Kubernetes

Следующие поля будут экспортированы:

| Label        | Pod spec path           |
|--------------|-------------------------|
| `pod`        | metadata.name           |
| `namespace`  | metadata.namespace      |
| `pod_labels` | metadata.labels         |
| `pod_ip`     | status.podIP            |
| `image`      | spec.containers[].image |
| `container`  | spec.containers[].name  |
| `node`       | spec.nodeName           |
| `pod_owner`  | metadata.ownerRef[0]    |

| Label        | Node spec path                            |
|--------------|-------------------------------------------|
| `node_group` | metadata.labels[].node.deckhouse.io/group |

{% alert -%}
Для Splunk поля `pod_labels` не экспортируются, потому что это вложенный объект, который не поддерживается самим Splunk.
{%- endalert %}

##### File

Единственный лейбл — это `host`, в котором записан hostname сервера.

#### Фильтры сообщений

Существуют два фильтра, чтобы снизить количество отправляемых сообщений в хранилище, — `log filter` и `label filter`.

![log-shipper pipeline](./images/log-shipper/log_shipper_pipeline.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1SnC29zf4Tse4vlW_wfzhggAeTDY2o9wx9nWAZa_A6RM/edit -->

Они запускаются сразу после объединения строк с помощью multiline parser'а.

1. `label filter` — правила запускаются для метаданных сообщения. Поля для метаданных (или лейблов) наполняются на основании источника логов, так что для разных источников будет разный набор полей. Эти правила полезны, например, чтобы отбросить сообщения от определенного контейнера или пода с/без какой-то метки.
2. `log filter` — правила запускаются для исходного сообщения. Есть возможность отбросить сообщение на основании JSON-поля или, если сообщение не в формате JSON, использовать регулярное выражение для поиска по строке.

Оба фильтра имеют одинаковую структурированную конфигурацию:
* `field` — источник данных для запуска фильтрации (чаще всего это значение label'а или поля из JSON-документа).
* `operator` — действие для сравнения, доступные варианты — In, NotIn, Regex, NotRegex, Exists, DoesNotExist.
* `values` — эта опция имеет разные значения для разных операторов:
  * DoesNotExist, Exists — не поддерживается;
  * In, NotIn — значение поля должно равняться или не равняться одному из значений в списке values;
  * Regex, NotRegex — значение должно подходить хотя бы под одно или не подходить ни под одно регулярное выражение из списка values.

Вы можете найти больше примеров в разделе [Примеры](examples.html) документации.

{% alert -%}
Extra labels добавляются на этапе `Destination`, поэтому невозможно фильтровать логи на их основании.
{%- endalert %}

### Модуль log-shipper: настройки

Модуль начинает чтение логов, только если создан pipeline в виде связанных между собой [ClusterLoggingConfig](cr.html#clusterloggingconfig)/[PodLoggingConfig](cr.html#podloggingconfig) и [ClusterLogDestination](cr.html#clusterlogdestination).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['log-shipper'].config-values | format_module_configuration: moduleKebabName }}

### Модуль log-shipper: Custom Resources
{{ site.data.schemas.log-shipper.crds.cluster-log-destination | format_crd: "log-shipper" }}
{{ site.data.schemas.log-shipper.crds.cluster-logging-config | format_crd: "log-shipper" }}
{{ site.data.schemas.log-shipper.crds.pod-logging-config | format_crd: "log-shipper" }}

### Модуль log-shipper: примеры

{% raw %}

#### Чтение логов из всех подов кластера и направление их в Loki

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Чтение логов подов из указанного namespace с указанным label и перенаправление одновременно в Loki и Elasticsearch

Чтение логов подов из namespace `whispers` только с label `app=booking` и перенаправление одновременно в Loki и Elasticsearch:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: whispers-booking-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      matchNames:
        - whispers
    labelSelector:
      matchLabels:
        app: booking
  destinationRefs:
  - loki-storage
  - es-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: logs-%F
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

#### Создание source в namespace и чтение логов всех подов в этом NS с направлением их в Loki

Следующий pipeline создает source в namespace `test-whispers`, читает логи всех подов в этом NS и пишет их в Loki:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Чтение только подов в указанном namespace и с определенным label

Пример чтения только подов, имеющих label `app=booking`, в namespace `test-whispers`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: PodLoggingConfig
metadata:
  name: whispers-logs
  namespace: tests-whispers
spec:
  labelSelector:
    matchLabels:
      app: booking
  clusterDestinationRefs:
    - loki-storage
---
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

#### Переход с Promtail на Log-Shipper

В ранее используемом URL Loki требуется убрать путь `/loki/api/v1/push`.

**Vector** сам добавит этот путь при работе с Loki.

#### Работа с Grafana Cloud

Данная документация подразумевает, что у вас уже создан ключ API.

Для начала вам потребуется закодировать в base64 ваш токен доступа к Grafana Cloud.

![Grafana cloud API key](./images/log-shipper/grafana_cloud.png)

```bash
echo -n "<YOUR-GRAFANACLOUD-TOKEN>" | base64 -w0
```

Затем нужно создать **ClusterLogDestination**

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  loki:
    auth:
      password: PFlPVVItR1JBRkFOQUNMT1VELVRPS0VOPg==
      strategy: Basic
      user: "<YOUR-GRAFANACLOUD-USER>"
    endpoint: <YOUR-GRAFANACLOUD-URL> # Например https://logs-prod-us-central1.grafana.net или https://logs-prod-eu-west-0.grafana.net
  type: Loki
```

Теперь можно создать PodLogginConfig или ClusterPodLoggingConfig и отправлять логи в **Grafana Cloud**.

#### Добавление Loki в Deckhouse Grafana

Вы можете работать с Loki из встроенной в Deckhouse Grafana. Достаточно добавить [**GrafanaAdditionalDatasource**](./modules/prometheus/cr.html#grafanaadditionaldatasource).

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: loki
spec:
  access: Proxy
  basicAuth: false
  jsonData:
    maxLines: 5000
    timeInterval: 30s
  type: loki
  url: http://loki.loki:3100
```

#### Поддержка Elasticsearch < 6.X

Для Elasticsearch < 6.0 нужно включить поддержку doc_type индексов.
Сделать это можно следующим образом:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    docType: "myDocType" # Укажите значение здесь. Оно не должно начинаться с '_'.
    auth:
      strategy: Basic
      user: elastic
      password: c2VjcmV0IC1uCg==
```

#### Шаблон индекса для Elasticsearch

Существует возможность отправлять сообщения в определенные индексы на основе метаданных с помощью шаблонов индексов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ namespace }}-%F"
```

В приведенном выше примере для каждого пространства имен Kubernetes будет создан свой индекс в Elasticsearch.

Эта функция также хорошо работает в комбинации с `extraLabels`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: es-storage
spec:
  type: Elasticsearch
  elasticsearch:
    endpoint: http://192.168.1.1:9200
    index: "k8s-{{ service }}-{{ namespace }}-%F"
  extraLabels:
    service: "{{ service_name }}"
```

1. Если сообщение имеет формат JSON, поле `service_name` этого документа JSON перемещается на уровень метаданных.
2. Новое поле метаданных `service` используется в шаблоне индекса.

#### Пример интеграции со Splunk

Существует возможность отсылать события из Deckhouse в Splunk.

1. Endpoint должен быть таким же, как имя вашего экземпляра Splunk с портом `8088` и без указания пути, например `https://prd-p-xxxxxx.splunkcloud.com:8088`.
2. Чтобы добавить token для доступа, откройте пункт меню `Setting` -> `Data inputs`, добавьте новый `HTTP Event Collector` и скопируйте token.
3. Укажите индекс Splunk для хранения логов, например `logs`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: splunk
spec:
  type: Splunk
  splunk:
    endpoint: https://prd-p-xxxxxx.splunkcloud.com:8088
    token: xxxx-xxxx-xxxx
    index: logs
    tls:
      verifyCertificate: false
      verifyHostname: false
```

{% endraw %}
{% alert -%}
`destination` не поддерживает метки пода для индексирования. Рассмотрите возможность добавления нужных меток с помощью опции `extraLabels`.
{%- endalert %}
{% raw %}

```yaml
extraLabels:
  pod_label_app: '{{ pod_labels.app }}'
```

#### Простой пример Logstash

Чтобы отправлять логи в Logstash, на стороне Logstash должен быть настроен входящий поток `tcp` и его кодек должен быть `json`.

Пример минимальной конфигурации Logstash:

```hcl
input {
  tcp {
    port => 12345
    codec => json
  }
}
output {
  stdout { codec => json }
}
```

Пример манифеста `ClusterLogDestination`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: logstash
spec:
  type: Logstash
  logstash:
    endpoint: logstash.default:12345
```

#### Syslog

Следующий пример показывает, как отправлять сообщения через сокет по протоколу TCP в формате syslog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: rsyslog
spec:
  type: Socket
  socket:
    mode: TCP
    address: 192.168.0.1:3000
    encoding: 
      codec: Syslog
  extraLabels:
    syslog.severity: "alert"
    # поле request_id должно присутствовать в сообщении
    syslog.message_id: "{{ request_id }}"
```

#### Пример интеграции с Graylog

Убедитесь, что в Graylog настроен входящий поток для приема сообщений по протоколу TCP на указанном порту. Пример манифеста для интеграции с Graylog:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: test-socket2-dest
spec:
  type: Socket
  socket:
    address: graylog.svc.cluster.local:9200
    mode: TCP
    encoding:
      codec: GELF
```

#### Логи в CEF формате

Существует способ формировать логи в формате CEF, используя `codec: CEF`, с переопределением `cef.name` и `cef.severity` по значениям из поля `message` (лога приложения) в формате JSON.

В примере ниже `app` и `log_level` это ключи содержащие значения для переопределения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: siem-kafka
spec:
  extraLabels:
    cef.name: '{{ app }}'
    cef.severity: '{{ log_level }}'
  type: Kafka
  kafka:
    bootstrapServers:
      - my-cluster-kafka-brokers.kafka:9092
    encoding:
      codec: CEF
    tls:
      verifyCertificate: false
      verifyHostname: true
    topic: logs
```

Так же можно вручную задать свои значения:

```yaml
extraLabels:
  cef.name: 'TestName'
  cef.severity: '1'
```

#### Сбор событий Kubernetes

События Kubernetes могут быть собраны log-shipper'ом, если `events-exporter` включен в настройках модуля [extended-monitoring](./extended-monitoring/).

Включите events-exporter, изменив параметры модуля `extended-monitoring`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: extended-monitoring
spec:
  version: 1
  settings:
    events:
      exporterEnabled: true
```

Выложите в кластер следующий `ClusterLoggingConfig`, чтобы собирать сообщения с пода `events-exporter`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubernetes-events
spec:
  type: KubernetesPods
  kubernetesPods:
    labelSelector:
      matchLabels:
        app: events-exporter
    namespaceSelector:
      matchNames:
      - d8-monitoring
  destinationRefs:
  - loki-storage
```

#### Фильтрация логов

Пользователи могут фильтровать логи, используя следующие фильтры:

* `labelFilter` — применяется к метаданным, например имени контейнера (`container`), пространству имен (`namespace`) или имени пода (`pod_name`);
* `logFilter` — применяется к полям самого сообщения, если оно в JSON-формате.

##### Сборка логов только для контейнера `nginx`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: nginx-logs
spec:
  type: KubernetesPods
  labelFilter:
  - field: container
    operator: In
    values: [nginx]
  destinationRefs:
  - loki-storage
```

##### Сборка логов без строки, содержащей `GET /status" 200`

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: all-logs
spec:
  type: KubernetesPods
  destinationRefs:
  - loki-storage
  labelFilter:
  - field: message
    operator: NotRegex
    values:
    - .*GET /status" 200$
```

##### Аудит событий kubelet'а

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: kubelet-audit-logs
spec:
  type: File
  file:
    include:
    - /var/log/kube-audit/audit.log
  logFilter:
  - field: userAgent  
    operator: Regex
    values: ["kubelet.*"]
  destinationRefs:
  - loki-storage
```

##### Системные логи Deckhouse

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: system-logs
spec:
  type: File
  file:
    include:
    - /var/log/syslog
  labelFilter:
  - field: message
    operator: Regex
    values:
    - .*d8-kubelet-forker.*
    - .*containerd.*
    - .*bashible.*
    - .*kernel.*
  destinationRefs:
  - loki-storage
```

{% endraw %}
{% alert -%}
Если вам нужны только логи одного пода или малой группы подов, постарайтесь использовать настройки `kubernetesPods`, чтобы сузить количество читаемых файлов. Фильтры необходимы только для высокогранулярной настройки.
{%- endalert %}
{% raw %}

#### Настройка сборки логов с продуктовых namespace'ов, используя опцию namespace label selector

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: production-logs
spec:
  type: KubernetesPods
  kubernetesPods:
    namespaceSelector:
      labelSelector:
        matchLabels:
          environment: production
  destinationRefs:
  - loki-storage
```

#### Исключение подов и пространств имён, используя label

Существует преднастроенный label для исключения определенных подов и пространств имён: `log-shipper.deckhouse.io/exclude=true`.
Он помогает остановить сбор логов с подов и пространств имён без изменения глобальной конфигурации.

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
  labels:
    log-shipper.deckhouse.io/exclude: "true"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  ...
  template:
    metadata:
      labels:
        log-shipper.deckhouse.io/exclude: "true"
```

#### Включение буферизации

Настройка буферизации логов необходима для улучшения надежности и производительности системы сбора логов. Буферизация может быть полезна в следующих случаях:

1. Временные перебои с подключением. Если есть временные перебои или нестабильность соединения с системой хранения логов (например, с Elasticsearch), буфер позволяет временно сохранять логи и отправить их, когда соединение восстановится.

1. Сглаживание пиков нагрузки. При внезапных всплесках объема логов буфер позволяет сгладить пиковую нагрузку на систему хранения логов, предотвращая её перегрузку и потенциальную потерю данных.

1. Оптимизация производительности. Буферизация помогает оптимизировать производительность системы сбора логов за счёт накопления логов и отправки их группами, что снижает количество сетевых запросов и улучшает общую пропускную способность.

##### Пример включения буферизации в оперативной памяти

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    memory:
      maxEvents: 4096
    type: Memory
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

##### Пример включения буферизации на диске

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

##### Пример определения поведения при переполнении буфера

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLogDestination
metadata:
  name: loki-storage
spec:
  buffer:
    disk:
      maxSize: 1Gi
    type: Disk
    whenFull: DropNewest
  type: Loki
  loki:
    endpoint: http://loki.loki:3100
```

Более подробное описание параметров доступно [в ресурсе ClusterLogDestination](cr.html#clusterlogdestination).

{% endraw %}

### The log-shipper module: FAQ

#### Как добавить авторизацию в ресурс _ClusterLogDestination_?

Чтобы добавить параметры авторизации в ресурс [ClusterLogDestination](cr.html#clusterlogdestination), необходимо:
- изменить [протокол](cr.html#clusterlogdestination-v1alpha1-spec-loki-endpoint) подключения к Loki на HTTPS;
- добавить секцию [auth](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth), в которой:
  - параметр [strategy](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-strategy) установить в `Bearer`;
  - в параметре [token](cr.html#clusterlogdestination-v1alpha1-spec-loki-auth-token) указать токен `log-shipper-token` из пространства имен `d8-log-shipper`.

Пример:

- Ресурс _ClusterLogDestination_ без авторизации:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "http://loki.d8-monitoring:3100"
  ```

- Получите токен `log-shipper-token` из пространства имен `d8-log-shipper`:

  ```bash
  kubectl -n d8-log-shipper get secret log-shipper-token -o jsonpath='{.data.token}' | base64 -d
  ```

- Ресурс _ClusterLogDestination_ с авторизацией:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ClusterLogDestination
  metadata:
    name: loki
  spec:
    type: Loki
    loki:
      endpoint: "https://loki.d8-monitoring:3100"
      auth:
        strategy: "Bearer"
        token: <log-shipper-token>
      tls:
        verifyHostname: false
        verifyCertificate: false
  ```

### Модуль monitoring-custom

Модуль расширяет возможности модуля [prometheus](./modules/prometheus/) по мониторингу приложений пользователей.

Чтобы организовать сбор метрик с приложений модулем `monitoring-custom`, необходимо:

- Поставить лейбл `prometheus.deckhouse.io/custom-target` на Service или под. Значение лейбла определит имя в списке target'ов Prometheus.
  - В качестве значения label'а prometheus.deckhouse.io/custom-target стоит использовать название приложения (маленькими буквами, разделитель `-`), которое позволяет его уникально идентифицировать в кластере.

     При этом, если приложение ставится в кластер больше одного раза (staging, testing и т. д.) или даже ставится несколько раз в один namespace, достаточно одного общего названия, так как у всех метрик в любом случае будут лейблы namespace, pod и, если доступ осуществляется через Service, лейбл service. То есть это название, уникально идентифицирующее приложение в кластере, а не единичную его инсталляцию.
- Порту, с которого нужно собирать метрики, указать имя `http-metrics` и `https-metrics` для подключения по HTTP или HTTPS соответственно.

  Если это невозможно (например, порт уже определен и назван другим именем), необходимо воспользоваться аннотациями: `prometheus.deckhouse.io/port: номер_порта` — для указания порта и `prometheus.deckhouse.io/tls: "true"` — если сбор метрик будет проходить по HTTPS.

  > **Важно!** При указании аннотации на Service в качестве значения порта необходимо использовать `targetPort`. То есть тот порт, что открыт и слушается приложением, а не порт Service'а.

  - Пример 1:

    ```yaml
    ports:
    - name: https-metrics
      containerPort: 443
    ```

  - Пример 2:

    ```yaml
    annotations:
      prometheus.deckhouse.io/port: "443"
      prometheus.deckhouse.io/tls: "true"  # Если метрики отдаются по HTTP, эту аннотацию указывать не нужно.
    ```

- При использовании service mesh [Istio](./istio/) в режиме STRICT mTLS указать для сбора метрик следующую аннотацию у Service или Pod: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Не обязательно)* Указать дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: ''):
    - возможно указать несколько таких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Эту аннотацию надо вешать на тот же ресурс, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

##### Пример: Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
  annotations:
    prometheus.deckhouse.io/port: "8061"                      # По умолчанию будет использоваться порт сервиса с именем http-metrics или https-metrics.
    prometheus.deckhouse.io/path: "/my_app/metrics"           # По умолчанию /metrics.
    prometheus.deckhouse.io/query-param-format: "prometheus"  # По умолчанию ''.
    prometheus.deckhouse.io/allow-unready-pod: "true"         # По умолчанию поды НЕ в Ready игнорируются.
    prometheus.deckhouse.io/sample-limit: "5000"              # По умолчанию принимается не больше 5000 метрик от одного пода.
spec:
  ports:
  - name: my-app
    port: 8060
  - name: http-metrics
    port: 8061
    targetPort: 8061
  selector:
    app: my-app
```

##### Пример: Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  labels:
    app: my-app
spec:
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
        prometheus.deckhouse.io/custom-target: my-app
      annotations:
        prometheus.deckhouse.io/sample-limit: "5000"  # По умолчанию принимается не больше 5000 метрик от одного пода.
    spec:
      containers:
      - name: my-app
        image: my-app:1.7.9
        ports:
        - name: https-metrics
          containerPort: 443
```

### Модуль monitoring-custom: настройки

Модуль работает, если включен модуль `prometheus`, и не имеет параметров для настройки.

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-custom'].config-values | format_module_configuration: moduleKebabName }}

### Модуль monitoring-kubernetes

Модуль предназначен для базового мониторинга узлов кластера.

Обеспечивает безопасный сбор метрик и предоставляет базовый набор правил для мониторинга:
- текущей версии container runtime (docker, containerd) на узле и ее соответствия версиям, разрешенным для использования;
- общей работоспособности подсистемы мониторинга кластера (Dead man's switch);
- доступных файловых дескрипторов, сокетов, свободного места и inode;
- работы `kube-state-metrics`, `node-exporter`, `kube-dns`;
- состояния узлов кластера (NotReady, drain, cordon);
- состояния синхронизации времени на узлах;
- случаев продолжительного превышения CPU steal;
- состояния таблицы Conntrack на узлах;
- подов с некорректным состоянием (как возможное следствие проблем с kubelet) и др.

### Модуль monitoring-kubernetes: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-kubernetes'].config-values | format_module_configuration: moduleKebabName }}

### Мониторинг control plane

Мониторинг control plane осуществляется с помощью модуля `monitoring-kubernetes-control-plane`, который организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:
* kube-apiserver;
* kube-controller-manager;
* kube-scheduler;
* kube-etcd.

### Мониторинг control plane: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-kubernetes-control-plane'].config-values | format_module_configuration: moduleKebabName }}

### Модуль monitoring-ping

#### Описание

Данный модуль предназначен для мониторинга сетевого взаимодействия между всеми узлами кластера, а также — опционально — до дополнительных внешних узлов.

Каждый узел два раза в секунду отправляет ICMP-пакеты на все другие узлы кластера (и на опциональные внешние узлы) и экспортирует данные в `Prometheus`.
В комплекте идет dashboard для `Grafana`, на котором отражаются соответствующие графики.

#### Как работает

Модуль следит за любыми изменениями поля `.status.addresses` узла. В случае выявления таковых
запускается хук, который собирает полный список имен узлов и их адресов и передает в daemonSet, что в свою очередь пересоздает поды.
Таким образом, `ping` проверяет всегда актуальный список узлов.

### Модуль monitoring-ping: настройки

У модуля нет обязательных настроек.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['monitoring-ping'].config-values | format_module_configuration: moduleKebabName }}

### Модуль operator-prometheus

Модуль устанавливает prometheus operator, который позволяет создавать и автоматизированно управлять инсталляциями Prometheus.

<!-- Исходник картинок: https://docs.google.com/drawings/d/1KMgawZD4q7jEYP-_g6FvUeJUaT3edro_u6_RsI3ZVvQ/edit -->

Функционал устанавливаемого оператора:
- определяет следующие custom resource'ы:
  - `Prometheus` — определяет инсталляцию (кластер) *Prometheus*
  - `ServiceMonitor` — определяет, как собирать метрики с сервисов
  - `Alertmanager` — определяет кластер *Alertmanager*'ов
  - `PrometheusRule` — определяет список *Prometheus rules*
- следит за этими ресурсами и:
  - генерирует `StatefulSet` с самим *Prometheus* и необходимые для его работы конфигурационные файлы, сохраняя их в `Secret`;
  - следит за ресурсами `ServiceMonitor` и `PrometheusRule` и на их основании обновляет конфигурационные файлы *Prometheus* через внесение изменений в `Secret`.

#### Prometheus

##### Что делает Prometheus?

В целом, сервер Prometheus делает две ключевых вещи — **собирает метрики** и **выполняет правила**:
* Для каждого *target'а* (цель для мониторинга), каждый `scrape_interval`, делает HTTP запрос на этот *target*, получает в ответ метрики в своем формате, которые сохраняет к себе в базу
* Каждый `evaluation_interval` обрабатывает *rules*, на основании чего:
  * или шлет алерты
  * или записывает (себе же в базу) новые метрики (результат выполнения *rule'а*)

##### Как настраивается Prometheus?

* У сервера Prometheus есть *config* и есть *rule files* (файлы с правилами)
* В `config` имеются следующие секции:
  * `scrape_configs` — настройки поиска *target'ов* (целей для мониторинга, см. подробней следующий раздел).
  * `rule_files` — список директорий, в которых лежат *rule'ы*, которые необходимо загружать:

    ```yaml
    rule_files:
    - /etc/prometheus/rules/rules-0/*
    - /etc/prometheus/rules/rules-1/*
    ```

  * `alerting` — настройки поиска *Alert Manager'ов*, в которые слать алерты. Секция очень похожа на `scrape_configs`, только результатом ее работы является список *endpoint'ов*, в которые Prometheus будет слать алерты.

##### Где Prometheus берет список *target'ов*?

* В целом Prometheus работает следующим образом:

  ![Работа Prometheus](./images/operator-prometheus/targets.png)

  * **(1)** Prometheus читает секцию конфига `scrape_configs`, согласно которой настраивает свой внутренний механизм Service Discovery
  * **(2)** Механизм Service Discovery взаимодействует с API Kubernetes (в основном — получает endpoint`ы)
  * **(3)** На основании происходящего в Kubernetes механизм Service Discovery обновляет Targets (список *target'ов*)
* В `scrape_configs` указан список *scrape job'ов* (внутреннее понятие Prometheus), каждый из которых определяется следующим образом:

  ```yaml
  scrape_configs:
    # Общие настройки
  - job_name: d8-monitoring/custom/0    # просто название scrape job'а, показывается в разделе Service Discovery
    scrape_interval: 30s                  # как часто собирать данные
    scrape_timeout: 10s                   # таймаут на запрос
    metrics_path: /metrics                # path, который запрашивать
    scheme: http                          # http или https
    # Настройки service discovery
    kubernetes_sd_configs:                # означает, что target'ы мы получаем из Kubernetes
    - api_server: null                    # означает, что адрес API-сервера использовать из переменных окружения (которые есть в каждом Pod'е)
      role: endpoints                     # target'ы брать из endpoint'ов
      namespaces:
        names:                            # искать endpoint'ы только в этих namespace'ах
        - foo
        - baz
    # Настройки "фильтрации" (какие enpoint'ы брать, а какие нет) и "релейблинга" (какие лейблы добавить или удалить, на все получаемые метрики)
    relabel_configs:
    # Фильтр по значению label'а prometheus_custom_target (полученного из связанного с endpoint'ом service'а)
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: .+                           # подходит любой НЕ пустой лейбл
      action: keep
    # Фильтр по имени порта
    - source_labels: [__meta_kubernetes_endpointslice_port_name]
      regex: http-metrics                 # подходит, только если порт называется http-metrics
      action: keep
    # Добавляем label job, используем значение label'а prometheus_custom_target у service'а, к которому добавляем префикс "custom-"
    #
    # Лейбл job это служебный лейбл Prometheus:
    #    * он определяет название группы, в которой будет показываться target на странице targets
    #    * и конечно же он будет у каждой метрики, полученной у этих target'ов, чтобы можно было удобно фильтровать в rule'ах и dashboard'ах
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: (.*)
      target_label: job
      replacement: custom-$1
      action: replace
    # Добавляем label namespace
    - source_labels: [__meta_kubernetes_namespace]
      regex: (.*)
      target_label: namespace
      replacement: $1
      action: replace
    # Добавляем label service
    - source_labels: [__meta_kubernetes_service_name]
      regex: (.*)
      target_label: service
      replacement: $1
      action: replace
    # Добавляем label instance (в котором будет имя Pod'а)
    - source_labels: [__meta_kubernetes_pod_name]
      regex: (.*)
      target_label: instance
      replacement: $1
      action: replace
  ```

* Таким образом, Prometheus сам отслеживает:
  * добавление и удаление Pod'ов (при добавлении/удалении Pod'ов Kubernetes изменяет endpoint'ы, а Prometheus это видит и добавляет/удаляет *target'ы*)
  * добавление и удаление сервисов (точнее endpoint'ов) в указанных namespace'ах
* Изменение конфига требуется в следующих случаях:
  * нужно добавить новый scrape config (обычно — новый вид сервисов, которые надо мониторить)
  * нужно изменить список namespace'ов

#### Prometheus Operator

##### Что делает Prometheus Operator?

* С помощью механизма CRD (Custom Resource Definitions) определяет четыре custom ресурса:
  * prometheus — определяет инсталляцию (кластер) Prometheus
  * servicemonitor — определяет, как "мониторить" (собирать метрики) набор сервисов
  * alertmanager — определяет кластер Alertmanager'ов
  * prometheusrule — определяет список Prometheus rules
* Следит за ресурсами `prometheus` и генерирует для каждого:
  * StatefulSet (с самим Prometheus'ом)
  * Secret с `prometheus.yaml` (конфиг Prometheus'а) и `configmaps.json` (конфиг для `prometheus-config-reloader`)
* Следит за ресурсами `servicemonitor` и `prometheusrule` и на их основании обновляет конфиги (`prometheus.yaml` и `configmaps.json`, которые лежат в секрете).

##### Что в Pod'е с Prometheus'ом?

![Что в Pod Prometheus](./images/operator-prometheus/pod.png)

* Два контейнера:
  * `prometheus` — сам Prometheus
  * `prometheus-config-reloader` — обвязка, которая:
    * следит за изменениями `prometheus.yaml` и, при необходимости, вызывает reload конфигурации Prometheus'у (специальным HTTP-запросом, см. [подробнее ниже](#как-обрабатываются-service-monitorы))
    * следит за PrometheusRule'ами (см. [подробнее ниже](#как-обрабатываются-custome-resources-с-ruleами)) и по необходимости скачивает их и перезапускает Prometheus
* Pod использует три volume:
  * config — примонтированный secret (два файла: `prometheus.yaml` и `configmaps.json`). Подключен в оба контейнера.
  * rules — `emptyDir`, который наполняет `prometheus-config-reloader`, а читает `prometheus`. Подключен в оба контейнера, но в `prometheus` в режиме read only.
  * data — данные Prometheus. Подмонтирован только в `prometheus`.

##### Как обрабатываются Service Monitor'ы?

![Как обрабатываются Service Monitor'ы](./images/operator-prometheus/servicemonitors.png)

* **(1)** Prometheus Operator читает (а также следит за добавлением/удалением/изменением) Service Monitor'ы (какие именно Service Monitor'ы — указано в самом ресурсе `prometheus`, см. подробней официальную документацию).
* **(2)** Для каждого Service Monitor'а, если в нем НЕ указан конкретный список namespace'ов (указано `any: true`), Prometheus Operator вычисляет (обращаясь к API Kubernetes) список namespace'ов, в которых есть Service'ы (подходящие под указанные в Service Monitor'е label'ы).
* **(3)** На основании прочитанных ресурсов `servicemonitor` (см. официальную документацию) и на основании вычисленных namespace'ов Prometheus Operator генерирует часть конфига (секцию `scrape_configs`) и сохраняет конфиг в соответствующий Secret.
* **(4)** Штатными средствами самого Kubernetes данные из секрета прилетают в Pod (файл `prometheus.yaml` обновляется).
* **(5)** Изменение файла замечает `prometheus-config-reloader`, который по HTTP отправляет запрос Prometheus'у на перезагрузку.
* **(6)** Prometheus перечитывает конфиг и видит изменения в scrape_configs, которые обрабатывает уже согласно своей логике работы (см. подробнее выше).

##### Как обрабатываются Custome Resources с *rule'ами*?

![Как обрабатываются Custome Resources с rule'ами](./images/operator-prometheus/rules.png)

* **(1)** Prometheus Operator следит за PrometheusRule'ами (подходящими под указанный в ресурсе `prometheus` `ruleSelector`).
* **(2)** Если появился новый (или был удален существующий) PrometheusRule — Prometheus Operator обновляет `prometheus.yaml` (а дальше срабатывает логика в точности соответствующая обработке Service Monitor'ов, которая описана выше).
* **(3)** Как в случае добавления/удаления PrometheusRule'а, так и при изменении содержимого PrometheusRule'а, Prometheus Operator обновляет ConfigMap `prometheus-main-rulefiles-0`.
* **(4)** Штатными средствами самого Kubernetes данные из ConfigMap прилетают в Pod
* Изменение файла замечает `prometheus-config-reloader`, который:
  * **(5)** скачивает изменившиеся ConfigMap'ы в директорию rules (это `emptyDir`)
  * **(6)** по HTTP отправляет запрос Prometheus'у на перезагрузку
* **(7)** Prometheus перечитывает конфиг и видит изменившиеся *rule'ы*.

### Модуль operator-prometheus: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['operator-prometheus'].config-values | format_module_configuration: moduleKebabName }}

### Prometheus-operator: примеры конфигурации

#### Установка еще одного prometheus-operator в кластер

Пользователю может понадобится установить в кластер еще один prometheus-operator,
чтобы добавить Prometheus'ы или alertmanager'ы в кластер.

1. Чтобы не пересекаться с prometheus-operator из Deckhouse, необходимо указать флаг
   `--deny-namespaces=d8-monitoring` для пользовательской инсталляции prometheus-operator.

2. Prometheus-operator из Deckhouse следит за ресурсами правил и мониторов только в пространствах имен
   с меткой `heritage: deckhouse`. Не устанавливайте эту метку на пользовательские пространства имен.

### Prometheus-мониторинг

Устанавливает и полностью настраивает Prometheus, настраивает сбор метрик со многих распространенных приложений, а также предоставляет необходимый минимальный набор alert'ов для Prometheus и dashboard Grafana.

Если используется StorageClass с поддержкой автоматического расширения (`allowVolumeExpansion: true`), при нехватке места на диске для данных Prometheus его емкость будет увеличена.

Ресурсы CPU и memory автоматически выставляются при пересоздании пода на основе истории потребления, благодаря модулю [Vertical Pod Autoscaler](./modules/vertical-pod-autoscaler/). Также, благодаря кэшированию запросов к Prometheus с помощью Trickster, потребление памяти Prometheus сильно сокращается.

Поддерживается как pull-, так и push-модель получения метрик.

#### Мониторинг аппаратных ресурсов

Реализовано отслеживание нагрузки на аппаратные ресурсы кластера с графиками по утилизации:
- процессора;
- памяти;
- диска;
- сети.

Графики доступны с агрегацией в разрезе:
- по подам;
- контроллерам;
- пространствам имен;
- узлам.

#### Мониторинг Kubernetes

Deckhouse настраивает мониторинг широкого набора параметров «здоровья» Kubernetes и его компонентов, в частности:
- общей утилизации кластера;
- связанности узлов Kubernetes между собой (измеряется rtt между всеми узлами);
- доступности и работоспособности компонентов control plane:
  - `etcd`;
  - `coredns` и `kube-dns`;
  - `kube-apiserver` и др.
- синхронизации времени на узлах и др.

#### Мониторинг Ingress

Подробно описан [здесь](./modules/ingress-nginx/#мониторинг-и-статистика)

#### Режим расширенного мониторинга

В Deckhouse возможно использование [режима расширенного мониторинга](./extended-monitoring/), который предоставляет возможности алертов по дополнительным метрикам: свободному месту и inode на дисках узлов, утилизации узлов, доступности подов и образов контейнеров, истечении действия сертификатов, другим событиям кластера.

##### Алертинг в режиме расширенного мониторинга

Deckhouse позволяет гибко настроить алертинг на каждый из namespace'ов и указывать разную критичность в зависимости от порогового значения. Есть возможность указать множество пороговых значений отправки алертов в различные namespace'ы, например, для таких параметров, как:
- значения свободного места и inodes на диске;
- утилизация CPU узлов и контейнера;
- процент 5xx ошибок на `nginx-ingress`;
- количество возможных недоступных подов в `Deployment`, `StatefulSet`, `DaemonSet`.

#### Алерты

Мониторинг в составе Deckhouse включает также и возможности уведомления о событиях. В стандартной поставке уже идет большой набор только необходимых алертов, покрывающих состояние кластера и его компонентов. При этом всегда остается возможность добавления кастомных алертов.

##### Отправка алертов во внешние системы

Deckhouse поддерживает отправку алертов с помощью `Alertmanager`:
- по протоколу SMTP;
- в Telegram;
- посредством Webhook.

#### Включенные модули

![Схема взаимодействия](./images/prometheus/prometheus_monitoring_new.svg)

##### Компоненты, устанавливаемые Deckhouse

| Компонент                   | Описание                                                                                                                                                                                                                                                                                        |
|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **prometheus-main**         | Основной Prometheus, который выполняет scrape каждые 30 секунд (с помощью параметра `scrapeInterval` можно изменить это значение). Именно он обрабатывает все правила, отправляет алерты и является основным источником данных.                                                                 |
| **prometheus-longterm**     | Дополнительный Prometheus, который выполняет scrape данных из основного Prometheus (`prometheus-main`) каждые 5 минут (с помощью параметра `longtermScrapeInterval` можно изменить это значение). Используется для продолжительного хранения истории и отображения больших промежутков времени. |
| **trickster**               | Кэширующий прокси, снижающий нагрузку на Prometheus.                                                                                                                                                                                                                                            |
| **aggregating-proxy**       | Агрегирующий и кеширующий прокси, снижающий нагрузку на Prometheus и объединяющий main и longterm в один источник.                                                                                                                                                                             |
| **memcached**               | Сервис кэширования данных в оперативной памяти.                                                                                                                                                                                                                                                 |
| **grafana**                 | Управляемая платформа визуализации данных. Включает подготовленные dashboard'ы для всех модулей Deckhouse и некоторых популярных приложений. Grafana умеет работать в режиме высокой доступности, не хранит состояние и настраивается с помощью CRD.                                            |
| **metrics-adapter**         | Компонент, соединяющий Prometheus и Kubernetes metrics API. Включает поддержку HPA в кластере Kubernetes.                                                                                                                                                                                       |
| **vertical-pod-autoscaler** | Компонент, позволяющий автоматически изменять размер запрошенных ресурсов для подов с целью оптимальной утилизации CPU и памяти.                                                                                                                                                                |
| **Различные exporter'ы**    | Подготовленные и подключенные к Prometheus exporter'ы. Список включает множество exporter'ов для всех необходимых метрик: `kube-state-metrics`, `node-exporter`, `oomkill-exporter`, `image-availability-exporter` и многие другие.                                                             |

##### Внешние компоненты

Deckhouse может интегрироваться с большим количеством разнообразных решений следующими способами:

| Название                       | Описание|
|--------------------------------|--------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanager'ы могут быть подключены к Prometheus и Grafana и находиться как в кластере Deckhouse, так и за его пределами.|
| **Long-term metrics storages** | Используя протокол `remote write`, возможно отсылать метрики из Deckhouse в большое количество хранилищ, включающее Cortex, Thanos, VictoriaMetrics.|

### Prometheus-мониторинг: настройки

Модуль не требует обязательной конфигурации (все работает из коробки).

 
<!-- SCHEMA -->

#### Аутентификация

По умолчанию используется модуль [user-authn](/products/kubernetes-platform/documentation/v1/modules/user-authn/). Также можно настроить аутентификацию через `externalAuthentication` (см. ниже).
Если эти варианты отключены, модуль включит basic auth со сгенерированным паролем и пользователем `admin`.

Посмотреть сгенерированный пароль можно командой:

```shell
kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values prometheus -o json | jq '.internal.auth.password'
```

Чтобы сгенерировать новый пароль, нужно удалить Secret:

```shell
kubectl -n d8-monitoring delete secret/basic-auth
```

> **Внимание!** Параметр `auth.password` больше не поддерживается.

#### Примечание

* `retentionSize` для `main` и `longterm` **рассчитывается автоматически, возможности задать значение нет!**
  * Алгоритм расчета:
    * `pvc_size * 0.85` — если PVC существует;
    * `10 GiB` — если PVC нет и StorageClass поддерживает ресайз;
    * `25 GiB` — если PVC нет и StorageClass не поддерживает ресайз.
  * Если используется `local-storage` и требуется изменить `retentionSize`, необходимо вручную изменить размер PV и PVC в нужную сторону. **Внимание!** Для расчета берется значение из `.status.capacity.storage` PVC, поскольку оно отражает реальный размер PV в случае ручного ресайза.
* `40 GiB` — размер PersistentVolumeClaim создаваемого по умолчанию.
* Размер дисков Prometheus можно изменить стандартным для Kubernetes способом (если в StorageClass это разрешено), отредактировав в PersistentVolumeClaim поле `.spec.resources.requests.storage`.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['prometheus'].config-values | format_module_configuration: moduleKebabName }}

### Prometheus-мониторинг: custom resources
{{ site.data.schemas.prometheus.crds.clusteralerts | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.customalertmanager | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.customprometheusrules | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanaadditionaldatasources | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanaalertschannel | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.grafanadashboarddefinition | format_crd: "prometheus" }}
{{ site.data.schemas.prometheus.crds.prometheusremotewrite | format_crd: "prometheus" }}

### Prometheus-мониторинг: FAQ

{% raw %}

#### Как собирать метрики с приложений, расположенных вне кластера?

1. Сконфигурировать Service по аналогии с сервисом для [сбора метрик с вашего приложения](./monitoring-custom/#пример-service), но без указания параметра `spec.selector`.
1. Создать Endpoints для этого Service, явно указав в них `IP:PORT`, по которым ваши приложения отдают метрики.
> Важный момент: имена портов в Endpoints должны совпадать с именами этих портов в Service.

##### Пример

Метрики приложения доступны без TLS, по адресу `http://10.182.10.5:9114/metrics`.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: my-namespace
  labels:
    prometheus.deckhouse.io/custom-target: my-app
spec:
  ports:
  - name: http-metrics
    port: 9114
---
apiVersion: v1
kind: Endpoints
metadata:
  name: my-app
  namespace: my-namespace
subsets:
  - addresses:
    - ip: 10.182.10.5
    ports:
    - name: http-metrics
      port: 9114
```

#### Как добавить дополнительные dashboard'ы в вашем проекте?

Добавление пользовательских dashboard'ов для Grafana в Deckhouse реализовано с помощью подхода Infrastructure as a Code.
Чтобы ваш dashboard появился в Grafana, необходимо создать в кластере специальный ресурс — [`GrafanaDashboardDefinition`](cr.html#grafanadashboarddefinition).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # Папка, в которой в Grafana будет отображаться ваш dashboard.
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```

**Важно!** Системные и добавленные через [GrafanaDashboardDefinition](cr.html#grafanadashboarddefinition) dashboard'ы нельзя изменить через интерфейс Grafana.

#### Как добавить алерты и/или recording-правила для вашего проекта?

Для добавления алертов существует специальный ресурс — `CustomPrometheusRules`.

Параметры:
- `groups` — единственный параметр, в котором необходимо описать группы алертов. Структура групп полностью совпадает с аналогичной в prometheus-operator.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: my-rules
spec:
  groups:
  - name: cluster-state-alert.rules
    rules:
    - alert: CephClusterErrorState
      annotations:
        description: Storage cluster is in error state for more than 10m.
        summary: Storage cluster is in error state
        plk_markup_format: markdown
      expr: |
        ceph_health_status{job="rook-ceph-mgr"} > 1
```

##### Как подключить дополнительные data source для Grafana?

Для подключения дополнительных data source к Grafana существует специальный ресурс — `GrafanaAdditionalDatasource`.

Параметры ресурса подробно описаны в документации к Grafana. Тип ресурса смотрите в документации по конкретному datasource.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaAdditionalDatasource
metadata:
  name: another-prometheus
spec:
  type: prometheus
  access: Proxy
  url: https://another-prometheus.example.com/prometheus
  basicAuth: true
  basicAuthUser: foo
  jsonData:
    timeInterval: 30s
    httpMethod: POST
  secureJsonData:
    basicAuthPassword: bar
```

#### Как обеспечить безопасный доступ к метрикам?

Для обеспечения безопасности настоятельно рекомендуем использовать `kube-rbac-proxy`.

##### Пример безопасного сбора метрик с приложения, расположенного в кластере

Для настройки защиты метрик приложения с использованием `kube-rbac-proxy` и последующей сборки метрик с него средствами Prometheus выполните следующие шаги:

1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: rbac-proxy-test
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: d8:rbac-proxy
   subjects:
   - kind: ServiceAccount
     name: rbac-proxy-test
     namespace: default
   ```

   > Обратите внимание, что используется встроенная в Deckhouse ClusterRole `d8:rbac-proxy`.

2. Создайте конфигурацию для `kube-rbac-proxy`:

   ```yaml
   ---
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: rbac-proxy-config-test
     namespace: rbac-proxy-test
   data:
     config-file.yaml: |+
       authorization:
         resourceAttributes:
           namespace: default
           apiVersion: v1
           resource: services
           subresource: proxy
           name: rbac-proxy-test
   ```

   > Более подробную информацию по атрибутам можно найти в документации Kubernetes.

3. Создайте `Service` и `Deployment` для вашего приложения, где `kube-rbac-proxy` займет позицию sidecar-контейнера:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: rbac-proxy-test
     labels:
       prometheus.deckhouse.io/custom-target: rbac-proxy-test
   spec:
     ports:
     - name: https-metrics
       port: 8443
       targetPort: https-metrics
     selector:
       app: rbac-proxy-test
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: rbac-proxy-test
   spec:
     replicas: 1
     selector:
       matchLabels:
         app: rbac-proxy-test
     template:
       metadata:
         labels:
           app: rbac-proxy-test
       spec:
         securityContext:
           runAsUser: 65532
         serviceAccountName: rbac-proxy-test
         containers:
         - name: kube-rbac-proxy
           image: quay.io/brancz/kube-rbac-proxy:v0.14.0
           args:
           - "--secure-listen-address=0.0.0.0:8443"
           - "--upstream=http://127.0.0.1:8081/"
           - "--config-file=/kube-rbac-proxy/config-file.yaml"
           - "--logtostderr=true"
           - "--v=10"
           ports:
           - containerPort: 8443
             name: https-metrics
           volumeMounts:
           - name: config
             mountPath: /kube-rbac-proxy
         - name: prometheus-example-app
           image: quay.io/brancz/prometheus-example-app:v0.1.0
           args:
           - "--bind=127.0.0.1:8081"
         volumes:
         - name: config
           configMap:
             name: rbac-proxy-config-test
   ```

4. Назначьте необходимые права на ресурс для Prometheus:

   ```yaml
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: rbac-proxy-test-client
   rules:
   - apiGroups: [""]
     resources: ["services/proxy"]
     verbs: ["get"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: rbac-proxy-test-client
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: rbac-proxy-test-client
   subjects:
   - kind: ServiceAccount
     name: prometheus
     namespace: d8-monitoring
   ```

После шага 4 метрики вашего приложения должны появиться в Prometheus.

##### Пример безопасного сбора метрик с приложения, расположенного вне кластера

Предположим, что есть доступный через интернет сервер, на котором работает `node-exporter`. По умолчанию `node-exporter` слушает на порту `9100` и доступен на всех интерфейсах. Необходимо обеспечить контроль доступа к `node-exporter` для безопасного сбора метрик. Ниже приведен пример такой настройки.

Требования:
- Из кластера должен быть доступ до сервиса `kube-rbac-proxy`, запущенного на *удаленном сервере*.
- От *удаленного сервера* должен быть доступ до API-сервера кластера.

Выполните следующие шаги:
1. Создайте `ServiceAccount` с указанными ниже правами:

   ```yaml
   ---
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: prometheus-external-endpoint
   rules:
   - apiGroups: ["authentication.k8s.io"]
     resources:
     - tokenreviews
     verbs: ["create"]
   - apiGroups: ["authorization.k8s.io"]
     resources:
     - subjectaccessreviews
     verbs: ["create"]
   ---
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRoleBinding
   metadata:
     name: prometheus-external-endpoint-server-01
   roleRef:
     apiGroup: rbac.authorization.k8s.io
     kind: ClusterRole
     name: prometheus-external-endpoint
   subjects:
   - kind: ServiceAccount
     name: prometheus-external-endpoint-server-01
     namespace: d8-service-accounts
   ```

2. Сгенерируйте `kubeconfig` для созданного `ServiceAccount` ([пример генерации kubeconfig для `ServiceAccount`](./user-authz/usage.html#создание-serviceaccount-для-сервера-и-предоставление-ему-доступа)).

3. Положите получившийся `kubeconfig` на *удаленный сервер*. В дальнейшем понадобится указать путь к этому `kubeconfig` в настройках `kube-rbac-proxy` (в примере используется путь `${PWD}/.kube/config`).

4. Настройте `node-exporter` на *удаленном сервере*, чтобы он был доступен только на локальном интерфейсе (слушал `127.0.0.1:9100`).
5. Запустите `kube-rbac-proxy` на *удаленном сервере*:

   ```shell
   docker run --network host -d -v ${PWD}/.kube/config:/config quay.io/brancz/kube-rbac-proxy:v0.14.0 --secure-listen-address=0.0.0.0:8443 \
     --upstream=http://127.0.0.1:9100 --kubeconfig=/config --logtostderr=true --v=10
   ```

6. Проверьте, что порт `8443` доступен по внешнему адресу *удаленного сервера*.

7. Создайте в кластере `Service` и `Endpoint`, указав в качестве `<server_ip_address>` внешний адрес *удаленного сервера*:

   ```yaml
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: prometheus-external-endpoint-server-01
     labels:
       prometheus.deckhouse.io/custom-target: prometheus-external-endpoint-server-01
   spec:
     ports:
     - name: https-metrics
       port: 8443
   ---
   apiVersion: v1
   kind: Endpoints
   metadata:
     name: prometheus-external-endpoint-server-01
   subsets:
     - addresses:
       - ip: <server_ip_address>
       ports:
       - name: https-metrics
         port: 8443
   ```

#### Как добавить Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `Internal`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: webhook
spec:
  type: Internal
  internal:
    route:
      groupBy: ['job']
      groupWait: 30s
      groupInterval: 5m
      repeatInterval: 12h
      receiver: 'webhook'
    receivers:
    - name: 'webhook'
      webhookConfigs:
      - url: 'http://webhookserver:8080/'
```

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

#### Как добавить внешний дополнительный Alertmanager?

Создайте custom resource `CustomAlertmanager` с типом `External`, который может указывать на Alertmanager по FQDN или через сервис в Kubernetes-кластере.

Пример FQDN Alertmanager:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-fqdn-alertmanager
spec:
  external:
    address: https://alertmanager.mycompany.com/myprefix
  type: External
```

Пример Alertmanager с Kubernetes service:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: CustomAlertmanager
metadata:
  name: my-service-alertmanager
spec:
  external:
    service:
      namespace: myns
      name: my-alertmanager
      path: /myprefix/
  type: External
```

Подробно о всех параметрах можно прочитать в описании custom resource [CustomAlertmanager](cr.html#customalertmanager).

#### Как в Alertmanager игнорировать лишние алерты?

Решение сводится к настройке маршрутизации алертов в вашем Alertmanager.

Потребуется:

1. Завести получателя без параметров.
1. Смаршрутизировать лишние алерты в этого получателя.

Ниже приведены примеры настройки `CustomAlertmanager`.

Чтобы получать только алерты с лейблами `service: foo|bar|baz`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель  
  - name: some-other-receiver
    # ...
route:
  # receiver по умолчанию.
  receiver: blackhole
  routes:
    # Дочерний маршрут
    - matchers:
        - matchType: =~
          name: service
          value: ^(foo|bar|baz)$
      receiver: some-other-receiver
```

Чтобы получать все алерты, кроме `DeadMansSwitch`:

```yaml
receivers:
  # Получатель, определенный без параметров, будет работать как "/dev/null".
  - name: blackhole
  # Действующий получатель.
  - name: some-other-receiver
  # ...
route:
  # receiver по умолчанию.
  receiver: some-other-receiver
  routes:
    # Дочерний маршрут.
    - matchers:
        - matchType: =
          name: alertname
          value: DeadMansSwitch
      receiver: blackhole
```

С подробным описанием всех параметров можно ознакомиться в официальной документации.

#### Почему нельзя установить разный scrapeInterval для отдельных таргетов?

Наиболее полный ответ на этот вопрос дает разработчик Prometheus Brian Brazil.
Если коротко, разные scrapeInterval'ы принесут следующие проблемы:
* увеличение сложности конфигурации;
* проблемы при написании запросов и создании графиков;
* короткие интервалы больше похожи на профилирование приложения, и, скорее всего, Prometheus — не самый подходящий инструмент для этого.

Наиболее разумное значение для scrapeInterval находится в диапазоне 10–60 секунд.

#### Как ограничить потребление ресурсов Prometheus?

Чтобы избежать ситуаций, когда VPA запрашивает для Prometheus или Longterm Prometheus ресурсов больше, чем есть на выделенном для этого узле, можно явно ограничить VPA с помощью [параметров модуля](configuration.html):
- `vpa.longtermMaxCPU`;
- `vpa.longtermMaxMemory`;
- `vpa.maxCPU`;
- `vpa.maxMemory`.

#### Как настроить ServiceMonitor или PodMonitor для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Pod/Service Monitor.
Добавьте в namespace, в котором находится Pod/Service Monitor, лейбл `prometheus.deckhouse.io/monitor-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/monitor-watcher-enabled: "true"
---
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: example-app
  namespace: frontend
  labels:
    prometheus: main
spec:
  selector:
    matchLabels:
      app: example-app
  endpoints:
    - port: web
```

#### Как настроить Probe для работы с Prometheus?

Добавьте лейбл `prometheus: main` к Probe.
Добавьте в namespace, в котором находится Probe, лейбл `prometheus.deckhouse.io/probe-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/probe-watcher-enabled: "true"
---
apiVersion: monitoring.coreos.com/v1
kind: Probe
metadata:
  labels:
    app: prometheus
    component: probes
    prometheus: main
  name: cdn-is-up
  namespace: frontend
spec:
  interval: 30s
  jobName: httpGet
  module: http_2xx
  prober:
    path: /probe
    scheme: http
    url: blackbox-exporter.blackbox-exporter.svc.cluster.local:9115
  targets:
    staticConfig:
      static:
      - https://example.com/status
```

#### Как настроить PrometheusRules для работы с Prometheus?

Добавьте в namespace, в котором находятся PrometheusRules, лейбл `prometheus.deckhouse.io/rules-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/rules-watcher-enabled: "true"
```

#### Как увеличить размер диска

1. Для увеличения размера отредактируйте PersistentVolumeClaim, указав новый размер в поле `spec.resources.requests.storage`.
   * Увеличение размера возможно, если в StorageClass поле `allowVolumeExpansion` установлено в `true`.
2. Если используемое хранилище не поддерживает изменение диска на лету, в статусе PersistentVolumeClaim появится сообщение `Waiting for user to (re-)start a pod to finish file system resize of volume on node.`.
3. Перезапустите под для завершения изменения размера файловой системы.

#### Как получить информацию об алертах в кластере?

Информацию об активных алертах можно получить не только в веб-интерфейсе Grafana/Prometheus, но и в CLI. Это может быть полезным, если у вас есть только доступ к API-серверу кластера и нет возможности открыть веб-интерфейс Grafana/Prometheus.

Выполните следующую команду для получения списка алертов в кластере:

```shell
kubectl get clusteralerts
```

Пример:

```shell
### kubectl get clusteralerts
NAME               ALERT                                      SEVERITY   AGE     LAST RECEIVED   STATUS
086551aeee5b5b24   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
226d35c886464d6e   ExtendedMonitoringDeprecatatedAnnotation   4          3h25m   38s             firing
235d4efba7df6af4   D8SnapshotControllerPodIsNotReady          8          5d4h    44s             firing
27464763f0aa857c   D8PrometheusOperatorPodIsNotReady          7          5d4h    43s             firing
ab17837fffa5e440   DeadMansSwitch                             4          5d4h    41s             firing
```

Выполните следующую команду для просмотра конкретного алерта:

```shell
kubectl get clusteralerts <ALERT_NAME> -o yaml
```

Пример:

```shell
### kubectl get clusteralerts 235d4efba7df6af4 -o yaml
alert:
  description: |
    The recommended course of action:
    1. Retrieve details of the Deployment: `kubectl -n d8-snapshot-controller describe deploy snapshot-controller`
    2. View the status of the Pod and try to figure out why it is not running: `kubectl -n d8-snapshot-controller describe pod -l app=snapshot-controller`
  labels:
    pod: snapshot-controller-75bd776d76-xhb2c
    prometheus: deckhouse
    tier: cluster
  name: D8SnapshotControllerPodIsNotReady
  severityLevel: "8"
  summary: The snapshot-controller Pod is NOT Ready.
apiVersion: deckhouse.io/v1alpha1
kind: ClusterAlert
metadata:
  creationTimestamp: "2023-05-15T14:24:08Z"
  generation: 1
  labels:
    app: prometheus
    heritage: deckhouse
  name: 235d4efba7df6af4
  resourceVersion: "36262598"
  uid: 817f83e4-d01a-4572-8659-0c0a7b6ca9e7
status:
  alertStatus: firing
  lastUpdateTime: "2023-05-15T18:10:09Z"
  startsAt: "2023-05-10T13:43:09Z"
```

Помните о специальном алерте `DeadMansSwitch` — его присутствие в кластере говорит о работоспособности Prometheus.

#### Как добавить дополнительные эндпоинты в scrape config?

Добавьте в namespace, в котором находится ScrapeConfig, лейбл `prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"`.

Пример:

```yaml
---
apiVersion: v1
kind: Namespace
metadata:
  name: frontend
  labels:
    prometheus.deckhouse.io/scrape-configs-watcher-enabled: "true"
```

Добавьте ScrapeConfig, который имеет обязательный лейбл `prometheus: main`:

```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: ScrapeConfig
metadata:
  name: example-scrape-config
  namespace: frontend
  labels:
    prometheus: main
spec:
  honorLabels: true
  staticConfigs:
    - targets: ['example-app.frontend.svc.{{ .Values.global.discovery.clusterDomain }}.:8080']
  relabelings:
    - regex: endpoint|namespace|pod|service
      action: labeldrop
    - targetLabel: scrape_endpoint
      replacement: main
    - targetLabel: job
      replacement: kube-state-metrics
  metricsPath: '/metrics'
```

{% endraw %}
## Подсистема Безопасность

### Модуль admission-policy-engine

Позволяет использовать в кластере политики безопасности согласно Pod Security Standards Kubernetes. Модуль для работы использует Gatekeeper.

Pod Security Standards определяют три политики, охватывающие весь спектр безопасности. Эти политики являются кумулятивными, то есть состоящими из набора политик, и варьируются по уровню ограничений от «неограничивающего» до «ограничивающего значительно».

{% alert level="info" %}
Модуль не применяет политики к системным пространствам имен.
{% endalert %}

Список политик, доступных для использования:
- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений;
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

Подробнее про каждый набор политик и их ограничения можно прочитать в документации Kubernetes.

Политика кластера используемая по умолчанию определяется следующим образом:
- При установке Deckhouse версии **ниже v1.55**, для всех несистемных пространств имен используется политика по умолчанию `Privileged`;
- При установке Deckhouse версии **v1.55 и выше**, для всех несистемных пространств имен используется политика по умолчанию `Baseline`;

**Обратите внимание,** что обновление Deckhouse в кластере на версию v1.55 не вызывает автоматической смены политики по умолчанию.

Политику по умолчанию можно переопределить как глобально ([в настройках модуля](configuration.html#parameters-podsecuritystandards-defaultpolicy)), так и для каждого пространства имен отдельно (лейбл `security.deckhouse.io/pod-policy=<POLICY_NAME>` на соответствующем пространстве имен).

Пример установки политики `Restricted` для всех подов в пространстве имен `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy=restricted
```

По умолчанию, политики Pod Security Standards применяются в режиме "Deny" и поды приложений, не удовлетворяющие данным политикам, не смогут быть запущены. Режим работы политик может быть задан как глобально для кластера так и для каждого namespace отдельно. Что бы задать режим работы политик глобально используйте [configuration](configuration.html#parameters-podsecuritystandards-enforcementaction). В случае если необходимо переопределить глобальный режим политик для определенного namespace, допускается использовать лейбл `security.deckhouse.io/pod-policy-action =<POLICY_ACTION>` на соответствующем namespace. Список допустимых режимом политик состоит из: "dryrun", "warn", "deny".

Пример установки "warn" режима политик PSS для всех подов в пространстве имен `my-namespace`:

```bash
kubectl label ns my-namespace security.deckhouse.io/pod-policy-action=warn
```

Предлагаемые модулем политики могут быть расширены. Примеры расширения политик можно найти в [FAQ](faq.html).

##### Операционные политики

Модуль предоставляет набор операционных политик и лучших практик для безопасной работы ваших приложений.
Мы рекомендуем устанавливать следующий минимальный набор операционных политик:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: OperationPolicy
metadata:
  name: common
spec:
  policies:
    allowedRepos:
      - myrepo.example.com
      - registry.deckhouse.io
    requiredResources:
      limits:
        - memory
      requests:
        - cpu
        - memory
    disallowedImageTags:
      - latest
    requiredProbes:
      - livenessProbe
      - readinessProbe
    maxRevisionHistoryLimit: 3
    imagePullPolicy: Always
    priorityClassNames:
    - production-high
    - production-low
    checkHostNetworkDNSPolicy: true
    checkContainerDuplicates: true
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          operation-policy.deckhouse.io/enabled: "true"
```

Для применения приведенной политики достаточно навесить лейбл `operation-policy.deckhouse.io/enabled: "true"` на желаемый namespace. Политика, приведенная в примере, рекомендована для использования командой Deckhouse. Аналогичным образом вы можете создать собственную политику с необходимыми настройками.

##### Политики безопасности

Модуль предоставляет возможность определять политики безопасности применимо к приложениям (контейнерам), запущенным в кластере.

Пример политики безопасности:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: mypolicy
spec:
  enforcementAction: Deny
  policies:
    allowHostIPC: true
    allowHostNetwork: true
    allowHostPID: false
    allowPrivileged: false
    allowPrivilegeEscalation: false
    allowedFlexVolumes:
    - driver: vmware
    allowedHostPorts:
    - max: 4000
      min: 2000
    allowedProcMount: Unmasked
    allowedAppArmor:
    - unconfined
    allowedUnsafeSysctls:
    - kernel.*
    allowedVolumes:
    - hostPath
    - projected
    fsGroup:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - ALL
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 200
        min: 100
      rule: MustRunAs
    seccompProfiles:
      allowedLocalhostFiles:
      - my_profile.json
      allowedProfiles:
      - Localhost
    supplementalGroups:
      ranges:
      - max: 133
        min: 129
      rule: MustRunAs
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          enforce: mypolicy
```

Для применения приведенной политики достаточно навесить лейбл `enforce: "mypolicy"` на желаемый namespace.

##### Изменение ресурсов Kubernetes

Модуль позволяет использовать [кастомные ресурсы Gatekeeper](gatekeeper-cr.html) для модификации объектов в кластере, такие как:
- [AssignMetadata](gatekeeper-cr.html#assignmetadata) — для изменения секции `metadata` в ресурсе;
- [Assign](gatekeeper-cr.html#assign) — для изменения других полей, кроме `metadata`;
- [ModifySet](gatekeeper-cr.html#modifyset) — для добавления или удаления значений из списка, например аргументов для запуска контейнера.
- [AssignImage](gatekeeper-cr.html#assignimage) — для изменения параметра `image` ресурса.

Подробнее про доступные варианты можно прочитать в документации Gatekeeper.

### Модуль admission-policy-engine: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['admission-policy-engine'].config-values | format_module_configuration: moduleKebabName }}

### Модуль admission-policy-engine: custom resources
{{ site.data.schemas.admission-policy-engine.crds.native.assign-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.assignimage-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.assignmetadata-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.config-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constraintpodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constrainttemplate-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constrainttemplatepodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.expansiontemplate-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.expansiontemplatepodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.modifyset-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.mutatorpodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.provider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.operation-policy | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.certificatestore-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.keymanagementprovider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedkeymanagementprovider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedpolicy-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedstore-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedverifier-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.policy-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.store-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.verifier-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.security-policy | format_crd: "admission-policy-engine" }}

### Модуль admission-policy-engine: Custom Resources (от Gatekeeper)
{{ site.data.schemas.admission-policy-engine.crds.native.assign-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.assignimage-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.assignmetadata-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.config-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constraintpodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constrainttemplate-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.constrainttemplatepodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.expansiontemplate-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.expansiontemplatepodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.modifyset-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.mutatorpodstatus-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.native.provider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.operation-policy | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.certificatestore-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.keymanagementprovider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedkeymanagementprovider-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedpolicy-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedstore-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.namespacedverifier-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.policy-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.store-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.ratify.verifier-customresourcedefinition | format_crd: "admission-policy-engine" }}
{{ site.data.schemas.admission-policy-engine.crds.security-policy | format_crd: "admission-policy-engine" }}

### Модуль admission-policy-engine: FAQ

#### Как настроить альтернативные решения по управлению политиками безопасности?

Для корректной работы DKP необходимы расширенные привилегии на запуск и работу полезной нагрузки системных компонентов. Если вместо модуля admission-policy-engine используется альтернативное решение по управлению политиками безопасности (например, Kyverno), необходима настройка исключений для следующих пространств имен:
- `kube-system`;
- все пространства имен с префиксом `d8-*` (например, `d8-system`).

#### Как расширить политики Pod Security Standards?

> Pod Security Standards реагируют на label `security.deckhouse.io/pod-policy: restricted` или `security.deckhouse.io/pod-policy: baseline`.

Чтобы расширить политику Pod Security Standards, добавив к существующим проверкам политики свои собственные, необходимо:
- создать шаблон проверки (ресурс `ConstraintTemplate`);
- привязать его к политике `restricted` или `baseline`.

Пример шаблона для проверки адреса репозитория образа контейнера:

```yaml
apiVersion: templates.gatekeeper.sh/v1
kind: ConstraintTemplate
metadata:
  name: k8sallowedrepos
spec:
  crd:
    spec:
      names:
        kind: K8sAllowedRepos
      validation:
        openAPIV3Schema:
          type: object
          properties:
            repos:
              type: array
              items:
                type: string
  targets:
    - target: admission.k8s.gatekeeper.sh
      rego: |
        package d8.pod_security_standards.extended

        violation[{"msg": msg}] {
          container := input.review.object.spec.containers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }

        violation[{"msg": msg}] {
          container := input.review.object.spec.initContainers[_]
          satisfied := [good | repo = input.parameters.repos[_] ; good = startswith(container.image, repo)]
          not any(satisfied)
          msg := sprintf("container <%v> has an invalid image repo <%v>, allowed repos are %v", [container.name, container.image, input.parameters.repos])
        }
```

Пример привязки проверки к политике `restricted`:

```yaml
apiVersion: constraints.gatekeeper.sh/v1beta1
kind: K8sAllowedRepos
metadata:
  name: prod-repo
spec:
  match:
    kinds:
      - apiGroups: [""]
        kinds: ["Pod"]
    namespaceSelector:
      matchLabels:
        security.deckhouse.io/pod-policy: restricted
  parameters:
    repos:
      - "mycompany.registry.com"
```

Пример демонстрирует настройку проверки адреса репозитория в поле `image` у всех подов, создающихся в пространстве имен, имеющих label `security.deckhouse.io/pod-policy: restricted`. Если адрес в поле `image` создаваемого пода начинается не с `mycompany.registry.com`, под создан не будет.

Подробнее о шаблонах и языке политик можно узнать в документации Gatekeeper.

Больше примеров описания проверок для расширения политики можно найти в библиотеке Gatekeeper.

#### Что, если несколько политик (операционных или безопасности) применяются на один объект?

В таком случае необходимо, чтобы конфигурация объекта соответствовала всем политикам, которые на него распространяются.

Например, рассмотрим две следующие политики безопасности:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    readOnlyRootFilesystem: true
    requiredDropCapabilities:
    - MKNOD
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: bar
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          name: test
  policies:
    requiredDropCapabilities:
    - NET_BIND_SERVICE
```

Тогда для выполнения требований приведенных политик безопасности в спецификации контейнера нужно указать:

```yaml
    securityContext:
      capabilities:
        drop:
          - MKNOD
          - NET_BIND_SERVICE
      readOnlyRootFilesystem: true
```

#### Проверка подписи образов

В модуле реализована функция проверки подписи образов контейнеров, подписанных с помощью инструмента Cosign. Проверка подписи образов контейнеров позволяет убедиться в их целостности (что образ не был изменен после его создания) и подлинности (что образ был создан доверенным источником). Включить проверку подписи образов контейнеров в кластере можно с помощью параметра [policies.verifyImageSignatures](cr.html#securitypolicy-v1alpha1-spec-policies-verifyimagesignatures) ресурса SecurityPolicy.

{% offtopic title="Как подписать образ..." %}
Шаги для подписания образа:
- Сгенерируйте ключи: `cosign generate-key-pair`
- Подпишите образ: `cosign sign --key <key> <image>`

Подробнее о работе с Cosign можно узнать в документации.
{% endofftopic %}

Пример SecurityPolicy для настройки проверки подписи образов контейнеров:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: verify-image-signatures
spec:
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          kubernetes.io/metadata.name: default
  policies:
    verifyImageSignatures:
      - reference: docker.io/myrepo/*
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
      - reference: company.registry.com/*
        dockerCfg: zxc==
        publicKeys:
        - |-
          -----BEGIN PUBLIC KEY-----
          MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE8nXRh950IZbRj8Ra/N9sbqOPZrfM
          5/KAQN0/KjHcorm/J5yctVd7iEcnessRQjU917hmKO6JWVGHpDguIyakZA==
          -----END PUBLIC KEY-----
```

Политика не влияет на создание подов, адреса образов контейнеров которых не подходят под описанные в параметре `reference`.  Если же адрес какого-либо образа контейнера подходит под описанные в параметре `reference` политики, и образ не подписан или подпись не соответствует указанным в политике ключам, создание пода будет запрещено.

Пример вывода ошибки при создании пода с образом контейнера, не прошедшим проверку подписи:

```console
[verify-image-signatures] Image signature verification failed: nginx:1.17.2
```

### Модуль cert-manager

Устанавливает надежную и высокодоступную инсталляцию cert-manager release v1.16.1.

При установке модуля автоматически учитываются особенности кластера:
- компонент (webhook), к которому обращается `kube-apiserver`, устанавливается на master-узлы;
- в случае недоступности webhook'а производится временное удаление `apiservice`, чтобы недоступность *cert-manager* не блокировала работу кластера.

Обновление самого модуля происходит в автоматическом режиме, в том числе с миграцией ресурсов cert-manager.

#### Возможности модуля cert-manager (с учетом внесенных изменений)

Модуль обеспечивает использование всех возможностей оригинального cert-manager, в том числе:
- заказ сертификатов во всех поддерживаемых источниках, таких как *Let’s Encrypt*, *HashiCorp Vault*, *Venafi*;
- выпуск самоподписанных сертификатов;
- поддержку актуальности сертификатов, автоматический перевыпуск и т. д.

Изменения в оригинальный cert-manager были внесены, чтобы поды `cm-acme-http-solver` могли выполняться на master-узлах и выделенных узлах.

#### Мониторинг

Модуль обеспечивает экспорт метрик в Prometheus, что позволяет мониторить:
- срок действия сертификатов;
- корректность перевыпуска сертификатов.

#### Роли доступа к ресурсам

В модуле предопределены несколько продуманных ролей для удобного доступа к ресурсам:
- `User` – доступ на чтение к ресурсам Certificate и Issuer в доступных ему namespace, а также к глобальным ClusterIssue;
- `Editor` – управление ресурсами Certificate и Issuer в доступных ему namespace;
- `ClusterEditor` – управление ресурсами Certificate и Issuer в любых namespace;
- `SuperAdmin` – управление внутренними служебными объектами.

### Модуль cert-manager: настройки

У модуля нет обязательных настроек.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['cert-manager'].config-values | format_module_configuration: moduleKebabName }}

### Модуль cert-manager: custom resources
{{ site.data.schemas.cert-manager.crds.crd-certificaterequests | format_crd: "cert-manager" }}
{{ site.data.schemas.cert-manager.crds.crd-certificates | format_crd: "cert-manager" }}
{{ site.data.schemas.cert-manager.crds.crd-challenges | format_crd: "cert-manager" }}
{{ site.data.schemas.cert-manager.crds.crd-clusterissuers | format_crd: "cert-manager" }}
{{ site.data.schemas.cert-manager.crds.crd-issuers | format_crd: "cert-manager" }}
{{ site.data.schemas.cert-manager.crds.crd-orders | format_crd: "cert-manager" }}

### Модуль cert-manager: FAQ


#### Какие виды сертификатов поддерживаются?

На данный момент модуль устанавливает следующие `ClusterIssuer`:
* `letsencrypt`
* `letsencrypt-staging`
* `selfsigned`
* `selfsigned-no-trust`

Если требуется поддержка других типов сертификатов, вы можете добавить их самостоятельно.

#### Как добавить дополнительный `ClusterIssuer`?

##### В каких случаях требуется дополнительный `ClusterIssuer`?

В стандартной поставке присутствуют `ClusterIssuer`, издающие либо сертификаты из доверенного публичного удостоверяющего центра Let's Encrypt, либо самоподписанные сертификаты.

Чтобы издать сертификаты на доменное имя через Let's Encrypt, сервис требует осуществить подтверждение владения доменом.
`Cert-manager` поддерживает несколько методов для такого подтверждения при использовании `ACME`(Automated Certificate Management Environment):
* `HTTP-01` — `cert-manager` создаст временный Pod в кластере, который будет слушать на определенном URL для подтверждения владения доменом. Для его работы необходимо иметь возможность направлять внешний трафик на этот Pod, обычно через `Ingress`.
* `DNS-01` —  `cert-manager` делает TXT-запись в DNS для подтверждения владения доменом. У `cert-manager` есть встроенная поддержка популярных провайдеров DNS: AWS Route53, Google Cloud DNS, Cloudflare и т.д. Полный перечень доступен в документации cert-manager.

{% alert level="danger" %}
Метод `HTTP-01` не поддерживает выпуск wildcard сертификатов.
{% endalert %}

Поставляемые `ClusterIssuers`, издающие сертификаты через Let's Encrypt, делятся на два типа:
1. `ClusterIssuer,` специфичные для используемого cloud-провайдера.  
Добавляются автоматически, при заполнении [настроек модуля](./configuration.html) связанных с cloud-провайдером. Поддерживают метод `DNS-01`.
   * `clouddns`
   * `cloudflare`
   * `digitalocean`
   * `route53`
1. `ClusterIssuer` использующие метод `HTTP-01`.  
   Добавляются автоматически, если их создание не отключено в [настройках модуля](./configuration.html#parameters-disableletsencrypt).
   * `letsencrypt`
   * `letsencrypt-staging`

Таким образом, дополнительный `ClusterIssuer` может потребоваться в случаях издания сертификатов:
1. В удостоверяющем центре (УЦ), отличном от Let's Encrypt (в т.ч. в приватном). Поддерживаемые УЦ доступны в документации `cert-manager`
2. Через Let's Encrypt с помощью метода `DNS-01` через сторонний провайдер.

##### Как добавить дополнительный `ClusterIssuer`, использующий Let's Encrypt и метод подтверждения `DNS-01`?

Для подтверждения владения доменом через Let's Encrypt с помощью метода `DNS-01` требуется настроить возможность создания TXT-записей в публичном DNS.

У `cert-manager` есть поддержка механизмов для создания TXT-записей в некоторых популярных DNS: `AzureDNS`, `Cloudflare`, `Google Cloud DNS` и т.д.  
Полный перечень доступен в документации `cert-manager`.

Пример использования AWS Route53 доступен в разделе [Как защитить учетные данные `cert-manager`](#как-защитить-учетные-данные-cert-manager).  
Актуальный перечень всех возможных для создания `ClusterIssuer` доступен в шаблонах модуля.

Использование сторонних DNS-провайдеров реализуется через метод `webhook`.  
Когда `cert-manager` выполняет вызов `ACME` `DNS01`, он отправляет запрос на вебхук-сервер, который затем выполняет нужные операции для обновления записи DNS.  
При использовании данного метода требуется разместить сервис, который будет обрабатывать хук и осуществлять создание TXT-записи в DNS-провайдере.

В качестве примера рассмотрим использование сервиса `Yandex Cloud DNS`.

1. Для обработки вебхука предварительно разместите в кластере сервис `Yandex Cloud DNS ACME webhook` согласно официальной документации.

1. Затем создайте ресурс `ClusterIssuer`:

   ```yaml
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: yc-clusterissuer
     namespace: default
   spec:
     acme:
       # Вы должны заменить этот адрес электронной почты на свой собственный.
       # Let's Encrypt будет использовать его, чтобы связаться с вами по поводу истекающих
       # сертификатов и вопросов, связанных с вашей учетной записью.
       email: your@email.com
       server: https://acme-staging-v02.api.letsencrypt.org/directory
       privateKeySecretRef:
         # Ресурс секретов, который будет использоваться для хранения закрытого ключа аккаунта.
         name: secret-ref
       solvers:
         - dns01:
             webhook:
               config:
                 # Идентификатор папки, в которой расположена DNS-зона
                 folder: <your folder ID>
                 # Это секрет, используемый для доступа к учетной записи сервиса
                 serviceAccountSecretRef:
                   name: cert-manager-secret
                   key: iamkey.json
               groupName: acme.cloud.yandex.com
               solverName: yandex-cloud-dns
   ```

##### Как добавить дополнительный `Issuer` и `ClusterIssuer`, использующий HashiСorp Vault для выпуска сертификатов?

Для выпуска сертификатов с помощью HashiСorp Vault, можете использовать инструкцию.

После конфигурации PKI и [включения авторизации](./modules/user-authz/) в Kubernetes, нужно:
- Создать `ServiceAccount` и скопировать ссылку на его `Secret`:

  ```shell
  kubectl create serviceaccount issuer
  
  ISSUER_SECRET_REF=$(kubectl get serviceaccount issuer -o json | jq -r ".secrets[].name")
  ```

- Создать `Issuer`:

  ```shell
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: Issuer
  metadata:
    name: vault-issuer
    namespace: default
  spec:
    vault:
      # Если Vault разворачивался по вышеуказанной инструкции, в этом месте в инструкции опечатка.
      server: http://vault.default.svc.cluster.local:8200
      # Указывается на этапе конфигурации PKI. 
      path: pki/sign/example-dot-com 
      auth:
        kubernetes:
          mountPath: /v1/auth/kubernetes
          role: issuer
          secretRef:
            name: $ISSUER_SECRET_REF
            key: token
  EOF
  ```

- Создать ресурс `Certificate` для получения TLS-сертификата, подписанного CA Vault:

  ```shell
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: Certificate
  metadata:
    name: example-com
    namespace: default
  spec:
    secretName: example-com-tls
    issuerRef:
      name: vault-issuer
    # Домены указываются на этапе конфигурации PKI в Vault.
    commonName: www.example.com 
    dnsNames:
    - www.example.com
  EOF
  ```

##### Как добавить `ClusterIssuer`, использующий свой или промежуточный CA для заказа сертификатов?

Для использования собственного или промежуточного CA:

- Сгенерируйте сертификат (при необходимости):

  ```shell
  openssl genrsa -out rootCAKey.pem 2048
  openssl req -x509 -sha256 -new -nodes -key rootCAKey.pem -days 3650 -out rootCACert.pem
  ```

- В пространстве имён `d8-cert-manager` создайте секрет, содержащий данные файлов сертификатов.
  Пример создания секрета с помощью команды kubectl:  

  ```shell
  kubectl create secret tls internal-ca-key-pair -n d8-cert-manager --key="rootCAKey.pem" --cert="rootCACert.pem"
  ```

  Пример создания секрета из YAML-файла (содержимое файлов сертификатов должно быть закодировано в Base64):  

  ```yaml
  apiVersion: v1
  data:
    tls.crt: <результат команды `cat rootCACert.pem | base64 -w0`>
    tls.key: <результат команды `cat rootCAKey.pem | base64 -w0`>
  kind: Secret
  metadata:
    name: internal-ca-key-pair
    namespace: d8-cert-manager
  type: Opaque
  ```

  Имя секрета может быть любым.

- Создайте `ClusterIssuer` из созданного секрета:

  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: inter-ca
  spec:
    ca:
      secretName: internal-ca-key-pair    # Имя созданного секрета.
  ```

  Имя `ClusterIssuer` также может быть любым.

Теперь можно использовать созданный `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse или конкретного компонента.

Например, чтобы использовать `ClusterIssuer` для получения сертификатов для всех компонентов Deckhouse, укажите его имя в глобальном параметре [clusterIssuerName](./deckhouse-configure-global.html#parameters-modules-https-certmanager-clusterissuername) (`kubectl edit mc global`):

  ```yaml
  spec:
    settings:
      modules:
        https:
          certManager:
            clusterIssuerName: inter-ca
          mode: CertManager
        publicDomainTemplate: '%s.<public_domain_template>'
    version: 1
  ```

#### Как защитить учетные данные `cert-manager`?

Если вы не хотите хранить учетные данные конфигурации Deckhouse (например, по соображениям безопасности), можете создать
свой собственный `ClusterIssuer` / `Issuer`.

Пример создания собственного `ClusterIssuer` для сервиса route53:
- Создайте Secret с учетными данными:

  ```shell
  kubectl apply -f - <<EOF
  apiVersion: v1
  kind: Secret
  type: Opaque
  metadata:
    name: route53
    namespace: default
  data:
    secret-access-key: {{ "MY-AWS-ACCESS-KEY-TOKEN" | b64enc | quote }}
  EOF
  ```

- Создайте простой `ClusterIssuer` со ссылкой на этот Secret:

  ```shell
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: route53
    namespace: default
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      privateKeySecretRef:
        name: route53-tls-key
      solvers:
      - dns01:
          route53:
            region: us-east-1
            accessKeyID: {{ "MY-AWS-ACCESS-KEY-ID" }}
            secretAccessKeySecretRef:
              name: route53
              key: secret-access-key
  EOF
  ```

- Закажите сертификаты как обычно, используя созданный `ClusterIssuer`:

  ```shell
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: Certificate
  metadata:
    name: example-com
    namespace: default
  spec:
    secretName: example-com-tls
    issuerRef:
      name: route53
    commonName: www.example.com 
    dnsNames:
    - www.example.com
  EOF
  ```

#### Работает ли старая аннотация TLS-acme?

Да, работает. Специальный компонент `cert-manager-ingress-shim` видит эти аннотации и на их основании автоматически создает ресурсы `Certificate` (в тех же namespaces, что и Ingress-ресурсы с аннотациями).

> **Важно!** При использовании аннотации ресурс Certificate создается «прилинкованным» к существующему Ingress-ресурсу, и для прохождения Challenge НЕ создается отдельный Ingress, а вносятся дополнительные записи в существующий. Это означает, что если на основном Ingress'е настроена аутентификация или whitelist — ничего не выйдет. Лучше не использовать аннотацию и переходить на ресурс Certificate.
>
> **Важно!** При переходе с аннотации на ресурс Certificate нужно удалить ресурс Certificate, который был создан по аннотации. Иначе по обоим ресурсам Certificate будет обновляться один Secret, и это может привести к достижению лимита запросов Let’s Encrypt.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    kubernetes.io/tls-acme: "true"           # Аннотация.
  name: example-com
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - host: example.com
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: www.example.com                    # Дополнительный домен.
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  - host: admin.example.com                  # Еще один дополнительный домен.
    http:
      paths:
      - backend:
          service:
            name: site
            port:
              number: 80
        path: /
        pathType: ImplementationSpecific
  tls:
  - hosts:
    - example.com
    - www.example.com                        # Дополнительный домен.
    - admin.example.com                      # Еще один дополнительный домен.
    secretName: example-com-tls              # Имя для Certificate и Secret.
```

#### Как посмотреть состояние сертификата?

```shell
kubectl -n default describe certificate example-com
...
Status:
  Acme:
    Authorizations:
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/qJA9MGCZnUnVjAgxhoxONvDnKAsPatRILJ4n0lJ7MMY/4062050823
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   admin.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/pW2tFKLBDTll2Gx8UBqmEl846x5W-YpBs8a4HqstJK8/4062050808
      Account:  https://acme-v01.api.letsencrypt.org/acme/reg/22442061
      Domain:   www.example.com
      Uri:      https://acme-v01.api.letsencrypt.org/acme/challenge/LaZJMM9_OKcTYbEThjT3oLtwgpkNfbHVdl8Dz-yypx8/4062050792
  Conditions:
    Last Transition Time:  2018-04-02T18:01:04Z
    Message:               Certificate issued successfully
    Reason:                CertIssueSuccess
    Status:                True
    Type:                  Ready
Events:
  Type     Reason                 Age                 From                     Message
  ----     ------                 ----                ----                     -------
  Normal   PrepareCertificate     1m                cert-manager-controller  Preparing certificate with issuer
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain www.example.com
  Normal   PresentChallenge       1m                cert-manager-controller  Presenting http-01 challenge for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain admin.example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain example.com
  Normal   SelfCheck              1m                cert-manager-controller  Performing self-check for domain www.example.com
  Normal   ObtainAuthorization    55s               cert-manager-controller  Obtained authorization for domain example.com
  Normal   ObtainAuthorization    54s               cert-manager-controller  Obtained authorization for domain admin.example.com
  Normal   ObtainAuthorization    53s               cert-manager-controller  Obtained authorization for domain www.example.com
```

#### Как получить список сертификатов?

```shell
kubectl get certificate --all-namespaces

NAMESPACE          NAME                            AGE
default            example-com                     13m
```

#### Что делать, если появляется ошибка: CAA record does not match issuer?

Если `cert-manager` не может заказать сертификаты с ошибкой:

```text
CAA record does not match issuer
```

то необходимо проверить `CAA (Certificate Authority Authorization)` DNS-запись у домена, для которого заказывается сертификат.
Если вы хотите использовать Let’s Encrypt-сертификаты, у домена должна быть CAA-запись: `issue "letsencrypt.org"`.
Подробнее про CAA можно почитать в документации Let’s Encrypt.

### Модуль multitenancy-manager

#### Описание

Модуль позволяет создавать проекты в кластере Kubernetes. **Проект** — это изолированное окружение, в котором можно развернуть приложения.

#### Для чего это нужно?

Стандартный ресурс `Namespace`, который используется для логического разделения ресурсов в Kubernetes, не предоставляет необходимых функций, поэтому не является изолированным окружением:
* Потребление ресурсов подами по умолчанию не ограничено;
* Сетевое взаимодействие с другими подами по умолчанию работает из любой точки кластера;
* Неограниченный доступ к ресурсам узла: адресное пространство, сетевое пространство, смонтированные директории хоста.

Возможности настройки пространств имен `Namespace` не полностью соответствуют современным требованиям к разработке. По умолчанию для `Namespace` не включены следующие функции:
* Сборка логов;
* Аудит;
* Сканирование уязвимостей.

Функционал проектов позволяет решить эти проблемы.

#### Преимущества модуля

Для администраторов платформы:
* **Единообразие**: Администраторы могут создавать проекты, используя один и тот же шаблон, что обеспечивает единообразие и упрощает управление.
* **Безопасность**: Проекты обеспечивают изоляцию ресурсов и политик доступа между различными проектами, что поддерживает безопасное многотенантное окружение.
* **Потребление ресурсов**: Администраторы могут легко устанавливать квоты на ресурсы и ограничения для каждого проекта, предотвращая избыточное использование ресурсов.

Для пользователей платформы:
* **Быстрый старт**: Разработчики могут запрашивать у администраторов проекты, созданные по готовым шаблонам, что позволяет быстро начать разработку нового приложения.
* **Изоляция**: Каждый проект обеспечивает изолированное окружение, где разработчики могут развертывать и тестировать свои приложения без влияния на другие проекты.

#### Внутренняя логика работы

##### Создание проекта

Для создания проекта используются ресурсы:
* [ProjectTemplate](cr.html#projecttemplate) — ресурс, который описывает шаблон проекта. При помощи него задается список ресурсов, которые будут созданы в проекте, а также схема параметров, которые можно передать при создании проекта;
* [Project](cr.html#project) — ресурс, который описывает конкретный проект.

При создании ресурса [Project](cr.html#project) из определенного [ProjectTemplate](cr.html#projecttemplate) происходит следующее:
1. Переданные [параметры](cr.html#project-v1alpha2-spec-parameters) валидируются по OpenAPI-спецификации (параметр [openAPI](cr.html#projecttemplate-v1alpha1-spec-parametersschema) ресурса [ProjectTemplate](cr.html#projecttemplate));
1. Выполняется рендеринг [шаблона для ресурсов](cr.html#projecttemplate-v1alpha1-spec-resourcestemplate) с помощью Helm. Значения для рендеринга берутся из параметра [parameters](cr.html#project-v1alpha2-spec-parameters) ресурса [Project](cr.html#project);
1. Cоздается `Namespace` с именем, которое совпадает c именем [Project](cr.html#project);
1. По очереди создаются все ресурсы, описанные в шаблоне.

> **Внимание!** При изменении шаблона проекта, все созданные проекты будут обновлены в соответствии с новым шаблоном.

##### Изоляция проекта

В основе проекта используется механизм изоляции ресурсов в рамках пространства имен (`Namespace`).
Пространства имен позволяют группировать поды, сервисы, секреты и другие объекты, но не обеспечивают полноценной изоляции.
Проект расширяет функциональность пространств имен, предлагая дополнительные инструменты для повышения уровня контроля и безопасности.
Для управления уровнем изоляции проекта можно использовать возможности Kubernetes, например:

- Ресурсы контроля доступа (`AuthorizationRule` / `RoleBinding`) — позволяют управлять взаимодействием объектов внутри `Namespace`. Вы можете задавать правила и назначать роли, чтобы точно контролировать, кто и что может делать в вашем проекте.
- Ресурсы контроля использования нагрузки (`ResourceQuota`) — с их помощью можно задать лимиты на использование процессорного времени (CPU), оперативной памяти (RAM), а также количества объектов внутри `Namespace`. Это помогает избежать чрезмерной нагрузки и обеспечивает мониторинг за приложениями в рамках проекта.
- Ресурсы контроля сетевой связности (`NetworkPolicy`) — управляют входящим и исходящим сетевым трафиком в `Namespace`. Таким образом, можно настроить разрешенные подключения между подами, улучшить безопасность и управляемость сетевого взаимодействия в рамках проекта.

Эти инструменты можно комбинировать, чтобы настроить проект в соответствии с требованиями вашего приложения.

### Модуль multitenancy-manager: настройки

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

У модуля нет обязательных настроек.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['multitenancy-manager'].config-values | format_module_configuration: moduleKebabName }}

### Модуль multitenancy-manager: Custom Resources
{{ site.data.schemas.multitenancy-manager.crds.projects | format_crd: "multitenancy-manager" }}
{{ site.data.schemas.multitenancy-manager.crds.projecttemplate | format_crd: "multitenancy-manager" }}

### Модуль operator-trivy

Модуль позволяет запускать периодическое сканирование на уязвимости. Базируется на проекте Trivy.

Модуль каждые 24 часа выполняет сканирование в пространствах имён, которые содержат метку `security-scanning.deckhouse.io/enabled=""`.
Если в кластере отсутствуют пространства имён с указанной меткой, сканируется пространство имён `default`.

Как только в кластере обнаруживается пространство имён с меткой `security-scanning.deckhouse.io/enabled=""`, сканирование пространства имён `default` прекращается.
Чтобы снова включить сканирование для пространства имён `default`, необходимо установить у него метку командой:

```shell
kubectl label namespace default security-scanning.deckhouse.io/enabled=""
```

### Модуль operator-trivy: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['modules'].config-values | format_module_configuration: moduleKebabName }}

### Модуль operator-trivy: FAQ
{% raw %}

#### Как посмотреть все ресурсы, которые не прошли CIS compliance-проверки?

```bash
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq '.status.detailReport.results | map(select(.checks | map(.success) | all | not))'
```

#### Как посмотреть ресурсы, которые не прошли конкретную CIS compliance-проверку?

По `id`:

```bash
check_id="5.7.3"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_id "$check_id" '.status.detailReport.results | map(select(.id == $check_id))'
```

По описанию:

```bash
check_desc="Apply Security Context to Your Pods and Containers"
kubectl get clustercompliancereports.aquasecurity.github.io cis -ojson |
  jq --arg check_desc "$check_desc" '.status.detailReport.results | map(select(.description == $check_desc))'
```

{% endraw %}

### Модуль user-authn

Модуль отвечает за единую систему аутентификации, интегрированную с Kubernetes и веб-интерфейсами, используемыми в других модулях, например, Grafana и Dashboard.

Модуль состоит из следующих компонентов:
- `dex` — федеративный OpenID Connect провайдер, поддерживающий работу со статическими пользователями и с возможностью подключения к различным внешним провайдерам аутентификации.
- `kubeconfig-generator` (он же `dex-k8s-authenticator`) — веб-приложение, генерирующее команды для настройки локального `kubectl` после аутентификации в Dex;
- `dex-authenticator` (он же `oauth2-proxy`) — приложение, получающее запросы от компонента NGINX Ingress (через модуль auth_request) и выполняющее их авторизацию с помощью сервиса Dex.

Управление статическими пользователями осуществляется с помощью custom ресурс [_User_](cr.html#user), в котором хранится вся информация о пользователе, включая его пароль.

Поддерживаются следующие внешние провайдеры/протоколы аутентификации:
- LDAP;
- OIDC.

Одновременно можно подключить более одного внешнего провайдера аутентификации.

#### Возможности интеграции

##### Базовая аутентификация в API Kubernetes

Базовая аутентификация в API Kubernetes на данный момент доступна только для провайдера Crowd (с включением параметра [`enableBasicAuth`](cr.html#dexprovider-v1-spec-crowd-enablebasicauth)).

> К API Kubernetes можно подключаться и [через другие поддерживаемые внешние провайдеры](#веб-интерфейс-для-генерации-готовых-kubeconfig-файлов).

##### Интеграция с приложениями

Чтобы обеспечить аутентификацию в любом веб-приложении, работающем в Kubernetes, можно создать ресурс [_DexAuthenticator_](cr.html#dexauthenticator) в пространстве имен (_Namespace_) приложения и добавить несколько аннотаций к ресурсу _Ingress_.
Это позволит:
* ограничить список групп, которым разрешен доступ;
* ограничить список адресов, с которых разрешена аутентификация;
* интегрировать приложение в единую систему аутентификации, если приложение поддерживает OIDC. Для этого в Kubernetes создается ресурс [_DexClient_](cr.html#dexclient) в _Namespace_ приложения. В том же _Namespace_ создается секрет с данными для подключения в Dex по OIDC.

После такой интеграции можно:
* ограничить перечень групп, которым разрешено подключаться;
* указать перечень клиентов, OIDC-токенам которых можно доверять (`trustedPeers`).

##### Веб-интерфейс для генерации готовых kubeconfig-файлов

Модуль позволяет автоматически создавать конфигурацию для kubectl или других утилит Kubernetes.

Пользователь получит набор команд для настройки kubectl после авторизации в веб-интерфейсе генератора. Эти команды можно скопировать и вставить в консоль для использования kubectl.
Механизм аутентификации для kubeconfig использует OIDC-токен. OIDC-сессия может продлеваться автоматически, если использованный в Dex провайдер аутентификации поддерживает продление сессий. Для этого в kubeconfig указывается `refresh token`.

Дополнительно можно настроить несколько адресов `kube-apiserver` и сертификаты ЦС (CA) для каждого из них. Например, это может потребоваться, если доступ к кластеру Kubernetes осуществляется через VPN или прямое подключение.

#### Публикация API kubernetes через Ingress

Компонент kube-apiserver без дополнительных настроек доступен только во внутренней сети кластера. Этот модуль решает проблему простого и безопасного доступа к API Kubernetes извне кластера. При этом API-сервер публикуется на специальном домене (подробнее см. [раздел о служебных доменах в документации](./deckhouse-configure-global.html)).

При настройке можно указать:
* перечень сетевых адресов, с которых разрешено подключение;
* перечень групп, которым разрешен доступ к API-серверу;
* Ingress-контроллер, на котором производится аутентификация.

По умолчанию будет сгенерирован специальный сертификат ЦС (CA) и автоматически настроен генератор kubeconfig.

#### Расширения от Фланта

Модуль использует модифицированную версию Dex для поддержки:
* групп для статических учетных записей пользователей и провайдера Bitbucket Cloud (параметр [`bitbucketCloud`](cr.html#dexprovider-v1-spec-bitbucketcloud));
* передачи параметра `group` клиентам;
* механизма `obsolete tokens`, который позволяет избежать состояния гонки при продлении токена OIDC-клиентом.

#### Отказоустойчивый режим

Модуль поддерживает режим высокой доступности `highAvailability`. При его включении аутентификаторы, отвечающие на `auth request`-запросы, развертываются с учетом требуемой избыточности для обеспечения непрерывной работы. В случае отказа любого из экземпляров аутентификаторов пользовательские аутентификационные сессии не прерываются.

### Модуль user-authn: настройки

 
<!-- SCHEMA -->

Автоматический деплой oauth2-proxy в namespace вашего приложения и подключение его к Dex происходят при создании custom resource [`DexAuthenticator`](cr.html#dexauthenticator).

**Важно!** Так как использование OpenID Connect по протоколу HTTP является слишком значительной угрозой безопасности (что подтверждается, например, тем, что Kubernetes API-сервер не поддерживает работу с OIDC по HTTP), данный модуль можно установить только при включенном HTTPS (`https.mode` выставить в отличное от `Disabled` значение или на уровне кластера, или в самом модуле).

**Важно!** При включении данного модуля аутентификация во всех веб-интерфейсах перестанет использовать HTTP Basic Auth и переключится на Dex (который, в свою очередь, будет использовать настроенные вами внешние провайдеры).
Для настройки kubectl необходимо перейти по адресу `https://kubeconfig.<modules.publicDomainTemplate>/`, авторизоваться в настроенном внешнем провайдере и скопировать shell-команды к себе в консоль.

**Важно!** Для работы аутентификации в dashboard и kubectl требуется [донастройка API-сервера](faq.html#настройка-kube-apiserver). Для автоматизации этого процесса реализован модуль [control-plane-manager](./modules/control-plane-manager/), который включен по умолчанию.
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['user-authn'].config-values | format_module_configuration: moduleKebabName }}

### Модуль user-authn: Custom Resources
{{ site.data.schemas.user-authn.crds.dex-authenticator | format_crd: "user-authn" }}
{{ site.data.schemas.user-authn.crds.dex-client | format_crd: "user-authn" }}
{{ site.data.schemas.user-authn.crds.dex-provider | format_crd: "user-authn" }}
{{ site.data.schemas.user-authn.crds.dex | format_crd: "user-authn" }}
{{ site.data.schemas.user-authn.crds.group | format_crd: "user-authn" }}
{{ site.data.schemas.user-authn.crds.user | format_crd: "user-authn" }}

### Модуль user-authn: FAQ

{% raw %}

#### Как защитить мое приложение?

Чтобы включить аутентификацию через Dex для приложения, выполните следующие шаги:
1. Создайте custom resource [DexAuthenticator](cr.html#dexauthenticator).

   Создание `DexAuthenticator` в кластере приводит к созданию экземпляра oauth2-proxy, подключенного к Dex. После появления custom resource `DexAuthenticator` в указанном namespace появятся необходимые объекты Deployment, Service, Ingress, Secret.

   Пример custom resource `DexAuthenticator`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DexAuthenticator
   metadata:
     # Префикс имени подов Dex authenticator.
     # Например, если префикс имени `app-name`, то поды Dex authenticator будут вида `app-name-dex-authenticator-7f698684c8-c5cjg`.
     name: app-name
     # Namespace, в котором будет развернут Dex authenticator.
     namespace: app-ns
   spec:
     # Домен вашего приложения. Запросы на него будут перенаправляться для прохождения аутентификацию в Dex.
     applicationDomain: "app-name.kube.my-domain.com"
     # Отправлять ли `Authorization: Bearer` header приложению. Полезно в связке с auth_request в NGINX.
     sendAuthorizationHeader: false
     # Имя Secret'а с SSL-сертификатом.
     applicationIngressCertificateSecretName: "ingress-tls"
     # Название Ingress-класса, которое будет использоваться в создаваемом для Dex authenticator Ingress-ресурсе.
     applicationIngressClassName: "nginx"
     # Время, на протяжении которого пользовательская сессия будет считаться активной.
     keepUsersLoggedInFor: "720h"
     # Список групп, пользователям которых разрешено проходить аутентификацию.
     allowedGroups:
     - everyone
     - admins
     # Список адресов и сетей, с которых разрешено проходить аутентификацию.
     whitelistSourceRanges:
     - 1.1.1.1/32
     - 192.168.0.0/24
   ```

2. Подключите приложение к Dex.

   Для этого добавьте в Ingress-ресурс приложения следующие аннотации:

   - `nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in`
   - `nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email`
   - `nginx.ingress.kubernetes.io/auth-url: https://<NAME>-dex-authenticator.<NS>.svc.{{ C_DOMAIN }}/dex-authenticator/auth`, где:
      - `NAME` — значение параметра `metadata.name` ресурса `DexAuthenticator`;
      - `NS` — значение параметра `metadata.namespace` ресурса `DexAuthenticator`;
      - `C_DOMAIN` — домен кластера (параметр [clusterDomain](./installing/configuration.html#clusterconfiguration-clusterdomain) ресурса `ClusterConfiguration`).

   Ниже представлен пример аннотаций на Ingress-ресурсе приложения, для подключения его к Dex:

   ```yaml
   annotations:
     nginx.ingress.kubernetes.io/auth-signin: https://$host/dex-authenticator/sign_in
     nginx.ingress.kubernetes.io/auth-url: https://app-name-dex-authenticator.app-ns.svc.cluster.local/dex-authenticator/auth
     nginx.ingress.kubernetes.io/auth-response-headers: X-Auth-Request-User,X-Auth-Request-Email
   ```

##### Настройка ограничений на основе CIDR

В DexAuthenticator нет встроенной системы управления разрешением аутентификации на основе IP-адреса пользователя. Вместо этого вы можете воспользоваться аннотациями для Ingress-ресурсов:

* Если нужно ограничить доступ по IP и оставить прохождение аутентификации в Dex, добавьте аннотацию с указанием разрешенных CIDR через запятую:

  ```yaml
  nginx.ingress.kubernetes.io/whitelist-source-range: 192.168.0.0/32,1.1.1.1
  ```

* Если необходимо, чтобы пользователи из указанных сетей освобождались от прохождения аутентификации в Dex, а пользователи из остальных сетей обязательно аутентифицировались в Dex, добавьте следующую аннотацию:

  ```yaml
  nginx.ingress.kubernetes.io/satisfy: "any"
  ```

#### Как работает аутентификация с помощью DexAuthenticator

![Как работает аутентификация с помощью DexAuthenticator](./images/user-authn/dex_login.svg)

1. Dex в большинстве случаев перенаправляет пользователя на страницу входа провайдера и ожидает, что пользователь будет перенаправлен на его `/callback` URL. Однако такие провайдеры, как LDAP или Atlassian Crowd, не поддерживают этот вариант. Вместо этого пользователь должен ввести свои логин и пароль в форму входа в Dex, и Dex сам проверит их верность, сделав запрос к API провайдера.

2. DexAuthenticator устанавливает cookie с целым refresh token (вместо того чтобы выдать тикет, как для ID token) потому что Redis не сохраняет данные на диск.
Если по тикету в Redis не найден ID token, пользователь сможет запросить новый ID token, предоставив refresh token из cookie.

3. DexAuthenticator выставляет HTTP-заголовок `Authorization`, равный значению ID token из Redis. Это необязательно для сервисов по типу [Upmeter](./upmeter/), потому что права доступа к Upmeter не такие проработанные.
С другой стороны, для [Kubernetes Dashboard](./dashboard/) это критичный функционал, потому что она отправляет ID token дальше для доступа к Kubernetes API.

#### Как я могу сгенерировать kubeconfig для доступа к Kubernetes API?

Сгенерировать `kubeconfig` для удаленного доступа к кластеру через `kubectl` можно через веб-интерфейс `kubeconfigurator`.

Настройте параметр [publishAPI](configuration.html#parameters-publishapi):
- Откройте настройки модуля `user-authn` (создайте ресурс moduleConfig `user-authn`, если его нет):

  ```shell
  kubectl edit mc user-authn
  ```

- Добавьте следующую секцию в блок `settings` и сохраните изменения:

  ```yaml
  publishAPI:
    enabled: true
  ```

Для доступа к веб-интерфейсу, позволяющему сгенерировать `kubeconfig`, зарезервировано имя `kubeconfig`. URL для доступа зависит от значения параметра [publicDomainTemplate](./deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) (например, для `publicDomainTemplate: %s.kube.my` это будет `kubeconfig.kube.my`, а для `publicDomainTemplate: %s-kube.company.my` — `kubeconfig-kube.company.my`)  
{% endraw %}

##### Настройка kube-apiserver

С помощью функционала модуля [control-plane-manager](./modules/control-plane-manager/) Deckhouse автоматически настраивает kube-apiserver, выставляя следующие флаги так, чтобы модули `dashboard` и `kubeconfig-generator` могли работать в кластере.

{% offtopic title="Аргументы kube-apiserver, которые будут настроены" %}

* `--oidc-client-id=kubernetes`
* `--oidc-groups-claim=groups`
* `--oidc-issuer-url=https://dex.%addonsPublicDomainTemplate%/`
* `--oidc-username-claim=email`

В случае использования самоподписанных сертификатов для Dex будет добавлен еще один аргумент, а также в под с apiserver будет смонтирован файл с CA:

* `--oidc-ca-file=/etc/kubernetes/oidc-ca.crt`
{% endofftopic %}

{% raw %}

##### Как работает подключение к Kubernetes API с помощью сгенерированного kubeconfig

![Схема взаимодействия при подключении к Kubernetes API с помощью сгенерированного kubeconfig](./images/user-authn/kubeconfig_dex.svg)

1. До начала работы kube-apiserver необходимо запросить конфигурационный endpoint OIDC провайдера (в нашем случае — Dex), чтобы получить issuer и настройки JWKS endpoint.

2. Kubeconfig generator сохраняет ID token и refresh token в файл kubeconfig.

3. После получения запроса с ID token kube-apiserver идет проверять, что token подписан провайдером, который мы настроили на первом шаге, с помощью ключей, полученных с точки доступа JWKS. В качестве следующего шага он сравнивает значения claim'ов `iss` и `aud` из token'а со значениями из конфигурации.

#### Как Dex защищен от подбора логина и пароля?

Одному пользователю разрешено только 20 попыток входа. Если указанный лимит израсходован, одна дополнительная попытка будет добавляться каждые 6 секунд.

{% endraw %}

### Модуль user-authz

Модуль отвечает за генерацию объектов ролевой модели доступа, основанной на базе стандартного механизма RBAC Kubernetes. Модуль создает набор кластерных ролей (`ClusterRole`), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
С версии Deckhouse Kubernetes Platform v1.64 в модуле реализована новая модель ролевого доступа. Старая модель ролевого доступа продолжит работать, но в будущем перестанет поддерживаться.

Функциональность старой и новой моделей ролевого доступа несовместимы. Автоматическая конвертация ресурсов невозможна.
{% endalert %}

{% alert level="warning" %}
Документация модуля подразумевает использование [новой ролевой модели](#новая-ролевая-модель), если не указано иное.
{% endalert %}

#### Новая ролевая модель

В отличие от [устаревшей ролевой модели](#устаревшая-ролевая-модель) DKP, новая ролевая модель не использует ресурсы `ClusterAuthorizationRule` и `AuthorizationRule`. Вся настройка прав доступа выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из подготовленных модулем `user-authz` ролей.

Модуль создаёт специальные агрегированные кластерные роли (`ClusterRole`). Используя эти роли в `RoleBinding` или `ClusterRoleBinding` можно решать следующие задачи:
- Управлять доступом к модулям определённой [подсистеме](#подсистемы-ролевой-модели) применения.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, `cni-cilium`, `ingress-nginx`, `istio` и т. д.), можно использовать в `ClusterRoleBinding` роль `d8:manage:networking:manager`.
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:use:role:manager` в `RoleBinding`, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](./log-shipper/cr.html#podloggingconfig) в пространстве имён, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](./log-shipper/cr.html#clusterloggingconfig) и [ClusterLogDestination](./log-shipper/cr.html#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

Роли, создаваемые модулем, делятся на два класса:
- [Use-роли](#use-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном пространстве имён**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам.

##### Use-роли

{% alert level="warning" %}
Use-роль можно использовать только в ресурсе `RoleBinding`.
{% endalert %}

Use-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DKP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Use-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap` и т. п.).

Модуль создаёт следующие use-роли:
- `d8:use:role:viewer` — позволяет в конкретном пространстве имён просматривать стандартные ресурсы Kubernetes, кроме секретов и ресурсов RBAC, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет в конкретном пространстве имён просматривать секреты и ресурсы RBAC, подключаться к подам, удалять поды (но не создавать или изменять их), выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет в конкретном пространстве имён управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.) и стандартными namespaced-ресурсами Kubernetes (`Pod`, `ConfigMap`, `CronJob` и т. п.);
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет в конкретном пространстве имён управлять ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`.

##### Manage-роли

{% alert level="warning" %}
Manage-роль не дает доступа к пространству имён пользовательских приложений.

Manage-роль определяет доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), и только к тем из них, в которых работают модули соответствующей подсистемы роли.
{% endalert %}

Manage-роли предназначены для назначения прав на управление всей платформой или её частью ([подсистемой](#подсистемы-ролевой-модели)), но не самими приложениями пользователей. С помощью manage-роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в пространстве имён приложений пользователей.

Manage-роль определяет права на доступ:
- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DKP (ресурсы `moduleConfig`) в рамках [подсистемы](#подсистемы-ролевой-модели) роли, или всеми модулями DKP для роли `d8:manage:all:*`;
- к управлению cluster-wide-ресурсами модулей DKP в рамках [подсистемы](#подсистемы-ролевой-модели) роли или всеми ресурсами модулей DKP для роли `d8:manage:all:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [подсистемы](#подсистемы-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:manage:all:*`.
  
Формат названия manage-роли — `d8:manage:<SUBSYSTEM>:<ACCESS_LEVEL>`, где:
- `SUBSYSTEM` — подсистема роли. Может быть либо одной из подсистем [списка](#подсистемы-ролевой-модели), либо `all` для доступа в рамках всех подсистем;
- `ACCESS_LEVEL` — уровень доступа.

  Примеры manage-ролей:
  - `d8:manage:all:viewer` — доступ на просмотр конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:all:manager` — аналогично роли `d8:manage:all:viewer`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:observability:viewer` — доступ на просмотр конфигурации модулей DKP (ресурсы `moduleConfig`) из подсистемы `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

Модуль предоставляет два уровня доступа для администратора:
- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes, конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей в пространстве имен модуля;
- `manager` — дополнительно к роли `viewer` позволяет управлять стандартными ресурсами Kubernetes, конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей в пространстве имен модуля;

##### Подсистемы ролевой модели

Каждый модуль DKP принадлежит определённой подсистемы. Для каждой подсистемы существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для подсистемы `networking` существуют следующие manage-роли, которые можно использовать в `ClusterRoleBinding`:
- `d8:manage:networking:viewer`
- `d8:manage:networking:manager`

Подсистема роли ограничивает её действие всеми системными (начинающимися с `d8-` или `kube-`) пространствами имён кластера (подсистема `all`) или теми пространствами имён, в которых работают модули подсистемы (см. таблицу состава подсистем).

Таблица состава подсистем ролевой модели.

{% include rbac/rbac-subsystems-list.liquid %}

#### Устаревшая ролевая модель

Особенности:
- Реализует role-based-подсистему сквозной авторизации, расширяя функционал стандартного механизма RBAC.
- Настройка прав доступа происходит с помощью [ресурсов](cr.html).
- Управление доступом к инструментам масштабирования (параметр `allowScale` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-allowscale) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-allowscale)).
- Управление доступом к форвардингу портов (параметр `portForwarding` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-portforwarding) или [AuthorizationRule](cr.html#authorizationrule-v1alpha1-spec-portforwarding)).
- Управление списком разрешённых пространств имён в формате labelSelector (параметр `namespaceSelector` ресурса [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule-v1-spec-namespaceselector)).

В модуле, кроме прямого использования RBAC, можно использовать удобный набор высокоуровневых ролей:
- `User` — позволяет получать информацию обо всех объектах (включая доступ к журналам подов), но не позволяет заходить в контейнеры, читать секреты и выполнять port-forward;
- `PrivilegedUser` — то же самое, что и `User`, но позволяет заходить в контейнеры, читать секреты, а также удалять поды (что обеспечивает возможность перезагрузки);
- `Editor` — то же самое, что и `PrivilegedUser`, но предоставляет возможность создавать, изменять и удалять все объекты, которые обычно нужны для прикладных задач;
- `Admin` — то же самое, что и `Editor`, но позволяет удалять служебные объекты (производные ресурсы, например `ReplicaSet`, `certmanager.k8s.io/challenges` и `certmanager.k8s.io/orders`);
- `ClusterEditor` — то же самое, что и `Editor`, но позволяет управлять ограниченным набором `cluster-wide`-объектов, которые могут понадобиться для прикладных задач (`ClusterXXXMetric`, `KeepalivedInstance`, `DaemonSet` и т. д). Роль для работы оператора кластера;
- `ClusterAdmin` — то же самое, что и `ClusterEditor` + `Admin`, но позволяет управлять служебными `cluster-wide`-объектами (производные ресурсы, например `MachineSets`, `Machines`, `OpenstackInstanceClasses` и т. п., а также `ClusterAuthorizationRule`, `ClusterRoleBindings` и `ClusterRole`). Роль для работы администратора кластера. **Важно**, что `ClusterAdmin`, поскольку он уполномочен редактировать `ClusterRoleBindings`, может **сам себе расширить полномочия**;
- `SuperAdmin` — разрешены любые действия с любыми объектами, при этом ограничения `namespaceSelector` и `limitNamespaces` продолжат работать.

{% alert level="warning" %}
Режим multi-tenancy (авторизация по пространству имён) в данный момент реализован по временной схеме и **не гарантирует безопасность!**
{% endalert %}

В случае, если в [`ClusterAuthorizationRule`](cr.html#clusterauthorizationrule)-ресурсе используется `namespaceSelector`, параметры `limitNamespaces` и `allowAccessToSystemNamespace` не учитываются.

Если вебхук, который реализовывает систему авторизации, по какой-то причине будет недоступен, в это время опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource перестанут применяться и пользователи будут иметь доступ во все пространства имён. После восстановления доступности вебхука опции продолжат работать.

##### Список доступа для каждой роли модуля по умолчанию

Сокращения для `verbs`:
<!-- start user-authz roles placeholder -->
* read - `get`, `list`, `watch`
* read-write - `get`, `list`, `watch`, `create`, `delete`, `deletecollection`, `patch`, `update`
* write - `create`, `delete`, `deletecollection`, `patch`, `update`

{{site.data.i18n.common.role[page.lang] | capitalize }} `User`:

```text
read:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - apps/deployments
    - apps/replicasets
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - events
    - events.k8s.io/events
    - extensions/daemonsets
    - extensions/deployments
    - extensions/ingresses
    - extensions/replicasets
    - extensions/replicationcontrollers
    - limitranges
    - metrics.k8s.io/nodes
    - metrics.k8s.io/pods
    - namespaces
    - networking.k8s.io/ingresses
    - networking.k8s.io/networkpolicies
    - nodes
    - persistentvolumeclaims
    - persistentvolumes
    - pods
    - pods/log
    - policy/poddisruptionbudgets
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - replicationcontrollers
    - resourcequotas
    - serviceaccounts
    - services
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `PrivilegedUser` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`):

```text
create:
    - pods/eviction
create,get:
    - pods/attach
    - pods/exec
delete,deletecollection:
    - pods
read:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Editor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`):

```text
read-write:
    - apps/deployments
    - apps/statefulsets
    - autoscaling.k8s.io/verticalpodautoscalers
    - autoscaling/horizontalpodautoscalers
    - batch/cronjobs
    - batch/jobs
    - configmaps
    - discovery.k8s.io/endpointslices
    - endpoints
    - extensions/deployments
    - extensions/ingresses
    - networking.k8s.io/ingresses
    - persistentvolumeclaims
    - policy/poddisruptionbudgets
    - serviceaccounts
    - services
write:
    - secrets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `Admin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
create,patch,update:
    - pods
delete,deletecollection:
    - apps/replicasets
    - extensions/replicasets
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterEditor` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`):

```text
read:
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
write:
    - apiextensions.k8s.io/customresourcedefinitions
    - apps/daemonsets
    - extensions/daemonsets
    - storage.k8s.io/storageclasses
```

{{site.data.i18n.common.role[page.lang] | capitalize }} `ClusterAdmin` ({{site.data.i18n.common.includes_rules_from[page.lang]}} `User`, `PrivilegedUser`, `Editor`, `Admin`, `ClusterEditor`):

```text
read-write:
    - deckhouse.io/clusterauthorizationrules
write:
    - limitranges
    - namespaces
    - networking.k8s.io/networkpolicies
    - rbac.authorization.k8s.io/clusterrolebindings
    - rbac.authorization.k8s.io/clusterroles
    - rbac.authorization.k8s.io/rolebindings
    - rbac.authorization.k8s.io/roles
    - resourcequotas
```
<!-- end user-authz roles placeholder -->

Вы можете получить дополнительный список правил доступа для роли модуля из кластера ([существующие пользовательские правила](usage.html#настройка-прав-высокоуровневых-ролей) и нестандартные правила из других модулей Deckhouse):

```bash
D8_ROLE_NAME=Editor
kubectl get clusterrole -A -o jsonpath="{range .items[?(@.metadata.annotations.user-authz\.deckhouse\.io/access-level=='$D8_ROLE_NAME')]}{.rules}{'
'}{end}" | jq -s add
```

### Модуль user-authz: настройки

> **Внимание!** Мы категорически не рекомендуем создавать поды и ReplicaSet'ы — эти объекты являются второстепенными и должны создаваться из других контроллеров. Доступ к созданию и изменению подов и ReplicaSet'ов полностью отсутствует.
>
> **Внимание!** Режим multi-tenancy (авторизация по пространству имён) в данный момент реализован по временной схеме и **не гарантирует безопасность**! Если вебхук, который реализовывает систему авторизации, по какой-то причине будет недоступен, авторизация по пространству имён (опции `allowAccessToSystemNamespaces`, `namespaceSelector` и `limitNamespaces` в custom resource) перестанет работать и пользователи получат доступы во все пространства имён. После восстановления доступности вебхука всё вернётся на свои места.

Вся настройка прав доступа происходит с помощью [custom resources](cr.html).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['user-authz'].config-values | format_module_configuration: moduleKebabName }}

### Модуль user-authz: Custom Resources
{{ site.data.schemas.user-authz.crds.authorizationrule | format_crd: "user-authz" }}
{{ site.data.schemas.user-authz.crds.clusterauthorizationrule | format_crd: "user-authz" }}

### Модуль user-authz: FAQ

#### Как создать пользователя?

[Создание пользователя](usage.html#создание-пользователя).

#### Как ограничить права пользователю конкретными пространствами имён?

Чтобы ограничить права пользователя конкретными пространствами имён, используйте в `RoleBinding` [use-роль](./#use-роли) с соответствующим уровнем доступа. [Пример...](usage.html#пример-назначения-административных-прав-пользователю-в-рамках-пространства-имён)

##### Как ограничить права пользователю конкретными пространствами имён (устаревшая ролевая модель)

{% alert level="warning" %}
Используется [устаревшая ролевая модель](./#устаревшая-ролевая-модель).
{% endalert %}

Использовать параметры `namespaceSelector` или `limitNamespaces` (устарел) в кастомном ресурсе [`ClusterAuthorizationRule`](./modules/user-authz/cr.html#clusterauthorizationrule).

#### Что, если два ClusterAuthorizationRules подходят для одного пользователя?

Представьте, что пользователь `jane.doe@example.com` состоит в группе `administrators`. Созданы два ClusterAuthorizationRules:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: jane
spec:
  subjects:
    - kind: User
      name: jane.doe@example.com
  accessLevel: User
  namespaceSelector:
    labelSelector:
      matchLabels:
        env: review
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: Group
    name: administrators
  accessLevel: ClusterAdmin
  namespaceSelector:
    labelSelector:
      matchExpressions:
      - key: env
        operator: In
        values:
        - prod
        - stage
```

1. `jane.doe@example.com` имеет право запрашивать и просматривать объекты среди всех пространств имён, помеченных `env=review`.
2. `Administrators` могут запрашивать, редактировать, получать и удалять объекты на уровне кластера и из пространств имён, помеченных `env=prod` и `env=stage`.

Так как для `Jane Doe` подходят два правила, необходимо провести вычисления:
* Она будет иметь самый сильный accessLevel среди всех подходящих правил — `ClusterAdmin`.
* Опции `namespaceSelector` будут объединены так, что `Jane Doe` будет иметь доступ в пространства имён, помеченные меткой `env` со значением `review`, `stage` или `prod`.

> **Note!** Если есть правило без опции `namespaceSelector` и без опции `limitNamespaces` (устаревшая), это значит, что доступ разрешён во все пространства имён, кроме системных, что повлияет на результат вычисления доступных пространств имён для пользователя.

#### Как расширить роли или создать новую?

[Новая ролевая модель](./#новая-ролевая-модель) построена на принципе агрегации, она собирает более мелкие роли в более обширные,
тем самым предоставляя лёгкие способы расширения модели собственными ролями.

##### Создание новой роли подсистемы

Предположим, что текущие подсистемы не подходят под ролевое распределение в компании и требуется создать новую [подсистему](./#подсистемы-ролевой-модели),
которая будет включать в себя роли из подсистемы `deckhouse`, подсистемы `kubernetes` и модуля user-authn.

Для решения этой задачи создайте следующую роль:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: custom:manage:mycustom:manager
  labels:
    rbac.deckhouse.io/use-role: admin
    rbac.deckhouse.io/kind: manage
    rbac.deckhouse.io/level: subsystem
    rbac.deckhouse.io/subsystem: custom
    rbac.deckhouse.io/aggregate-to-all-as: manager
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
    - matchLabels:
        rbac.deckhouse.io/kind: manage
        module: user-authn
rules: []
```

В начале указаны лейблы для новой роли:

- показывает, какую роль хук должен использовать при создании use ролей:

  ```yaml
  rbac.deckhouse.io/use-role: admin
  ```

- показывает, что роль должна обрабатываться как manage-роль:

  ```yaml
  rbac.deckhouse.io/kind: manage
  ```

  > Этот лейбл должен быть обязательно указан!

- показывает, что роль является ролью подсистемы, и обрабатываться будет соответственно:

  ```yaml
  rbac.deckhouse.io/level: subsystem
  ```

- указывает подсистему, за которую отвечает роль:

  ```yaml
  rbac.deckhouse.io/subsystem: custom
  ```

- позволяет `manage:all`-роли сагрегировать эту роль:

  ```yaml
  rbac.deckhouse.io/aggregate-to-all-as: manager
  ```

Далее указаны селекторы, именно они реализуют агрегацию:

- агрегирует роль менеджера из подсистемы `deckhouse`:

  ```yaml
  rbac.deckhouse.io/kind: manage
  rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
  ```

- агрерирует все правила от модуля user-authn:

  ```yaml
   rbac.deckhouse.io/kind: manage
   module: user-authn
  ```

Таким образом роль получает права от подсистем `deckhouse`, `kubernetes` и от модуля user-authn.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль;
* use-роли будут созданы в пространстве имён агрегированных подсистем и модуля, тип роли выбран лейблом.

##### Расширение пользовательской роли

Например, в кластере появился новый кластерный (пример для manage-роли) CRD-объект — MySuperResource, и нужно дополнить собственную роль из примера выше правами на взаимодействие с этим ресурсом.

Первым делом нужно дополнить роль новым селектором:

```yaml
rbac.deckhouse.io/kind: manage
rbac.deckhouse.io/aggregate-to-custom-as: manager
```

Этот селектор позволит агрегировать роли к новой подсистеме через указание этого лейбла. После добавления нового селектора роль будет выглядеть так:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   name: custom:manage:mycustom:manager
   labels:
     rbac.deckhouse.io/use-role: admin
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/level: subsystem
     rbac.deckhouse.io/subsystem: custom
     rbac.deckhouse.io/aggregate-to-all-as: manager
 aggregationRule:
   clusterRoleSelectors:
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-kubernetes-as: manager
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         module: user-authn
     - matchLabels:
         rbac.deckhouse.io/kind: manage
         rbac.deckhouse.io/aggregate-to-custom-as: manager
 rules: []
 ```

 Далее нужно создать новую роль, в которой следует определить права для нового ресурса. Например, только чтение:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-custom-as: manager
     rbac.deckhouse.io/kind: manage
   name: custom:manage:permission:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Роль дополнит своими правами роль подсистемы, дав права на просмотр нового объекта.

Особенности:

* ограничений на имя роли нет, но для читаемости лучше использовать этот стиль.

##### Расширение существующих manage subsystem-ролей

Если необходимо расширить существующую роль, нужно выполнить те же шаги, что и в пункте выше, но изменив лейблы и название роли.

Пример для расширения роли менеджера из подсистемы `deckhouse`(`d8:manage:deckhouse:manager`):

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
    rbac.deckhouse.io/kind: manage
  name: custom:manage:permission:mycustommodule:superresource:view
rules:
- apiGroups:
  - mygroup.io
  resources:
  - mysuperresources
  verbs:
  - get
  - list
  - watch
```

Таким образом новая роль расширит роль `d8:manage:deckhouse`.

##### Расширение manage subsystem-ролей с добавлением нового пространства имён

Если необходимо добавить новое пространство имён (для создания в нём use-роли с помощью хука), потребуется добавить лишь один лейбл:

```yaml
"rbac.deckhouse.io/namespace": namespace
```

Этот лейбл сообщает хуку, что в этом пространстве имён нужно создать use-роль:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-deckhouse-as: manager
     rbac.deckhouse.io/kind: manage
     rbac.deckhouse.io/namespace: namespace
   name: custom:manage:permission:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Хук мониторит `ClusterRoleBinding` и при создании биндинга ходит по всем manage-ролям, чтобы найти все сагрерированные роли с помощью проверки правила агрегации. Затем он берёт пространство имён из лейбла `rbac.deckhouse.io/namespace` и создает use-роль в этом пространстве имён.

##### Расширение существующих use-ролей

Если ресурс принадлежит пространству имён, необходимо расширить use-роль вместо manage-роли. Разница лишь в лейблах и имени:

 ```yaml
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     rbac.deckhouse.io/aggregate-to-kubernetes-as: user
     rbac.deckhouse.io/kind: use
   name: custom:use:capability:mycustom:superresource:view
 rules:
 - apiGroups:
   - mygroup.io
   resources:
   - mysuperresources
   verbs:
   - get
   - list
   - watch
 ```

Эта роль дополнит роль `d8:use:role:user:kubernetes`.

### Модуль runtime-audit-engine

#### Описание

Модуль предназначен для поиска угроз безопасности.

Модуль собирает события ядра Linux и итоги аудита API Kubernetes (с помощью плагина `k8saudit`), обогащает их метаданными о подах Kubernetes и генерирует события аудита безопасности по установленным правилам.

Модуль runtime-audit-engine:
* Находит угрозы в окружениях, анализируя приложения и контейнеры.
* Помогает обнаружить попытки применения уязвимостей из базы CVE и запуска криптовалютных майнеров.
* Повышает безопасность Kubernetes, выявляя:
  * оболочки командной строки, запущенные в контейнерах или подах в Kubernetes;
  * контейнеры, работающие в привилегированном режиме; монтирование небезопасных путей (например, `/proc`) в контейнеры;
  * попытки чтения секретных данных из, например, `/etc/shadow`.

#### Архитектура

Ядро модуля основано на системе обнаружения угроз Falco.
Deckhouse запускает агенты Falco (объединены в DaemonSet) на каждом узле, после чего те приступают к сбору событий ядра и данных, полученных в ходе аудита Kubernetes.

![Falco DaemonSet](./images/runtime-audit-engine/falco_daemonset.svg)
<!--- Source: https://docs.google.com/drawings/d/1NZ91z8NXNiuS50ybcMoMsZI3SbQASZXJGLANdaNNm_U --->

{% alert %}
Для максимальной безопасности разработчики Falco рекомендуют запускать Falco как systemd-сервис, однако в кластерах Kubernetes с поддержкой автомасштабирования это может быть затруднительно. Дополнительные средства безопасности Deckhouse (реализованные другими модулями), такие как multi-tenancy или политики контроля создаваемых ресурсов, предоставляют достаточный уровень безопасности для предотвращения атак на DaemonSet Falco.
{% endalert %}

Один под Falco состоит из четырех контейнеров:
![Falco Pod](./images/runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

1. `falco` — собирает события, обогащает их метаданными и отправляет их в stdout.
2. `rules-loader` — собирает custom resourcе'ы ([FalcoAuditRules](cr.html#falcoauditrules)) из Kubernetes и сохраняет их в общую папку.
3. `falcosidekick` — принимает события от `Falco` и перенаправляет их разными способами. По умолчанию экспортирует события как метрики, по которым потом можно настроить алерты. Исходный код Falcosidekick.
4. `kube-rbac-proxy` — защищает endpoint метрик `falcosidekick` (запрещает неавторизованный доступ).

#### Правила аудита

Сборка событий сама по себе не дает ничего, поскольку объем данных, собираемый с ядра Linux, слишком велик для анализа человеком.
Правила позволяют решить эту проблему: события отбираются по определенным условиям. Условия настраиваются на выявление любой подозрительной активности.

В основе каждого правила лежит выражение, содержащее определенное условие, написанное в соответствии с синтаксисом условий.

##### Встроенные правила

Существуют два встроенных набора правил, которые нельзя отключить.
Они помогают выявить проблемы с безопасностью Deckhouse и с самим модулем `runtime-audit-engine`:

- правила, статично размещенные в контейнере `falco`, по пути `/etc/falco/k8s_audit_rules.yaml` — правила для аудита Kubernetes.
- правила, размещенные в формате custom resource [FalcoAuditRules](cr.html#falcoauditrules), `fstec` — правила аудита удовлетворяющие требованиям приказа ФСТЭК России №118 от 4 июля 2022г. (Требования по безопасности информации к средствам контейнеризации).

##### Пользовательские правила

Добавить пользовательские правила можно с помощью custom resource [FalcoAuditRules](cr.html#falcoauditrules).
У каждого агента Falco есть sidecar-контейнер с экземпляром shell-operator.
Этот экземпляр считывает правила из custom resource'ов Kubernetes, конвертирует их в правила Falco и сохраняет правила Falco в директорию `/etc/falco/rules.d/` пода.
При добавлении нового правила Falco автоматически обновляет конфигурацию.

![Falco shell-operator](./images/runtime-audit-engine/falco_shop.svg)
<!--- Source: https://docs.google.com/drawings/d/13MFYtiwH4Y66SfEPZIcS7S2wAY6vnKcoaztxsmX1hug --->

Такая схема позволяет использовать подход «Инфраструктура как код» при работе с правилами Falco.

#### Требования

##### Операционная система

Модуль использует драйвер eBPF для Falco при сборке событий ядра операционной системы. Этот драйвер особенно полезен в окружениях, в которых невозможна сборка модуля ядра (например, GKE, EKS и другие решения Managed Kubernetes).
У драйвера eBPF есть следующие требования:
* Ядро Linux >= 5.8.
* Включённый eBPF. Проверьте командой `ls -lah /sys/kernel/btf/vmlinux`, либо найдите `CONFIG_DEBUG_INFO_BTF=y` в списке параметров сборки ядра.

> На некоторых системах пробы (probe) eBPF могут не работать.

##### Процессор / Память

Агенты Falco работают на каждом узле. Поды агентов потребляют ресурсы в зависимости от количества применяемых правил или собираемых событий.

#### Kubernetes Audit Webhook

Режим Webhook audit mode должен быть настроен на получение событий аудита от `kube-apiserver`.
Если модуль [control-plane-manager](./control-plane-manager/) включен, настройки автоматически применятся при включении модуля `runtime-audit-engine`.

В кластерах Kubernetes, в которых control plane не управляется Deckhouse, webhook необходимо настроить вручную. Для этого:

1. Создайте файл kubeconfig для webhook с адресом `https://127.0.0.1:9765/k8s-audit` и CA (ca.crt) из Secret'а `d8-runtime-audit-engine/runtime-audit-engine-webhook-tls`.

   Пример:

   ```yaml
   apiVersion: v1
   kind: Config
   clusters:
   - name: webhook
     cluster:
       certificate-authority-data: BASE64_CA
       server: "https://127.0.0.1:9765/k8s-audit"
   users:
   - name: webhook
   contexts:
   - context:
      cluster: webhook
      user: webhook
     name: webhook
   current-context: webhook
   ```

2. Добавьте к `kube-apiserver` флаг `--audit-webhook-config-file`, который будет указывать на файл, созданный на предыдущем шаге.

{% alert level="warning" %}
Не забудьте настроить audit policy, поскольку Deckhouse по умолчанию собирает только события аудита Kubernetes для системных пространств имен.
Пример конфигурации можно найти в документации модуля [control-plane-manager](./control-plane-manager/).
{% endalert %}

#### Алерты

Если несколько подов `runtime-audit-engine` не назначены на узлы планировщиком, будет сгенерирован алерт `D8RuntimeAuditEngineNotScheduledInCluster`.

### Модуль runtime-audit-engine: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['modules'].config-values | format_module_configuration: moduleKebabName }}

### Модуль runtime-audit-engine: Custom Resources
{{ site.data.schemas.modules.650-runtime-audit-engine.crds.falco-audit-rules | format_crd: "modules" }}

### Модуль runtime-audit-engine: примеры

#### Добавление одного правила

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: ownership-permissions
spec:
  rules:
  - macro:
      name: spawned_process
      condition: (evt.type in (execve, execveat) and evt.dir=<)
  - rule:
      name: Detect Ownership Change
      desc: detect file permission/ownership change
      condition: >
        spawned_process and proc.name in (chmod, chown) and proc.args contains "/tmp/"
      output: >
        The file or directory below has had its permissions or ownership changed (user=%user.name
        command=%proc.cmdline file=%fd.name parent=%proc.pname pcmdline=%proc.pcmdline gparent=%proc.aname[2])
      priority: Warning
      tags: [filesystem]
```

#### Добавление двух правил с макросом и списком

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: nginx-unexpected-port
spec:
  rules:
  - macro:
      name: container
      condition: (container.id != host)

  - macro:
      name: inbound
      condition: >
        (((evt.type in (accept,listen) and evt.dir=<)) or
        (fd.typechar = 4 or fd.typechar = 6) and
        (fd.ip != "0.0.0.0" and fd.net != "127.0.0.0/8") and (evt.rawres >= 0 or evt.res = EINPROGRESS))

  - macro:
      name: outbound
      condition: >
        (((evt.type = connect and evt.dir=<)) or
        (fd.typechar = 4 or fd.typechar = 6) and
        (fd.ip != "0.0.0.0" and fd.net != "127.0.0.0/8") and (evt.rawres >= 0 or evt.res = EINPROGRESS))

  - macro:
      name: app_nginx
      condition: container and container.image contains "nginx"

  - rule:
      name: Unauthorized process opened an outbound connection (nginx)
      desc: nginx process tried to open an outbound connection and is not whitelisted
      condition: outbound and evt.rawres >= 0 and app_nginx
      output: |-
        Non-whitelisted process opened an outbound connection (command=%proc.cmdline connection=%fd.name)
      priority: Warning

  - list:
      name: nginx_allowed_inbound_ports_tcp
      items: [80, 443, 8080, 8443]

  - rule:
      name: Unexpected inbound TCP connection nginx
      desc: detect inbound traffic to nginx using tcp on a port outside of expected set
      condition: |
        inbound and evt.rawres >= 0 and not fd.sport in (nginx_allowed_inbound_ports_tcp) and app_nginx
      output: |-
        Inbound network connection to nginx on unexpected port
        (command=%proc.cmdline pid=%proc.pid connection=%fd.name sport=%fd.sport user=%user.name %container.info image=%container.image)
      priority: Notice
```

#### Добавление правила для отправки уведомлений о запуске shell-оболочки в контейнере

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: run-shell-in-container
spec:
  rules:
  - macro: 
      name: container
      condition: container.id != host
  
  - macro: 
      name: spawned_process
      condition: evt.type = execve and evt.dir=<
  
  - rule: 
      name: run_shell_in_container
      desc: a shell was spawned by a non-shell program in a container. Container entrypoints are excluded.
      condition: container and proc.name = bash and spawned_process and proc.pname exists and not proc.pname in (bash, docker)
      output: "Shell spawned in a container other than entrypoint (user=%user.name container_id=%container.id container_name=%container.name shell=%proc.name parent=%proc.pname cmdline=%proc.cmdline)"
      priority: Warning
```

#### Дополнительные примеры

Если вам необходимо больше примеров правил, изучите следующие ресурсы:

- falco rules repository;
- artifacthub falco rules.

### Модуль runtime-audit-engine: FAQ

{% raw %}

#### Как собирать события?

Поды `runtime-audit-engine` выводят все события в стандартный вывод.
Далее [агенты log-shipper](./log-shipper/) могут собирать их и отправлять в хранилище логов.

Пример конфигурации [ClusterLoggingConfig](./log-shipper/cr.html#clusterloggingconfig) для модуля `log-shipper`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ClusterLoggingConfig
metadata:
  name: falco-events
spec:
  destinationRefs:
  - xxxx
  kubernetesPods:
    namespaceSelector:
      matchNames:
      - d8-runtime-audit-engine
  labelFilter:
  - operator: Regex
    values: ["\{.*"] # to collect only JSON logs
    field: "message"
  type: KubernetesPods
```

#### Как оповещать о критических событиях?

Prometheus автоматически собирает метрики о событиях.
Чтобы включить оповещения, добавьте в кластер правило [CustomPrometheusRule](./prometheus/cr.html#customprometheusrules).

Пример настройки такого правила:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: falco-critical-alerts
spec:
  groups:
  - name: falco-critical-alerts
    rules:
    - alert: FalcoCriticalAlertsAreFiring
      for: 1m
      annotations:
        description: |
          There is a suspicious activity on a node {{ $labels.node }}. 
          Check you events journal for more details.
        summary: Falco detects a critical security incident
      expr: |
        sum by (node) (rate(falco_events{priority="Critical"}[5m]) > 0)
```

{% endraw %}
{% alert %}
Алерты лучше всего работают в комбинации с хранилищами событий, такими как Elasticsearch или Loki. Их задача — оповестить пользователя о подозрительном поведении на узле.
После получения алерта рекомендуется «пойти» в хранилище и посмотреть на события, которые его вызвали.
{% endalert %}

#### Как применить правила для Falco, найденные в интернете?

Структура правил Falco отличается от схемы CRD.
Это связано со сложностями при проверке правильности ресурсов в Kubernetes.

Скрипт для конвертации правил Falco в ресурсы [FalcoAuditRules](cr.html#falcoauditrules) упрощает процесс миграции и позволять применять правила Falco в Deckhouse:

```shell
git clone github.com/deckhouse/deckhouse
cd deckhouse/ee/modules/runtime-audit-engine/hack/far-converter
go run main.go -input /path/to/falco/rule_example.yaml > ./my-rules-cr.yaml
```

Пример результата работы скрипта:

```yaml
### /path/to/falco/rule_example.yaml
- macro: spawned_process
  condition: (evt.type in (execve, execveat) and evt.dir=<)

- rule: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
  desc: "This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel."
  condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
  output: "Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)"
  priority: CRITICAL
  tags: [process, mitre_privilege_escalation]
```

```yaml
### ./my-rules-cr.yaml
apiVersion: deckhouse.io/v1alpha1
kind: FalcoAuditRules
metadata:
  name: rule-example
spec:
  rules:
    - macro:
        name: spawned_process
        condition: (evt.type in (execve, execveat) and evt.dir=<)
    - rule:
        name: Linux Cgroup Container Escape Vulnerability (CVE-2022-0492)
        condition: container.id != "" and proc.name = "unshare" and spawned_process and evt.args contains "mount" and evt.args contains "-o rdma" and evt.args contains "/release_agent"
        desc: This rule detects an attempt to exploit a container escape vulnerability in the Linux Kernel.
        output: Detect Linux Cgroup Container Escape Vulnerability (CVE-2022-0492) (user=%user.loginname uid=%user.loginuid command=%proc.cmdline args=%proc.args)
        priority: Critical
        tags:
          - process
          - mitre_privilege_escalation
```

### Модуль secret-copier
 
Этот модуль отвечает за копирование секретов во все пространства имён.

Он полезен тем, что позволяет не копировать каждый раз в CI секреты для пуллинга образов и заказа RBD в Ceph.

##### Как работает?

Модуль `secret-copier` следит за изменениями секретов в пространстве имён `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.
* Созданный секрет будет скопирован во все пространства имён.
* Изменённый секрет с его новым содержимым будет раскопирован во все пространства имён.
* При удалении секрет будет удален из всех пространств имён.
* При изменении скопированного секрета в прикладном пространстве имён, тот будет перезаписан оригинальным содержимым.
* При создании любого пространства имён в него копируются все секреты из пространства имён `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.

Кроме этого, каждую ночь секреты будут повторно синхронизированы и приведены к состоянию в пространств имён `default`.

##### Что нужно настроить?

Чтобы все заработало, достаточно создать в пространстве имён `default` секрет с лейблом `secret-copier.deckhouse.io/enabled: ""`.

> **Внимание!** Рабочим пространством имён для модуля является `default`, Секреты будут копироваться только из него. Ресурсы с лейблом `secret-copier.deckhouse.io/enabled: ""`, созданные в других пространствах имён при включенном модуле будут автоматически удалены.

##### Как ограничить список пространств имён, в которые будет производиться копирование?

Для этого нужно задать label–селектор в значении аннотации `secret-copier.deckhouse.io/target-namespace-selector`. Например: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. Модуль создаст копию этого секрета во всех пространствах имён, соответствующих заданному label–селектору.

### Модуль secrets-store-integration

Модуль secrets-store-integration реализует доставку секретов для приложения в Kubernetes-кластерах
путем подключения секретов, ключей и сертификатов, хранящихся во внешних хранилищах секретов.

Секреты монтируются в поды в виде тома с использованием реализации драйвера CSI.
Хранилища секретов должны быть совместимы с API-интерфейсом HashiCorp Vault.

#### Доставка секретов в приложения

Доставить секреты в приложение из vault-совместимого хранилища можно несколькими способами:

1. Пользовательское приложение само обращается в хранилище.

   > Это наиболее безопасный вариант, но требует модификации приложений.

1. В хранилище обращается приложение-прослойка, а ваше приложение получает доступ к секретам из файлов, созданных в контейнере.

   > Если нет возможности модифицировать приложение, используйте этот вариант. Он проще в реализации, но менее безопасный, так как секретные данные хранятся в файлах в контейнере.

1. В хранилище обращается приложение-прослойка, и пользовательское приложение получает доступ к секретам из переменных среды.

   > Если нет возможности читать из файлов, можно использовать этот вариант, но он небезопасен. При таком подходе секретные данные хранятся в Kubernetes (а так же в etcd) и потенциально могут быть прочитаны на любом узле кластера.

<table>
<thead>
<tr>
<th>Вариант доставки</th>
<th>Потребление ресурсов</th>
<th>Как приложение получает данные?</th>
<th>Где хранится в Kubernetes?</th>
<th>Статус</th>
</tr>
</thead>
<tbody>
<tr>
<td><a style="color: ##0066FF;" href="#вариант-1-получение-секретов-самим-приложением">Приложение</a></td>
<td>Не меняется</td>
<td>Напрямую из хранилища секретов</td>
<td>Не хранится</td>
<td>Реализовано</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#механизм-csi">Механизм CSI</a></td>
<td>Два пода на каждую ноду (daemonset)</td>
<td><ul><li>Из дискового тома (как файл)</li><li>Из переменной окружения</li></ul></td>
<td>Не хранится</td>
<td>Реализовано</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#вариант-3-инъекция-entrypoint">Инъекция entrypoint</a></td>
<td>Один под на каждую ноду (daemonset)</td>
<td>Секреты доставляются из хранилища в момент запуска приложения в виде переменных окружения</td>
<td>Не хранится</td>
<td>В процессе реализации</td>
</tr>
<tr>
<td><a style="color: ##0066FF;" href="#вариант-4-доставка-секретов-через-механизмы-kubernetes">Секреты Kubernetes</a></td>
<td>Одно приложение на кластер (deployment)</td>
<td><ul><li>Из дискового тома (как файл)</li><li>Из переменной окружения</li></ul></td>
<td>Хранится в Secrets</td>
<td>Планируется</td>
</tr>
<tr>
<td><a style="color: #A9A9A9; font-style: italic;" href="#справочно-инжектор-vault-agent">Инжектор vault-agent</a></td>
<td style="color: #A9A9A9; font-style: italic;">По одному агенту на каждый под (sidecar)</td>
<td style="color: #A9A9A9; font-style: italic;">Из дискового тома (как файл)</td>
<td style="color: #A9A9A9; font-style: italic;">Не хранится</td>
<td style="color: #A9A9A9; font-style: italic;"><sup><b>*</b></sup>Не будет реализовано</td>
</tr>
</tbody>
</table>

<i><sup>*</sup>Поддержка отсутствует и не планируется, поскольку этот вариант не имеет преимуществ перед использованием механизма CSI.</i>

##### Вариант №1: Получение секретов самим приложением

> *Статус:* наиболее безопасный вариант. Рекомендован к использованию, если есть возможность модификации приложений.

Приложение обращается к API `stronghold` и запрашивает необходимый секрет по HTTPS-протоколу с использованием токена авторизации (токен из SA).

Плюсы:
- Секрет, полученный приложением, нигде не хранится, кроме как в самом приложении, нет опасности что он будет скомпрометирован в процессе передачи.

Минусы:

- Требует доработки приложения для возможности работы со `stronghold`.
- Требует повторения реализации доступа к секретам в каждом приложении. В случае обновления библиотеки требует пересборки всех приложений.
- Приложение должно поддерживать TLS и проверку сертификатов.
- Нет кэширования. При перезапуске приложения нужно повторно запросить секрет напрямую из хранилища.

##### Вариант №2: Доставка секретов через файлы

###### Механизм CSI

> *Статус:* безопасный вариант. Рекомендован к использованию, если отсутствует возможность модификация приложений.

При создании подов, запрашивающих тома CSI, драйвер хранилища секретов CSI отправляет запрос к Vault CSI. Затем Vault CSI использует указанный SecretProviderClass и ServiceAccount пода для получения секретов из хранилища и монтирования их в том пода.

###### Инъекция переменных окружений:

Если нет возможности изменить код приложения, то можно реализовать безопасную инъекцию секрета в качестве переменной окружения для приложения.

Для этого нужно:
- прочитать все файлы, примонтированные CSI в контейнер;
- определить переменные окружения с именами, соответствующими именам файлов, и значениями, соответствующим содержимому файлов.
- запустить оригинальное приложение.

Пример на Bash:

```bash
bash -c "for file in $(ls /mnt/secrets); do export  $file=$(cat /mnt/secrets/$file); done ; exec my_original_file_to_startup"
```

Плюсы:

- Всего два контейнера с прогнозируемыми ресурсами на каждом узле для обслуживания системы доставки секретов в приложения;
- Создание ресурсов _SecretsStore/SecretProviderClass_ уменьшает количество повторяемого кода по сравнению с другими вариантами реализации vault agent;
- При необходимости есть возможность создавать копию секрета из хранилища в виде секрета Kubernetes.
- Секрет извлекается из хранилища драйвером CSI на этапе создания контейнера. Это означает, что запуск подов заблокируется до тех пор, пока секреты не будут прочитаны из хранилища и записаны в том.

##### Вариант №3: Инъекция entrypoint

###### Доставка переменных окружения через инъекцию entrypoint в контейнер

> *Статус:* безопасный вариант. В процессе реализации.

Переменные доставляются из хранилища в момент запуска приложения и находятся только в памяти. В момент первого этапа реализации метода переменные будут доставляться через entrypoint, проброшенный в контейнер. В дальнейшем планируется интеграция функционала доставки секретов в containerd.

##### Вариант №4: Доставка секретов через механизмы Kubernetes

> *Статус:* небезопасный вариант, не рекомендован к использованию. Поддержка отсутствует, но планируется в будущем.

Этот метод интеграции, который реализует оператор секретов Kubernetes с набором CRD, отвечающих за синхронизацию секретов из Vault в секреты Kubernetes.

Минусы:

- Секрет находится и в хранилище секретов, и в секрете Kubernetes (доступном через API Kubernetes). Секрет также хранится в etcd и потенциально может быть считан на любом узле кластера или извлечён из резервной копии etcd. Нет возможности не хранить данные в секретах Kubernetes.

Плюсы:

- Классический способ передачи секрета в приложение через переменные окружения — достаточно подключить секрет Kubernetes.

##### Справочно: Инжектор vault-agent

> *Статус:* не имеет плюсов в сравнении с механизмом CSI. Поддержка отсутствует и не планируется, поскольку этот вариант не имеет преимуществ перед использованием механизма CSI.

При создании пода происходит мутация, которая добавляет контейнер с vault-agent. Агент обращается к хранилищу секретов, извлекает их, и помещает в общий том на диске, к которому может обратиться приложение.

Минусы:

- Для каждого пода нужен sidecar-контейнер, который так или иначе потребляет ресурсы.

  Например, возьмем кластер в котором 50 приложений, и каждое приложение имеет от 3 до 15 реплик. Так как для каждого sidecar-контейнера с агентом нужно выделить ресурсы CPU и памяти, то даже при незначительных ресурсах для sidecar-контейнера в размере 0.05 CPU и 100 MiB памяти, на все приложения в сумме получаются десятки ядер CPU и десятки ГБ памяти.
- Так как сбор метрик осуществляется с каждого контейнера, то с таким подходом мы получим в два раза больше метрик только по контейнерам.

### Модуль secrets-store-integration: настройки

> Если модуль был выключен и вы его включаете, обратите внимание на глобальный параметр [publicDomainTemplate](./deckhouse-configure-global.html#параметры). Укажите его, если он не указан, иначе Ingress-ресурсы для служебных компонентов Deckhouse (dashboard, user-auth, grafana, upmeter  и т. п.) создаваться не будут.

Конфигурация Ingress-контроллеров выполняется с помощью Custom Resource [IngressNginxController](cr.html#ingressnginxcontroller).

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['secrets-store-integration'].config-values | format_module_configuration: moduleKebabName }}

### Модуль secrets-store-integration: Custom Resources
{{ site.data.schemas.secrets-store-integration.crds.secrets-store-import | format_crd: "secrets-store-integration" }}
## Подсистема Хранение данных

### Модуль snapshot-controller

Этот модуль включает поддержку снапшотов для совместимых CSI-драйверов в кластере Kubernetes.

CSI-драйверы в Deckhouse, которые поддерживают снапшоты:
- csi-ceph
- sds-replicated-volume
- csi-nfs

### Модуль snapshot-controller: настройки

> Модуль работает только в кластерах Kubernetes, начиная с версии 1.20.

В общем случае конфигурация модуля не требуется.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['snapshot-controller'].config-values | format_module_configuration: moduleKebabName }}

### Модуль csi-ceph

Модуль устанавливает и настраивает CSI-драйвер для RBD и CephFS.

Настройка выполняется посредством [custom resources](cr.html), что позволяет подключить более одного Ceph-кластера (UUID не должны совпадать).

### Модуль csi-ceph: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['csi-ceph'].config-values | format_module_configuration: moduleKebabName }}

### Модуль csi-ceph: custom resources
{{ site.data.schemas.csi-ceph.crds.cephclusterauthentication | format_crd: "csi-ceph" }}
{{ site.data.schemas.csi-ceph.crds.cephclusterconnection | format_crd: "csi-ceph" }}
{{ site.data.schemas.csi-ceph.crds.cephstorageclass | format_crd: "csi-ceph" }}

### Модуль csi-ceph: примеры

#### Пример описания `CephClusterConnection`

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterConnection
metadata:
  name: ceph-cluster-1
spec:
  clusterID: 0324bfe8-c36a-4829-bacd-9e28b6480de9
  monitors:
  - 172.20.1.28:6789
  - 172.20.1.34:6789
  - 172.20.1.37:6789
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get cephclusterconnection <имя cephclusterconnection>
```
#### Пример описания `CephClusterAuthentication`

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephClusterAuthentication
metadata:
  name: ceph-auth-1
spec:
  userID: user
  userKey: AQDiVXVmBJVRLxAAg65PhODrtwbwSWrjJwssUg==
```

- Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get cephclusterauthentication <имя cephclusterauthentication>
```

#### Пример описания `CephStorageClass`

##### RBD

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-rbd-sc
spec:
  clusterConnectionName: ceph-cluster-1
  clusterAuthenticationName: ceph-auth-1
  reclaimPolicy: Delete
  type: RBD
  rbd:
    defaultFSType: ext4
    pool: ceph-rbd-pool  
```

##### CephFS

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: CephStorageClass
metadata:
  name: ceph-fs-sc
spec:
  clusterConnectionName: ceph-cluster-1
  clusterAuthenticationName: ceph-auth-1
  reclaimPolicy: Delete
  type: CephFS
  cephFS:
    fsName: cephfs
```

##### Проверить создание объекта можно командой (Phase должен быть `Created`):

```shell
kubectl get cephstorageclass <имя storage class>
```

### Модуль csi-ceph: FAQ

#### Как получить список томов RBD, разделенный по узлам?

```shell
kubectl -n d8-csi-ceph get po -l app=csi-node-rbd -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName --no-headers \
  | awk '{print "echo "$2"; kubectl -n d8-csi-ceph exec  "$1" -c node -- rbd showmapped"}' | bash
```

### Модуль csi-nfs

Модуль предоставляет CSI для управления томами на основе `NFS`. Модуль позволяет создавать `StorageClass` в `Kubernetes` через создание [пользовательских ресурсов Kubernetes](./cr.html#nfsstorageclass) `NFSStorageClass`.

> **Внимание!** Создание `StorageClass` для CSI-драйвера `nfs.csi.k8s.io` пользователем запрещено.

#### Системные требования и рекомендации

##### Требования

- Использование стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](/supported_versions.html#linux);
- Наличие развернутого и настроенного `NFS` сервера.

#### Быстрый старт

Все команды следует выполнять на машине, имеющей доступ к API Kubernetes с правами администратора.

##### Включение модуля

- Включить модуль `csi-nfs`.  Это приведет к тому, что на всех узлах кластера будет:
    - зарегистрирован CSI драйвер;
    - запущены служебные поды компонентов `csi-nfs`.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
EOF
```

- Дождаться, когда модуль перейдет в состояние `Ready`.

```shell
kubectl get module csi-nfs -w
```

##### Создание StorageClass

Для создания StorageClass необходимо использовать ресурс [NFSStorageClass](./cr.html#nfsstorageclass). Пример команды для создания такого ресурса:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: 10.223.187.3
    share: /
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```
 
Для каждой `PV` будет создаваться каталог `<директория из share>/<имя PV>`.

##### Проверка работоспособности модуля.

Проверить работоспособность модуля можно [так](./faq.html#как-проверить-работоспособность-модуля)

### Модуль csi-nfs: FAQ

#### Как проверить работоспособность модуля?

Для этого необходимо проверить состояние подов в namespace `d8-csi-nfs`. Все поды должны быть в состоянии `Running` или `Completed` и запущены на всех узлах.

```shell
kubectl -n d8-csi-nfs get pod -owide -w
```

#### Возможно ли изменение параметров NFS-сервера уже созданных PV?

Нет, данные для подключения к NFS-серверу сохраняются непосредственно в манифесте PV, и не подлежат изменению. Изменение Storage Class также не повлечет изменений настроек подключения в уже существующих PV.

#### Как делать снимки томов (snapshots)?

В `csi-nfs` снимки создаются путем архивирования папки тома. Архив сохраняется в корне папки NFS сервера, указанной в параметре `spec.connection.share`.

##### Шаг 1: Включение snapshot-controller

Для начала необходимо включить snapshot-controller:

```shell
kubectl apply -f -<<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: snapshot-controller
spec:
  enabled: true
  version: 1
EOF

```

##### Шаг 2: Создание снимка тома

Теперь вы можете создавать снимки томов. Для этого выполните следующую команду, указав нужные параметры:

```shell
kubectl apply -f -<<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-snapshot
  namespace: <имя namespace, в котором находится PVC>
spec:
  volumeSnapshotClassName: csi-nfs-snapshot-class
  source:
    persistentVolumeClaimName: <имя PVC, для которого необходимо создать снимок>
EOF

```

##### Шаг 3: Проверка состояния снимка 

Чтобы проверить состояние созданного снимка, выполните команду:

```shell
kubectl get volumesnapshot

```

Эта команда покажет список всех снимков и их текущее состояние.

### Модуль sds-local-volume

Модуль предназначен для управления локальным блочным хранилищем на базе LVM. С его помощью можно создавать StorageClass в Kubernetes, используя ресурс [LocalStorageClass](cr.html#localstorageclass).

#### Шаги настройки модуля

Для корректной работы модуля `sds-local-volume` выполните следующие шаги:

- Настройте LVMVolumeGroup.

  Перед созданием StorageClass необходимо создать ресурс [LVMVolumeGroup](./sds-node-configurator/stable/cr.html#lvmvolumegroup) модуля `sds-node-configurator` на узлах кластера.

- Включите модуль [sds-node-configurator](./sds-node-configurator/stable/).

  Убедитесь, что модуль `sds-node-configurator` включен **до** включения модуля `sds-local-volume`.

- Создайте соответствующие StorageClass'ы.

  Создание StorageClass для CSI-драйвера `local.csi.storage.deckhouse.io` пользователем **запрещено**.

Модуль поддерживает два режима работы: LVM и LVMThin.
У каждого из них есть свои особенности, преимущества и ограничения. Подробнее о различиях можно узнать в [FAQ](./faq.html#когда-следует-использовать-lvm-а-когда-lvmthin).

#### Быстрый старт

Все команды выполняются на машине с доступом к API Kubernetes и правами администратора.

##### Включение модулей

Включение модуля `sds-node-configurator`:

1. Создайте ресурс ModuleConfig для включения модуля:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-node-configurator
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь состояния модуля `Ready`. На этом этапе не требуется проверять поды в пространстве имен `d8-sds-node-configurator`.

   ```shell
   kubectl get modules sds-node-configurator -w
   ```

Включение модуля `sds-local-volume`:

1. Активируйте модуль `sds-local-volume`. Перед включением рекомендуется ознакомиться с [доступными настройками](./configuration.html). Пример ниже запускает модуль с настройками по умолчанию, что приведет к созданию служебных подов компонента `sds-local-volume` на всех узлах кластера:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: sds-local-volume
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Дождитесь состояния модуля `Ready`.

   ```shell
   kubectl get modules sds-local-volume -w
   ```

1. Убедитесь, что в пространствах имен `d8-sds-local-volume` и `d8-sds-node-configurator` все поды находятся в статусе `Running` или `Completed` и запущены на всех узлах, где планируется использовать ресурсы LVM.

   ```shell
   kubectl -n d8-sds-local-volume get pod -owide -w
   kubectl -n d8-sds-node-configurator get pod -o wide -w
   ```

##### Подготовка узлов к созданию хранилищ на них

Для корректной работы хранилищ на узлах необходимо, чтобы поды `sds-local-volume-csi-node` были запущены на выбранных узлах.

По умолчанию эти поды запускаются на всех узлах кластера. Проверить их наличие можно с помощью команды:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

Размещение подов `sds-local-volume-csi-node` управляется специальными метками (nodeSelector). Эти метки задаются в параметре [spec.settings.dataNodes.nodeSelector](configuration.html#parameters-datanodes-nodeselector) модуля. Подробнее о настройке и выборе узлов для работы модуля можно узнать [в FAQ](./faq.html#я-не-хочу-чтобы-модуль-использовался-на-всех-узлах-кластера-как-мне-выбрать-желаемые-узлы).

##### Настройка хранилища на узлах

Для настройки хранилища на узлах необходимо создать группы томов LVM с использованием ресурсов LVMVolumeGroup. В данном примере создается хранилище Thick.

{{< alert level="warning" >}}
Перед созданием ресурса LVMVolumeGroup убедитесь, что на данном узле запущен под `sds-local-volume-csi-node`. Это можно сделать командой:

```shell
kubectl -n d8-sds-local-volume get pod -owide
```

{{< /alert >}}

###### Шаги настройки

1. Получите все ресурсы [BlockDevice](./sds-node-configurator/stable/cr.html#blockdevice), которые доступны в вашем кластере:

   ```shell
   kubectl get bd
  
   NAME                                           NODE       CONSUMABLE   SIZE           PATH
   dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa   worker-0   false        976762584Ki    /dev/nvme1n1
   dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd   worker-0   false        894006140416   /dev/nvme0n1p6
   dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0   worker-1   false        976762584Ki    /dev/nvme1n1
   dev-b103062f879a2349a9c5f054e0366594568de68d   worker-1   false        894006140416   /dev/nvme0n1p6
   dev-53d904f18b912187ac82de29af06a34d9ae23199   worker-2   false        976762584Ki    /dev/nvme1n1
   dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1   worker-2   false        894006140416   /dev/nvme0n1p6
   ```

1. Создайте ресурс [LVMVolumeGroup](./sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-0`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-0" # The name can be any fully qualified resource name in Kubernetes. This LVMVolumeGroup resource name will be used to create LocalStorageClass in the future
   spec:
     type: Local
     local:
       nodeName: "worker-0"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-ef4fb06b63d2c05fb6ee83008b55e486aa1161aa
             - dev-0cfc0d07f353598e329d34f3821bed992c1ffbcd
     actualVGNameOnTheNode: "vg-1" # the name of the LVM VG to be created from the above block devices on the node 
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-0 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-0` из блочных устройств `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](./sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-1`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-1"
   spec:
     type: Local
     local:
       nodeName: "worker-1"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-7e4df1ddf2a1b05a79f9481cdf56d29891a9f9d0
             - dev-b103062f879a2349a9c5f054e0366594568de68d
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-1 -w
   ```

   Если ресурс перешел в состояние `Ready`, это значит, что на узле `worker-1` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LVMVolumeGroup](./sds-node-configurator/stable/cr.html#lvmvolumegroup) для узла `worker-2`:

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LVMVolumeGroup
   metadata:
     name: "vg-1-on-worker-2"
   spec:
     type: Local
     local:
       nodeName: "worker-2"
     blockDeviceSelector:
       matchExpressions:
         - key: kubernetes.io/metadata.name
           operator: In
           values:
             - dev-53d904f18b912187ac82de29af06a34d9ae23199
             - dev-6c5abbd549100834c6b1668c8f89fb97872ee2b1
     actualVGNameOnTheNode: "vg-1"
   EOF
   ```

1. Дождитесь, когда созданный ресурс LVMVolumeGroup перейдет в состояние `Ready`:

   ```shell
   kubectl get lvg vg-1-on-worker-2 -w
   ```

   Если ресурс перешел в состояние `Ready`, то это значит, что на узле `worker-2` из блочного устройства `/dev/nvme1n1` и `/dev/nvme0n1p6` была создана LVM VG с именем `vg-1`.

1. Создайте ресурс [LocalStorageClass](./cr.html#localstorageclass):

   ```yaml
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: LocalStorageClass
   metadata:
     name: local-storage-class
   spec:
     lvm:
       lvmVolumeGroups:
         - name: vg-1-on-worker-0
         - name: vg-1-on-worker-1
         - name: vg-1-on-worker-2
       type: Thick
     reclaimPolicy: Delete
     volumeBindingMode: WaitForFirstConsumer
   EOF
   ```

1. Дождитесь, когда созданный ресурс LocalStorageClass перейдет в состояние `Created`:

   ```shell
   kubectl get lsc local-storage-class -w
   ```

1. Проверьте, что соответствующий StorageClass создался:

   ```shell
   kubectl get sc local-storage-class
   ```

Если StorageClass с именем `local-storage-class` появился, значит настройка модуля `sds-local-volume` завершена. Теперь пользователи могут создавать PVC, указывая StorageClass с именем `local-storage-class`.

#### Системные требования и рекомендации

- Используйте стоковые ядра, поставляемые вместе с [поддерживаемыми дистрибутивами](/supported_versions.html#linux).
- Не используйте другой SDS (Software defined storage) для предоставления дисков SDS Deckhouse.

### Модуль sds-local-volume: настройки

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['sds-local-volume'].config-values | format_module_configuration: moduleKebabName }}

### Модуль sds-local-volume: Custom Resources
{{ site.data.schemas.sds-local-volume.crds.localstorageclass | format_crd: "sds-local-volume" }}

### Модуль sds-local-volume: FAQ

#### Когда следует использовать LVM, а когда LVMThin?

- LVM проще и обладает высокой производительностью, сравнимой с производительностью накопителя;
- LVMThin позволяет использовать overprovisioning, но производительность ниже, чем у LVM.

{{< alert level="warning" >}}
Overprovisioning в LVMThin нужно использовать с осторожностью, контроллируя наличие свободного места в пуле (В системе мониторинга кластера есть отдельные события при достижении 20%, 10%, 5% и 1% свободного места в пуле)

При отсутствии свободного места в пуле будет наблюдатся деградация в работе модуля в целом, а также существует реальная вероятность потери данных!
{{< /alert >}}

#### Как назначить StorageClass по умолчанию?

Добавьте аннотацию `storageclass.kubernetes.io/is-default-class: "true"` в соответствующий ресурс StorageClass:

```shell
kubectl annotate storageclasses.storage.k8s.io <storageClassName> storageclass.kubernetes.io/is-default-class=true
```

#### Я не хочу, чтобы модуль использовался на всех узлах кластера. Как мне выбрать желаемые узлы?

Узлы, которые будут задействованы модулем, определяются специальными метками, указанными в поле `nodeSelector` в настройках модуля.

Для отображения и редактирования настроек модуля, можно выполнить команду:

```shell
kubectl edit mc sds-local-volume
```

Примерный вывод команды:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: sds-local-volume
spec:
  enabled: true
  settings:
    dataNodes:
      nodeSelector:
        my-custom-label-key: my-custom-label-value
status:
  message: ""
  version: "1"
```

Для отображения существующих меток, указанных в поле `nodeSelector`, можно выполнить команду:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Узлы, метки которых включают в себя набор, указанный в настройках, выбираются модулем как целевые для использования. Соответственно, изменяя поле `nodeSelector` Вы можете влиять на список узлов, которые будут использованы модулем.

{{< alert level="warning" >}}
В поле `nodeSelector` может быть указано любое количество меток, но важно, чтобы каждая из указанных меток присутствовала на узле, который Вы собираетесь использовать для работы с модулем. Именно при наличии всех указанных меток на выбранном узле, произойдет запуск pod-а `sds-local-volume-csi-node`.
{{< /alert >}}

После добавление меток на узлах должны быть запущены pod-ы `sds-local-volume-csi-node`. Проверить их наличие можно командой:

```shell
 kubectl -n d8-sds-local-volume get pod -owide
 ```

#### Почему не удается создать PVC на выбранном узле с помощью модуля?

Пожалуйста, проверьте, что на выбранном узле работает pod `sds-local-volume-csi-node`.

```shell
kubectl -n d8-sds-local-volume get po -owide
```

Если pod отсутствует, пожалуйста, убедитесь, что на выбранном узле присутствуют все метки, указанные в настройках модуля в поле `nodeSelector`. Подробнее об этом [здесь](#служебные-pod-ы-компонентов-sds-local-volume-не-создаются-на-нужном-мне-узле-почему).

#### Я хочу вывести узел из-под управления модуля, что делать?

Для вывода узла из-под управления модуля необходимо убрать метки, указанные в поле `nodeSelector` в настройках модуля `sds-local-volume`.

Проверить наличие существующих меток в `nodeSelector` можно командой:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Снимите указанные в `nodeSelector` метки с желаемых узлов.

```shell
kubectl label node %node-name% %label-from-selector%-
```

{{< alert level="warning" >}}
Для снятия метки необходимо после его ключа вместо значения сразу же поставить знак минуса.
{{< /alert >}}

В результате pod `sds-local-volume-csi-node` должен быть удален с желаемого узла. Для проверки состояния можно выполнить команду:

```shell
kubectl -n d8-sds-local-volume get po -owide
```

Если pod `sds-local-volume-csi-node` после удаления метки `nodeSelector` все же остался на узле, пожалуйста, убедитесь, что указанные в конфиге `d8-sds-local-volume-controller-config` в `nodeSelector` метки действительно успешно снялись с выбранного узла.
Проверить это можно командой:

```shell
kubectl get node %node-name% --show-labels
```

Если метки из `nodeSelector` не присутствуют на узле, то убедитесь, что данному узлу не принадлежат `LVMVolumeGroup` ресурсы, использующиеся `LocalStorageClass` ресурсами. Подробнее об этой проверке можно прочитать [здесь](#как-проверить-имеются-ли-зависимые-ресурсы-lvmvolumegroup-на-узле).

{{< alert level="warning" >}}
Обратите внимание, что на ресурсах `LVMVolumeGroup` и `LocalStorageClass`, из-за которых не удается вывести узел из-под управления модуля будет отображена метка `storage.deckhouse.io/sds-local-volume-candidate-for-eviction`.

На самом узле будет присутствовать метка `storage.deckhouse.io/sds-local-volume-need-manual-eviction`.
{{< /alert >}}

#### Как проверить, имеются ли зависимые ресурсы `LVMVolumeGroup` на узле?

Для проверки таковых ресурсов необходимо выполнить следующие шаги:
1. Отобразить имеющиеся `LocalStorageClass` ресурсы

   ```shell
   kubectl get lsc
   ```

2. Проверить у каждого из них список используемых `LVMVolumeGroup` ресурсов

   > Вы можете сразу отобразить содержимое всех `LocalStorageClass` ресурсов, выполнив команду:
   >
   > ```shell
   > kubectl get lsc -oyaml
   > ```

   ```shell
   kubectl get lsc %lsc-name% -oyaml
   ```

   Примерный вид `LocalStorageClass`

   ```yaml
   apiVersion: v1
   items:
   - apiVersion: storage.deckhouse.io/v1alpha1
     kind: LocalStorageClass
     metadata:
       finalizers:
       - localstorageclass.storage.deckhouse.io
       name: test-sc
     spec:
       lvm:
         lvmVolumeGroups:
         - name: test-vg
         type: Thick
       reclaimPolicy: Delete
       volumeBindingMode: WaitForFirstConsumer
     status:
       phase: Created
   kind: List
   ```

   > Обратите внимание на поле spec.lvm.lvmVolumeGroups - именно в нем указаны используемые `LVMVolumeGroup` ресурсы.

3. Отобразите список существующих `LVMVolumeGroup` ресурсов

   ```shell
   kubectl get lvg
   ```

   Примерный вывод `LVMVolumeGroup` ресурсов:

   ```text
   NAME              HEALTH        NODE                         SIZE       ALLOCATED SIZE   VG        AGE
   lvg-on-worker-0   Operational   node-worker-0   40956Mi    0                test-vg   15d
   lvg-on-worker-1   Operational   node-worker-1   61436Mi    0                test-vg   15d
   lvg-on-worker-2   Operational   node-worker-2   122876Mi   0                test-vg   15d
   lvg-on-worker-3   Operational   node-worker-3   307196Mi   0                test-vg   15d
   lvg-on-worker-4   Operational   node-worker-4   307196Mi   0                test-vg   15d
   lvg-on-worker-5   Operational   node-worker-5   204796Mi   0                test-vg   15d
   ```

4. Проверьте, что на узле, который вы собираетесь вывести из-под управления модуля, не присутствует какой-либо `LVMVolumeGroup` ресурс, используемый в `LocalStorageClass` ресурсах.

   Во избежание непредвиденной потери контроля за уже созданными с помощью модуля томами пользователю необходимо вручную удалить зависимые ресурсы, совершив необходимые операции над томом.

#### Я убрал метки с узла, но pod `sds-local-volume-csi-node` остался. Почему так произошло?

Вероятнее всего, на узле присутствуют `LVMVolumeGroup` ресурсы, которые используются в одном из `LocalStorageClass` ресурсов.

Во избежание непредвиденной потери контроля за уже созданными с помощью модуля томами пользователю необходимо вручную удалить зависимые ресурсы, совершив необходимые операции над томом.

Процесс проверки на наличие вышеуказанных ресурсов описан [здесь](#как-проверить-имеются-ли-зависимые-ресурсы-lvmvolumegroup-на-узле).

#### Служебные pod-ы компонентов `sds-local-volume` не создаются на нужном мне узле. Почему?

С высокой вероятностью проблемы связаны с метками на узле.

Узлы, которые будут задействованы модулем, определяются специальными метками, указанными в поле `nodeSelector` в настройках модуля.

Для отображения существующих меток, указанных в поле `nodeSelector`, можно выполнить команду:

```shell
kubectl get mc sds-local-volume -o=jsonpath={.spec.settings.dataNodes.nodeSelector}
```

Примерный вывод команды:

```yaml
nodeSelector:
  my-custom-label-key: my-custom-label-value
```

Узлы, метки которых включают в себя набор, указанный в настройках, выбираются модулем как целевые для использования.

Также Вы можете дополнительно проверить селекторы, которые используются модулем в конфиге секрета `d8-sds-local-volume-controller-config` в пространстве имен `d8-sds-local-volume`.

```shell
kubectl -n d8-sds-local-volume get secret d8-sds-local-volume-controller-config  -o jsonpath='{.data.config}' | base64 --decode
```

Примерный вывод команды:

```yaml
nodeSelector:
  kubernetes.io/os: linux
  my-custom-label-key: my-custom-label-value
```

В выводе данной команды должны быть указаны все метки из настроек модуля `data.nodeSelector`, а также `kubernetes.io/os: linux`.

Проверьте метки на нужном вам узле:

```shell
kubectl get node %node-name% --show-labels
```

При необходимости добавьте недостающие метки на желаемый узел:

```shell
kubectl label node %node-name% my-custom-label-key=my-custom-label-value
```

Если метки присутствуют, необходимо проверить наличие метки `storage.deckhouse.io/sds-local-volume-node=` на узле. Если метка отсутствует, следует проверить работает ли `sds-local-volume-controller`, и в случае его работоспособности, проверить логи:

```shell
kubectl -n d8-sds-local-volume get po -l app=sds-local-volume-controller
kubectl -n d8-sds-local-volume logs -l app=sds-local-volume-controller
```

#### Как переместить данные между PVC?

Скопируйте следующий скрипт в файл migrate.sh на любом master узле.
Использование: migrate.sh NAMESPACE SOURCE_PVC_NAME DESTINATION_PVC_NAME

```shell
###!/bin/bash

ns=$1
src=$2
dst=$3

if [[ -z $3 ]]; then
  echo "You must give as args: namespace source_pvc_name destination_pvc_name"
  exit 1
fi

echo "Creating job yaml"
cat > migrate-job.yaml << EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-pv-$src
  namespace: $ns
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: debian
        command: [ "/bin/bash", "-c" ]
        args:
          -
            apt-get update && apt-get install -y rsync &&
            ls -lah /src_vol /dst_vol &&
            df -h &&
            rsync -avPS --delete /src_vol/ /dst_vol/ &&
            ls -lah /dst_vol/ &&
            du -shxc /src_vol/ /dst_vol/
        volumeMounts:
        - mountPath: /src_vol
          name: src
          readOnly: true
        - mountPath: /dst_vol
          name: dst
      restartPolicy: Never
      volumes:
      - name: src
        persistentVolumeClaim:
          claimName: $src
      - name: dst
        persistentVolumeClaim:
          claimName: $dst
  backoffLimit: 1
EOF

kubectl create -f migrate-job.yaml
kubectl -n $ns get jobs -o wide
kubectl_completed_check=0

echo "Waiting for data migration to be completed"
while [[ $kubectl_completed_check -eq 0 ]]; do
   kubectl -n $ns get pods | grep migrate-pv-$src
   sleep 5
   kubectl_completed_check=`kubectl -n $ns get pods | grep migrate-pv-$src | grep "Completed" | wc -l`
done
echo "Data migration completed"
```

### Модуль sds-node-configurator
{{< alert level="warning" >}}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](/supported_versions.html#linux).

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{{< /alert >}}

Модуль управляет `LVM` на узлах кластера через [пользовательские ресурсы Kubernetes](./cr.html), выполняя следующие операции:

  - Обнаружение блочных устройств и создание/обновление/удаление соответствующих им [ресурсов BlockDevice](./cr.html#blockdevice).

   > **Внимание!** Ручное создание и изменение ресурса `BlockDevice` запрещено.

  - Обнаружение на узлах `LVM Volume Group` с LVM тегом `storage.deckhouse.io/enabled=true` и `Thin-pool` на них, а также управление соответствующими [ресурсами LVMVolumeGroup](./cr.html#lvmvolumegroup). Модуль автоматически создает ресурс `LVMVolumeGroup`, если его еще не существует для обнаруженной `LVM Volume Group`.

  - Сканирование на узлах `LVM Physical Volumes`, которые входят в управляемые `LVM Volume Group`. В случае расширения размеров нижестоящих блочных устройств, соотвующие `LVM Physical Volumes` будут автоматически расширены (произойдёт `pvresize`).

  > **Внимание!** Уменьшение размеров блочного устройства не поддерживается.

  - Создание/расширение/удаление `LVM Volume Group` на узле в соответствии с пользовательскими изменениями в ресурсах `LVMVolumeGroup`. [Примеры использования](./usage.html#работа-с-ресурсами-lvmvolumegroup)

### Модуль sds-node-configurator: настройки

Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](/supported_versions.html#linux).

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.

 
<!-- SCHEMA -->
#### {{ site.data.i18n.common['parameters'][page.lang] }}
{{ site.data.schemas['sds-node-configurator'].config-values | format_module_configuration: moduleKebabName }}

### Модуль sds-node-configurator: Custom Resources
{{ site.data.schemas.sds-node-configurator.crds.blockdevices | format_crd: "sds-node-configurator" }}
{{ site.data.schemas.sds-node-configurator.crds.lvmlogicalvolume | format_crd: "sds-node-configurator" }}
{{ site.data.schemas.sds-node-configurator.crds.lvmlogicalvolumesnapshot | format_crd: "sds-node-configurator" }}
{{ site.data.schemas.sds-node-configurator.crds.lvmvolumegroup | format_crd: "sds-node-configurator" }}
{{ site.data.schemas.sds-node-configurator.crds.lvmvolumegroupbackup | format_crd: "sds-node-configurator" }}
{{ site.data.schemas.sds-node-configurator.crds.lvmvolumegroupset | format_crd: "sds-node-configurator" }}

###  Модуль sds-node-configurator: FAQ
{{< alert level="warning" >}}
Работоспособность модуля гарантируется только при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](/supported_versions.html#linux).

Работоспособность модуля при использовании других ядер или дистрибутивов возможна, но не гарантируется.
{{< /alert >}}

#### Почему в кластере не создаются ресурсы `BlockDevice` и `LVMVolumeGroup`?

* В большинстве случаев `BlockDevice`-ресурсы могут не создаваться по причине того, что существующие девайсы не проходят фильтрацию на стороне контроллера. Пожалуйста, убедитесь, что ваши девайсы соответствуют указанным [требованиям](./usage.html#требования-контроллера-к-девайсу).

* `LVMVolumeGroup`-ресурсы могут не создаваться по причине отсутствия в кластере `BlockDevice`-ресурсов, так как их имена используются в спецификации `LVMVolumeGroup`.

* В том случае, если `BlockDevice`-ресурсы существуют, а `LVMVolumeGroup`-ресурсы отсутствуют, пожалуйста, убедитесь, что у существующих `LVM Volume Group` на узле имеется специальный тег `storage.deckhouse.io/enabled=true`.

#### Я выполнил команду на удаление ресурса `LVMVolumeGroup`, но и ресурс, и `Volume Group` осталась. Почему так?

Такая ситуация возможна в двух случаях: 

1. В `Volume Group` имеются `LV`. 
Контроллер не берет ответственность за удаление LV с узла, поэтому, если в созданной с помощью ресурса `Volume Group` имеются какие-либо логические тома, Вам необходимо вручную удалить их на узле. После этого и ресурс, и `Volume Group` (вместе с `PV`) будут удалены автоматически.

2. На ресурсе имеется аннотация `storage.deckhouse.io/deletion-protection`.
Данная аннотация защищает удаление ресурса и, как следствие, созданной им `Volume Group`. Вам необходимо самостоятельно убрать аннотацию командой 
```shell
kubectl annotate lvg %lvg-name% storage.deckhouse.io/deletion-protection-
```

После выполнения данной команды и ресурс, и `Volume Group` будут удалены автоматически.

#### Я пытаюсь создать `Volume Group`, используя ресурс `LVMVolumeGroup`, но у меня ничего не получается. Почему?

Скорее всего, ваш ресурс не проходит валидацию со стороны контроллера (при этом, валидация со стороны Kubernetes прошла успешно).
С конкретной причиной неработоспособности вы можете ознакомиться в самом ресурсе в поле `status.message` либо обратиться
к логам контроллера.

Как правило, проблема кроется в некорректно указанных ресурсах `BlockDevice`. Пожалуйста, убедитесь, что выбранные
ресурсы удовлетворяют следующим требованиям:
- Поле `Consumable` имеет значение `true`.
- Для `Volume Group` типа `Local` указанные `BlockDevice` принадлежат одному узлу.<!-- > - Для `Volume Group` типа `Shared` указан единственный ресурс `BlockDevice`. -->
- Указаны актуальные имена ресурсов `BlockDevice`.

С полным списком ожидаемых значений вы можете ознакомиться с помощью [CR-референса](./cr.html) `LVMVolumeGroup`-ресурса.

#### Что произойдет, если я отключу один из девайсов в `Volume Group`? Соответствующий ресурс `LVMVolumeGroup` удалится?

Ресурс `LVMVolumeGroup` будет существовать до тех пор, пока существует соответствующая `Volume Group`. До тех пор, пока
существует хоть один девайс, `Volume Group` будет существовать, но в «нездоровом» состоянии.
Эти проблемы будут отображены в `status` ресурса.

После восстановления отключенного девайса на узле, `LVM Volume Group` восстановит свою работоспособность и соответствующий ресурс `LVMVolumeGroup` также отобразит актуальное состояние.

#### Как передать контроллеру управление существующей на узле `LVM Volume Group`?

Достаточно добавить LVM-тег `storage.deckhouse.io/enabled=true` на `LVM Volume Group` на узле: 

```shell
vgchange myvg-0 --addtag storage.deckhouse.io/enabled=true
```

#### Я хочу, чтобы контроллер перестал следить за `LVM Volume Group` на узле. Как мне это сделать?

Достаточно удалить LVM-тег `storage.deckhouse.io/enabled=true` у нужной `LVM Volume Group` на узле:

```shell
vgchange myvg-0 --deltag storage.deckhouse.io/enabled=true
```

После этого контроллер перестанет отслеживать выбранную `Volume Group` и самостоятельно удалит связанный с ней ресурс `LVMVolumeGroup`.

#### Я не вешал LVM-тег `storage.deckhouse.io/enabled=true` на `Volume Group`, но он появился. Как это возможно?

Это возможно в случае, если вы создавали `LVM Volume Group` через ресурс `LVMVolumeGroup` (в таком случае контроллер автоматически вешает данный LVM-тег на созданную `LVM Volume Group`). Либо на данной `Volume Group` или ее `Thin-pool` был LVM-тег модуля `linstor` — `linstor-*`.

При миграции с встроенного модуля `linstor` на модули `sds-node-configurator` и `sds-drbd` автоматически происходит изменение LVM-тегов `linstor-*` на LVM-тег `storage.deckhouse.io/enabled=true` в `Volume Group`. Таким образом, управление этими `Volume Group` передается модулю `sds-node-configurator`.

#### Как использовать ресурс `LVMVolumeGroupSet` для создания `LVMVolumeGroup`?

Для создания `LVMVolumeGroup` с помощью `LVMVolumeGroupSet` необходимо указать в спецификации `LVMVolumeGroupSet` селекторы для узлов и шаблон для создаваемых ресурсов `LVMVolumeGroup`. На данный момент поддерживается только стратегия `PerNode`, при которой контроллер создаст по одному ресурсу `LVMVolumeGroup` из шаблона для каждого узла, удовлетворяющего селектору.

Пример спецификации `LVMVolumeGroupSet`:

```yaml
apiVersion: storage.deckhouse.io/v1alpha1
kind: LVMVolumeGroupSet
metadata:
  name: my-lvm-volume-group-set
  labels:
    my-label: my-value
spec:
  strategy: PerNode
  nodeSelector:
    matchLabels:
      node-role.kubernetes.io/worker: ""
  lvmVolumeGroupTemplate:
    metadata:
      labels:
        my-label-for-lvg: my-value-for-lvg
    spec:
      type: Local
      blockDeviceSelector:
        matchLabels:
          status.blockdevice.storage.deckhouse.io/model: <model>
      actualVGNameOnTheNode: <actual-vg-name-on-the-node>


```
