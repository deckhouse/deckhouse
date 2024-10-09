---
title: Документация в PDF
permalink: ru/deckhouse-pdf-docs.html
lang: ru
sidebar: none
toc: true
---

## Введение в документацию

Приветствуем вас на главной странице документации Deckhouse Kubernetes Platform — платформы для управления Kubernetes-кластерами. Если вы еще не использовали платформу, рекомендуем начать с раздела {% if site.mode == 'module' %}[Быстрый старт🔗]({{ site.urls[page.lang] }}/gs/){% else %}[Быстрый старт](/gs/){% endif %}, где вы найдете пошаговые инструкции по развертыванию платформы на любой инфраструктуре.

Как быстро найти то, что нужно:
- Документация по Deckhouse Kubernetes Platform разных версий может отличаться. Выберите нужную версию в выпадающем списке вверху страницы. В списке доступны актуальные версии документации по Deckhouse.
- Если знаете, что ищете — используйте поиск.
- Если нужен конкретный модуль — найдите его в [списке](revision-comparison.html).
- Для поиска по области применения воспользуйтесь меню.

Если возникнут вопросы, вы можете обратиться за помощью в наш [Telegram-канал]({{ site.social_links[page.lang]['telegram'] }}). Мы обязательно поможем и проконсультируем.

Если вы используете Enterprise-редакцию, можете написать нам [на почту](mailto:support@deckhouse.ru), мы также окажем вам поддержку.

Хотите улучшить Deckhouse? Можете завести [задачу](https://github.com/deckhouse/deckhouse/issues/), предложить свою [идею](https://github.com/deckhouse/deckhouse/discussions) или [решение](https://github.com/deckhouse/deckhouse/blob/main/CONTRIBUTING.md) на GitHub.

А если вам хочется большего, присоединяйтесь к нашей [команде](https://job.flant.ru/)! Мы рады новым специалистам.

## Установка

<div class="docs__information warning active">
Страница находится в активной разработке и может содержать неполную информацию. Ниже приведена обзорная информация об этапах установки Deckhouse. Рекомендуем воспользоваться разделом [Быстрый старт](/gs/), где вы сможете найти пошаговые инструкции.
</div>

Инсталлятор Deckhouse доступен в виде образа контейнера. В основе инсталлятора лежит утилита [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>), в задачи которой входят:
* создание и настройка объектов в облачной инфраструктуре с помощью Terraform;
* установка необходимых пакетов ОС на узлах (в том числе установка пакетов Kubernetes);
* установка Deckhouse;
* создание, настройка узлов кластера Kubernetes;
* поддержание (приведение) кластера в состояние, описанное в конфигурации.

Варианты установки Deckhouse:
- **В поддерживаемом облаке.** В этом случае dhctl создает и настраивает все необходимые ресурсы (включая виртуальные машины), разворачивает кластер Kubernetes и  устанавливает Deckhouse. Информацию по поддерживаемым облачным провайдерам можно найти в разделе [Кластер Kubernetes](../kubernetes.html) документации.
- В кластерах на **bare metal** и **в неподдерживаемых облаках.** В этом случае dhctl настраивает сервер (виртуальную машину), разворачивает кластер Kubernetes с одним master-узлом и устанавливает Deckhouse. Далее с помощью готовых скриптов настройки можно вручную добавить дополнительные узлы в кластер.
- **В существующем кластере Kubernetes.** В этом случае dhctl устанавливает Deckhouse в существующем кластере.

### Подготовка инфраструктуры

Перед установкой проверьте следующее:
- *(для кластеров на bare metal и в неподдерживаемых облаках)* ОС сервера находится в [списке поддерживаемых ОС](../supported_versions.html) (или совместима с ними) и до сервера есть SSH-доступ по ключу;
- *(в поддерживаемых облаках)* наличие квот, необходимых для создания ресурсов, и параметров доступа к облаку (набор зависит от конкретной облачной инфраструктуры или облачного провайдера);
- доступ до container registry с образами Deckhouse (по умолчанию — `registry.deckhouse.io`).

### Подготовка конфигурации

Для установки Deckhouse нужно подготовить YAML-файл конфигурации установки и, при необходимости, YAML-файл ресурсов, которые нужно создать после успешной установки Deckhouse.

#### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):
- [InitConfiguration](configuration.html#initconfiguration) — начальные параметры [конфигурации Deckhouse](../#конфигурация-deckhouse). С этой конфигурацией Deckhouse запустится после установки.

  В этом ресурсе, в частности, указываются параметры, без которых Deckhouse не запустится или будет работать некорректно. Например, параметры [размещения компонентов Deckhouse](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys), используемый [storageClass](../deckhouse-configure-global.html#parameters-storageclass), параметры доступа к [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg), [шаблон используемых DNS-имен](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) и другие.
  
- [ClusterConfiguration](configuration.html#clusterconfiguration) — общие параметры кластера, такие как версия control plane, сетевые параметры, параметры CRI и т. д.
  
  > Использовать ресурс `ClusterConfiguration` в конфигурации необходимо, только если при установке Deckhouse нужно предварительно развернуть кластер Kubernetes. То есть `ClusterConfiguration` не нужен, если Deckhouse устанавливается в существующем кластере Kubernetes.

- [StaticClusterConfiguration](configuration.html#staticclusterconfiguration) — параметры кластера Kubernetes, развертываемого на серверах bare metal или на виртуальных машинах в неподдерживаемых облаках.

  > Как и в случае с ресурсом `ClusterConfiguration`, ресурс`StaticClusterConfiguration` не нужен, если Deckhouse устанавливается в существующем кластере Kubernetes.  

- `<CLOUD_PROVIDER>ClusterConfiguration` — набор ресурсов, содержащих параметры конфигурации поддерживаемых облачных провайдеров.
  
  Ресурсы конфигурации облачных провайдеров содержат такие параметры, как параметры доступа к облачной инфраструктуре (параметры аутентификации), тип и параметры схемы размещения ресурсов, параметры сети, параметры создаваемых групп узлов.

  Список ресурсов конфигурации поддерживаемых облачных провайдеров:
  - [AWSClusterConfiguration](../modules/030-cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
  - [AzureClusterConfiguration](../modules/030-cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
  - [GCPClusterConfiguration](../modules/030-cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
  - [OpenStackClusterConfiguration](../modules/030-cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack;
  - [VsphereInstanceClass](../modules/030-cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
  - [YandexInstanceClass](../modules/030-cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud.

- `ModuleConfig` — набор ресурсов, содержащих параметры конфигурации [встроенных модулей Deckhouse](../).

{% offtopic title="Пример файла конфигурации установки..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
cloud:
  provider: Azure
  prefix: cloud-demo
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: InitConfiguration
deckhouse:
  releaseChannel: Stable
---
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: <SSH_PUBLIC_KEY>
vNetCIDR: 10.241.0.0/16
subnetCIDR: 10.241.0.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    machineSize: Standard_D4ds_v4
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140
    enableExternalIP: true
provider:
  subscriptionId: <SUBSCRIPTION_ID>
  clientId: <CLIENT_ID>
  clientSecret: <CLIENT_SECRET>
  tenantId: <TENANT_ID>
  location: westeurope
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cni-flannel
spec:
  enabled: true
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  enabled: true
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-manager
spec:
  version: 1
  enabled: true
  settings:
    allowedBundles: ["ubuntu-lts"]
```

{% endofftopic %}

#### Файл ресурсов установки

Необязательный YAML-файл ресурсов установки содержит манифесты ресурсов Kubernetes, которые инсталлятор применит после успешной установки Deckhouse.

Файл ресурсов может быть полезен для дополнительной настройки кластера после установки Deckhouse: развертывание Ingress-контроллера, создание дополнительных групп узлов, ресурсов конфигурации, настройки прав и пользователей и т. д.

**Внимание!** В файле ресурсов установки нельзя использовать [ModuleConfig](../) для **встроенных** модулей. Для конфигурирования встроенных модулей используйте [файл конфигурации](#файл-конфигурации-установки).

{% offtopic title="Пример файла ресурсов..." %}

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  controllerVersion: "1.1"
  inlet: "LoadBalancer"
  nodeSelector:
    node.deckhouse.io/group: worker
---
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: worker
spec:
  machineSize: Standard_F4
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AzureInstanceClass
      name: worker
    maxPerZone: 3
    minPerZone: 1
    zones: ["1"]
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@deckhouse.io
  accessLevel: SuperAdmin
  portForwarding: true
---
apiVersion: deckhouse.io/v1
kind: User
metadata:
  name: admin
spec:
  email: admin@deckhouse.io
  password: '$2a$10$isZrV6uzS6F7eGfaNB1EteLTWky7qxJZfbogRs1egWEPuT1XaOGg2'
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse-admin
spec:
  enabled: true
```

{% endofftopic %}

#### Post-bootstrap-скрипт

После успешной установки Deckhouse инсталлятор может запустить скрипт на одном из master-узлов. С помощью скрипта можно выполнять дополнительную настройку, собирать информацию о настройке и т. п.

Post-bootstrap-скрипт можно указать с помощью параметра `--post-bootstrap-script-path` при запуске инсталляции (см. далее).

{% offtopic title="Пример скрипта, выводящего IP-адрес балансировщика..." %}
Пример скрипта, который выводит IP-адрес балансировщика после развертывания кластера в облаке и установки Deckhouse:

```shell
#!/usr/bin/env bash

set -e
set -o pipefail


INGRESS_NAME="nginx"


echo_err() { echo "$@" 1>&2; }

# declare the variable
lb_ip=""

# get the load balancer IP
for i in {0..100}
do
  if lb_ip="$(kubectl -n d8-ingress-nginx get svc "${INGRESS_NAME}-load-balancer" -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"; then
    if [ -n "$lb_ip" ]; then
      break
    fi
  fi

  lb_ip=""

  sleep 5
done

if [ -n "$lb_ip" ]; then
  echo_err "The load balancer external IP: $lb_ip"
else
  echo_err "Could not get the external IP of the load balancer"
  exit 1
fi

outContent="{\"frontend_ips\":[\"$lb_ip\"]}"

if [ -z "$OUTPUT" ]; then
  echo_err "The OUTPUT env is empty. The result was not saved to the output file."
else
  echo "$outContent" > "$OUTPUT"
fi
```

{% endofftopic %}

### Установка Deckhouse

> При установке Deckhouse редакции Enterprise Edition из официального container registry `registry.deckhouse.io` необходимо предварительно авторизоваться с помощью лицензионного ключа:
>
> ```shell
> docker login -u license-token registry.deckhouse.io
> ```

Запуск контейнера инсталлятора из публичного container registry Deckhouse в общем случае выглядит так:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.io/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

где:
- `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
- `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как:
  - SSH-ключи доступа;
  - файл конфигурации;
  - файл ресурсов и т. д.
- `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
  - `alpha` — для канала обновлений *Alpha*;
  - `beta` — для канала обновлений *Beta*;
  - `early-access` — для канала обновлений *Early Access*;
  - `stable` — для канала обновлений *Stable*;
  - `rock-solid` — для канала обновлений *Rock Solid*.

Пример запуска контейнера инсталлятора Deckhouse CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/resources.yml:/resources.yml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.io/deckhouse/ce/install:stable bash
```

Установка Deckhouse запускается в контейнере инсталлятора с помощью команды `dhctl`:
- Для запуска установки Deckhouse с развертыванием кластера (это все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
- Для запуска установки Deckhouse в существующем кластере используйте команду `dhctl bootstrap-phase install-deckhouse`.

> Для получения справки по параметрам выполните `dhctl bootstrap -h`.

Пример запуска установки Deckhouse с развертыванием кластера в облаке:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml --config=/resources.yml
```

где:
- `/config.yml` — файл конфигурации установки;
- `/resources.yml` — файл манифестов ресурсов;
- `<SSH_USER>` — пользователь на сервере для подключения по SSH;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.

#### Проверки перед началом установки

{% offtopic title="Схема выполнения проверок перед началом установки Deckhouse..." %}
![Схема выполнения проверок перед началом установки Deckhouse](../images/installing/preflight-checks.png)
{% endofftopic %}

Список проверок, которые выполняет инсталлятор перед началом установки Deckhouse:
- Общие проверки:
  - Значения параметров [PublicDomainTemplate](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) [clusterDomain](configuration.html#clusterconfiguration-clusterdomain) не совпадают.
  - Данные аутентификации для хранилища образов контейнеров, указанные в конфигурации установки, корректны.
  - Имя хоста соответствует следующим требованиям:
    - длина <= 63 символов;
    - в нижнем регистре;
    - не содержит спецсимволов (допускаются символы `-` и `.`, которые не могут стоять в начале и конце имени).
  - На сервере (ВМ) отсутствует установленный CRI (containerd).
  - Имя хоста уникально относительно других имен хостов кластера.
- Проверки для установки статического и гибридного кластера:
  - Указан только один параметр `--ssh-host`. При статической конфигурации кластера можно указать только один IP адрес для настройки первого мастер-узла.
  - Возможно подключиться по SSH с использованием указанных данных аутентификации.
  - Возможно установить SSH-туннель до сервера (ВМ) мастер-узла.
  - Сервер (ВМ) удовлетворяет минимальным требованиям для настройки мастер-узла.
  - На сервере (ВМ) для мастер-узла установлен python и необходимые библиотеки.
  - Хранилище образов контейнеров доступно через прокси (если настройки прокси указаны в конфигурации установки).
  - На сервере (ВМ) для мастер-узла и хосте инсталлятора свободны порты, необходимые для процесса установки.
  - localhost в DNS разрешается в IP 127.0.0.1.
  - На сервере (ВМ) пользователю доступна команда `sudo`.
- Проверки для установки облачного кластера:
  - Конфигурация виртуальной машины мастер-узла удовлетворяет минимальным требованиям.

#### Откат установки

При установке в поддерживаемом облаке в случае прерывания установки или возникновения проблем во время установки в облаке могут остаться созданные ресурсы. Для их удаления используйте команду `dhctl bootstrap-phase abort`, выполнив ее в контейнере инсталлятора.

> Обратите внимание, что **файл конфигурации**, передаваемый через параметр `--config` при запуске инсталлятора, должен быть **тот же, с которым производилась установка**.

## Как настроить?

Deckhouse состоит из оператора Deckhouse и модулей. Модуль — это набор из Helm-чарта, хуков [Addon-operator'а](https://github.com/flant/addon-operator/), правил сборки компонентов модуля (компонентов Deckhouse) и других файлов.

<div markdown="0" style="height: 0;" id="конфигурация-deckhouse"></div>

Deckhouse настраивается с помощью:
- **[Глобальных настроек](deckhouse-configure-global.html).** Глобальные настройки хранятся в custom resource `ModuleConfig/global`. Глобальные настройки можно рассматривать как специальный модуль `global`, который нельзя отключить.
- **[Настроек модулей](#настройка-модуля).** Настройки каждого модуля хранятся в custom resource `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case).
- **Custom resource'ов.** Некоторые модули настраиваются с помощью дополнительных custom resource'ов.

Пример набора custom resource'ов конфигурации Deckhouse:

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

Посмотреть список custom resource'ов `ModuleConfig`, состояние модулей (включен/выключен) и их статус можно с помощью команды `kubectl get moduleconfigs`:

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

### Настройка модуля

> При работе с модулями Deckhouse использует проект [addon-operator](https://github.com/flant/addon-operator/). Ознакомьтесь с его документацией, если хотите понять, как Deckhouse работает с [модулями](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [хуками модулей](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) и [параметрами модулей](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md). Будем признательны, если поставите проекту *звезду*.

Модуль настраивается с помощью custom resource `ModuleConfig`, имя которого совпадает с именем модуля (в kebab-case). Custom resource `ModuleConfig` имеет следующие поля:

- `metadata.name` — название модуля Deckhouse в kebab-case (например, `prometheus`, `node-manager`);
- `spec.version` — версия схемы настроек модуля (целое число, больше нуля). Обязательное поле, если `spec.settings` не пустое. Номер актуальной версии можно увидеть в документации модуля в разделе *Настройки*:
  - Deckhouse поддерживает обратную совместимость версий схемы настроек модуля. Если используется схема настроек устаревшей версии, при редактировании или просмотре custom resource'а будет выведено предупреждение о необходимости обновить схему настроек модуля;
- `spec.settings` — настройки модуля. Необязательное поле, если используется поле `spec.enabled`. Описание возможных настроек можно найти в документации модуля в разделе *Настройки*;
- `spec.enabled` — необязательное поле для явного [включения или отключения модуля](#включение-и-отключение-модуля). Если не задано, модуль может быть включен по умолчанию в одном из [наборов модулей](#наборы-модулей).

> Deckhouse не изменяет custom resource'ы `ModuleConfig`. Это позволяет применять подход Infrastructure as Code (IaC) при хранении конфигурации. Другими словами, можно воспользоваться всеми преимуществами системы контроля версий для хранения настроек Deckhouse, использовать Helm, kubectl и другие привычные инструменты.

Пример custom resource для настройки модуля `kube-dns`:

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

Некоторые модули настраиваются с помощью дополнительных custom resource'ов. Воспользуйтесь поиском (вверху страницы) или выберите модуль в меню слева, чтобы просмотреть документацию по его настройкам и используемым custom resource'ам.

#### Включение и отключение модуля

> Некоторые модули могут быть включены по умолчанию в зависимости от используемого [набора модулей](#наборы-модулей).

Для явного включения или отключения модуля необходимо установить `true` или `false` в поле `.spec.enabled` в соответствующем custom resource `ModuleConfig`. Если для модуля нет такого custom resource `ModuleConfig`, его нужно создать.

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

### Наборы модулей

В зависимости от используемого [набора модулей](./modules/002-deckhouse/configuration.html#parameters-bundle) (bundle) модули могут быть включены или выключены по умолчанию.

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
{%- assign isExcluded = site.data.exclude.module_names | where: "name", moduleName %}
{%- if isExcluded.size > 0 %}{% continue %}{% endif %}
<li>
{{ moduleName }}</li>
{%- endfor %}
</ul>
</td>
</tr>
{%- endfor %}
</tbody>
</table>

> **Обратите внимание,** что в наборе модулей `Minimal` не включен ряд базовых модулей (например, модуль работы с CNI). Deckhouse с набором модулей `Minimal` без включения базовых модулей сможет работать только в уже развернутом кластере.

### Управление размещением компонентов Deckhouse

#### Выделение узлов под определенный вид нагрузки

Если в параметрах модуля не указаны явные значения `nodeSelector/tolerations`, то для всех модулей используется следующая стратегия:
1. Если параметр `nodeSelector` модуля не указан, то Deckhouse попытается вычислить `nodeSelector` автоматически. В этом случае, если в кластере присутствуют узлы с [лейблами из списка или лейблами определенного формата](#особенности-автоматики-зависящие-от-типа-модуля), Deckhouse укажет их в качестве `nodeSelector` ресурсам модуля.
1. Если параметр `tolerations` модуля не указан, то Pod'ам модуля автоматически устанавливаются все возможные toleration'ы ([подробнее](#особенности-автоматики-зависящие-от-типа-модуля)).
1. Отключить автоматическое вычисление параметров `nodeSelector` или `tolerations` можно, указав значение `false`.
1. При отсутствии в кластере [выделенных узлов](#особенности-автоматики-зависящие-от-типа-модуля) и автоматическом выборе `nodeSelector` (см. п. 1), `nodeSelector` в ресурсах модуля указан не будет. Модуль в таком случае будет использовать любой узел с не конфликтующими `taints`.

Возможность настройки `nodeSelector` и `tolerations` отключена для модулей:
- которые работают на всех узлах кластера (например, `cni-flannel`, `monitoring-ping`);
- которые работают на всех master-узлах (например, `prometheus-metrics-adapter`, `vertical-pod-autoscaler`).

#### Особенности автоматики, зависящие от типа модуля

{% raw %}
* Модули *monitoring* (`operator-prometheus`, `prometheus` и `vertical-pod-autoscaler`):
  * Порядок поиска узлов (для определения [nodeSelector](modules/300-prometheus/configuration.html#parameters-nodeselector)):
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

Если у вас нет возможности заводить wildcard-записи DNS, для тестирования можно воспользоваться сервисом [sslip.io](https://sslip.io) или его аналогами.

Домен, используемый в шаблоне, не должен совпадать с доменом, указанным в параметре [clusterDomain](installing/configuration.html#clusterconfiguration-clusterdomain). Например, если `clusterDomain` установлен в `cluster.local` (значение по умолчанию), то `publicDomainTemplate` не может быть `%s.cluster.local`.
{% endalert %}

Пример ресурса `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings: # <-- Параметры модуля из раздела "Параметры" ниже.
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M      
      placement:
        customTolerationKeys:
        - dedicated.example.com
    storageClass: sc-fast
```

### Параметры

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}

## Удаление

### Удаление кластера, развёрнутого в облачном провайдере

Для удаления кластера, развёрнутого в облачном провайдере, нужно выполнить несколько шагов:

1. Узнайте канал обновления, заданный в кластере. Для этого выполните команду:

   ```shell
   kubectl get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

2. Запустите инсталлятор Deckhouse:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   где:
   - `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как SSH-ключи доступа;
   - `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например, `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
   - `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
     - `alpha` — для канала обновлений *Alpha*;
     - `beta` — для канала обновлений *Beta*;
     - `early-access` — для канала обновлений *Early Access*;
     - `stable` — для канала обновлений *Stable*;
     - `rock-solid` — для канала обновлений *Rock Solid*.

   Пример запуска контейнера инсталлятора Deckhouse CE:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

3. В запустившемся контейнере выполните команду:

   ```shell
   dhctl destroy --ssh-user=<USER> \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --yes-i-am-sane-and-i-understand-what-i-am-doing \
     --ssh-host=<MASTER_IP>
   ```

   где:
   - `<USER>` — пользователь удалённой машины, из-под которого производилась установка;
   - `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к кластеру, получит необходимые данные и произведёт удаление всех ресурсов и объектов в облаке, созданных при установке и работе DKP.

### Удаление гибридного кластера

Для удаления гибридного кластера, состоящего из автоматически развёрнутых узлов в облаке и вручную подключённых статических узлов, выполните следующие действия:

1. [Удалите](../modules/040-node-manager/faq.html#как-зачистить-узел-для-последующего-ввода-в-кластер) из кластера все вручную подключённые [дополнительные узлы](../modules/040-node-manager/cr.html#nodegroup-v1-spec-nodetype) — CloudStatic и Static.

2. Узнайте канал обновления, заданный в кластере. Для этого выполните команду:

   ```shell
   kubectl get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

3. Запустите инсталлятор Deckhouse:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   где:
   - `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как SSH-ключи доступа:
   - `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например, `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
   - `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
     - `alpha` — для канала обновлений *Alpha*;
     - `beta` — для канала обновлений *Beta*;
     - `early-access` — для канала обновлений *Early Access*;
     - `stable` — для канала обновлений *Stable*;
     - `rock-solid` — для канала обновлений *Rock Solid*.

   Пример запуска контейнера инсталлятора Deckhouse CE:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

4. В запустившемся контейнере выполните команду:

   ```shell
   dhctl destroy --ssh-user=<USER> \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --yes-i-am-sane-and-i-understand-what-i-am-doing \
     --ssh-host=<MASTER_IP>
   ```

   где:
   - `<USER>` — пользователь удалённой машины, из-под которого производилась установка;
   - `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к кластеру, получит необходимые данные и произведёт удаление всех ресурсов и объектов в облаке, созданных при установке и работе DKP.

### Удаление статического кластера

Для удаления кластера, установленного вручную (например, bare metal), нужно выполнить несколько шагов:

1. [Удалите](../modules/040-node-manager/faq.html#как-зачистить-узел-для-последующего-ввода-в-кластер) из кластера все дополнительные узлы.

2. Узнайте канал обновления, заданный в кластере. Для этого выполните команду:

   ```shell
   kubectl get mc deckhouse  -o jsonpath='{.spec.settings.releaseChannel}'
   ```

3. Запустите инсталлятор Deckhouse:

   ```shell
   docker run --pull=always -it [<MOUNT_OPTIONS>] \
     registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
   ```

   где:
   - `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, таких как SSH-ключи доступа;
   - `<DECKHOUSE_REVISION>` — [редакция](../revision-comparison.html) Deckhouse (например, `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.)
   - `<RELEASE_CHANNEL>` — [канал обновлений](../modules/002-deckhouse/configuration.html#parameters-releasechannel) Deckhouse в kebab-case. Должен совпадать с установленным в `config.yml`:
     - `alpha` — для канала обновлений *Alpha*;
     - `beta` — для канала обновлений *Beta*;
     - `early-access` — для канала обновлений *Early Access*;
     - `stable` — для канала обновлений *Stable*;
     - `rock-solid` — для канала обновлений *Rock Solid*.

   Пример запуска контейнера инсталлятора Deckhouse CE:

   ```shell
   docker run -it --pull=always \
     -v "$PWD/dhctl-tmp:/tmp/dhctl" \
     -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
   ```

4. Выполните команду удаления кластера:

   ```shell
   dhctl destroy --ssh-user=<USER> \
     --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
     --yes-i-am-sane-and-i-understand-what-i-am-doing \
     --ssh-host=<MASTER_IP>
   ```

   где:
   - `<USER>` — пользователь удалённой машины, из-под которого производилась установка;
   - `<MASTER_IP>` — IP-адрес master-узла кластера.

Инсталлятор подключится к master-узлу и удалит на нём все компоненты Deckhouse и кластера Kubernetes.

## Каналы обновлений

<link rel="stylesheet" type="text/css" href='{{ assets["releases.css"].digest_path }}' />
{%- assign releases = site.data.releases.channels | sort: "stability" -%}

<div class="docs__information warning active">
Информацию о том, какие версии Deckhouse находятся в настоящий момент на каких каналах обновлений, а также о планируемой дате смены версии на канале обновлений смотрите на сайте <a href="https://releases.deckhouse.io" target="_blank">releases.deckhouse.io</a>.
</div>  

К кластерам как элементам инфраструктуры обычно предъявляются различные требования.

Например, production-кластер, в отличие от кластера разработки, более требователен к надежности: в нем нежелательно часто обновлять или изменять какие-либо компоненты без особой необходимости, при этом сами компоненты должны быть тщательно протестированы.

Deckhouse использует **пять каналов обновлений**. *Мягко* переключаться между ними можно с помощью модуля [deckhouse](modules/002-deckhouse/): достаточно указать желаемый канал обновлений в [конфигурации](modules/002-deckhouse/configuration.html#parameters-releasechannel) модуля.

<div id="releases__stale__block" class="releases__info releases__stale__warning" >
  <strong>Внимание!</strong> В этом кластере не используется какой-либо канал обновлений.
</div>

{%- assign channels_sorted = site.data.releases.channels | sort: "stability" %}
{%- assign channels_sorted_reverse = site.data.releases.channels | sort: "stability" | reverse  %}

<div class="page__container page_releases" markdown="0">
<div class="releases__menu">
{%- for channel in channels_sorted_reverse %}
    <div class="releases__menu-item releases__menu--channel--{{ channel.name }}">
        <div class="releases__menu-item-header">
            <div class="releases__menu-item-title releases__menu--channel--{{ channel.name }}">
                {{ channel.title }}
            </div>
        </div>
        <div class="releases__menu-item-description">
            {{ channel.description[page.lang] }}
        </div>
    </div>
{%- endfor %}
</div>
</div>

## Поддерживаемые версии Kubernetes и ОС

{% include SUPPORTED_VERSIONS.liquid %}

## Сравнение редакций Deckhouse Kubernetes Platform

Deckhouse Kubernetes Platform поставляется в редакциях _Community Edition (CE)_, _Basic Edition (BE)_, _Standard Edition (SE)_, _Enterprise Edition (EE)_ и _Certified Security Edition (CSE)_. Они отличаются набором функциональных возможностей и уровнем доступной поддержки. Краткое сравнение с перечислением ключевых возможностей и особенностей можно найти на [этой странице]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/).

В таблице ниже перечислены модули Deckhouse Kubernetes Platform, доступные для использования в разных редакциях платформы:

{% include revision_comparison_detail_table.liquid %}

## Настройка ПО безопасности для работы с Deckhouse

Если узлы кластера Kubernetes анализируются сканерами безопасности (антивирусными средствами), то может потребоваться их настройка для исключения ложноположительных срабатываний.

Deckhouse использует следующие директории при работе ([скачать в csv...](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

### Рекомендации по настройке KESL (Kaspersky Endpoint Security для Linux) для работы совместно с Deckhouse

Чтобы KESL не влиял на работоспособность Deckhouse, необходимо выполнить описанные ниже рекомендации по настройке через командную строку непосредственно на хосте или через систему централизованного удалённого управления Kaspersky Security Center.

Управление KESL осуществляется [с помощью задач](https://support.kaspersky.com/KES4Linux/11.2.0/ru-RU/199339.htm), имеющих определённые номера. Ниже рассмотрена общая настройка, и конфигурация основных задач, используемых при настройке KESL c Deckhouse.

#### Общая настройка

Измените настройки KESL относительно битов маркировки сетевых пакетов, так как существуют пересечения с маркировкой сетевых пакетов самим Deckhouse. Для этого:
1. Остановите KESL, если он запущен, и измените настройки:

   * в параметре `BypassFwMark` измените значение с `0x400` на `0x700`;
   * в параметре `NtpFwMark` измените значение с `0x200` на `0x600`.

1. Запустите KESL.

   Пример команд для запуска KESL:

   ```shell
   systemctl stop kesl
   sed -i "s/NtpFwMark=0x200/NtpFwMark=0x600/" /var/opt/kaspersky/kesl/common/kesl.ini
   sed -i "s/BypassFwMark=0x400/BypassFwMark=0x700/" /var/opt/kaspersky/kesl/common/kesl.ini
   systemctl start kesl
   ```

#### Задача №1 «Защита от файловых угроз» (File_Threat_Protection)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 1 --add-exclusion /etc/cni
kesl-control --set-settings 1 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 1 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 1 --add-exclusion /mnt/vector-data
kesl-control --set-settings 1 --add-exclusion /opt/cni/bin
kesl-control --set-settings 1 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 1 --add-exclusion /var/lib/bashable
kesl-control --set-settings 1 --add-exclusion /var/lib/containerd
kesl-control --set-settings 1 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 1 --add-exclusion /var/lib/etcd
kesl-control --set-settings 1 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 1 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 1 --add-exclusion /var/log/containers
kesl-control --set-settings 1 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №2 «Поиск вредоносного ПО» (Scan_My_Computer)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 2 --add-exclusion /etc/cni
kesl-control --set-settings 2 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 2 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 2 --add-exclusion /mnt/vector-data
kesl-control --set-settings 2 --add-exclusion /opt/cni/bin
kesl-control --set-settings 2 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 2 --add-exclusion /var/lib/bashable
kesl-control --set-settings 2 --add-exclusion /var/lib/containerd
kesl-control --set-settings 2 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 2 --add-exclusion /var/lib/etcd
kesl-control --set-settings 2 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 2 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 2 --add-exclusion /var/log/containers
kesl-control --set-settings 2 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №3 «Выборочная проверка» (Scan_File)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 3 --add-exclusion /etc/cni
kesl-control --set-settings 3 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 3 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 3 --add-exclusion /mnt/vector-data
kesl-control --set-settings 3 --add-exclusion /opt/cni/bin
kesl-control --set-settings 3 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 3 --add-exclusion /var/lib/bashable
kesl-control --set-settings 3 --add-exclusion /var/lib/containerd
kesl-control --set-settings 3 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 3 --add-exclusion /var/lib/etcd
kesl-control --set-settings 3 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 3 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 3 --add-exclusion /var/log/containers
kesl-control --set-settings 3 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №4 «Проверка важных областей» (Critical_Areas_Scan)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 4 --add-exclusion /etc/cni
kesl-control --set-settings 4 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 4 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 4 --add-exclusion /mnt/vector-data
kesl-control --set-settings 4 --add-exclusion /opt/cni/bin
kesl-control --set-settings 4 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 4 --add-exclusion /var/lib/bashable
kesl-control --set-settings 4 --add-exclusion /var/lib/containerd
kesl-control --set-settings 4 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 4 --add-exclusion /var/lib/etcd
kesl-control --set-settings 4 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 4 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 4 --add-exclusion /var/log/containers
kesl-control --set-settings 4 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №11 «Контроль целостности системы» (System_Integrity_Monitoring)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 11 --add-exclusion /etc/cni
kesl-control --set-settings 11 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 11 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 11 --add-exclusion /mnt/vector-data
kesl-control --set-settings 11 --add-exclusion /opt/cni/bin
kesl-control --set-settings 11 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 11 --add-exclusion /var/lib/bashable
kesl-control --set-settings 11 --add-exclusion /var/lib/containerd
kesl-control --set-settings 11 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 11 --add-exclusion /var/lib/etcd
kesl-control --set-settings 11 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 11 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 11 --add-exclusion /var/log/containers
kesl-control --set-settings 11 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №12 «Управление сетевым экраном» (Firewall_Management)

{% alert level="danger" %}
Задачу необходимо отключить и не включать. Включение задачи приведёт к неработоспособности Deckhouse.
{% endalert %}

Эта задача удаляет все правила iptables, не относящиеся к KESL ([ссылка на документацию KESL](https://support.kaspersky.com/KES4Linux/11.3.0/ru-RU/234820.htm)).

Если задача включена, отключите её, выполнив команду:

```shell
kesl-control --stop-task 12
```

#### Задача №13 «Защита от шифрования» (Anti_Cryptor)

Добавьте исключение для директорий Deckhouse, выполнив команды:

```shell
kesl-control --set-settings 13 --add-exclusion /etc/cni
kesl-control --set-settings 13 --add-exclusion /etc/Kubernetes
kesl-control --set-settings 13 --add-exclusion /mnt/kubernetes-data
kesl-control --set-settings 13 --add-exclusion /mnt/vector-data
kesl-control --set-settings 13 --add-exclusion /opt/cni/bin
kesl-control --set-settings 13 --add-exclusion /opt/deckhouse/bin
kesl-control --set-settings 13 --add-exclusion /var/lib/bashable
kesl-control --set-settings 13 --add-exclusion /var/lib/containerd
kesl-control --set-settings 13 --add-exclusion /var/lib/deckhouse
kesl-control --set-settings 13 --add-exclusion /var/lib/etcd
kesl-control --set-settings 13 --add-exclusion /var/lib/kubelet
kesl-control --set-settings 13 --add-exclusion /var/lib/upmeter
kesl-control --set-settings 13 --add-exclusion /var/log/containers
kesl-control --set-settings 13 --add-exclusion /var/log/pods
```

{% alert level="info" %}
При добавлении может быть показано уведомление, что некоторых директорий не существует. Правило при этом будет добавлено — это нормально.
{% endalert %}

#### Задача №14 «Защита от веб-угроз» (Web_Threat_Protection)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 14
```

#### Задача №17 «Защита от сетевых угроз» (Network_Threat_Protection)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 17
```

#### Задача №20 «Анализ поведения» (Behavior_Detection)

С настройками по умолчанию задача негативного влияния на работоспособность Deckhouse не оказывает.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 20
```

#### Задача №21 «Контроль программ» (Application_Control)

С настройками по умолчанию задача негативного влияния на работоспособность Deckhouse не оказывает.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 21
```

#### Задача №22 «Веб-контроль» (Web_Control)

Задачу рекомендуется отключить.

Если есть необходимость включить задачу, выполните настройку таким образом, чтобы не было влияния на работоспособность Deckhouse.

Если задача включена и обнаружено её негативное влияние на Deckhouse, отключите задачу, выполнив команду:

```shell
kesl-control --stop-task 22
```

## Настройка сетевых политик для работы Deckhouse

Если на площадке, где работает Deckhouse Kubernetes Platform, есть требования для ограничения сетевого взаимодействия между серверами на уровне инфраструктуры, то необходимо соблюсти следующие условия:

* Включен режим туннелирования трафика между подами ([настройки](modules/021-cni-cilium/configuration.html#parameters-tunnelmode) для CNI Cilium, [настройки](modules/035-cni-flannel/configuration.html#parameters-podnetworkmode) для CNI Flannel).
* В случае необходимости интеграции с внешними системами (например, LDAP, SMTP или прочие внешние API), с ними разрешено сетевое взаимодействие.
* Локальное сетевое взаимодействие полностью разрешено в рамках каждого отдельно взятого узла кластера.
* Разрешено взаимодействие между узлами по портам, приведенным в таблицах на текущей странице. Обратите внимание, что большинство портов входит в диапазон 4200-4299. При добавлении новых компонентов платформы им будут назначаться порты из этого диапазона (при наличии возможности).

{% include network_security_setup.liquid %}

## FAQ

### Как узнать все параметры Deckhouse?

Deckhouse настраивается с помощью глобальных настроек, настроек модулей и различных сustom resource’ов. Подробнее — [в документации](./).

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

### Как найти документацию по установленной у меня версии?

Документация запущенной в кластере версии Deckhouse доступна по адресу `documentation.<cluster_domain>`, где `<cluster_domain>` — DNS-имя в соответствии с шаблоном из параметра [modules.publicDomainTemplate](deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобальной конфигурации.

{% alert level="warning" %}
Документация доступна, если в кластере включен модуль [documentation](modules/810-documentation/). Он включен по умолчанию, кроме [варианта поставки](modules/002-deckhouse/configuration.html#parameters-bundle) `Minimal`.
{% endalert %}

### Обновление Deckhouse

#### Как понять, в каком режиме обновляется кластер?

Посмотреть режим обновления кластера можно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse`. Для этого выполните следующую команду:

```shell
kubectl get mc deckhouse -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  creationTimestamp: "2022-12-14T11:13:03Z"
  generation: 1
  name: deckhouse
  resourceVersion: "3258626079"
  uid: c64a2532-af0d-496b-b4b7-eafb5d9a56ee
spec:
  settings:
    releaseChannel: Stable
    update:
      windows:
      - days:
        - Mon
        from: "19:00"
        to: "20:00"
  version: 1
status:
  state: Enabled
  status: ""
  type: Embedded
  version: "1"
```

Существуют три возможных режима обновления:
* **Автоматический + окна обновлений не заданы.** Кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).
* **Автоматический + заданы окна обновлений.** Кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.
* **Ручной режим.** Для применения обновления требуются [ручные действия](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

#### Как установить желаемый канал обновлений?

Чтобы перейти на другой канал обновлений автоматически, нужно в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` изменить (установить) параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае включится механизм [автоматической стабилизации релизного канала](#как-работает-автоматическое-обновление-deckhouse).

Пример конфигурации модуля `deckhouse` с установленным каналом обновлений `Stable`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
```

#### Как отключить автоматическое обновление?

Чтобы полностью отключить механизм обновления Deckhouse, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

#### Как применить обновление минуя окна обновлений, canary-release и ручной режим обновлений?

Чтобы применить обновление немедленно, установите в соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

{% alert level="info" %}
**Обратите внимание!** В этом случае будут проигнорированы окна обновления, настройки [canary-release](cr.html#deckhouserelease-v1alpha1-spec-applyafter) и режим [ручного обновления кластера](modules/002-deckhouse/configuration.html#parameters-update-disruptionapprovalmode). Обновление применится сразу после установки аннотации.
{% endalert %}

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

#### Как понять, какие изменения содержит обновление и как это повлияет на работу кластера?

Информацию о всех версиях Deckhouse можно найти в [списке релизов](https://github.com/deckhouse/deckhouse/releases) Deckhouse.

Сводную информацию о важных изменениях, об обновлении версий компонентов, а также о том, какие компоненты в кластере буду перезапущены в процессе обновления, можно найти в описании нулевой patch-версии релиза. Например, [v1.46.0](https://github.com/deckhouse/deckhouse/releases/tag/v1.46.0) для релиза v1.46 Deckhouse.

Подробный список изменений можно найти в Changelog, ссылка на который есть в каждом [релизе](https://github.com/deckhouse/deckhouse/releases).

#### Как понять, что в кластере идет обновление?

Во время обновления:
- горит алерт `DeckhouseUpdating`;
- под `deckhouse` не в статусе `Ready`. Если под долго не переходит в статус `Ready`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

#### Как понять, что обновление прошло успешно?

Если алерт `DeckhouseUpdating` погас, значит, обновление завершено.

Вы также можете проверить состояние [релизов](cr.html#deckhouserelease) Deckhouse.

Пример:

```console
$ kubectl get deckhouserelease
NAME       PHASE        TRANSITIONTIME   MESSAGE
v1.46.8    Superseded   13d
v1.46.9    Superseded   11d
v1.47.0    Superseded   4h12m
v1.47.1    Deployed     4h12m
```

Статус `Deployed` у соответствующей версии говорит о том, что переключение на соответствующую версию было выполнено (но это не значит, что оно закончилось успешно).

Проверьте состояние пода Deckhouse:

```shell
$ kubectl -n d8-system get pods -l app=deckhouse
NAME                   READY  STATUS   RESTARTS  AGE
deckhouse-7844b47bcd-qtbx9  1/1   Running  0       1d
```

* Если статус пода `Running` и в колонке READY указано `1/1` — обновление закончилось успешно.
* Если статус пода `Running` и в колонке READY указано `0/1` — обновление еще не закончилось. Если это продолжается более 20–30 минут, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.
* Если статус пода не `Running`, это может говорить о наличии проблем в работе Deckhouse. Необходима диагностика.

{% alert level="info" %}
Возможные варианты действий, если что-то пошло не так:
- Проверьте логи, используя следующую команду:

  ```shell
  kubectl -n d8-system logs -f -l app=deckhouse | jq -Rr 'fromjson? | .msg'
  ```

- Соберите [отладочную информацию](modules/002-deckhouse/faq.html#как-собрать-информацию-для-отладки) и свяжитесь с технической поддержкой.
- Попросите помощи у [сообщества](https://deckhouse.ru/community/about.html).
{% endalert %}

#### Как узнать, что для кластера доступна новая версия?

Как только на установленном в кластере канале обновления появляется новая версия Deckhouse:
- Загорается алерт `DeckhouseReleaseIsWaitingManualApproval`, если кластер использует ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
- Появляется новый custom resource [DeckhouseRelease](cr.html#deckhouserelease). Используйте команду `kubectl get deckhousereleases`, чтобы посмотреть список релизов. Если `DeckhouseRelease` новой версии находится в состоянии `Pending`, указанная версия еще не установлена. Возможные причины, при которых `DeckhouseRelease` может находиться в `Pending`:
  - Установлен ручной режим обновлений (параметр [update.mode](modules/002-deckhouse/configuration.html#parameters-update-mode) установлен в `Manual`).
  - Установлен автоматический режим обновлений и настроены [окна обновлений](modules/002-deckhouse/usage.html#конфигурация-окон-обновлений), интервал которых еще не наступил.
  - Установлен автоматический режим обновлений, окна обновлений не настроены, но применение версии отложено на случайный период времени из-за механизма снижения нагрузки на репозиторий образов контейнеров. В поле `status.message` ресурса `DeckhouseRelease` будет соответствующее сообщение.
  - Установлен параметр [update.notification.minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime) и указанное в нем время еще не прошло.

#### Как заранее получать информацию о предстоящем обновлении?

Получать заранее информацию об обновлении минорных версий Deckhouse на канале обновлений можно следующими способами:
- Настроить ручной [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode). В этом случае при появлении новой версии на канале обновлений загорится алерт `DeckhouseReleaseIsWaitingManualApproval` и в кластере появится новый custom resource [DeckhouseRelease](cr.html#deckhouserelease).
- Настроить автоматический [режим обновлений](modules/002-deckhouse/configuration.html#parameters-update-mode) и указать минимальное время в параметре [minimalNotificationTime](modules/002-deckhouse/configuration.html#parameters-update-notification-minimalnotificationtime), на которое будет отложено обновление. В этом случае при появлении новой версии на канале обновлений в кластере появится новый custom resource [DeckhouseRelease](cr.html#deckhouserelease). А если указать URL в параметре [update.notification.webhook](modules/002-deckhouse/configuration.html#parameters-update-notification-webhook), дополнительно произойдет вызов webhook'а.

#### Как узнать, какая версия Deckhouse находится на каком канале обновлений?

Информацию о том, какая версия Deckhouse находится на каком канале обновлений, можно получить на <https://releases.deckhouse.io>.

#### Как работает автоматическое обновление Deckhouse?

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse будет каждую минуту проверять данные о релизе на канале обновлений.

При появлении нового релиза Deckhouse скачивает его в кластер и создает custom resource [DeckhouseRelease](cr.html#deckhouserelease).

После появления custom resource'а `DeckhouseRelease` в кластере Deckhouse выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

{% alert %}
Patch-релизы (например, обновление на версию `1.30.2` при установленной версии `1.30.1`) устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза он всегда будет установлен.
{% endalert %}

#### Что происходит при смене канала обновлений?

* При смене канала обновлений на **более стабильный** (например, с `Alpha` на `EarlyAccess`) Deckhouse скачивает данные о релизе (в примере — из канала `EarlyAccess`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`:
  * Более *поздние* релизы, которые еще не были применены (в статусе `Pending`), удаляются.
  * Если более *поздние* релизы уже применены (в статусе `Deployed`), смены релиза не происходит. В этом случае Deckhouse останется на таком релизе до тех пор, пока на канале обновлений `EarlyAccess` не появится более поздний релиз.
* При смене канала обновлений на **менее стабильный** (например, с `EarlyAcess` на `Alpha`):
  * Deckhouse скачивает данные о релизе (в примере — из канала `Alpha`) и сравнивает их с данными из существующих в кластере custom resource'ов `DeckhouseRelease`.
  * Затем Deckhouse выполняет обновление согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update).

{% offtopic title="Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse" %}
![Схема использования параметра releaseChannel при установке и в процессе работы Deckhouse](images/common/deckhouse-update-process.png)
{% endofftopic %}

#### Что делать, если Deckhouse не получает обновлений из канала обновлений?

* Проверьте, что [настроен](#как-установить-желаемый-канал-обновлений) нужный канал обновлений.
* Проверьте корректность разрешения DNS-имени хранилища образов Deckhouse.

  Получите и сравните IP-адреса хранилища образов Deckhouse (`registry.deckhouse.ru`) на одном из узлов и в поде Deckhouse. Они должны совпадать.

  Пример получения IP-адреса хранилища образов Deckhouse на узле:

  ```shell
  $ getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Пример получения IP-адреса хранилища образов Deckhouse в поде Deckhouse:

  ```shell
  $ kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.ru
  185.193.90.38    STREAM registry.deckhouse.ru
  185.193.90.38    DGRAM  registry.deckhouse.ru
  ```

  Если полученные IP-адреса не совпадают, проверьте настройки DNS на узле. В частности, обратите внимание на список доменов в параметре `search` файла `/etc/resolv.conf` (он влияет на разрешение имен в поде Deckhouse). Если в параметре `search` файла `/etc/resolv.conf` указан домен, в котором настроено разрешение wildcard-записей, это может привести к неверному разрешению IP-адреса хранилища образов Deckhouse (см. пример).

{% offtopic title="Пример настроек DNS, которые могут привести к ошибкам в разрешении IP-адреса хранилища образов Deckhouse..." %}

Далее описан пример, когда настройки DNS приводят к различному результату при разрешении имен на узле и в поде Kubernetes:
- Пример файла `/etc/resolv.conf` на узле:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > Обратите внимание, что по умолчанию на узле параметр `ndot` равен 1 (`options ndots:1`). Но в подах Kubernetes параметр `ndot` равен **5**. Таким образом, логика разрешения DNS-имен, имеющих в имени 5 точек и менее, различается на узле и в поде.

- В DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. То есть любое DNS-имя в зоне `company.my`, для которого нет конкретной записи в DNS, разрешается в адрес `10.0.0.100`.

Тогда с учетом параметра `search`, указанного в файле `/etc/resolv.conf`, при обращении на адрес `registry.deckhouse.ru` на узле система попробует получить IP-адрес для имени `registry.deckhouse.ru` (так как считает его полностью определенным, учитывая настройку по умолчанию параметра `options ndots:1`).

При обращении же на адрес `registry.deckhouse.ru` **из пода** Kubernetes, учитывая параметры `options ndots:5` (используется в Kubernetes по умолчанию) и `search`, система первоначально попробует получить IP-адрес для имени `registry.deckhouse.ru.company.my`. Имя `registry.deckhouse.ru.company.my` разрешится в IP-адрес `10.0.0.100`, так как в DNS-зоне `company.my` настроено разрешение wildcard-записей `*.company.my` в адрес `10.0.0.100`. В результате к хосту `registry.deckhouse.ru` будет невозможно подключиться и скачать информацию о доступных обновлениях Deckhouse.
{% endofftopic %}

#### Как проверить очередь заданий в Deckhouse?

Для просмотра состояния всех очередей заданий Deckhouse, выполните следующую команду:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

Пример вывода (очереди пусты):

```console
$ kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
Summary:
- 'main' queue: empty.
- 88 other queues (0 active, 88 empty): 0 tasks.
- no tasks to handle.
```

Для просмотра состояния очереди заданий `main` Deckhouse, выполните следующую команду:

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
```

Пример вывода (в очереди `main` 38 заданий):

```console
$ kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
Queue 'main': length 38, status: 'run first task'
```

Пример вывода (очередь `main` пуста):

```console
$ kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
Queue 'main': length 0, status: 'waiting for task 0s'
```

### Закрытое окружение, работа через proxy и сторонние registry

#### Как установить Deckhouse из стороннего registry?

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

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

Для настройки нестандартных конфигураций сторонних registry в ресурсе `InitConfiguration` предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

#### Особенности настройки Nexus

{% alert level="warning" %}
При взаимодействии с репозиторием типа `docker` расположенным в Nexus (например, при выполнении команд `docker pull`, `docker push`) требуется указывать адрес в формате `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Использование значения `URL` из параметров репозитория Nexus **недопустимо**
{% endalert %}

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Включен `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*).
* Создан **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*):
  * Параметр `Allow anonymous docker pull` для репозитория должен быть включен. Данный параметр включает поддержку авторизации с помощью Bearer-токенов, при этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * Параметр `Maximum metadata age` для репозитория должен быть установлен в `0`.
* Должен быть настроен контроль доступа:
  * Создана роль **Nexus** (*Administration* -> *Security* -> *Roles*) со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

**Настройка**:

* Включите `Docker Bearer Token Realm` (*Administration* -> *Security* -> *Realms*):
  ![Включение `Docker Bearer Token Realm`](images/registry/nexus/nexus-realm.png)

* Создайте **проксирующий** репозиторий Docker (*Administration* -> *Repository* -> *Repositories*), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
  ![Создание проксирующего репозитория Docker](images/registry/nexus/nexus-repository.png)

* Заполните поля страницы создания репозитория следующим образом:
  * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
  * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
  * `Allow anonymous docker pull` должно быть включено, чтобы работала авторизация с помощью Bearer-токенов. При этом анонимный доступ [не будет работать](https://help.sonatype.com/en/docker-authentication.html#unauthenticated-access-to-docker-repositories), если он не был явно включен в *Administration* -> *Security* -> *Anonymous Access* и пользователю `anonymous` не были даны права на доступ к репозиторию.
  * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
  * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
  * `Maximum Metadata Age` должно быть равно `0`.
  * Если планируется использовать Deckhouse Enterprise Edition, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
    * `Authentication Type` должно иметь значение `Username`.
    * `Username` должно иметь значение `license-token`.
    * `Password` должно содержать ключ лицензии Deckhouse Enterprise Edition.

  ![Пример настроек репозитория 1](images/registry/nexus/nexus-repo-example-1.png)
  ![Пример настроек репозитория 2](images/registry/nexus/nexus-repo-example-2.png)
  ![Пример настроек репозитория 3](images/registry/nexus/nexus-repo-example-3.png)

* Настройте контроль доступа Nexus для доступа Deckhouse к созданному репозиторию:
  * Создайте роль **Nexus** (*Administration* -> *Security* -> *Roles*) с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

    ![Создание роли Nexus](images/registry/nexus/nexus-role.png)

  * Создайте пользователя (*Administration* -> *Security* -> *Users*) с ролью, созданной выше.

    ![Создание пользователя Nexus](images/registry/nexus/nexus-user.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

#### Особенности настройки Harbor

Необходимо использовать такой функционал [Harbor](https://github.com/goharbor/harbor), как Proxy Cache.

* Настройте Registry:
  * `Administration -> Registries -> New Endpoint`.
  * `Provider`: `Docker Registry`.
  * `Name` — укажите любое, на ваше усмотрение.
  * `Endpoint URL`: `https://registry.deckhouse.ru`.
  * Укажите `Access ID` и `Access Secret` для Deckhouse Enterprise Edition.

  ![Настройка Registry](images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * `Projects -> New Project`.
  * `Project Name` будет частью URL. Используйте любой, например, `d8s`.
  * `Access Level`: `Public`.
  * `Proxy Cache` — включите и выберите в списке Registry, созданный на предыдущем шаге.

  ![Создание нового проекта](images/registry/harbor/harbor2.png)

В результате образы Deckhouse будут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

#### Ручная загрузка образов в изолированный приватный registry

{% alert level="warning" %}
Доступно только в Standard Edition (SE), Enterprise Edition (EE) и Certified Security Edition (CSE).
{% endalert %}

{% alert level="info" %}
О текущем статусе версий на каналах обновлений можно узнать на [releases.deckhouse.io](https://releases.deckhouse.io).
{% endalert %}

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
   - `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы сертификатов SSL. Если переменная установлена, системные сертификаты не используются. [Подробнее...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html);
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

1. После загрузки образов в хранилище можно переходить к установке Deckhouse. Воспользуйтесь [руководством по быстрому старту](/gs/bm-private/step2.html).

   При запуске установщика используйте не официальное публичное хранилище образов Deckhouse, а хранилище в которое ранее были загружены образы Deckhouse. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable`, вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе [InitConfiguration](installing/configuration.html#initconfiguration) при установке также используйте адрес вашего хранилища и данные авторизации (параметры [imagesRepo](installing/configuration.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](installing/configuration.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/gs/bm-private/step3.html) руководства по быстрому старту).

   После завершения установки примените сгенерированные во время загрузки манифесты [DeckhouseReleases](cr.html#deckhouserelease) к вашему кластеру, используя [Deckhouse CLI](deckhouse-cli/):

   ```shell
   d8 k apply -f ./deckhousereleases.yaml
   ```

#### Ручная загрузка образов подключаемых модулей Deckhouse в изолированный приватный registry

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

   После применения манифеста модули готовы к использованию. Обратитесь к [документации по разработке модуля]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}/documentation/latest{% endif %}/module-development/) для получения дополнительной информации.

#### Как переключить работающий кластер Deckhouse на использование стороннего registry?

{% alert level="warning" %}
Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

Для переключения кластера Deckhouse на использование стороннего registry выполните следующие действия:

* Выполните команду `deckhouse-controller helper change-registry` из пода Deckhouse с параметрами нового registry.
  * Пример запуска:

    ```shell
    kubectl exec -ti -n d8-system svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry \
      --user MY-USER --password MY-PASSWORD registry.example.com/deckhouse/ee
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

#### Как создать кластер и запустить Deckhouse без использования каналов обновлений?

Данный способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.

* Если вы хотите установить Deckhouse с отключенным автоматическим обновлением:
  * Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.60.5`, используйте образ `your.private.registry.com/deckhouse/install:v1.60.5`.
  * **Не указывайте** параметр [deckhouse.releaseChannel](installing/configuration.html#initconfiguration-deckhouse-releasechannel) в ресурсе [InitConfiguration](installing/configuration.html#initconfiguration).
* Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

#### Использование proxy-сервера

{% alert level="warning" %}
Доступно только в Enterprise Edition.
{% endalert %}

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

### Изменение конфигурации

#### Как изменить конфигурацию кластера?

Общие параметры кластера хранятся в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration).

Чтобы изменить общие параметры кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit cluster-configuration
```

После сохранения изменений Deckhouse приведет конфигурацию кластера к измененному состоянию. В зависимости от размеров кластера это может занять какое-то время.

#### Как изменить конфигурацию облачного провайдера в кластере?

Настройки используемого облачного провайдера в облачном или гибридном кластере хранятся в структуре `<PROVIDER_NAME>ClusterConfiguration`, где `<PROVIDER_NAME>` — название/код провайдера. Например, для провайдера OpenStack структура будет называться [OpenStackClusterConfiguration]({% if site.mode == 'module' and site.d8Revision == 'CE' %}{{ site.urls[page.lang] }}/products/kubernetes-platform/documentation/v1/{% endif %}modules/030-cloud-provider-openstack/cluster_configuration.html).

Независимо от используемого облачного провайдера его настройки можно изменить с помощью следующей команды:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

#### Как изменить конфигурацию статического кластера?

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](installing/configuration.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit static-cluster-configuration
```

#### Как переключить Deckhouse EE на CE?

{% alert level="warning" %}
Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. Использование registry отличных от `registry.deckhouse.io` и `registry.deckhouse.ru` доступно только в Enterprise Edition.
{% endalert %}

{% alert level="warning" %}
В Deckhouse CE не поддерживается работа облачных кластеров на OpenStack и VMware vSphere.
{% endalert %}

Для переключения кластера Deckhouse Enterprise Edition на Community Edition выполните следующие действия (все команды выполняются на master-узле действующего кластера):

1. Убедитесь, что используемые в кластере модули [поддерживаются в версии CE](revision-comparison.html). Отключите модули, которые не поддерживаются в Deckhouse CE.

1. Выполните следующую команду:

   ```shell
   kubectl exec -ti -n d8-system svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry registry.deckhouse.ru/deckhouse/ce
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и [выполнения всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse):

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse EE:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "	" + .metadata.name' | sort | uniq
   ```

   Иногда могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает под несмотря на изменение соответствующего манифеста, так как используемый образ одинаков для Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться, что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.ru/deckhouse/ee' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

#### Как переключить Deckhouse CE на EE?

Вам потребуется действующий лицензионный ключ (вы можете [запросить временный ключ](https://deckhouse.ru/products/enterprise_edition.html) при необходимости).

{% alert %}
Инструкция подразумевает использование публичного адреса container registry: `registry.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).
{% endalert %}

Для переключения кластера Deckhouse Community Edition на Enterprise Edition выполните следующие действия (все команды выполняются на master-узле действующего кластера):

1. Выполните следующую команду:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   kubectl exec -ti -n d8-system svc/deckhouse-leader -c deckhouse -- deckhouse-controller helper change-registry --user license-token --password $LICENSE_TOKEN registry.deckhouse.ru/deckhouse/ee
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready`:

   ```shell
   kubectl -n d8-system get po -l app=deckhouse
   ```

1. Если под будет находиться в статусе `ImagePullBackoff`, перезапустите его:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse
   ```

1. Дождитесь перезапуска Deckhouse и [выполнения всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse):

   ```shell
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. На master-узле проверьте применение новых настроек.

   В журнале systemd-сервиса bashible на master-узле должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Jan 12 12:38:20 demo-master-0 bashible.sh[868379]: Configuration is in sync, nothing to do.
   Jan 12 12:38:20 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   Jan 12 12:39:18 demo-master-0 systemd[1]: Started Bashible service.
   Jan 12 12:39:19 demo-master-0 bashible.sh[869714]: Configuration is in sync, nothing to do.
   Jan 12 12:39:19 demo-master-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse CE:

   ```shell
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ce"))) | .metadata.namespace + "	" + .metadata.name' | sort | uniq
   ```

   Иногда могут оставаться запущенными некоторые static Pod'ы (например, `kubernetes-api-proxy-*`). Это связанно с тем, что kubelet не перезапускает под несмотря на изменение соответствующего манифеста, так как используемый образ одинаков для Deckhouse CE и EE. Выполните на любом master-узле следующую команду, чтобы убедиться, что соответствующие манифесты также были изменены:

   ```shell
   grep -ri 'deckhouse.ru/deckhouse/ce' /etc/kubernetes | grep -v backup
   ```

   Вывод команды должен быть пуст.

#### Как переключить Deckhouse EE на CSE?

{% alert %}
- Инструкция подразумевает использование публичного адреса container registry: `registry-cse.deckhouse.ru`. В случае использования другого адреса container registry измените команды или воспользуйтесь [инструкцией по переключению Deckhouse на использование стороннего registry](#как-переключить-работающий-кластер-deckhouse-на-использование-стороннего-registry).
- В Deckhouse CSE не поддерживается работа облачных кластеров и ряда модулей.
- На текущий момент мигрировать на CSE-редакцию можно только с релиза 1.58.
- При переключении на CSE-редакцию может наблюдаться недоступность компонентов кластера.
{% endalert %}

Для переключения кластера Deckhouse Enterprise Edition на Certified Security Edition выполните следующие действия (все команды выполняются на master-узле действующего кластера):

1. Подготовьте переменные с токеном лицензии и создайте NodeGroupConfiguration для переходной авторизации в `registry-cse.deckhouse.ru`:

   ```shell
   LICENSE_TOKEN=<PUT_YOUR_LICENSE_TOKEN_HERE>
   AUTH_STRING="$(echo -n license-token:${LICENSE_TOKEN} | base64 )"
   kubectl apply -f - <<EOF
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-cse-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 30
     content: |
       _on_containerd_config_changed() {
         bb-flag-set containerd-need-restart
       }
       bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/cse-registry.toml - containerd-config-file-changed << "EOF_TOML"
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
           [plugins."io.containerd.grpc.v1.cri".registry.mirrors."registry-cse.deckhouse.ru"]
             endpoint = ["https://registry-cse.deckhouse.ru"]
           [plugins."io.containerd.grpc.v1.cri".registry.configs]
             [plugins."io.containerd.grpc.v1.cri".registry.configs."registry-cse.deckhouse.ru".auth]
               auth = "$AUTH_STRING"
       EOF_TOML
   EOF
   ```

   Дождитесь появления файла на узлах и завершения синхронизации bashible:

   ```shell
   /etc/containerd/conf.d/cse-registry.toml
   ```

   Статус синхронизации можно отследить по значению `UPTODATE` (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Пример:

   ```console
   root@master-ee-to-cse-0:~# kubectl  get ng  -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   NAME     NODES   READY   UPTODATE
   master   1       1       1
   worker   2       2       2
   ```

   Также в журнале systemd-сервиса bashible должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Succesful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Выполните следующие команды для запуска временного пода CSE-редакции для получения актуальных дайджестов и списка модулей:

   ```console
   kubectl run cse-image --image=registry-cse.deckhouse.ru/deckhouse/cse/install:v1.58.2 --command sleep -- infinity
   ```

   Как только под перейдёт в статус `Running`, выполните следующие команды:

   ```console
   CSE_SANDBOX_IMAGE=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | grep  pause | grep -oE 'sha256:\w*')
   CSE_K8S_API_PROXY=$(kubectl exec cse-image -- cat deckhouse/candi/images_digests.json | grep kubernetesApiProxy | grep -oE 'sha256:\w*')
   CSE_MODULES=$(kubectl exec cse-image -- ls -l deckhouse/modules/ | awk {'print $9'}  |grep -oP "\d.*-\w*"  | cut -c5-)
   USED_MODULES=$(kubectl get modules | grep -v 'snapshot-controller-crd' | grep Enabled |awk {'print $1'})
   MODULES_WILL_DISABLE=$(echo $USED_MODULES | tr ' ' '
' | grep -Fxv -f <(echo $CSE_MODULES | tr ' ' '
'))
   ```

1. Убедитесь, что используемые в кластере модули поддерживаются в CSE-редакции.
   Например, на текущий момент в CSE-редакции отсутствует модуль cert-manager, поэтому перед его отключением необходимо перевести `https.mode` для связанных компонентов (например [user-authn](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/150-user-authn/configuration.html#parameters-https-mode) или [prometheus](https://deckhouse.ru/products/kubernetes-platform/documentation/v1.58/modules/300-prometheus/configuration.html#parameters-https-mode)) на альтернативные варианты.

   Отобразить список модулей, которые не поддерживаются в CSE-редакции и будут отключены, можно следующей командой:

   ```shell
   echo $MODULES_WILL_DISABLE
   ```

   > Проверьте список и убедитесь, что функциональность указанных модулей не задействована вами в кластере, и вы готовы к их отключению.

   Отключите неподдерживаемые в CSE-редакции модули:

   ```shell
   echo $MODULES_WILL_DISABLE | 
     tr ' ' '
' | awk {'print "kubectl -n d8-system exec  deploy/deckhouse -- deckhouse-controller module disable",$1'} | bash
   ```

   Дождитесь перехода пода Deckhouse в статус `Ready` и [выполнения всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse).

1. Создайте NodeGroupConfiguration:

   ```shell
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: cse-set-sha-images.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 50
     content: |
        _on_containerd_config_changed() {
          bb-flag-set containerd-need-restart
        }
        bb-event-on 'containerd-config-file-changed' '_on_containerd_config_changed'

        bb-sync-file /etc/containerd/conf.d/cse-sandbox.toml - containerd-config-file-changed << "EOF_TOML"
        [plugins]
          [plugins."io.containerd.grpc.v1.cri"]
            sandbox_image = "registry-cse.deckhouse.ru/deckhouse/cse@$CSE_SANDBOX_IMAGE"
        EOF_TOML

        sed -i 's|image: .*|image: registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
        sed -i 's|crictl pull .*|crictl pull registry-cse.deckhouse.ru/deckhouse/cse@$CSE_K8S_API_PROXY|' /var/lib/bashible/bundle_steps/051_pull_and_configure_kubernetes_api_proxy.sh
   EOF
   ```

   Дождитесь завершения синхронизации bashible на всех узлах.

   Состояние синхронизации можно отследить по значению `UPTODATE` статуса (отображаемое число узлов в этом статусе должно совпадать с общим числом узлов (`NODES`) в группе):

   ```shell
   kubectl get ng -o custom-columns=NAME:.metadata.name,NODES:.status.nodes,READY:.status.ready,UPTODATE:.status.upToDate -w
   ```

   Также в журнале systemd-сервиса bashible на узлах должно появиться сообщение `Configuration is in sync, nothing to do`.

   Пример:

   ```console
   # journalctl -u bashible -n 5
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Configuration is in sync, nothing to do.
   Aug 21 11:04:28 master-ee-to-cse-0 bashible.sh[53407]: Annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 bashible.sh[53407]: Succesful annotate node master-ee-to-cse-0 with annotation node.deckhouse.io/configuration-checksum=9cbe6db6c91574b8b732108a654c99423733b20f04848d0b4e1e2dadb231206a
   Aug 21 11:04:29 master-ee-to-cse-0 systemd[1]: bashible.service: Deactivated successfully.
   ```

1. Актуализируйте секрет доступа к registry Deckhouse, выполнив следующую команду:

   ```console
   kubectl -n d8-system create secret generic deckhouse-registry \
     --from-literal=".dockerconfigjson"="{\"auths\": { \"registry-cse.deckhouse.ru\": { \"username\": \"license-token\", \"password\": \"$LICENSE_TOKEN\", \"auth\": \"$AUTH_STRING\" }}}" \
     --from-literal="address"=registry-cse.deckhouse.ru \
     --from-literal="path"=/deckhouse/cse \
     --from-literal="scheme"=https \
     --type=kubernetes.io/dockerconfigjson \
     --dry-run='client' \
     -o yaml | kubectl replace -f -
   ```

1. Примените CSE-образ:

   ```console
   kubectl -n d8-system set image deployment/deckhouse deckhouse=registry-cse.deckhouse.ru/deckhouse/cse:v1.58.2
   ```

1. Дождитесь перехода пода Deckhouse в статус `Ready` и [выполнения всех задач в очереди](#как-проверить-очередь-заданий-в-deckhouse). Если в процессе возникает ошибка `ImagePullBackOff`, подождите автоматического перезапуска пода.

   Посмотреть статус пода Deckhouse:

   ```console
   kubectl -n d8-system get po -l app=deckhouse
   ```

   Проверить состояние очереди Deckhouse:

   ```console
   kubectl -n d8-system exec deploy/deckhouse -c deckhouse -- deckhouse-controller queue list
   ```

1. Проверьте, не осталось ли в кластере подов с адресом registry для Deckhouse EE:

   ```console
   kubectl get pods -A -o json | jq -r '.items[] | select(.spec.containers[]
     | select(.image | contains("deckhouse.ru/deckhouse/ee"))) | .metadata.namespace + "	" + .metadata.name' | sort | uniq
   ```

   Если в выводе присутствуют поды модуля chrony, заново включите данный модуль (в CSE этот модуль по умолчанию выключен):

   ```console
   kubectl -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable chrony
   ```

1. Очистите временные файлы, ngc и переменные:

   ```console
   rm /tmp/cse-deckhouse-registry.yaml

   kubectl delete ngc containerd-cse-config.sh cse-set-sha-images.sh

   kubectl delete pod cse-image

   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: del-temp-config.sh
   spec:
     nodeGroups:
     - '*'
     bundles:
     - '*'
     weight: 90
     content: |
       if [ -f /etc/containerd/conf.d/cse-registry.toml ]; then
         rm -f /etc/containerd/conf.d/cse-registry.toml
       fi
       if [ -f /etc/containerd/conf.d/cse-sandbox.toml ]; then
         rm -f /etc/containerd/conf.d/cse-sandbox.toml
       fi
   EOF
   ```

   После синхронизации bashible (статус синхронизации на узлах можно отследить по значению `UPTODATE` у nodegroup) удалите созданную ngc:

   ```shell
   kubectl  delete ngc del-temp-config.sh
   ```

#### Как получить доступ к контроллеру Deckhouse в multi-master-кластере?

В кластерах с несколькими master-узлами Deckhouse запускается в режиме высокой доступности (в нескольких экземплярах). Для доступа к активному контроллеру Deckhouse можно использовать следующую команду (на примере команды `deckhouse-controller queue list`):

```shell
kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

#### Как обновить версию Kubernetes в кластере?

Чтобы обновить версию Kubernetes в кластере, измените параметр [kubernetesVersion](installing/configuration.html#clusterconfiguration-kubernetesversion) в структуре [ClusterConfiguration](installing/configuration.html#clusterconfiguration), выполнив следующие шаги:
1. Выполните команду:

   ```shell
   kubectl -n d8-system exec -ti svc/deckhouse-leader \
     -c deckhouse -- deckhouse-controller edit cluster-configuration
   ```

1. Измените параметр `kubernetesVersion`.
1. Сохраните изменения. Узлы кластера начнут последовательно обновляться.
1. Дождитесь окончания обновления. Отслеживать ход обновления можно с помощью команды `kubectl get no`. Обновление можно считать завершенным, когда в выводе команды у каждого узла кластера в колонке `VERSION` появится обновленная версия.

#### Как запускать Deckhouse на произвольном узле?

Для запуска Deckhouse на произвольном узле установите у модуля `deckhouse` соответствующий [параметр](modules/002-deckhouse/configuration.html) `nodeSelector` и не задавайте `tolerations`.  Необходимые значения `tolerations` в этом случае будут проставлены автоматически.

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

## Модуль monitoring-kubernetes

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

## Модуль monitoring-custom

Модуль расширяет возможности модуля [prometheus](../../modules/300-prometheus/) по мониторингу приложений пользователей.

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

- При использовании service mesh [Istio](../110-istio/) в режиме STRICT mTLS указать для сбора метрик следующую аннотацию у Service: `prometheus.deckhouse.io/istio-mtls: "true"`. Важно, что метрики приложения должны экспортироваться по протоколу HTTP без TLS.

- *(Не обязательно)* Указать дополнительные аннотации для более тонкой настройки:

  * `prometheus.deckhouse.io/path` — путь для сбора метрик (по умолчанию: `/metrics`).
  * `prometheus.deckhouse.io/query-param-$name` — GET-параметры, будут преобразованы в map вида `$name=$value` (по умолчанию: ''):
    - возможно указать несколько таких аннотаций.

      Например, `prometheus.deckhouse.io/query-param-foo=bar` и `prometheus.deckhouse.io/query-param-bar=zxc` будут преобразованы в query: `http://...?foo=bar&bar=zxc`.
  * `prometheus.deckhouse.io/allow-unready-pod` — разрешает сбор метрик с подов в любом состоянии (по умолчанию метрики собираются только с подов в состоянии Ready). Эта опция полезна в очень редких случаях. Например, если ваше приложение запускается очень долго (при старте загружаются данные в базу или прогреваются кэши), но в процессе запуска уже отдаются полезные метрики, которые помогают следить за запуском приложения.
  * `prometheus.deckhouse.io/sample-limit` — сколько семплов разрешено собирать с пода (по умолчанию 5000). Значение по умолчанию защищает от ситуации, когда приложение внезапно начинает отдавать слишком большое количество метрик, что может нарушить работу всего мониторинга. Эту аннотацию надо вешать на тот же ресурс, на котором висит лейбл  `prometheus.deckhouse.io/custom-target`.

#### Пример: Service

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

#### Пример: Deployment

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

## Модуль namespace-configurator

Позволяет автоматически управлять аннотациями и label'ами на namespace'ах.

Модуль полезен тем, что помогает автоматически включать новые namespace'ы в мониторинг посредством добавления лейбла `extended-monitoring.deckhouse.io/enabled=true`.

#### Как работает

Модуль следит за изменениями namespace и своей конфигурации:
* Всем namespace'ам, попадающим под шаблон `includeNames` и не попадающим под шаблон `excludeNames`, будут назначены соответствующие label'ы и аннотации из конфигурации.
* При изменении конфигурации модуля соответствующие label'ы и аннотации на namespace'ах будут переназначены согласно конфигурациии.

#### Что нужно настроить?

Необходимо перечислить список желаемых label'ов и аннотаций, а также список шаблонов поиска namespace в конфигурации модуля.

## Модуль pod-reloader

Модуль создан на основе [Reloader](https://github.com/stakater/Reloader).
Он предоставляет возможность автоматически произвести rollout в случае изменения ConfigMap или Secret.
Для управления используются аннотации. Модуль запускается на **системных** узлах.

> **Важно!** У Reloader отсутствует отказоустойчивость.

В этом документе описаны основные аннотации. Вы можете найти больше примеров в разделе [Примеры](examples.html) документации.

| Аннотация                                    | Ресурс                             | Описание                                                                                                                                                                 | Примеры значений                              |
| -------------------------------------------- |------------------------------------| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------- |
| `pod-reloader.deckhouse.io/auto`             | Deployment, DaemonSet, StatefulSet | В случае изменения в связанных, то есть примонтированных или использованных как переменные окружения, ConfigMap'ах или Secret'ах произойдет перезапуск подов этого контроллера | `"true"`, `"false"`  |
| `pod-reloader.deckhouse.io/search`           | Deployment, DaemonSet, StatefulSet | В случае наличия этой аннотации перезапуск будет производиться исключительно при изменении ConfigMap'ов или Secret'ов с аннотацией `pod-reloader.deckhouse.io/match: "true"` | `"true"`, `"false"` |
| `pod-reloader.deckhouse.io/configmap-reload` | Deployment, DaemonSet, StatefulSet | Указать список ConfigMap'ов, от которых зависит контроллер                                                                                                                   | `"some-cm"`, `"some-cm1,some-cm2"` |
| `pod-reloader.deckhouse.io/secret-reload`    | Deployment, DaemonSet, StatefulSet | Указать список Secret'ов, от которых зависит контроллер                                                                                                                      | `"some-secret"`, `"some-secret1,some-secret2"` |
| `pod-reloader.deckhouse.io/match`            | Secret, ConfigMap                  | Аннотация, по которой из связанных ресурсов выбираются те, за изменениями которых мы следим                                                                               | `"true"`, `"false"` |

**Важно** Аннотация `pod-reloader.deckhouse.io/search` не может быть использована вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.

**Важно** Аннотации `pod-reloader.deckhouse.io/configmap-reload` и `pod-reloader.deckhouse.io/secret-reload` не могут быть использованы вместе с `pod-reloader.deckhouse.io/auto: "true"`, так как Reloader будет игнорировать `pod-reloader.deckhouse.io/search` и `pod-reloader.deckhouse.io/match`. Для корректной работы установите аннотации `pod-reloader.deckhouse.io/auto` значение `"false"` или удалите ее.

## Модуль admission-policy-engine

Позволяет использовать в кластере политики безопасности согласно [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) Kubernetes. Модуль для работы использует [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/).

Pod Security Standards определяют три политики, охватывающие весь спектр безопасности. Эти политики являются кумулятивными, то есть состоящими из набора политик, и варьируются по уровню ограничений от «неограничивающего» до «ограничивающего значительно».

{% alert level="info" %}
Модуль не применяет политики к системным пространствам имен.
{% endalert %}

Список политик, доступных для использования:
- `Privileged` — неограничивающая политика с максимально широким уровнем разрешений;
- `Baseline` — минимально ограничивающая политика, которая предотвращает наиболее известные и популярные способы повышения привилегий. Позволяет использовать стандартную (минимально заданную) конфигурацию пода;
- `Restricted` — политика со значительными ограничениями. Предъявляет самые жесткие требования к подам.

Подробнее про каждый набор политик и их ограничения можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/concepts/security/pod-security-standards/#profile-details).

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

#### Операционные политики

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

#### Политики безопасности

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

#### Изменение ресурсов Kubernetes

Модуль также позволяет использовать custom resource'ы Gatekeeper для легкой модификации объектов в кластере, такие как:
- `AssignMetadata` — для изменения секции metadata в ресурсе;
- `Assign` — для изменения других полей, кроме metadata;
- `ModifySet` — для добавления или удаления значений из списка, например аргументов для запуска контейнера.

Пример:

```yaml
apiVersion: mutations.gatekeeper.sh/v1
kind: AssignMetadata
metadata:
  name: demo-annotation-owner
spec:
  match:
    scope: Namespaced
    namespaces: ["default"]
    kinds:
    - apiGroups: [""]
      kinds: ["Pod"]
  location: "metadata.annotations.foo"
  parameters:
    assign:
      value:  "bar"
```

Подробнее про доступные варианты можно прочитать в документации [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/mutation/).

## Модуль Prometheus Pushgateway

Данный модуль устанавливает в кластер [Prometheus Pushgateway](https://github.com/prometheus/pushgateway). Он предназначен для приема метрик от приложения и отдачи их Prometheus.

Дополнительная информация:
- [Когда стоит использовать `Prometheus Pushgateway`](https://prometheus.io/docs/practices/pushing/).
- [Как собирать метрики с помощью `Prometheus Pushgateway`](https://prometheus.io/docs/instrumenting/pushing/).

## Модуль ceph-csi

Модуль устанавливает и настраивает CSI-драйвер для RBD и CephFS.

Настройка выполняется посредством [custom resources](cr.html), что позволяет подключить более одного Ceph-кластера (UUID не должны совпадать).

## Модуль cni-simple-bridge

Модуль не имеет настроек.

Включается автоматически для следующих облачных провайдеров:
- [AWS](../../modules/030-cloud-provider-aws/).
- [Azure](../../modules/030-cloud-provider-azure/).
- [GCP](../../modules/030-cloud-provider-gcp/).
- [Yandex](../../modules/030-cloud-provider-yandex/).

## Cloud provider — AWS

Взаимодействие с облачными ресурсами провайдера [AWS](https://aws.amazon.com/) осуществляется с помощью модуля `cloud-provider-aws`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы AWS при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-aws`:
- Управляет ресурсами AWS с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне AWS.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых более нет в AWS.
- Заказывает диски в AWS с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [AWSInstanceClass'ы](cr.html#awsinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

## Модуль user-authn

Модуль отвечает за единую систему аутентификации, интегрированную с Kubernetes и веб-интерфейсами, используемыми в других модулях, например, Grafana и Dashboard.

Модуль состоит из следующих компонентов:
- [`dex`](https://github.com/dexidp/dex) — федеративный OpenID Connect провайдер, поддерживающий работу со статическими пользователями и с возможностью подключения к различным внешним провайдерам аутентификации, например, SAML, GitLab, GitHub.
- `kubeconfig-generator` (он же [`dex-k8s-authenticator`](https://github.com/mintel/dex-k8s-authenticator)) — веб-приложение, генерирующее команды для настройки локального `kubectl` после аутентификации в Dex;
- `dex-authenticator` (он же [`oauth2-proxy`](https://github.com/oauth2-proxy/oauth2-proxy)) — приложение, получающее запросы от компонента NGINX Ingress (через модуль auth_request) и выполняющее их авторизацию с помощью сервиса Dex.

Управление статическими пользователями осуществляется с помощью custom ресурс [_User_](cr.html#user), в котором хранится вся информация о пользователе, включая его пароль.

Поддерживаются следующие внешние провайдеры/протоколы аутентификации:
- GitHub;
- GitLab;
- BitBucket Cloud;
- Crowd;
- LDAP;
- OIDC.

Одновременно можно подключить более одного внешнего провайдера аутентификации.

### Возможности интеграции

#### Базовая аутентификация в API Kubernetes

[Базовая аутентификация](https://en.wikipedia.org/wiki/Basic_access_authentication) в API Kubernetes на данный момент доступна только для провайдера Crowd (с включением параметра [`enableBasicAuth`](cr.html#dexprovider-v1-spec-crowd-enablebasicauth)).

> К API Kubernetes можно подключаться и [через другие поддерживаемые внешние провайдеры](#веб-интерфейс-для-генерации-готовых-kubeconfig-файлов).

#### Интеграция с приложениями

Чтобы обеспечить аутентификацию в любом веб-приложении, работающем в Kubernetes, можно создать ресурс [_DexAuthenticator_](cr.html#dexauthenticator) в пространстве имен (_Namespace_) приложения и добавить несколько аннотаций к ресурсу _Ingress_.
Это позволит:
* ограничить список групп, которым разрешен доступ;
* ограничить список адресов, с которых разрешена аутентификация;
* интегрировать приложение в единую систему аутентификации, если приложение поддерживает OIDC. Для этого в Kubernetes создается ресурс [_DexClient_](cr.html#dexclient) в _Namespace_ приложения. В том же _Namespace_ создается секрет с данными для подключения в Dex по OIDC.

После такой интеграции можно:
* ограничить перечень групп, которым разрешено подключаться;
* указать перечень клиентов, OIDC-токенам которых можно доверять (`trustedPeers`).

#### Веб-интерфейс для генерации готовых kubeconfig-файлов

Модуль позволяет автоматически создавать конфигурацию для kubectl или других утилит Kubernetes.

Пользователь получит набор команд для настройки kubectl после авторизации в веб-интерфейсе генератора. Эти команды можно скопировать и вставить в консоль для использования kubectl.
Механизм аутентификации для kubeconfig использует OIDC-токен. OIDC-сессия может продлеваться автоматически, если использованный в Dex провайдер аутентификации поддерживает продление сессий. Для этого в kubeconfig указывается `refresh token`.

Дополнительно можно настроить несколько адресов `kube-apiserver` и сертификаты ЦС (CA) для каждого из них. Например, это может потребоваться, если доступ к кластеру Kubernetes осуществляется через VPN или прямое подключение.

### Публикация API kubernetes через Ingress

Компонент kube-apiserver без дополнительных настроек доступен только во внутренней сети кластера. Этот модуль решает проблему простого и безопасного доступа к API Kubernetes извне кластера. При этом API-сервер публикуется на специальном домене (подробнее см. [раздел о служебных доменах в документации](../../deckhouse-configure-global.html)).

При настройке можно указать:
* перечень сетевых адресов, с которых разрешено подключение;
* перечень групп, которым разрешен доступ к API-серверу;
* Ingress-контроллер, на котором производится аутентификация.

По умолчанию будет сгенерирован специальный сертификат ЦС (CA) и автоматически настроен генератор kubeconfig.

### Расширения от Фланта

Модуль использует модифицированную версию Dex для поддержки:
* групп для статических учетных записей пользователей и провайдера Bitbucket Cloud (параметр [`bitbucketCloud`](cr.html#dexprovider-v1-spec-bitbucketcloud));
* передачи параметра `group` клиентам;
* механизма `obsolete tokens`, который позволяет избежать состояния гонки при продлении токена OIDC-клиентом.

### Отказоустойчивый режим

Модуль поддерживает режим высокой доступности `highAvailability`. При его включении аутентификаторы, отвечающие на `auth request`-запросы, развертываются с учетом требуемой избыточности для обеспечения непрерывной работы. В случае отказа любого из экземпляров аутентификаторов пользовательские аутентификационные сессии не прерываются.

## Модуль documentation

Этот модуль создает веб-интерфейс с документацией, соответствующей запущенной версии Deckhouse.

Это может быть полезно, например, когда Deckhouse работает в сети с ограничением доступа в интернет.

Адрес веб-интерфейса формируется следующим образом: в шаблоне [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse ключ `%s` заменяется на `documentation`.

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс документации будет доступен по адресу `documentation-kube.company.my`.

## Модуль okmeter

Данный модуль устанавливает агент [Okmeter](http://okmeter.io) как `daemonset` в namespace `d8-okmeter` и удаляет ранее установленный вручную `okmeter`.

## The descheduler module

The module runs a [descheduler](https://github.com/kubernetes-sigs/descheduler) with [strategies](#strategies) defined in a `Descheduler` custom resource.

descheduler every 15 minutes evicts Pods that satisfy strategies enabled in the `Descheduler` custom resource. This leads to forced run the scheduling process for evicted Pods.

### Nuances of descheduler operation

* descheduler do not take into account pods in `d8-*` and `kube-system` namespaces;
* Pods with local storage enabled are never evicts;
* Pods that are associated with a DaemonSet are never evicts;
* Pods with [priorityClassName](../001-priority-class/) set to `system-cluster-critical` or `system-node-critical` (*critical* Pods) are never evicts;
* descheduler takes into account the priority class when evicting Pods from a high-loaded node (check out the [priority-class](../001-priority-class/) module);
* The Best effort Pods are evicted before Burstable and Guaranteed ones;
* descheduler takes into account the [Pod Disruption Budget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/): the Pod will not be evicted if descheduling violates the PDB;
* descheduler takes into account node fitting. If no nodes available to start evicted pod, pod is not evicted.

To limit pods set by labels, `podlabelSelector` parameter is used. `podlabelSelector` has the same syntax as kubernetes labelSelector.
To limit pods set by namespace, `namespaceLabelSelector` parameter is used. `namespaceLabelSelector` has the same syntax as kubernetes labelSelector.
To set up node fit list, `nodeLabelSelector` parameter is used. `nodeLabelSelector` has the same syntax as kubernetes labelSelector.

### Strategies

#### HighNodeUtilization

This strategy finds nodes that are under utilized and evicts pods from the nodes in the hope that these pods will be scheduled compactly into fewer nodes. Used in conjunction with node auto-scaling, this strategy is intended to help trigger down scaling of under utilized nodes. This strategy must be used with the scheduler scoring strategy `MostAllocated`.
The under utilization of nodes is determined by a configurable threshold `thresholds`. The threshold `thresholds` can be configured for cpu, memory, number of pods, and extended resources in terms of percentage. The percentage is calculated as the current resources requested on the node vs total allocatable. For pods, this means the number of pods on the node as a fraction of the pod capacity set for that node.
If a node's usage is below threshold for all (cpu, memory, number of pods and extended resources), the node is considered underutilized. Currently, pods request resource requirements are considered for computing node resource utilization. Any node above `thresholds` is considered appropriately utilized and is not considered for eviction.
The `thresholds` parameter could be tuned as per your cluster requirements. Note that this strategy evicts pods from underutilized nodes (those with usage below `thresholds`) so that they can be recreated in appropriately utilized nodes. The strategy will abort if any number of underutilized nodes or appropriately utilized nodes is zero.
`
NOTE: Node resource consumption is determined by the requests and limits of pods, not actual usage. This approach is chosen in order to maintain consistency with the kube-scheduler, which follows the same design for scheduling pods onto nodes. This means that resource usage as reported by Kubelet (or commands like kubectl top) may differ from the calculated consumption, due to these components reporting actual usage metrics.

#### LowNodeUtilization

This strategy finds nodes that are under utilized and evicts pods, if possible, from other nodes in the hope that recreation of evicted pods will be scheduled on these underutilized nodes.
The under utilization of nodes is determined by a configurable threshold `thresholds`. The threshold `thresholds` can be configured for cpu, memory, number of pods, and extended resources in terms of percentage (the percentage is calculated as the current resources requested on the node vs total allocatable. For pods, this means the number of pods on the node as a fraction of the pod capacity set for that node).
If a node's usage is below threshold for all (cpu, memory, number of pods and extended resources), the node is considered underutilized. Currently, pods request resource requirements are considered for computing node resource utilization.
There is another configurable threshold, `targetThresholds`, that is used to compute those potential nodes from where pods could be evicted. If a node's usage is above `targetThresholds` for any (cpu, memory, number of pods, or extended resources), the node is considered over utilized. Any node between the thresholds, `thresholds` and `targetThresholds` is considered appropriately utilized and is not considered for eviction. The threshold, `targetThresholds`, can be configured for cpu, memory, and number of pods too in terms of percentage.
These thresholds, `thresholds` and `targetThresholds`, could be tuned as per your cluster requirements. Note that this strategy evicts pods from overutilized nodes (those with usage above `targetThresholds`) to underutilized nodes (those with usage below `thresholds`), it will abort if any number of underutilized nodes or overutilized nodes is zero.

NOTE: Node resource consumption is determined by the requests and limits of pods, not actual usage. This approach is chosen in order to maintain consistency with the kube-scheduler, which follows the same design for scheduling pods onto nodes. This means that resource usage as reported by Kubelet (or commands like kubectl top) may differ from the calculated consumption, due to these components reporting actual usage metrics.

## Модуль operator-prometheus

Модуль устанавливает [prometheus operator](https://github.com/coreos/prometheus-operator), который позволяет создавать и автоматизированно управлять инсталляциями [Prometheus](https://prometheus.io/).

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

### Prometheus

#### Что делает Prometheus?

В целом, сервер Prometheus делает две ключевых вещи — **собирает метрики** и **выполняет правила**:
* Для каждого *target'а* (цель для мониторинга), каждый `scrape_interval`, делает HTTP запрос на этот *target*, получает в ответ метрики в [своем формате](https://github.com/prometheus/docs/blob/master/content/docs/instrumenting/exposition_formats.md#text-format-details), которые сохраняет к себе в базу
* Каждый `evaluation_interval` обрабатывает *rules*, на основании чего:
  * или шлет алерты
  * или записывает (себе же в базу) новые метрики (результат выполнения *rule'а*)

#### Как настраивается Prometheus?

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

#### Где Prometheus берет список *target'ов*?

* В целом Prometheus работает следующим образом:

  ![Работа Prometheus](../../images/200-operator-prometheus/targets.png)

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

### Prometheus Operator

#### Что делает Prometheus Operator?

* С помощью механизма CRD (Custom Resource Definitions) определяет четыре custom ресурса:
  * [prometheus](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheus) — определяет инсталляцию (кластер) Prometheus
  * [servicemonitor](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#servicemonitor) — определяет, как "мониторить" (собирать метрики) набор сервисов
  * [alertmanager](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#alertmanager) — определяет кластер Alertmanager'ов
  * [prometheusrule](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheusrule) — определяет список Prometheus rules
* Следит за ресурсами `prometheus` и генерирует для каждого:
  * StatefulSet (с самим Prometheus'ом)
  * Secret с `prometheus.yaml` (конфиг Prometheus'а) и `configmaps.json` (конфиг для `prometheus-config-reloader`)
* Следит за ресурсами `servicemonitor` и `prometheusrule` и на их основании обновляет конфиги (`prometheus.yaml` и `configmaps.json`, которые лежат в секрете).

#### Что в Pod'е с Prometheus'ом?

![Что в Pod Prometheus](../../images/200-operator-prometheus/pod.png)

* Два контейнера:
  * `prometheus` — сам Prometheus
  * `prometheus-config-reloader` — [обвязка](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader), которая:
    * следит за изменениями `prometheus.yaml` и, при необходимости, вызывает reload конфигурации Prometheus'у (специальным HTTP-запросом, см. [подробнее ниже](#как-обрабатываются-service-monitorы))
    * следит за PrometheusRule'ами (см. [подробнее ниже](#как-обрабатываются-custome-resources-с-ruleами)) и по необходимости скачивает их и перезапускает Prometheus
* Pod использует три volume:
  * config — примонтированный secret (два файла: `prometheus.yaml` и `configmaps.json`). Подключен в оба контейнера.
  * rules — `emptyDir`, который наполняет `prometheus-config-reloader`, а читает `prometheus`. Подключен в оба контейнера, но в `prometheus` в режиме read only.
  * data — данные Prometheus. Подмонтирован только в `prometheus`.

#### Как обрабатываются Service Monitor'ы?

![Как обрабатываются Service Monitor'ы](../../images/200-operator-prometheus/servicemonitors.png)

* **(1)** Prometheus Operator читает (а также следит за добавлением/удалением/изменением) Service Monitor'ы (какие именно Service Monitor'ы — указано в самом ресурсе `prometheus`, см. подробней [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#prometheusspec)).
* **(2)** Для каждого Service Monitor'а, если в нем НЕ указан конкретный список namespace'ов (указано `any: true`), Prometheus Operator вычисляет (обращаясь к API Kubernetes) список namespace'ов, в которых есть Service'ы (подходящие под указанные в Service Monitor'е label'ы).
* **(3)** На основании прочитанных ресурсов `servicemonitor` (см. [официальную документацию](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api.md#servicemonitorspec)) и на основании вычисленных namespace'ов Prometheus Operator генерирует часть конфига (секцию `scrape_configs`) и сохраняет конфиг в соответствующий Secret.
* **(4)** Штатными средствами самого Kubernetes данные из секрета прилетают в Pod (файл `prometheus.yaml` обновляется).
* **(5)** Изменение файла замечает `prometheus-config-reloader`, который по HTTP отправляет запрос Prometheus'у на перезагрузку.
* **(6)** Prometheus перечитывает конфиг и видит изменения в scrape_configs, которые обрабатывает уже согласно своей логике работы (см. подробнее выше).

#### Как обрабатываются Custome Resources с *rule'ами*?

![Как обрабатываются Custome Resources с rule'ами](../../images/200-operator-prometheus/rules.png)

* **(1)** Prometheus Operator следит за PrometheusRule'ами (подходящими под указанный в ресурсе `prometheus` `ruleSelector`).
* **(2)** Если появился новый (или был удален существующий) PrometheusRule — Prometheus Operator обновляет `prometheus.yaml` (а дальше срабатывает логика в точности соответствующая обработке Service Monitor'ов, которая описана выше).
* **(3)** Как в случае добавления/удаления PrometheusRule'а, так и при изменении содержимого PrometheusRule'а, Prometheus Operator обновляет ConfigMap `prometheus-main-rulefiles-0`.
* **(4)** Штатными средствами самого Kubernetes данные из ConfigMap прилетают в Pod
* Изменение файла замечает `prometheus-config-reloader`, который:
  * **(5)** скачивает изменившиеся ConfigMap'ы в директорию rules (это `emptyDir`)
  * **(6)** по HTTP отправляет запрос Prometheus'у на перезагрузку
* **(7)** Prometheus перечитывает конфиг и видит изменившиеся *rule'ы*.

## Модуль vertical-pod-autoscaler

Vertical Pod Autoscaler ([VPA](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)) — это инфраструктурный сервис, который позволяет не выставлять точные resource requests, если неизвестно, сколько ресурсов необходимо контейнеру для работы. При использовании VPA и включении соответствующего режима работы resource requests выставляются автоматически на основе потребления ресурсов (полученных данных из Prometheus).
Как вариант, возможно только получать рекомендации по ресурсам, без их автоматического изменения.

У VPA есть следующие режимы работы:
- `"Auto"` (default) — в данный момент режимы работы Auto и Recreate делают одно и то же. Однако, когда в kubernetes появится [Pod in-place resource update](https://github.com/kubernetes/design-proposals-archive/blob/main/autoscaling/vertical-pod-autoscaler.md#in-place-updates), данный режим будет делать именно его.
- `"Recreate"` — данный режим разрешает VPA изменять ресурсы у запущенных подов, то есть рестартить их при работе. В случае работы одного пода (`replicas: 1`) это приведет к недоступности сервиса на время рестарта. В данном режиме VPA не пересоздает поды, которые были созданы без контроллера.
- `"Initial"` — VPA изменяет ресурсы подов только при создании подов, но не во время работы.
- `"Off"` — VPA не изменяет автоматически никакие ресурсы. В данном случае, если есть VPA c таким режимом работы, мы можем посмотреть, какие ресурсы рекомендует поставить VPA (kubectl describe vpa <vpa-name>).

Ограничения VPA:
- Обновление ресурсов запущенных подов — это экспериментальная возможность VPA. Каждый раз, когда VPA обновляет `resource requests` пода, под пересоздается. Соответственно, под может быть создан на другом узле.
- VPA **не должен использоваться с [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) по CPU и памяти** в данный момент. Однако VPA можно использовать с HPA на custom/external metrics.
- VPA реагирует почти на все `out-of-memory` events, но не гарантирует реакцию (почему так — выяснить из документации не удалось).
- Производительность VPA не тестировалась на огромных кластерах.
- Рекомендации VPA могут превышать доступные ресурсы в кластере, что **может приводить к подам в состоянии Pending**.
- Использование нескольких VPA-ресурсов над одним подом может привести к неопределенному поведению.
- В случае удаления VPA или его «выключения» (режим `Off`) изменения, внесенные ранее VPA, не сбрасываются, а остаются в последнем измененном значении. Из-за этого может возникнуть путаница, что в Helm будут описаны одни ресурсы, при этом в контроллере тоже будут описаны одни ресурсы, но реально у подов ресурсы будут совсем другие и может сложиться впечатление, что они взялись «непонятно откуда».

> **Важно!** При использовании VPA настоятельно рекомендуется использовать [Pod Disruption Budget](https://kubernetes.io/docs/tasks/run-application/configure-pdb/).

### Grafana dashboard

На досках:
- `Main / Namespace`, `Main / Namespace / Controller`, `Main / Namespace / Controller / Pod` — столбец `VPA type` показывает значение `updatePolicy.updateMode`;
- `Main / Namespaces` — столбец `VPA %` показывает процент подов с включенным VPA.

### Архитектура Vertical Pod Autoscaler

VPA состоит из 3 компонентов:
- `Recommender` — мониторит настоящее (делая запросы в [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md), который реализован в модуле [`prometheus-metrics-adapter`](../../modules/301-prometheus-metrics-adapter/)) и прошлое потребление ресурсов (делая запросы в Trickster перед Prometheus) и предоставляет рекомендации по CPU и памяти для контейнеров.
- `Updater` — проверяет, что у подов с VPA выставлены корректные ресурсы, если нет — убивает эти поды, чтобы контроллер пересоздал поды с новыми resource requests.
- `Admission Plugin` — задает resource requests при создании новых подов (контроллером или из-за активности Updater'а).

При изменении ресурсов компонентом Updater это происходит с помощью [Eviction API](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/#the-eviction-api), поэтому учитываются `Pod Disruption Budget` для обновляемых подов.

![Архитектура VPA](https://raw.githubusercontent.com/kubernetes/design-proposals-archive/acc25e14ca83dfda4f66d8cb1f1b491f26e78ffe/autoscaling/images/vpa-architecture.png)

## 
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: speaker
  namespace: d8-{{ .Chart.Name }}
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: DaemonSet
    name: speaker
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: speaker
      minAllowed:
        {{- include "speaker_resources" . | nindent 8 }}
      maxAllowed:
        cpu: 20m
        memory: 60Mi
    {{- include "helm_lib_vpa_kube_rbac_proxy_resources" . | nindent 4 }}
```

Helm-функции, описанные в файле `vpa.yaml` используются так же для установки ресурсов контейнеров в случае, если модуль `vertical-pod-autoscaler` отключен.

Для проставления ресурсов для `kube-rbac-proxy` используется специальная helm-функция `helm_lib_container_kube_rbac_proxy_resources`.

Пример:

```yaml
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: speaker
  namespace: d8-{{ .Chart.Name }}
spec:
  selector:
    matchLabels:
      app: speaker
  template:
    metadata:
      labels:
        app: speaker
    spec: 
    containers:
      - name: speaker
        resources:
          requests:
          {{- if not ( .Values.global.enabledModules | has "vertical-pod-autoscaler") }}
            {{- include "speaker_resources" . | nindent 14 }}
          {{- end }}
      - name: kube-rbac-proxy
        resources:
          requests:
          {{- if not ( .Values.global.enabledModules | has "vertical-pod-autoscaler") }}
            {{- include "helm_lib_container_kube_rbac_proxy_resources" . | nindent 12 }}
          {{- end }}
```

### Специальные лейблы для VPA-ресурсов

Если Pod'ы присутствуют только на мастер-узлах, для VPA-ресурса добавляется label `workload-resource-policy.deckhouse.io: master`.

Если Pod'ы присутствуют на каждом узле, для VPA-ресурса добавляется label `workload-resource-policy.deckhouse.io: every-node`.

### TODO

* В настоящий момент для проставления ресурсов контейнеров используются значения из `minAllowed`. В этом случае возможен оверпровижининг на узле. Возможно правильнее было бы использовать значения `maxAllowed`.
* Значения `minAllowed` и `maxAllowed` проставляются вручную, возможно, определять нужно что-то одно, а второе вычислять. Например, определять `minAllowed` а `maxAllowed` считать как `minAllowed` X 2.
* Возможно стоит придумать другой механизм задания значений `minAllowed`, например, отдельный файл, в котором в YAML-структуре будут собраны данные по ресурсам всех контейнеров всех модулей.
* [Issue #2084](https://github.com/deckhouse/deckhouse/issues/2084).

## Модуль secret-copier

Данный модуль отвечает за копирование Secret'ов во все namespace'ы.

Нам данный модуль полезен тем, чтобы не копировать каждый раз в CI Secret'ы для пуллинга образов и заказа RBD в Ceph.

#### Как работает

Данный модуль следит за изменениями Secret'ов в namespace `default` с лейблом `secret-copier.deckhouse.io/enabled: ""`.
* При создании такого Secret'а он будет скопирован во все namespace.
* При изменении Secret'а его новое содержимое будет раскопировано во все namespace.
* При удалении Secret'а он будет удален из всех namespace.
* При изменении скопированного Secret'а в прикладном namespace тот будет перезаписан оригинальным содержимым.
* При создании любого namespace в него копируются все Secret'ы из default namespace с лейблом `secret-copier.deckhouse.io/enabled: ""`.

Кроме этого, каждую ночь Secret'ы будут повторно синхронизированы и приведены к состоянию в default namespace.

#### Что нужно настроить?

Чтобы все заработало, достаточно создать в default namespace Secret с лейблом `secret-copier.deckhouse.io/enabled: ""`.

#### Как ограничить список namespace'ов, в которые будет производиться копирование?

Задайте label–селектор в значении аннотации `secret-copier.deckhouse.io/target-namespace-selector`. Например: `secret-copier.deckhouse.io/target-namespace-selector: "app=custom"`. Модуль создаст копию этого Secret'а во всех пространствах имен, соответствующих заданному label–селектору.

## Модуль local-path-provisioner

Позволяет пользователям Kubernetes использовать локальное хранилище на узлах.

### Как это работает?

Для каждого custom resource [LocalPathProvisioner](cr.html) создается соответствующий `StorageClass`.

Допустимая топология для `StorageClass` вычисляется на основе списка `nodeGroup` из custom resource. Топология используется при шедулинге подов.

Когда под заказывает диск, то:
- создается `HostPath` PV;
- `Provisioner` создает на нужном узле локальную папку по пути, состоящем из параметра `path` custom resource, имени PV и имени PVC.
  
  Пример пути:

  ```shell
  /opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
  ```

### Ограничения

- Ограничение на размер диска не поддерживается для локальных томов.

## Модуль kube-dns

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

> **Внимание!** Модуль удаляет ранее установленные kubeadm'ом Deployment, ConfigMap и RBAC для CoreDNS.

## Управление control plane

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

### Управление сертификатами

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

### Масштабирование

Поддерживается работа `control-plane` в конфигурации как *single-master*, так и *multi-master*.

В конфигурации *single-master*:
- `kube-apiserver` использует только тот экземпляр `etcd`, который размещен с ним на одном узле;
- На узле настраивается прокси-сервер, отвечающий на localhost,`kube-apiserver` отвечает на IP-адрес master-узла.

В конфигурации *multi-master* компоненты `control-plane` автоматически разворачиваются в отказоустойчивом режиме:
- `kube-apiserver` настраивается для работы со всеми экземплярами `etcd`.
- На каждом master-узле настраивается дополнительный прокси-сервер, отвечающий на localhost. Прокси-сервер по умолчанию обращается к локальному экземпляру `kube-apiserver`, но в случае его недоступности последовательно опрашивает остальные экземпляры `kube-apiserver`.

#### Масштабирование master-узлов

Масштабирование узлов `control-plane` осуществляется автоматически, с помощью лейбла `node-role.kubernetes.io/control-plane=””`:
- Установка лейбла `node-role.kubernetes.io/control-plane=””` на узле приводит к развертыванию на нем компонентов `control-plane`, подключению нового узла `etcd` в etcd-кластер, а также перегенерации необходимых сертификатов и конфигурационных файлов.
- Удаление лейбла `node-role.kubernetes.io/control-plane=””` с узла приводит к удалению всех компонентов `control-plane`, перегенерации необходимых конфигурационных файлов и сертификатов, а также корректному исключению узла из etcd-кластера.

> **Важно!** При масштабировании узлов с 2 до 1 требуются **ручные действия** с `etcd`. В остальных случаях все необходимые действия происходят автоматически.

### Управление версиями

Обновление **patch-версии** компонентов control plane (то есть в рамках минорной версии, например с `1.27.3` на `1.27.5`) происходит автоматически вместе с обновлением версии Deckhouse. Управлять обновлением patch-версий нельзя.

Обновлением **минорной-версии** компонентов control plane (например, с `1.26.*` на `1.28.*`) можно управлять с помощью параметра [kubernetesVersion](../../installing/configuration.html#clusterconfiguration-kubernetesversion), в котором можно выбрать автоматический режим обновления (значение `Automatic`) или указать желаемую минорную версию control plane. Версию control plane, которая используется по умолчанию (при `kubernetesVersion: Automatic`), а также список поддерживаемых версий Kubernetes можно найти в [документации](../../supported_versions.html#kubernetes).

Обновление control plane выполняется безопасно и для single-master-, и для multi-master-кластеров. Во время обновления может быть кратковременная недоступность API-сервера. На работу приложений в кластере обновление не влияет и может выполняться без выделения окна для регламентных работ.

Если указанная для обновления версия (параметр [kubernetesVersion](../../installing/configuration.html#clusterconfiguration-kubernetesversion)) не соответствует текущей версии control plane в кластере, запускается умная стратегия изменения версий компонентов:
- Общие замечания:
  - Обновление в разных NodeGroup выполняется параллельно. Внутри каждой NogeGroup узлы обновляются последовательно, по одному.
- При upgrade:
  - Обновление происходит **последовательными этапами**, по одной минорной версии: 1.26 -> 1.27, 1.27 -> 1.28, 1.28 -> 1.29.
  - На каждом этапе сначала обновляется версия control plane, затем происходит обновление kubelet на узлах кластера.  
- При downgrade:
  - Успешный downgrade гарантируется только на одну версию вниз от максимальной минорной версии control plane, когда-либо использовавшейся в кластере.
  - Сначала происходит downgrade kubelet'a на узлах кластера, затем — downgrade компонентов control plane.

### Аудит

Если требуется журналировать операции с API или отдебажить неожиданное поведение, для этого в Kubernetes предусмотрен [Auditing](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/). Его можно настроить путем создания правил [Audit Policy](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy), а результатом работы аудита будет лог-файл `/var/log/kube-audit/audit.log` со всеми интересующими операциями.

В установках Deckhouse по умолчанию созданы базовые политики, которые отвечают за логирование событий:
- связанных с операциями создания, удаления и изменения ресурсов;
- совершаемых от имен сервисных аккаунтов из системных Namespace `kube-system`, `d8-*`;
- совершаемых с ресурсами в системных пространствах имен `kube-system`, `d8-*`.

Для выключения базовых политик установите флаг [basicAuditPolicyEnabled](configuration.html#parameters-apiserver-basicauditpolicyenabled) в `false`.

Настройка политик аудита подробно рассмотрена в [одноименной секции FAQ](faq.html#как-настроить-дополнительные-политики-аудита).

## Модуль istio

### Таблица совместимости поддерживаемых версий

| Версия Istio | [Версии K8S, поддерживаемые Istio](https://istio.io/latest/docs/releases/supported-releases/#support-status-of-istio-releases) | Статус в текущем релизе D8 |
|:------------:|:------------------------------------------------------------------------------------------------------------------------------:|:--------------------------:|
|     1.16     |                                           1.22<sup>*</sup>, 1.23<sup>*</sup>, 1.24<sup>*</sup>, 1.25                                            |  Устарела и будет удалена  |
|     1.19     |                                                     1.25, 1.26, 1.27, 1.28                                                     |       Поддерживается       |

<sup>*</sup> — версия Kubernetes **НЕ поддерживается** в текущем релизе Deckhouse Kubernetes Platform.

{::options parse_block_html="false" /}

### Задачи, которые решает Istio

[Istio](https://istio.io/) — фреймворк централизованного управления сетевым трафиком, реализующий подход Service Mesh.

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

> Рекомендуем ознакомиться с [видео](https://www.youtube.com/watch?v=9CUfaeT3T-A), где мы обсуждаем архитектуру Istio и оцениваем накладные расходы.

### Mutual TLS

Данный механизм — это главный метод взаимной аутентификации сервисов. Принцип основывается на том, что при всех исходящих запросах проверяется серверный сертификат, а при входящих — клиентский. После проверок sidecar-proxy получает возможность идентифицировать удаленный узел и использовать эти данные для авторизации либо в прикладных целях.

Каждый сервис получает собственный идентификатор в формате `<TrustDomain>/ns/<Namespace>/sa/<ServiceAccount>`, где `TrustDomain` в нашем случае — это домен кластера. Каждому сервису можно выделять собственный ServiceAccount или использовать стандартный «default». Полученный идентификатор сервиса можно использовать как в правилах авторизации, так и в прикладных целях. Именно этот идентификатор используется в качестве удостоверяемого имени в TLS-сертификатах.

Данные настройки можно переопределить на уровне namespace.

### Авторизация

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

### Маршрутизация запросов

Основной ресурс для управления маршрутизацией — [VirtualService](istio-cr.html#virtualservice), он позволяет переопределять судьбу HTTP- или TCP-запроса. Доступные аргументы для принятия решения о маршрутизации:

* Host и любые другие заголовки;
* URI;
* метод (GET, POST и пр.);
* лейблы пода или namespace источника запросов;
* dst-IP или dst-порт для не-HTTP-запросов.

### Управление балансировкой запросов между endpoint'ами сервиса

Основной ресурс для управления балансировкой запросов — [DestinationRule](istio-cr.html#destinationrule), он позволяет настроить нюансы исходящих из подов запросов:

* лимиты/таймауты для TCP;
* алгоритмы балансировки между endpoint'ами;
* правила определения проблем на стороне endpoint'а для выведения его из балансировки;
* нюансы шифрования.

> **Важно!** Все настраиваемые лимиты работают для каждого пода клиента по отдельности! Если настроить для сервиса ограничение на одно TCP-соединение, а клиентских подов — три, то сервис получит три входящих соединения.

### Observability

#### Трассировка

Istio позволяет осуществлять сбор трейсов с приложений и инъекцию трассировочных заголовков, если таковых нет. При этом важно понимать следующее:

* Если запрос инициирует на сервисе вторичные запросы, для них необходимо наследовать трассировочные заголовки средствами приложения.
* Jaeger для сбора и отображения трейсов потребуется устанавливать самостоятельно.

#### Grafana

В стандартной комплектации с модулем предоставлены дополнительные доски:

* доска для оценки производительности и успешности запросов/ответов между приложениями;
* доска для оценки работоспособности и нагрузки на control plane.

#### Kiali

Инструмент для визуализации дерева сервисов вашего приложения. Позволяет быстро оценить обстановку в сетевой связности благодаря визуализации запросов и их количественных характеристик непосредственно на схеме.

### Архитектура кластера с включенным Istio

Компоненты кластера делятся на две категории:

* control plane — управляющие и обслуживающие сервисы. Под control plane обычно подразумевают поды istiod.
* data plane — прикладная часть Istio. Представляет собой контейнеры sidecar-proxy.

![Архитектура кластера с включенным Istio](../../images/110-istio/istio-architecture.svg)
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

* К подам контроллера добавляется sidecar-proxy, который обслуживает только трафик от контроллера в сторону прикладных сервисов (параметр IngressNginxController [`enableIstioSidecar`](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у ресурса IngressNginxController).
* Сервисы не под управлением Istio продолжают работать как раньше, запросы в их сторону не перехватываются сайдкаром контроллера.
* Запросы в сторону сервисов под управлением Istio перехватываются сайдкаром и обрабатываются в соответствии с правилами Istio (подробнее о том, [как активировать Istio для приложения](#как-активировать-istio-для-приложения)).

Контроллер istiod и каждый контейнер sidecar-proxy экспортируют собственные метрики, которые собирает кластерный Prometheus.

### Архитектура прикладного сервиса с включенным Istio

#### Особенности

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

#### Жизненный цикл пользовательского запроса

##### Приложение с выключенным Istio

<div data-presentation="../../presentations/110-istio/request_lifecycle_istio_disabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1_lw3EyDNTFTYNirqEfrRANnEAVjGhrOCdFJc-zCOuvs/ --->

##### Приложение с включенным Istio

<div data-presentation="../../presentations/110-istio/request_lifecycle_istio_enabled_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1gQfX9ge2vhp74yF5LOfpdK2nY47l_4DIvk6px_tAMPU/ --->

### Как активировать Istio для приложения

Основная цель активации — добавить sidecar-контейнер к подам приложения, после чего Istio сможет управлять трафиком.

Рекомендованный способ добавления sidecar-ов — использовать sidecar-injector. Istio умеет «подселять» к вашим подам sidecar-контейнер с помощью механизма [Admission Webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/). Настраивается с помощью лейблов и аннотаций:

* Лейбл к namespace — обращает внимание компонента sidecar-injector на ваш namespace. После применения лейбла к новым подам будут добавлены sidecar-контейнеры:
  * `istio-injection=enabled` — использует глобальную версию Istio (`spec.settings.globalVersion` в `ModuleConfig`);
  * `istio.io/rev=v1x16` — использует конкретную версию Istio для этого namespace.
* Аннотация к **поду** `sidecar.istio.io/inject` (`"true"` или `"false"`) позволяет локально переопределить политику `sidecarInjectorPolicy`. Эти аннотации работают только в namespace, обозначенных лейблами из списка выше.

Также существует возможность добавить sidecar к индивидуальному поду в namespace без установленных лейблов `istio-injection=enabled` или `istio.io/rev=vXxYZ` путем установки лейбла `sidecar.istio.io/inject=true`.

**Важно!** Istio-proxy, который работает в качестве sidecar-контейнера, тоже потребляет ресурсы и добавляет накладные расходы:

* Каждый запрос DNAT'ится в Envoy, который обрабатывает это запрос и создает еще один. На принимающей стороне — аналогично.
* Каждый Envoy хранит информацию обо всех сервисах в кластере, что требует памяти. Больше кластер — больше памяти потребляет Envoy. Решение — CustomResource [Sidecar](istio-cr.html#sidecar).

Также важно подготовить Ingress-контроллер и Ingress-ресурсы приложения:

* Включить [`enableIstioSidecar`](../402-ingress-nginx/cr.html#ingressnginxcontroller-v1-spec-enableistiosidecar) у ресурса IngressNginxController.
* Добавить аннотации на Ingress-ресурсы приложения:
  * `nginx.ingress.kubernetes.io/service-upstream: "true"` — Ingress-контроллер в качестве upstream будет использовать ClusterIP сервиса вместо адресов подов. Балансировкой трафика между подами теперь занимается sidecar-proxy. Используйте эту опцию, только если у вашего сервиса есть ClusterIP;
  * `nginx.ingress.kubernetes.io/upstream-vhost: "myservice.myns.svc"` — sidecar-proxy Ingress-контроллера принимает решения о маршрутизации на основе заголовка Host. Без данной аннотации контроллер оставит заголовок с адресом сайта, например `Host: example.com`.

### Федерация и мультикластер

> Доступно только в редакции Enterprise Edition.

Поддерживаются две схемы межкластерного взаимодействия:

* [федерация](#федерация);
* [мультикластер](#мультикластер).

Принципиальные отличия:

* Федерация объединяет суверенные кластеры:
  * у каждого кластера собственное пространство имен (для namespace, Service и пр.);
  * доступ к отдельным сервисам между кластерами явно обозначен.
* Мультикластер объединяет созависимые кластеры:
  * пространство имен у кластеров общее — каждый сервис доступен для соседних кластеров так, словно он работает на локальном кластере (если это не запрещают правила авторизации).

#### Федерация

##### Требования к кластерам

* У каждого кластера должен быть уникальный домен в параметре [`clusterDomain`](../../installing/configuration.html#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](../../installing/configuration.html#clusterconfiguration). По умолчанию значение параметра — `cluster.local`.
* Подсети подов и сервисов в параметрах [`podSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-podsubnetcidr) и [`serviceSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-servicesubnetcidr) ресурса [*ClusterConfiguration*](../../installing/configuration.html#clusterconfiguration) не должны быть уникальными.

##### Общие принципы федерации

* Федерация требует установления взаимного доверия между кластерами. Соответственно, для установления федерации нужно в кластере A сделать кластер Б доверенным и аналогично в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для прикладной эксплуатации федерации необходимо также обменяться информацией о публичных сервисах. Чтобы опубликовать сервис bar из кластера Б в кластере А, необходимо в кластере А создать ресурс ServiceEntry, который описывает публичный адрес ingress-gateway кластера Б.

<div data-presentation="../../presentations/110-istio/federation_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1EI2MQMuVCGACnLNBXMGVDNJVhwU3vJYtVcHhrWfjLDc/ --->

##### Включение федерации

При включении федерации (параметр модуля `istio.federation.enabled = true`) происходит следующее:

* В кластер добавляется сервис `ingressgateway`, чья задача — проксировать mTLS-трафик извне кластера на прикладные сервисы.
* В кластер добавляется сервис, который экспортит метаданные кластера наружу:
  * корневой сертификат Istio (доступен без аутентификации);
  * список публичных сервисов в кластере (доступен только для аутентифицированных запросов из соседних кластеров);
  * список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).

##### Управление федерацией

<div data-presentation="../../presentations/110-istio/federation_istio_federation_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1MpmtwJwvSL32EdwOUNpJ6GjgWt0gplzjqL8OOprNqvc/ --->

Для построения федерации необходимо сделать следующее:

* В каждом кластере создать набор ресурсов `IstioFederation`, которые описывают все остальные кластеры.
  * После успешного автосогласования между кластерами, в ресурсе `IstioFederation` заполнятся разделы `status.metadataCache.public` и `status.metadataCache.private` служебными данными, необходимыми для работы федерации.
* Каждый ресурс(`service`), который считается публичным в рамках федерации, пометить лейблом `federation.istio.deckhouse.io/public-service=`.
  * В кластерах из состава федерации, для каждого `service` создадутся соответствующие `ServiceEntry`, ведущие на `ingressgateway` оригинального кластера.

> Важно чтобы в этих `service`, в разделе `.spec.ports` у каждого порта обязательно было заполнено поле `name`.

#### Мультикластер

##### Требования к кластерам

* Домены кластеров в параметре [`clusterDomain`](../../installing/configuration.html#clusterconfiguration-clusterdomain) ресурса [*ClusterConfiguration*](../../installing/configuration.html#clusterconfiguration) должны быть одинаковыми для всех членов мультикластера. По умолчанию значение параметра — `cluster.local`.
* Подсети подов и сервисов в параметрах [`podSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-podsubnetcidr) и [`serviceSubnetCIDR`](../../installing/configuration.html#clusterconfiguration-servicesubnetcidr) ресурса [*ClusterConfiguration*](../../installing/configuration.html#clusterconfiguration) должны быть уникальными для каждого члена мультикластера.

##### Общие принципы

<div data-presentation="../../presentations/110-istio/multicluster_common_principles_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1WeNrp0Ni2Tz3_Az0f45rkWRUZxZUDx93Om5MB3sEod8/ --->

* Мультикластер требует установления взаимного доверия между кластерами. Соответственно, для построения мультикластера нужно в кластере A сделать кластер Б доверенным и в кластере Б сделать кластер А доверенным. Технически это достигается взаимным обменом корневыми сертификатами.
* Для сбора информации о соседних сервисах Istio подключается напрямую к API-серверу соседнего кластера. Данный модуль Deckhouse берет на себя организацию соответствующего канала связи.

##### Включение мультикластера

При включении мультикластера (параметр модуля `istio.multicluster.enabled = true`) происходит следующее:

* В кластер добавляется прокси для публикации доступа к API-серверу посредством стандартного Ingress-ресурса:
  * Доступ через данный публичный адрес ограничен авторизацией на основе Bearer-токенов, подписанных доверенными ключами. Обмен доверенными публичными ключами происходит автоматически средствами Deckhouse при взаимной настройке мультикластера.
  * Непосредственно прокси имеет read-only-доступ к ограниченному набору ресурсов.
* В кластер добавляется сервис, который экспортит метаданные кластера наружу:
  * Корневой сертификат Istio (доступен без аутентификации).
  * Публичный адрес, через который доступен API-сервер (доступен только для аутентифицированных запросов из соседних кластеров).
  * Список публичных адресов сервиса `ingressgateway` (доступен только для аутентифицированных запросов из соседних кластеров).
  * Публичные ключи сервера для аутентификации запросов к API-серверу и закрытым метаданным (см. выше).

##### Управление мультикластером

<div data-presentation="../../presentations/110-istio/multicluster_istio_multicluster_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1D3nuoC0okJQRCOY4teJ6p598Bd4JwPXZT5cdG0hW8Hc/ --->

Для сборки мультикластера необходимо в каждом кластере создать набор ресурсов `IstioMulticluster`, которые описывают все остальные кластеры.

### Накладные расходы

Внедрение Istio повлечёт за собой дополнительные расходы ресурсов, как для **control-plane** (контроллер istiod), так и для **data-plane** (istio-сайдкары приложений).

#### control-plane

Контроллер istiod непрерывно наблюдает за конфигурацией кластера, компонует настройки для istio-сайдкаров data-plane и рассылает их по сети. Соответственно, чем больше приложений и их экземпляров, чем больше сервисов и чем чаще эта конфигурация меняется, тем больше требуется вычислительных ресурсов и больше нагрузка на сеть. При этом, поддерживается два подхода для снижения нагрузки на экземпляры контроллеров:
* горизонтальное масштабирование (настройка модуля [`controlPlane.replicasManagement`](configuration.html#parameters-controlplane-replicasmanagement)) — чем больше экземпляров контроллеров, тем меньше экземпляров istio-сайдкаров обслуживать каждому из них и тем меньше нагрузка на CPU и на сеть.
* сегментация data-plane с помощью ресурса [*Sidecar*](istio-cr.html#sidecar) (рекомендуемый подход) — чем меньше область видимости у отдельного istio-сайдкара, тем меньше требуется обновлять данных в data-plane и тем меньше нагрузка на CPU и на сеть.

Примерная оценка накладных расходов для экземпляра control-plane, который обслуживает 1000 сервисов и 2000 istio-сайдкаров — 1 vCPU и 1.5GB RAM.

#### data-plane

На потребление ресурсов data-plane (istio-сайдкары) влияет множество факторов:

* количество соединений,
* интенсивность запросов,
* размер запросов и ответов,
* протокол (HTTP/TCP),
* количество ядер CPU,
* сложность конфигурации Service Mesh.

Примерная оценка накладных расходов для экземпляра istio-сайдкара — 0.5 vCPU на 1000 запросов/сек и 50MB RAM.

istio-сайдкары также вносят задержку в сетевые запросы — примерно 2.5мс на запрос.

## Модуль prometheus-metrics-adapter

Позволяет работать [HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)- и [VPA](../../modules/302-vertical-pod-autoscaler/)-автоскейлерам по «любым» метрикам.

Устанавливает в кластер [имплементацию](https://github.com/kubernetes-sigs/prometheus-adapter) Kubernetes [resource metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md), [custom metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/custom-metrics-api.md) и [external metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/external-metrics-api.md) для получения метрик из Prometheus.

Это позволяет:
- `kubectl top` брать метрики из Prometheus, через адаптер;
- использовать custom resource версии [autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling) для масштабирования приложений (HPA);
- получать информацию из Prometheus средствами API Kubernetes для других модулей (Vertical Pod Autoscaler, ...).

Модуль позволяет производить масштабирование по следующим параметрам:
* CPU (пода);
* память (пода);
* rps (Ingress'а) — за 1, 5, 15 минут (`rps_Nm`);
* CPU (пода) — за 1, 5, 15 минут (`cpu_Nm`) — среднее потребление CPU за N минут;
* память (пода) — за 1, 5, 15 минут (`memory_Nm`) — среднее потребление памяти за N минут;
* любые Prometheus-метрики и любые запросы на их основе.

### Как работает

Данный модуль регистрирует `k8s-prometheus-adapter` в качестве external API-сервиса, который расширяет возможности Kubernetes API. Когда какому-то из компонентов Kubernetes (VPA, HPA) требуется информация об используемых ресурсах, он делает запрос в Kubernetes API, а тот, в свою очередь, проксирует запрос в адаптер. Адаптер на основе своего [конфигурационного файла](https://github.com/deckhouse/deckhouse/blob/main/modules/301-prometheus-metrics-adapter/templates/config-map.yaml) выясняет, как посчитать метрику, и отправляет запрос в Prometheus.

## Модуль chrony

Обеспечивает синхронизацию времени на всех узлах кластера с помощью модуля [chrony](https://chrony.tuxfamily.org/).

## Модуль log-shipper

Модуль разворачивает агенты `log-shipper` для сборки логов на узлы кластера.
Предназначение этих агентов — с минимальными изменениями отправить логи дальше из кластера.
Каждый агент — это отдельный [vector](https://vector.dev/), конфигурацию для которого сгенерировал Deckhouse.

![log-shipper architecture](../../images/460-log-shipper/log_shipper_architecture.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1cOm5emdfPqWp9NT1UrB__TTL31lw7oCgh0VicQH-ouc/edit -->

1. Deckhouse следит за ресурсами [ClusterLoggingConfig](cr.html#clusterloggingconfig), [ClusterLogDestination](cr.html#clusterlogdestination) и [PodLoggingConfig](cr.html#podloggingconfig).
   Комбинация конфигурации для сбора логов и направления для отправки называется `pipeline`.
2. Deckhouse генерирует конфигурационный файл и сохраняет его в `Secret` в Kubernetes.
3. `Secret` монтируется всем подам агентов `log-shipper`, конфигурация обновляется при ее изменении с помощью sidecar-контейнера `reloader`.

### Топологии отправки

Этот модуль отвечает за агентов на каждом узле. Однако подразумевается, что логи из кластера отправляются согласно одной из описанных ниже топологий.

#### Распределенная

Агенты шлют логи напрямую в хранилище, например в Loki или Elasticsearch.

![log-shipper distributed](../../images/460-log-shipper/log_shipper_distributed.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1FFuPgpDHUGRdkMgpVWXxUXvfZTsasUhEh8XNz7JuCTQ/edit -->

* Менее сложная схема для использования.
* Доступна из коробки без лишних зависимостей, кроме хранилища.
* Сложные трансформации потребляют больше ресурсов на узлах для приложений.

#### Централизованная

Все логи отсылаются в один из доступных агрегаторов, например, Logstash, Vector.
Агенты на узлах стараются отправить логи с узла максимально быстро с минимальным потреблением ресурсов.
Сложные преобразования применяются на стороне агрегатора.

![log-shipper centralized](../../images/460-log-shipper/log_shipper_centralized.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1TL-YUBk0CKSJuKtRVV44M9bnYMq6G8FpNRjxGxfeAhQ/edit -->

* Меньше потребление ресурсов для приложений на узлах.
* Пользователи могут настроить в агрегаторе любые трансформации и слать логи в гораздо большее количество хранилищ.
* Количество выделенных узлов под агрегаторы может увеличиваться вверх и вниз в зависимости от нагрузки.

#### Потоковая

Главная задача данной архитектуры — как можно быстрее отправить логи в очередь сообщений, из которой они в служебном порядке будут переданы в долгосрочное хранилище для дальнейшего анализа.

![log-shipper stream](../../images/460-log-shipper/log_shipper_stream.svg)
<!-- Исходник картинок: https://docs.google.com/drawings/d/1R7vbJPl93DZPdrkSWNGfUOh0sWEAKnCfGkXOvRvK3mQ/edit -->

* Те же плюсы и минусы, что и у централизованной архитектуры, но добавляется еще одно промежуточное хранилище.
* Повышенная надежность. Подходит тем, для кого доставка логов является наиболее критичной.

### Метаданные

При сборе логов сообщения будут обогащены метаданными в зависимости от способа их сбора. Обогащение происходит на этапе `Source`.

#### Kubernetes

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

#### File

Единственный лейбл — это `host`, в котором записан hostname сервера.

### Фильтры сообщений

Существуют два фильтра, чтобы снизить количество отправляемых сообщений в хранилище, — `log filter` и `label filter`.

![log-shipper pipeline](../../images/460-log-shipper/log_shipper_pipeline.svg)
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

## Cloud provider — Azure

Взаимодействие с облачными ресурсами провайдера [Azure](https://portal.azure.com/) осуществляется с помощью модуля `cloud-provider-azure`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Azure при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-azure`:
- Управляет ресурсами Azure с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Azure.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в Azure.
- Заказывает диски в Azure с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [`node-manager`](../../modules/040-node-manager/), чтобы [AzureInstanceClass'ы](cr.html#azureinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

> **Внимание!** При использовании балансировщиков нагрузки исходящий трафик также идет через них. Если ни у одного балансировщика нет правил для UDP, весь исходящий UDP-трафик блокируется, вследствие чего не работают такие утилиты, как `ntpdate` и `chrony`. Для решения проблемы необходимо самостоятельно добавить load balancing rule с любым UDP-портом к уже существующему балансировщику либо в кластере создать сервис с типом LoadBalancer с любым UDP-портом.

## Модуль loki

Модуль предназначен для организации хранилища логов.

Модуль использует проект [Grafana Loki](https://grafana.com/oss/loki/).

Модуль разворачивает хранилище логов на базе Grafana Loki, при необходимости настраивает модуль [log-shipper](../460-log-shipper/) на использование модуля loki и добавляет в Grafana соответствующий datasource.

{% alert level="warning" %}
Модуль не поддерживает работу в режиме высокой доступности, что ограничивает его использование. Для хранения важных журналов рекомендуется использовать другое хранилище.
{% endalert %}

## Модуль openvpn

Дает доступ к ресурсам кластера посредством OpenVPN с аутентификацией по сертификатам, предоставляет простой веб-интерфейс.

Функции веб-интерфейса:
- выпуск сертификатов;
- отзыв сертификатов;
- отмена отзыва сертификатов;
- получение готового пользовательского конфигурационного файла.

Веб-интерфейс проинтегрирован с модулем [user-authn](../150-user-authn/), что позволяет управлять возможностью доступа различных пользователей в этот веб-интерфейс.

### Варианты публикации VPN-сервиса

Обычно для подключения выбирается один или несколько внешних IP-адресов. Поддерживаются следующие методы подключения:
- по внешнему IP-адресу (`ExternalIP`) — когда имеются узлы с публичными IP-адресами.
- посредством `LoadBalancer` — поддерживаются LB для AWS, Google Сloud и др.
- `Direct` — для нестандартных случаев, позволяет вручную настроить путь трафика от точки входа в кластер до пода с OpenVPN.

### Доступные ресурсы кластера после подключения к VPN

На компьютер пользователя после подключения к VPN доставляются (push) следующие параметры:
- адрес `kube-dns` добавляется в DNS-серверы клиента для возможности прямого обращения к сервисам Kubernetes по FQDN;
- маршрут в локальную сеть;
- маршрут в сервисную сеть кластера;
- маршрут в сеть подов.

### Аудит пользовательских соединений

Модуль позволяет включить логирование пользовательской активности через VPN в JSON-формате. Группировка трафика происходит по полям `src_ip`, `dst_ip`, `src_port`, `dst_port`, `ip_proto`. Логи контейнера могут быть собраны и отправлены на хранение для последующего аудита с помощью модуля [log-shipper](../460-log-shipper/).

## Модуль flow-schema

Модуль применяет [FlowSchema and PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) для предотвращения перегрузки API.

`FlowSchema` устанавливает `PriorityLevel` для `list`-запросов от всех сервис-аккаунтов в пространствах имен Deckhouse (у которых установлен label `heritage: deckhouse`) к следующим apiGroup:
* `v1` (Pod, Secret, ConfigMap, Node и т. д.). Это помогает в случае большого количества основных ресурсов в кластере (например, Secret'ов или подов).
* `apps/v1` (DaemonSet, Deployment, StatefulSet, ReplicaSet и т. д.). Это помогает в случае развертывания большого количества приложений в кластере (например, Deployment'ов).
* `deckhouse.io` (custom resource'ы Deckhouse). Это помогает в случае большого количества различных кастомных ресурсов Deckhouse в кластере.
* `cilium.io` (custom resource'ы cilium). Это помогает в случае большого количества политик cilium в кластере.

Все запросы к API, соответствующие `FlowSchema`, помещаются в одну очередь.

## Модуль ingress-nginx

Устанавливает и управляет [NGINX Ingress controller](https://github.com/kubernetes/ingress-nginx) с помощью Custom Resources. Если узлов для размещения Ingress-контроллера больше одного, он устанавливается в отказоустойчивом режиме и учитывает все особенности реализации инфраструктуры облаков и bare metal, а также кластеров Kubernetes различных типов.

Поддерживает запуск и раздельное конфигурирование одновременно нескольких NGINX Ingress controller'ов — один **основной** и сколько угодно **дополнительных**. Например, это позволяет отделять внешние и intranet Ingress-ресурсы приложений.

### Варианты терминирования трафика

Трафик к nginx-ingress может быть отправлен несколькими способами:
- напрямую без внешнего балансировщика;
- через внешний LoadBalancer, в том числе поддерживаются:
  - Qrator,
  - Cloudflare,
  - AWS LB,
  - GCE LB,
  - ACS LB,
  - Yandex LB,
  - OpenStack LB.

### Терминация HTTPS

Модуль позволяет управлять для каждого из NGINX Ingress controller'а политиками безопасности HTTPS, в частности:
- параметрами HSTS;
- набором доступных версий SSL/TLS и протоколов шифрования.

Также модуль интегрирован с модулем [cert-manager](../../modules/101-cert-manager/), при взаимодействии с которым возможны автоматический заказ SSL-сертификатов и их дальнейшее использование NGINX Ingress controller'ами.

### Мониторинг и статистика

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

### Статистика

#### Основные принципы сбора статистики

1. На каждый запрос на стадии `log_by_lua_block` вызывается наш модуль, который рассчитывает необходимые данные и складывает их в буфер (у каждого nginx worker'а свой буфер).
2. На стадии `init_by_lua_block` для каждого nginx worker'а запускается процесс, который раз в секунду асинхронно отправляет данные в формате `protobuf` через TCP socket в `protobuf_exporter` (наша собственная разработка).
3. `protobuf_exporter` запущен sidecar-контейнером в поде с ingress-controller'ом, принимает сообщения в формате `protobuf`, разбирает, агрегирует их по установленным нами правилам и экспортирует в формате для Prometheus.
4. Prometheus каждые 30 секунд scrape'ает как сам ingress-controller (там есть небольшое количество нужных нам метрик), так и protobuf_exporter, на основании этих данных все и работает!

#### Какая статистика собирается и как она представлена

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

## Модуль user-authz

Модуль отвечает за генерацию объектов ролевой модели доступа, основанной на базе стандартного механизма RBAC Kubernetes. Модуль создает набор кластерных ролей (`ClusterRole`), подходящий для большинства задач по управлению доступом пользователей и групп.

{% alert level="warning" %}
С версии Deckhouse Kubernetes Platform v1.64 в модуле реализована новая модель ролевого доступа. Старая модель ролевого доступа продолжит работать, но в будущем перестанет поддерживаться.

Функциональность старой и новой моделей ролевого доступа несовместимы. Автоматическая конвертация ресурсов невозможна.
{% endalert %}

{% alert level="warning" %}
Документация модуля подразумевает использование [новой ролевой модели](#новая-ролевая-модель), если не указано иное.
{% endalert %}

### Новая ролевая модель

В отличие от [устаревшей ролевой модели](#устаревшая-ролевая-модель) DKP, новая ролевая модель не использует ресурсы `ClusterAuthorizationRule` и `AuthorizationRule`. Вся настройка прав доступа выполняется стандартным для RBAC Kubernetes способом: с помощью создания ресурсов `RoleBinding` или `ClusterRoleBinding`, с указанием в них одной из подготовленных модулем `user-authz` ролей.

Модуль создаёт специальные агрегированные кластерные роли (`ClusterRole`). Используя эти роли в `RoleBinding` или `ClusterRoleBinding` можно решать следующие задачи:
- Управлять доступом к модулям определённой [области](#области-ролевой-модели) применения.

  Например, чтобы дать возможность пользователю, выполняющему функции сетевого администратора, настраивать *сетевые* модули (например, `cni-cilium`, `ingress-nginx`, `istio` и т. д.), можно использовать в `ClusterRoleBinding` роль `d8:manage:networking:manager`.
- Управлять доступом к *пользовательским* ресурсам модулей в рамках пространства имён.

  Например, использование роли `d8:use:role:manager` в `RoleBinding`, позволит удалять/создавать/редактировать ресурс [PodLoggingConfig](../460-log-shipper/cr.html#podloggingconfig) в пространстве имён, но не даст доступа к cluster-wide-ресурсам [ClusterLoggingConfig](../460-log-shipper/cr.html#clusterloggingconfig) и [ClusterLogDestination](../460-log-shipper/cr.html#clusterlogdestination) модуля `log-shipper`, а также не даст возможность настраивать сам модуль `log-shipper`.

Роли, создаваемые модулем, делятся на два класса:
- [Use-роли](#use-роли) — для назначения прав пользователям (например, разработчикам приложений) **в конкретном пространстве имён**.
- [Manage-роли](#manage-роли) — для назначения прав администраторам.

#### Use-роли

{% alert level="warning" %}
Use-роль можно использовать только в ресурсе `RoleBinding`.
{% endalert %}

Use-роли предназначены для назначения прав пользователю **в конкретном пространстве имён**. Под пользователями понимаются, например, разработчики, которые используют настроенный администратором кластер для развёртывания своих приложений. Таким пользователям не нужно управлять модулями DKP или кластером, но им нужно иметь возможность, например, создавать свои Ingress-ресурсы, настраивать аутентификацию приложений и сбор логов с приложений.

Use-роль определяет права на доступ к namespaced-ресурсам модулей и стандартным namespaced-ресурсам Kubernetes (`Pod`, `Deployment`, `Secret`, `ConfigMap` и т. п.).

Модуль создаёт следующие use-роли для каждого [уровня доступа](#уровни-доступа-ролевой-модели):
- `d8:use:role:viewer` — позволяет в конкретном пространстве имён просматривать стандартные ресурсы Kubernetes, кроме секретов и ресурсов RBAC, а также выполнять аутентификацию в кластере;
- `d8:use:role:user` — дополнительно к роли `d8:use:role:viewer` позволяет в конкретном пространстве имён просматривать секреты и ресурсы RBAC, подключаться к подам, удалять поды (но не создавать или изменять их), выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `d8:use:role:manager` — дополнительно к роли `d8:use:role:user` позволяет в конкретном пространстве имён управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.) и стандартными namespaced-ресурсами Kubernetes (`Pod`, `ConfigMap`, `CronJob` и т. п.);
- `d8:use:role:admin` — дополнительно к роли `d8:use:role:manager` позволяет в конкретном пространстве имён управлять ресурсами `ResourceQuota`, `ServiceAccount`, `Role`, `RoleBinding`, `NetworkPolicy`.

#### Manage-роли

{% alert level="warning" %}
Manage-роль не дает доступа к пространству имён пользовательских приложений.

Manage-роль определяет доступ только к системным пространствам имён (начинающимся с `d8-` или `kube-`), причём только к тем из них, в которых работают модули соответствующей области роли.
{% endalert %}

Manage-роли предназначены для назначения прав на управление всей платформой или её частью ([областью](#области-ролевой-модели)), но не самими приложениями пользователей. С помощью manage-роли можно, например, дать возможность администратору безопасности управлять модулями, ответственными за функции безопасности кластера. Тогда администратор безопасности сможет настраивать аутентификацию, авторизацию, политики безопасности и т. п., но не сможет управлять остальными функциями кластера (например, настройками сети и мониторинга) и изменять настройки в пространстве имён приложений пользователей.

Manage-роль определяет права на доступ:
- к cluster-wide-ресурсам Kubernetes;
- к управлению модулями DKP (ресурсы `moduleConfig`) в рамках [области](#области-ролевой-модели) роли, или всеми модулями DKP для роли `d8:manage:all:*`;
- к управлению cluster-wide-ресурсами модулей DKP в рамках [области](#области-ролевой-модели) роли или всеми ресурсами модулей DKP для роли `d8:manage:all:*`;
- к системным пространствам имён (начинающимся с `d8-` или `kube-`), в которых работают модули [области](#области-ролевой-модели) роли, или ко всем системным пространствам имён для роли `d8:manage:all:*`.
  
Формат названия manage-роли — `d8:manage:<SCOPE>:<ACCESS_LEVEL>`, где:
- `SCOPE` — область роли. Может быть либо одной из областей [списка](#области-ролевой-модели), либо `all` для доступа в рамках всех областей;
- `ACCESS_LEVEL` — [уровень доступа](#уровни-доступа-ролевой-модели).

  Примеры manage-ролей:
  - `d8:manage:all:viewer` — доступ на просмотр конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:all:admin` — аналогично роли `d8:manage:all:user`, только доступ на уровне `admin`, т. е. просмотр/создание/изменение/удаление конфигурации всех модулей DKP (ресурсы `moduleConfig`), их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes во всех системных пространствах имён (начинающихся с `d8-` или `kube-`);
  - `d8:manage:observability:viewer` — доступ на просмотр конфигурации модулей DKP (ресурсы `moduleConfig`) из области `observability`, их cluster-wide-ресурсов, их namespaced-ресурсов и стандартных объектов Kubernetes (кроме секретов и ресурсов RBAC) в системных пространствах имён `d8-log-shipper`, `d8-monitoring`, `d8-okmeter`, `d8-operator-prometheus`, `d8-upmeter`, `kube-prometheus-pushgateway`.

#### Уровни доступа ролевой модели

В ролевой модели предусмотрены следующие уровни доступа (в порядке увеличения количества прав):
- `viewer` — позволяет просматривать стандартные ресурсы Kubernetes (кроме секретов и ресурсов RBAC). В manage-ролях позволяет просматривать конфигурацию модулей (ресурсы `moduleConfig`), cluster-wide-ресурсы модулей и namespaced-ресурсы модулей;
- `user` — дополнительно к роли `viewer` позволяет просматривать секреты, подключаться к подам, удалять поды, выполнять `kubectl port-forward` и `kubectl proxy`, изменять количество реплик контроллеров;
- `manager` — дополнительно к роли `user` позволяет управлять ресурсами модулей (например, `Certificate`, `PodLoggingConfig` и т. п.). В manage-ролях позволяет управлять конфигурацией модулей (ресурсы `moduleConfig`), cluster-wide-ресурсами модулей и namespaced-ресурсами модулей;
- `admin` — дополнительно к роли `manager` позволяет в зависимости от области роли управлять такими ресурсами, как `CustomResourceDefinition`, `Namespace`, `Node`, `ClusterRole`, `ClusterRoleBinding`, `PersistentVolume`, `MutatingWebhookConfiguration`, `ValidatingAdmissionPolicy` и т. п.

#### Области ролевой модели

Каждый модуль DKP принадлежит определённой области. Для каждой области существует набор ролей с разными уровнями доступа. Роли обновляются автоматически при включении или отключении модуля.

Например, для области `networking` существуют следующие manage-роли, которые можно использовать в `ClusterRoleBinding`:
- `d8:manage:networking:viewer`
- `d8:manage:networking:user`
- `d8:manage:networking:manager`
- `d8:manage:networking:admin`

Область роли ограничивает её действие всеми системными (начинающимися с `d8-` или `kube-`) пространствами имён кластера (область `all`) или теми пространствами имён, в которых работают модули области (см. таблицу состава областей).

Таблица состава областей ролевой модели.

{% include rbac-scopes-list.liquid %}

### Устаревшая ролевая модель

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

#### Список доступа для каждой роли модуля по умолчанию

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

## Управление узлами

### Основные функции

Управление узлами осуществляется с помощью модуля `node-manager`, основные функции которого:
1. Управление несколькими узлами как связанной группой (**NodeGroup**):
    * Возможность определить метаданные, которые наследуются всеми узлами группы.
    * Мониторинг группы узлов как единой сущности (группировка узлов на графиках по группам, группировка алертов о недоступности узлов, алерты о недоступности N узлов или N% узлов группы).
2. Систематическое прерывание работы узлов — **Chaos Monkey**. Предназначено для верификации отказоустойчивости элементов кластера и запущенных приложений.
3. Установка/обновление и настройка ПО узла (containerd, kubelet и др.), подключение узла в кластер:
    * Установка операционной системы (смотри [список поддерживаемых ОС](../../supported_versions.html#linux)) вне зависимости от типа используемой инфраструктуры (в любом облаке или на любом железе).
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
   * Автоматическое масштабирование.

     Доступно при использовании поддерживаемых облачных провайдеров ([подробнее](#масштабирование-узлов-в-облаке)) и недоступно для статических узлов. Облачный провайдер в автоматическом режиме может создавать или удалять виртуальные машины, подключать их к кластеру или отключать.

   * Поддержание желаемого количества узлов в группе.

     Доступно как для [облачных провайдеров](#масштабирование-узлов-в-облаке), так и для статических узлов (при использовании [Cluster API Provider Static](#работа-со-статическими-узлами)).
5. Управление Linux-пользователями на узлах.

Управление узлами осуществляется через управление группой узлов (ресурс [NodeGroup](cr.html#nodegroup)), где каждая такая группа выполняет определенные для нее задачи. Примеры групп узлов по выполняемым задачам:
- группы master-узлов;
- группа узлов маршрутизации HTTP(S)-трафика (front-узлы);
- группа узлов мониторинга;
- группа узлов приложений (worker-узлы) и т. п.

Узлы в группе имеют общие параметры и настраиваются автоматически в соответствии с параметрами группы. Deckhouse масштабирует группы, добавляя, исключая и обновляя ее узлы. Допускается иметь в одной группе как облачные, так и статические узлы (серверы bare metal, виртуальные машины). Это позволяет получать узлы на физических серверах, которые могут масштабироваться за счет облачных узлов (гибридные кластеры).

Работа в [облачной инфраструктуре](#работа-с-узлами-в-поддерживаемых-облаках) осуществляется с помощью поддерживаемых облачных провайдеров. Если поддержки необходимой облачной платформы нет, возможно использование ее ресурсов в виде статических узлов.

Работа со [статическими узлами](#работа-со-статическими-узлами) (например, серверами bare metal) выполняется с помощью в провайдера CAPS (Cluster API Provider Static).

Поддерживается работа со следующими сервисами Managed Kubernetes (может быть доступен не весь функционал сервиса):
- Google Kubernetes Engine (GKE);
- Elastic Kubernetes Service (EKS).

### Типы узлов

Типы узлов, с которыми возможна работа в рамках группы узлов (ресурс [NodeGroup](cr.html#nodegroup)):
- `CloudEphemeral` — такие узлы автоматически заказываются, создаются и удаляются в настроенном облачном провайдере.
- `CloudPermanent` — отличаются тем, что их конфигурация берется не из custom resource [nodeGroup](cr.html#nodegroup), а из специального ресурса `<PROVIDER>ClusterConfiguration` (например, [AWSClusterConfiguration](../030-cloud-provider-aws/cluster_configuration.html) для AWS). Также важное отличие узлов  в том, что для применения их конфигурации необходимо выполнить `dhctl converge` (запустив инсталлятор Deckhouse). Примером CloudPermanent-узла облачного кластера является мaster-узел кластера.  
- `CloudStatic` — узел, созданный *вручную* (статический узел), размещенный в том же облаке, с которым настроена интеграция у одного из облачных провайдеров. На таком узле работает CSI и такой узел управляется `cloud-controller-manager'ом`. Объект `Node` кластера обогащается информацией о зоне и регионе, в котором работает узел. Также при удалении узла из облака соответствующий ему Node-объект будет удален в кластере.
- `Static` — статический узел, размещенный на сервере bare metal или виртуальной машине. В случае облака, такой узел не управляется `cloud-controller-manager'ом`, даже если включен один из облачных провайдеров. [Подробнее про работу со статическими узлами...](#работа-со-статическими-узлами)

### Группировка узлов и управление группами

Группировка и управление узлами как связанной группой означает, что все узлы группы будут иметь одинаковые метаданные, взятые из custom resource'а [`NodeGroup`](cr.html#nodegroup).

Для групп узлов доступен мониторинг:
- с группировкой параметров узлов на графиках группы;
- с группировкой алертов о недоступности узлов;
- с алертами о недоступности N узлов или N% узлов группы и т. п.

### Автоматическое развертывание, настройка и обновление узлов Kubernetes

Автоматическое развертывание (в *static/hybrid* — частично), настройка и дальнейшее обновление ПО работают на любых кластерах, независимо от его размещения в облаке или на bare metal.

#### Развертывание узлов Kubernetes

Deckhouse автоматически разворачивает узлы кластера, выполняя следующие **идемпотентные** операции:
- Настройку и оптимизацию операционной системы для работы с containerd и Kubernetes:
  - устанавливаются требуемые пакеты из репозиториев дистрибутива;
  - настраиваются параметры работы ядра, параметры журналирования, ротация журналов и другие параметры системы.
- Установку требуемых версий containerd и kubelet, включение узла в кластер Kubernetes.
- Настройку Nginx и обновление списка upstream для балансировки запросов от узла к Kubernetes API.

#### Поддержка актуального состояния узлов

Для поддержания узлов кластера в актуальном состоянии могут применяться два типа обновлений:
- **Обычные**. Такие обновления всегда применяются автоматически, и не приводят к остановке или перезагрузке узла.
- **Требующие прерывания** (disruption). Пример таких обновлений — обновление версии ядра или containerd, значительная смена версии kubelet и т. д. Для этого типа обновлений можно выбрать ручной или автоматический режим (секция параметров [disruptions](cr.html#nodegroup-v1-spec-disruptions)). В автоматическом режиме перед обновлением выполняется корректная приостановка работы узла (drain) и только после этого производится обновление.

В один момент времени производится обновление только одного узла из группы и только в том случае, когда все узлы группы доступны.

Модуль `node-manager` имеет набор встроенных метрик мониторинга, которые позволяют контролировать прогресс обновления, получать уведомления о возникающих во время обновления проблемах или о необходимости получения разрешения на обновление (ручное подтверждение обновления).

### Работа с узлами в поддерживаемых облаках

У каждого поддерживаемого облачного провайдера существует возможность автоматического заказа узлов. Для этого необходимо указать требуемые параметры для каждого узла или группы узлов.

В зависимости от провайдера этими параметрами могут быть:
- тип узлов или количество ядер процессора и объем оперативной памяти;
- размер диска;
- настройки безопасности;
- подключаемые сети и др.

Создание, запуск и подключение виртуальных машин к кластеру выполняются автоматически.

#### Масштабирование узлов в облаке

Возможны два режима масштабирования узлов в группе:
- **Автоматическое масштабирование**.

  При дефиците ресурсов, наличии подов в состоянии `Pending`, в группу будут добавлены узлы. При отсутствии нагрузки на один или несколько узлов, они будут удалены из кластера. При работе автомасштабирования учитывается [приоритет](cr.html#nodegroup-v1-spec-cloudinstances-priority) группы (в первую очередь будет масштабироваться группа, у которой приоритет больше).
  
  Чтобы включить автоматическое масштабирование узлов, необходимо указать разные ненулевые значения [минимального](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) и [максимального](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone) количества узлов в группе.

- **Фиксированное количество узлов.**

  В этом случае Deckhouse будет поддерживать указанное количество узлов (например, заказывая новые в случае выхода из строя старых узлов).

  Чтобы указать фиксированное количество узлов в группе и отключить автоматическое масштабирование, необходимо указать одинаковые значения параметров [minPerZone](cr.html#nodegroup-v1-spec-cloudinstances-minperzone) и [maxPerZone](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone).

### Работа со статическими узлами

При работе со статическими узлами функции модуля `node-manager` выполняются со следующими ограничениями:
- **Отсутствует заказ узлов.** Непосредственное выделение ресурсов (серверов bare metal, виртуальных машин, связанных ресурсов) выполняется вручную. Дальнейшая настройка ресурсов  (подключение узла к кластеру, настройка мониторинга и т.п.) выполняются полностью автоматически (аналогично узлам в облаке) или частично.
- **Отсутствует автоматическое масштабирование узлов.** Доступно поддержание в группе указанного количества узлов при использовании [Cluster API Provider Static](#cluster-api-provider-static) (параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count)). Т.е. Deckhouse будет пытаться поддерживать указанное количество узлов в группе, очищая лишние узлы и настраивая новые при необходимости (выбирая их из ресурсов [StaticInstance](cr.html#staticinstance), находящихся в состоянии *Pending*).

Настройка/очистка узла, его подключение к кластеру и отключение могут выполняться следующими способами:
- **Вручную,** с помощью подготовленных скриптов.

  Для настройки сервера (ВМ) и ввода узла в кластер нужно загрузить и выполнить специальный bootstrap-скрипт. Такой скрипт генерируется для каждой группы статических узлов (каждого ресурса `NodeGroup`). Он находится в секрете `d8-cloud-instance-manager/manual-bootstrap-for-<ИМЯ-NODEGROUP>`. Пример добавления статического узла в кластер можно найти в [FAQ](examples.html#вручную).

  Для отключения узла кластера и очистки сервера (виртуальной машины) нужно выполнить скрипт `/var/lib/bashible/cleanup_static_node.sh`, который уже находится на каждом статическом узле. Пример отключения узла кластера и очистки сервера можно найти в [FAQ](faq.html#как-вручную-очистить-статический-узел).

- **Автоматически,** с помощью [Cluster API Provider Static](#cluster-api-provider-static).

  > Функционал доступен начиная с версии 1.54 Deckhouse и находится в стадии тестирования и активной разработки.

  Cluster API Provider Static (CAPS) подключается к серверу (ВМ) используя ресурсы [StaticInstance](cr.html#staticinstance) и [SSHCredentials](cr.html#sshcredentials), выполняет настройку, и вводит узел в кластер.

  При необходимости (например, если удален соответствующий серверу ресурс [StaticInstance](cr.html#staticinstance) или уменьшено [количество узлов группы](cr.html#nodegroup-v1-spec-staticinstances-count)), Cluster API Provider Static подключается к узлу кластера, очищает его и отключает от кластера.

- **Вручную с последующей передачей узла под автоматическое управление** [Cluster API Provider Static](#cluster-api-provider-static).

  > Функциональность доступна начиная с версии Deckhouse 1.63.

  Для передачи существующего узла кластера под управление CAPS необходимо подготовить для этого узла ресурсы [StaticInstance](cr.html#staticinstance) и [SSHCredentials](cr.html#sshcredentials), как при автоматическом управлении в пункте выше, однако ресурс [StaticInstance](cr.html#staticinstance) должен дополнительно быть помечен аннотацией `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

#### Cluster API Provider Static

> Cluster API Provider Static доступен начиная с версии 1.54 Deckhouse. Описанный функционал находится в стадии тестирования и активной разработки. Функционал и описание ресурсов могут измениться. Учитывайте это при использовании в продуктивных кластерах.

Cluster API Provider Static (CAPS), это реализация провайдера декларативного управления статическими узлами (серверами bare metal или виртуальными машинами) для проекта [Cluster API](https://cluster-api.sigs.k8s.io/) Kubernetes. По сути, CAPS это дополнительный слой абстракции к уже существующему функционалу Deckhouse по автоматической настройке и очистке статических узлов с помощью скриптов, генерируемых для каждой группы узлов (см. раздел [Работа со статическими узлами](#работа-со-статическими-узлами)).

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
   - `Bootstraping`. Выполняется процедура настройки сервера (ВМ) и подключения узла в кластер.
   - `Running`. Сервер настроен, и в кластер добавлен соответствующий узел.
   - `Cleaning`. Выполняется процедура очистки сервера и отключение узла из кластера.

   > Можно отдать существующий узел кластера, заранее введенный в кластер вручную, под управление CAPS, пометив его StaticInstance аннотацией `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

1. **Создание ресурса [NodeGroup](cr.html#nodegroup).**

   В контексте CAPS в ресурсе `NodeGroup` нужно обратить внимание на параметр [nodeType](cr.html#nodegroup-v1-spec-nodetype) (должен быть `Static`) и секцию параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances).

   Секция параметров [staticInstances.labelSelector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) определяет фильтр, по которому CAPS выбирает ресурсы `StaticInstance`, которые нужно использовать в группе. Фильтр позволяет использовать для разных групп узлов только определенные `StaticInstance`, а также позволяет использовать один `StaticInstance` в разных группах узлов. Фильтр можно не определять, чтобы использовать в группе узлов любой доступный `StaticInstance`.

   Параметр [staticInstances.count](cr.html#nodegroup-v1-spec-staticinstances-count) определяет желаемое количество узлов в группе.  При изменении параметра, CAPS начинает добавлять или удалять необходимое количество узлов, запуская этот процесс параллельно.

В соответствии с данными секции параметров [staticInstances](cr.html#nodegroup-v1-spec-staticinstances), CAPS будет пытаться поддерживать указанное (параметр [count](cr.html#nodegroup-v1-spec-staticinstances-count)) количество узлов в группе. При необходимости добавить узел в группу, CAPS выбирает соответствующий [фильтру](cr.html#nodegroup-v1-spec-staticinstances-labelselector) ресурс [StaticInstance](cr.html#staticinstance) находящийся в статусе `Pending`, настраивает сервер (ВМ) и добавляет узел в кластер. При необходимости удалить узел из группы, CAPS выбирает [StaticInstance](cr.html#staticinstance) находящийся в статусе `Running`, очищает сервер (ВМ) и удаляет узел из кластера (после чего, соответствующий `StaticInstance` переходит в состояние `Pending` и снова может быть использован).

### Пользовательские настройки на узлах

Для автоматизации действий на узлах группы предусмотрен ресурс [NodeGroupConfiguration](cr.html#nodegroupconfiguration). Ресурс позволяет выполнять на узлах bash-скрипты, в которых можно пользоваться набором команд [bashbooster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster), а также позволяет использовать шаблонизатор [Go Template](https://pkg.go.dev/text/template). Это удобно для автоматизации таких операций, как:
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

Код скрипта указывается в параметре [content](cr.html#nodegroupconfiguration-v1alpha1-spec-content) ресурса. При создании скрипта на узле содержимое параметра `content` проходит через шаблонизатор [Go Template](https://pkg.go.dev/text/template), который позволят встроить дополнительный уровень логики при генерации скрипта. При прохождении через шаблонизатор становится доступным контекст с набором динамических переменных.

Переменные, которые доступны для использования в шаблонизаторе:
<ul>
<li><code>.cloudProvider</code> (для групп узлов с nodeType <code>CloudEphemeral</code> или <code>CloudPermanent</code>) — массив данных облачного провайдера.
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
# Some code for tuning user environment
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

#### Особенности написания скриптов

При написании скриптов важно учитывать следующие особенности их использования в Deckhouse:

1. Скрипты в deckhouse выполняются раз в 4 часа или на основании внешних триггеров. Поэтому важно писать скрипты таким образом, чтобы они производили проверку необходимости своих изменений в системе перед выполнением действий, а не производили изменения каждый раз при запуске.
2. Существуют [предзаготовленные скрипты](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/node-group) которые производят различные действия в т.ч. установку и настройку сервисов. Важно учитывать это при выборе [приоритета](cr.html#nodegroupconfiguration-v1alpha1-spec-weight) пользовательских скриптов. Например, если в скрипте планируется произвести перезапуск сервиса, то данный скрипт должен вызываться после скрипта установки сервиса. В противном случае он не сможет выполниться при развертывании нового узла.

Полезные особенности некоторых скриптов:

* [`032_configure_containerd.sh`](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/common-steps/node-group/032_configure_containerd.sh.tpl) - производит объединение всех конфигурационных файлов сервиса `containerd` расположенных по пути `/etc/containerd/conf.d/*.toml`, а также **перезапуск** сервиса. Следует учитывать что директория `/etc/containerd/conf.d/` не создается автоматически, а также что создание файлов в этой директории следует производить в скриптах с приоритетом менее `32`

### Chaos Monkey

Инструмент (включается у каждой из `NodeGroup` отдельно), позволяющий систематически вызывать случайные прерывания работы узлов. Предназначен для проверки элементов кластера, приложений и инфраструктурных компонентов на реальную работу отказоустойчивости.

### Мониторинг

Для групп узлов (ресурс `NodeGroup`) DKP экспортирует метрики доступности группы.

#### Какую информацию собирает Prometheus?

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

## Модуль network-policy-engine

<div class="docs__information warning active">
Не используйте модуль, если включен модуль <a href="../021-cni-cilium/">cilium</a>, так как в нем уже есть функционал управления сетевыми политиками.
</div>

Модуль управления сетевыми политиками.

В Deckhouse был выбран консервативный подход к организации сети, при котором используются простые сетевые бэкенды (*«чистый»* CNI или flannel в режиме `host-gw`). Этот подход оказался прост и надежен, поэтому показал себя с лучшей стороны.

Имплементация сетевых политик (`NetworkPolicy`) в Deckhouse тоже представляет собой простую и надежную систему, основанную на базе `kube-router` в режиме *Network Policy Controller* (`--run-firewall`). В этом случае `kube-router` транслирует сетевые политики `NetworkPolicy` в правила `iptables`, а они, в свою очередь, работают в любых инсталляциях вне зависимости от облака или используемого CNI.

Модуль `network-policy-engine` разворачивает в namespace `d8-system` DaemonSet с [kube-router](https://github.com/cloudnativelabs/kube-router) в режиме поддержки [network policies](https://kubernetes.io/docs/concepts/services-networking/network-policies/). В результате в Kubernetes-кластере включается полная поддержка Network Policies.

Поддерживаются следующие форматы описания политик:
- *networking.k8s.io/NetworkPolicy API;*
- *network policy V1/GA semantics;*
- *network policy beta semantics.*

[Примеры](https://github.com/ahmetb/kubernetes-network-policy-recipes) использования.

## Модуль cilium-hubble

Модуль может обеспечивать визуализацию сетевого стека кластера, если включен Cilium CNI.

## Мониторинг control plane

Мониторинг control plane осуществляется с помощью модуля `monitoring-kubernetes-control-plane`, который организует безопасный сбор метрик и предоставляет базовый набор правил мониторинга следующих компонентов кластера:
* kube-apiserver;
* kube-controller-manager;
* kube-scheduler;
* kube-etcd.

## Cloud provider — GCP

Взаимодействие с облачными ресурсами провайдера [Google](https://cloud.google.com/) осуществляется с помощью модуля `cloud-provider-gcp`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы GCP при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-gcp`:
- Управляет ресурсами GCP с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне GCP.
  * Создает LoadBalancer'ы для Service-объектов Kubernetes с типом `LoadBalancer`.
  * Актуализирует метаданные узлов кластера согласно описанным параметрам конфигурации и удаляет из кластера узлы, которых уже нет в GCP.
- Заказывает диски в GCP с помощью компонента `CSI storage`.
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [GCPInstanceClass'ы](cr.html#gcpinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).

## Модуль cert-manager

Устанавливает надежную и высокодоступную инсталляцию cert-manager [release v1.12.3](https://github.com/jetstack/cert-manager).

При установке модуля автоматически учитываются особенности кластера:
- компонент (webhook), к которому обращается `kube-apiserver`, устанавливается на master-узлы;
- в случае недоступности webhook'а производится временное удаление `apiservice`, чтобы недоступность *cert-manager* не блокировала работу кластера.

Обновление самого модуля происходит в автоматическом режиме, в том числе с миграцией ресурсов cert-manager.

### Возможности модуля cert-manager (с учетом внесенных изменений)

Модуль обеспечивает использование всех возможностей оригинального cert-manager, в том числе:
- заказ сертификатов во всех поддерживаемых источниках, таких как *Let’s Encrypt*, *HashiCorp Vault*, *Venafi*;
- выпуск самоподписанных сертификатов;
- поддержку актуальности сертификатов, автоматический перевыпуск и т. д.

Изменения в оригинальный [cert-manager](https://github.com/jetstack/cert-manager) были внесены, чтобы поды `cm-acme-http-solver` могли выполняться на master-узлах и выделенных узлах.

### Мониторинг

Модуль обеспечивает экспорт метрик в Prometheus, что позволяет мониторить:
- срок действия сертификатов;
- корректность перевыпуска сертификатов.

### Роли доступа к ресурсам

В модуле предопределены несколько продуманных ролей для удобного доступа к ресурсам:
- `User` – доступ на чтение к ресурсам Certificate и Issuer в доступных ему namespace, а также к глобальным ClusterIssue;
- `Editor` – управление ресурсами Certificate и Issuer в доступных ему namespace;
- `ClusterEditor` – управление ресурсами Certificate и Issuer в любых namespace;
- `SuperAdmin` – управление внутренними служебными объектами.

## Модуль upmeter

Модуль собирает статистику по типам доступности для компонентов кластера и Deckhouse. Позволяет оценивать степень выполнения SLA на эти компоненты, показывает данные о доступности в веб-интерфейсе и предоставляет веб-страницу статуса работы компонентов кластера.

С помощью custom resource [UpmeterRemoteWrite](cr.html#upmeterremotewrite) можно экспортировать метрики доступности по протоколу [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/).

Состав модуля:
- **agent** — делает пробы доступности и отправляет результаты на сервер, работает на мастер-узлах.
- **upmeter** — агрегатор результатов и API-сервер для их извлечения.
- **front**:
  - **status** — показывает текущий уровень доступности за последние 10 минут (по умолчанию требует авторизации, но ее можно отключить);
  - **webui** — дашборд со статистикой по пробам и группам доступности (требует авторизации).
- **smoke-mini** — постоянное *smoke-тестирование* с помощью StatefulSet, похожего на настоящее приложение.

Модуль отправляет около 100 показаний метрик каждые 5 минут. Это значение зависит от количества включенных модулей Deckhouse.

### Интерфейс

Пример веб-интерфейса:
![Пример веб-интерфейса](../../images/500-upmeter/image1.png)

Пример графиков по метрикам из upmeter в Grafana:
![Пример графиков по метрикам из upmeter в Grafana](../../images/500-upmeter/image2.png)

## Модуль kube-proxy

Модуль удаляет весь комплект kube-proxy (`DaemonSet`, `ConfigMap`, `RBAC`) от `kubeadm` и устанавливает свой.

По умолчанию в целях безопасности при использовании сервисов с типом NodePort подключения принимаются только на InternalIP узлов. Для снятия данного ограничения предусмотрена аннотация на узел — `node.deckhouse.io/nodeport-bind-internal-ip: "false"`.

Пример аннотации для NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: myng
spec:
  nodeTemplate:
    annotations:
      node.deckhouse.io/nodeport-bind-internal-ip: "false"
...
```

> **Внимание!** После добавления, удаления или изменения значения аннотации необходимо самостоятельно выполнить рестарт подов kube-proxy.
>
> **Внимание!** Модуль kube-proxy автоматически отключается при включении модуля [cni-cilium](../021-cni-cilium/).

## Модуль terraform-manager

Модуль предоставляет инструменты для работы с состоянием Terraform'а в кластере Kubernetes.

* Модуль состоит из двух частей:
  * `terraform-auto-converger` — проверяет состояние Terraform'а и применяет недеструктивные изменения;
  * `terraform-state-exporter` — проверяет состояние Terraform'а и экспортирует метрики кластера.

* Модуль включен по умолчанию, если в кластере есть Secret'ы:
  * `kube-system/d8-provider-cluster-configuration`;
  * `d8-system/d8-cluster-terraform-state`.

## Модуль extended-monitoring

Содержит следующие Prometheus exporter'ы:

- `extended-monitoring-exporter` — включает расширенный сбор метрик и отправку алертов по свободному месту и inode на узлах, плюс включает «расширенный мониторинг» объектов в namespace, у которых есть лейбл `extended-monitoring.deckhouse.io/enabled=””`;
- `image-availability-exporter` — добавляет метрики и включает отправку алертов, позволяющих узнать о проблемах с доступностью образа контейнера в registry, прописанному в поле `image` из spec пода в `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`;
- `events-exporter` — собирает события в кластере Kubernetes и отдает их в виде метрик;
- `cert-exporter`— сканирует Secret'ы кластера Kubernetes и генерирует метрики об истечении срока действия сертификатов в них.

## Модуль deckhouse-tools

Этот модуль создает веб-интерфейс со ссылками на скачивание утилит Deckhouse (в настоящее время – [Deckhouse CLI](../../deckhouse-cli/) под различные операционные системы).

Адрес веб-интерфейса формируется в соответствии с шаблоном [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) глобального параметра конфигурации Deckhouse (ключ `%s` заменяется на `tools`).

Например, если `publicDomainTemplate` установлен как `%s-kube.company.my`, веб-интерфейс будет доступен по адресу `tools-kube.company.my`.

## Модуль deckhouse

Этот модуль настраивает в Deckhouse:
- **[Уровень логирования](configuration.html#parameters-loglevel)**
- **[Набор модулей](configuration.html#parameters-bundle), включенных по умолчанию**

  Обычно используется набор модулей `Default`, который подходит в большинстве случаев.

  Независимо от используемого набора включенных по умолчанию модулей любой модуль может быть явно включен или выключен в конфигурации Deckhouse (подробнее [про включение и отключение модуля](../../#включение-и-отключение-модуля)).
- **[Канал обновлений](configuration.html#parameters-releasechannel)**

  В Deckhouse реализован механизм автоматического обновления. Этот механизм использует [5 каналов обновлений](../../deckhouse-release-channels.html), различающиеся стабильностью и частотой выхода версий. Ознакомьтесь подробнее с тем, [как работает механизм автоматического обновления](../../deckhouse-faq.html#как-работает-автоматическое-обновление-deckhouse) и [как установить желаемый канал обновлений](../../deckhouse-faq.html#как-установить-желаемый-канал-обновлений).
- **[Режим обновлений](configuration.html#parameters-update-mode)** и **[окна обновлений](configuration.html#parameters-update-windows)**

  Deckhouse может использовать **ручной** или **автоматический** режим обновлений.

  В ручном режиме обновлений автоматически применяются только важные исправления (patch-релизы), и для перехода на более свежий релиз Deckhouse требуется [ручное подтверждение](../../cr.html#deckhouserelease-v1alpha1-approved).

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

*Закрепление* релиза нужно, если вы по какой-то причине не хотите обновлять Deckhouse.

Есть три варианта *закрепления* релиза:
- Установить [ручной режим обновления](usage.html#ручное-подтверждение-обновлений).
В этом случае вы остановитесь на текущем релизе, но будете получать patch-релизы, а minor-релиз не будет обновляться без явного одобрения.
  
  Например:
    текущий релиз `v1.29.3`, после установки ручного режима обновлений Deckhouse сможет обновиться до версии `v1.29.9`, но не будет обновляться на релиз `v1.30.0`.
- Установить конкретный тег для deployment/deckhouse. В таком случае Deckhouse останется на этом релизе до выхода следующего patch/minor-релиза.
Это может понадобиться, если в какой-то версии Deckhouse обнаружилась ошибка, которой не было раньше, и вы хотите откатиться на предыдущий релиз, но обновиться, как только выйдет релиз с исправлением данной ошибки.

  Например:
    `kubectl -n d8-system set image deployment/deckhouse deckhouse=registry.deckhouse.io/deckhouse/ee:v1.30.5`.
- Установить конкретный тег для deployment/deckhouse и удалить `releaseChannel` из конфигурации модуля `deckhouse`.
    В таком случае вы останетесь на конкретной версии Deckhouse и не будете получать обновлений.

  ```sh
  $ kubectl -n d8-system set image deployment/deckhouse deckhouse=registry.deckhouse.io/deckhouse/ee:v1.30.5
  $ kubectl edit mc deckhouse
    // удалить releaseChannel
  ```

## Модуль monitoring-ping

### Описание

Данный модуль предназначен для мониторинга сетевого взаимодействия между всеми узлами кластера, а также — опционально — до дополнительных внешних узлов.

Каждый узел два раза в секунду отправляет ICMP-пакеты на все другие узлы кластера (и на опциональные внешние узлы) и экспортирует данные в `Prometheus`.
В комплекте идет dashboard для `Grafana`, на котором отражаются соответствующие графики.

### Как работает

Модуль следит за любыми изменениями поля `.status.addresses` узла. В случае выявления таковых
запускается хук, который собирает полный список имен узлов и их адресов и передает в daemonSet, что в свою очередь пересоздает поды.
Таким образом, `ping` проверяет всегда актуальный список узлов.

## Модуль priority-class

Модуль создает в кластере набор классов приоритета ([PriorityClass](https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass)) и проставляет их компонентам, установленным Deckhouse, и приложениям в кластере.

Функционал работы классов приоритета реализуется планировщиком (scheduler), который позволяет учитывать приоритет пода (из его принадлежности к классу) при планировании.

К примеру, при выкате в кластер подов с `priorityClassName: production-low`, если в кластере не будет доступных ресурсов для данного пода, Kubernetes начнет вытеснять поды с наименьшим приоритетом в кластере.
То есть сначала будут выгнаны все поды с `priorityClassName: develop`, потом — с `cluster-low` и так далее.

При указании класса приоритета очень важно понимать, к какому типу относится приложение и в каком окружении оно будет работать. Любой установленный `priorityClassName` не уменьшит приоритета пода, так как если `priorityClassName` у пода не установлен, тогда планировщик считает его самым низким — `develop`. Очень важно правильно выставлять `priorityClassName`.

> **Внимание!** Нельзя использовать классы приоритета `system-node-critical`, `system-cluster-critical`, `cluster-medium`, `cluster-low`.

Устанавливаемые модулем классы приоритета (в порядке приоритета от большего к меньшему):

| Класс приоритета | Описание                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Значение   |
|-----------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|------------|
| `system-node-critical`            | Компоненты кластера, которые обязаны присутствовать на узле. Также полностью защищает от [вытеснения kubelet'ом](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>`node-exporter`, `csi`                                                                                                                                                                                                                                                   | 2000001000 |
| `system-cluster-critical`         | Компоненты кластера, без которых его корректная работа невозможна. Этим PriorityClass'ом в обязательном порядке помечаются MutatingWebhooks и Extension API servers. Также полностью защищает от [вытеснения kubelet'ом](https://kubernetes.io/docs/tasks/administer-cluster/out-of-resource/).<br>`kube-dns`, `coredns`, `kube-proxy`, `flannel`, `kube-api-server`, `kube-controller-manager`, `kube-scheduler`, `cluster-autoscaler`, `machine-controller-manager` | 2000000000 |
| `production-high`                 | Stateful-приложения, отсутствие которых в production-окружении приводит к полной недоступности сервиса или потере данных (PostgreSQL, memcached, Redis, Mongo и др.).                                                                                                                                                                                                                                                                                                | 9000       |
| `cluster-medium`                  | Компоненты кластера, влияющие на мониторинг (алерты, диагностика) кластера и автомасштабирование. Без мониторинга мы не можем оценить масштабы происшествия, без автомасштабирования не сможем дать приложениям необходимые ресурсы.<br>`deckhouse`, `node-local-dns`, `kube-state-metrics`, `madison-proxy`, `node-exporter`, `trickster`, `grafana`, `kube-router`, `monitoring-ping`, `okmeter`, `smoke-mini`                                                  | 7000       |
| `production-medium`               | Основные stateless-приложения в production-окружении, которые отвечают за работу сервиса для посетителей.                                                                                                                                                                                                                                                                                                                                                           | 6000       |
| `deployment-machinery`            | Компоненты кластера, с помощью которых происходят сборка и деплой в кластер (Helm, werf).                                                                                                                                                                                                                                                                                                                                            | 5000       |
| `production-low`                  | Приложения в production-окружении (cron'ы, админки, batch-процессинг), без которых можно прожить некоторое время. Если batch или cron никак нельзя прерывать, он должен быть в production-medium, а не здесь.                                                                                                                                                                                                                                               | 4000       |
| `staging`                         | Staging-окружения для приложений.                                                                                                                                                                                                                                                                                                                                                                                                                                   | 3000       |
| `cluster-low`                     | Компоненты кластера, без которых возможна эксплуатация кластера, но которые желательны. <br>`prometheus-operator`, `dashboard`, `dashboard-oauth2-proxy`, `cert-manager`, `prometheus`, `prometheus-longterm`                                                                                                                                                                                                                                                       | 2000       |
| `develop` (default)               | Develop-окружения для приложений. Класс по умолчанию, если не проставлены иные классы.                                                                                                                                                                                                                                                                                                                                                                              | 1000       |
| `standby`                         | Этот класс не предназначен для приложений. Используется в системных целях для резервирования узлов.                                                                                                                                                                                                                                                                                                                                                                 | -1         |

## Cloud provider — Yandex Cloud

Взаимодействие с облачными ресурсами провайдера [Yandex Cloud](https://cloud.yandex.ru/) осуществляется с помощью модуля `cloud-provider-yandex`. Он предоставляет возможность модулю [управления узлами](../../modules/040-node-manager/) использовать ресурсы Yandex Cloud при заказе узлов для описанной [группы узлов](../../modules/040-node-manager/cr.html#nodegroup).

Функционал модуля `cloud-provider-yandex`:
- Управляет ресурсами Yandex Cloud с помощью модуля `cloud-controller-manager`:
  * Создает сетевые маршруты для сети `PodNetwork` на стороне Yandex Cloud.
  * Актуализирует метаданные Yandex Cloud Instances и Kubernetes Nodes. Удаляет из Kubernetes узлы, которых уже нет в Yandex Cloud.
- Заказывает диски в Yandex Cloud с помощью компонента `CSI storage`.
- Регистрируется в модуле [node-manager](../../modules/040-node-manager/), чтобы [YandexInstanceClass'ы](cr.html#yandexinstanceclass) можно было использовать при описании [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
- Включает необходимый CNI (использует [simple bridge](../../modules/035-cni-simple-bridge/)).

### Интеграция с Yandex Cloud

#### Настройка групп безопасности

При создании [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network), Yandex Cloud создаёт [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups) по умолчанию для всех подключенных сетей, включая сеть кластера Deckhouse Kubernetes Platform. Эта группа безопасности по умолчанию содержит правила разрешающие любой входящий и исходящий трафик и применяется для всех подсетей облачной сети, если на объект (интерфейс ВМ) явно не назначена другая группа безопасности.

{% alert level="danger" %}
Не удаляйте правила по умолчанию, разрешающие любой трафик, до того как закончите настройку правил группы безопасности. Это может нарушить работоспособность кластера.
{% endalert %}

Здесь приведены общие рекомендации по настройке группы безопасности. Некорректная настройка групп безопасности может сказаться на работоспособности кластера. Пожалуйста ознакомьтесь с [особенностями работы групп безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups#security-groups-notes) в Yandex Cloud перед использованием в продуктивных средах.

1. Определите облачную сеть, в которой работает кластер Deckhouse Kubernetes Platform.

   Название сети совпадает с полем `prefix` ресурса [ClusterConfiguration](../../installing/configuration.html#clusterconfiguration). Его можно узнать с помощью команды:

   ```bash
   kubectl get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. В консоли Yandex Cloud выберите сервис Virtual Private Cloud и перейдите в раздел *Группы безопасности*. У вас должна отображаться одна группа безопасности с пометкой `Default`.

    ![Группа безопасности по умолчанию](../../images/030-cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

    ![Правила для группы безопасности](../../images/030-cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

#### Интеграция с Yandex Lockbox

С помощью инструмента [External Secrets Operator](https://github.com/external-secrets/external-secrets) вы можете настроить синхронизацию секретов [Yandex Lockbox](https://cloud.yandex.com/ru/docs/lockbox/concepts/) с секретами кластера Deckhouse Kubernetes Platform.

Приведенную инструкцию следует рассматривать как *Быстрый старт*. Для использования интеграции в продуктивных средах ознакомьтесь со следующими ресурсами:

- [Yandex Lockbox](https://cloud.yandex.ru/ru/docs/lockbox/)
- [Синхронизация с секретами Yandex Lockbox](https://cloud.yandex.ru/ru/docs/managed-kubernetes/tutorials/kubernetes-lockbox-secrets)
- [External Secret Operator](https://external-secrets.io/latest/)

##### Инструкция по развертыванию

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create), необходимый для работы External Secrets Operator:

   ```shell
   yc iam service-account create --name eso-service-account
   ```

1. [Создайте авторизованный ключ](https://cloud.yandex.ru/ru/docs/iam/operations/authorized-key/create) для сервисного аккаунта и сохраните его в файл:

   ```shell
   yc iam key create --service-account-name eso-service-account --output authorized-key.json
   ```

1. [Назначьте](https://cloud.yandex.ru/ru/docs/iam/operations/sa/assign-role-for-sa) сервисному аккаунту [роли](https://cloud.yandex.com/ru/docs/lockbox/security/#service-roles) `lockbox.editor`, `lockbox.payloadViewer` и `kms.keys.encrypterDecrypter` для доступа ко всем секретам каталога:

   ```shell
   folder_id=<идентификатор каталога>
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.editor
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role lockbox.payloadViewer
   yc resource-manager folder add-access-binding --id=${folder_id} --service-account-name eso-service-account --role kms.keys.encrypterDecrypter
   ```

   Для более тонкой настройки ознакомьтесь с [управлением доступом в Yandex Lockbox](https://cloud.yandex.com/ru/docs/lockbox/security).

1. Установите External Secrets Operator с помощью Helm-чарта согласно [инструкции](https://cloud.yandex.com/ru/docs/managed-kubernetes/operations/applications/external-secrets-operator#helm-install).

   Обратите внимание, что вам может понадобиться задать `nodeSelector`, `tolerations` и другие параметры. Для этого используйте файл `./external-secrets/values.yaml` после распаковки Helm-чарта.

   Скачайте и распакуйте чарт:

   ```shell
   helm pull oci://cr.yandex/yc-marketplace/yandex-cloud/external-secrets/chart/external-secrets \
     --version 0.5.5 \
     --untar
   ```

   Установите Helm-чарт:

   ```shell
   helm install -n external-secrets --create-namespace \
     --set-file auth.json=authorized-key.json \
     external-secrets ./external-secrets/
   ```

   Где:
   - `authorized-key.json` — название файла с авторизованным ключом из шага 2.

1. Создайте хранилище секретов [SecretStore](https://external-secrets.io/latest/api/secretstore/), содержащее секрет `sa-creds`:

   ```shell
   kubectl -n external-secrets apply -f - <<< '
   apiVersion: external-secrets.io/v1alpha1
   kind: SecretStore
   metadata:
     name: secret-store
   spec:
     provider:
       yandexlockbox:
         auth:
           authorizedKeySecretRef:
             name: sa-creds
             key: key'
   ```

   Где:
   - `sa-creds` — название `Secret`, содержащий авторизованный ключ. Этот секрет должен появиться после установки Helm-чарта.
   - `key` — название ключа в поле `.data` секрета выше.

##### Проверка работоспособности

1. Проверьте статус External Secrets Operator и созданного хранилища секретов:

   ```shell
   $ kubectl -n external-secrets get po
   NAME                                                READY   STATUS    RESTARTS   AGE
   external-secrets-55f78c44cf-dbf6q                   1/1     Running   0          77m
   external-secrets-cert-controller-78cbc7d9c8-rszhx   1/1     Running   0          77m
   external-secrets-webhook-6d7b66758-s7v9c            1/1     Running   0          77m

   $ kubectl -n external-secrets get secretstores.external-secrets.io 
   NAME           AGE   STATUS
   secret-store   69m   Valid
   ```

1. [Создайте секрет](https://cloud.yandex.ru/ru/docs/lockbox/operations/secret-create) Yandex Lockbox со следующими параметрами:

    - **Имя** — `lockbox-secret`.
    - **Ключ** — введите неконфиденциальный идентификатор `password`.
    - **Значение** — введите конфиденциальные данные для хранения `p@$$w0rd`.

1. Создайте объект [ExternalSecret](https://external-secrets.io/latest/api/externalsecret/), указывающий на секрет `lockbox-secret` в хранилище `secret-store`:

   ```shell
   kubectl -n external-secrets apply -f - <<< '
   apiVersion: external-secrets.io/v1alpha1
   kind: ExternalSecret
   metadata:
     name: external-secret
   spec:
     refreshInterval: 1h
     secretStoreRef:
       name: secret-store
       kind: SecretStore
     target:
       name: k8s-secret
     data:
     - secretKey: password
       remoteRef:
         key: <ИДЕНТИФИКАТОР_СЕКРЕТА>
         property: password'
   ```

   Где:

   - `spec.target.name` — имя нового секрета. External Secret Operator создаст этот секрет в кластере Deckhouse Kubernetes Platform и поместит в него параметры секрета Yandex Lockbox `lockbox-secret`.
   - `spec.data[].secretKey` — название ключа в поле `.data` секрета, который создаст External Secret Operator.
   - `spec.data[].remoteRef.key` — идентификатор созданного ранее секрета Yandex Lockbox `lockbox-secret`. Например, `e6q28nvfmhu539******`.
   - `spec.data[].remoteRef.property` — **ключ**, указанный ранее, для секрета Yandex Lockbox `lockbox-secret`.

1. Убедитесь, что новый ключ `k8s-secret` содержит значение секрета `lockbox-secret`:

   ```shell
   kubectl -n external-secrets get secret k8s-secret -ojson | jq -r '.data.password' | base64 -d
   ```

   В выводе команды будет содержаться **значение** ключа `password` секрета `lockbox-secret`, созданного ранее:

   ```shell
   p@$$w0rd
   ```

#### Интеграция с Yandex Managed Service for Prometheus

С помощью данной интеграции вы можете использовать [Yandex Managed Service for Prometheus](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/) в качестве внешнего хранилища метрик, например, для долгосрочного хранения.

##### Запись метрик

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create) с ролью `monitoring.editor`.
1. [Создайте API-ключ](https://cloud.yandex.ru/ru/docs/iam/operations/api-key/create) для сервисного аккаунта.
1. Создайте ресурс `PrometheusRemoteWrite`:

   ```shell
   kubectl apply -f - <<< '
   apiVersion: deckhouse.io/v1
   kind: PrometheusRemoteWrite
   metadata:
     name: yc-remote-write
   spec:
     url: <URL_ЗАПИСИ_МЕТРИК>
     bearerToken: <API_КЛЮЧ>
   '
   ```

   Где:

   - `<URL_ЗАПИСИ_МЕТРИК>` — URL со страницы Yandex Monitoring/Prometheus/Запись метрик.
   - `<API_КЛЮЧ>` — API-ключ, созданный на предыдущем шаге. Например, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`.

   Также вы можете указать дополнительные параметры в соответствии с [документацией](../../modules/300-prometheus/cr.html#prometheusremotewrite).

Подробнее с данной функциональностью можно ознакомиться в [документации Yandex Cloud](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/ingestion/remote-write).

##### Чтение метрик через Grafana

1. [Создайте сервисный аккаунт](https://cloud.yandex.com/ru/docs/iam/operations/sa/create) с ролью `monitoring.viewer`.
1. [Создайте API-ключ](https://cloud.yandex.ru/ru/docs/iam/operations/api-key/create) для сервисного аккаунта.
1. Создайте ресурс `GrafanaAdditionalDatasource`:

   ```shell
   kubectl apply -f - <<< '
   apiVersion: deckhouse.io/v1
   kind: GrafanaAdditionalDatasource
   metadata:
     name: managed-prometheus
   spec:
     type: prometheus
     access: Proxy
     url: <URL_ЧТЕНИЕ_МЕТРИК_ЧЕРЕЗ_GRAFANA>
     basicAuth: false
     jsonData:
       timeInterval: 30s
       httpMethod: POST
       httpHeaderName1: Authorization
     secureJsonData:
       httpHeaderValue1: Bearer <API_КЛЮЧ>
   '
   ```

   Где:

   - `<URL_ЧТЕНИЕ_МЕТРИК_ЧЕРЕЗ_GRAFANA>` — URL со страницы Yandex Monitoring/Prometheus/Чтение метрик через Grafana.
   - `<API_КЛЮЧ>` — API-ключ, созданный на предыдущем шаге. Например, `AQVN1HHJReSrfo9jU3aopsXrJyfq_UHs********`.

   Также вы можете указать дополнительные параметры в соответствии с [документацией](../../modules/300-prometheus/cr.html#grafanaadditionaldatasource).

Подробнее с данной функциональностью можно ознакомиться в [документации Yandex Cloud](https://cloud.yandex.ru/ru/docs/monitoring/operations/prometheus/querying/grafana).

## Модуль cni-cilium

Обеспечивает работу сети в кластере с помощью модуля [cilium](https://cilium.io/).

### Ограничения

1. Сервисы с типом `NodePort` и `LoadBalancer` не работают с hostNetwork-эндпоинтами в LB-режиме `DSR`. Переключитесь на режим `SNAT`, если это требуется.
2. `HostPort` поды биндятся только [к одному IP](https://github.com/deckhouse/deckhouse/issues/3035). Если в ОС есть несколько интерфейсов/IP, Cilium выберет один из них, предпочитая «серые» IP-адреса «белым».
3. Требования к ядру:
   * Для работы модуля `cni-cilium` необходимо ядро Linux версии >= `5.7`.
   * Для работы модуля `cni-cilium` совместно с модулем [istio](../110-istio/), [openvpn](../500-openvpn/) или [node-local-dns]({% if site.d8Revision == 'CE' %}{{ site.urls.ru}}/products/kubernetes-platform/documentation/v1/modules/{% else %}..{% endif %}/350-node-local-dns/) необходимо ядро Linux версии >= `5.7`.
4. Проблемы совместимости с ОС:
   * Ubuntu:
     * не работоспособно на 18.04
     * для работы на 20.04 необходима установка ядра HWE
   * Astra Linux:
     * не работоспособно на издании "Смоленск"
   * CentOS:
     * 7 (необходимо новое ядро из [репозитория](http://elrepo.org))
     * 8 (необходимо новое ядро из [репозитория](http://elrepo.org))

### Заметка о CiliumClusterwideNetworkPolicies

1. Убедитесь, что вы применили первичный набор объектов `CiliumClusterwideNetworkPolicy`, поставив конфигурационную опцию `policyAuditMode` в `true`.
   Отсутствие опции может привести к некорректной работе control plane или потере доступа ко всем узлам кластера по SSH.
   Вы можете удалить опцию после применения всех `CiliumClusterwideNetworkPolicy`-объектов и проверки корректности их работы в Hubble UI.
2. Убедитесь, что вы применили следующее правило. В противном случае control plane может некорректно работать до одной минуты во время перезагрузки `cilium-agent`-подов. Это происходит из-за [сброса conntrack таблицы](https://github.com/cilium/cilium/issues/19367). Привязка к entity `kube-apiserver` позволяет обойти баг.

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

### Заметка о смене режима работы Cilium

При смене режима работы Cilium (параметр [tunnelMode](configuration.html#parameters-tunnelmode)) c `Disabled` на `VXLAN` или обратно необходимо перезагрузить все узлы, иначе возможны проблемы с доступностью подов.

### Заметка о выключении модуля kube-proxy

Cilium полностью заменяет собой функционал модуля kube-proxy, поэтому тот автоматически отключается при включении модуля cni-cilium.

### Заметка об отказоустойчивом Egress Gateway

{% alert level="warning" %} Функция доступна только в Enterprise Edition {% endalert %}

#### Базовый режим

Используются предварительно настроенные IP-адреса на egress-узлах.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_base_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/12l4w9ZS3Hpax1B7eOptm2dQX55VVAFzRTtyihw4Ie0c/ --->

#### Режим с Virtual IP

Позволяет динамически назначать дополнительные IP-адреса узлам.

<div data-presentation="../../presentations/021-cni-cilium/egressgateway_virtualip_ru.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1tmhbydjpCwhNVist9RT6jzO1CMpc-G1I7rczmdLzV8E/ --->

## Prometheus-мониторинг

Устанавливает и полностью настраивает [Prometheus](https://prometheus.io/), настраивает сбор метрик со многих распространенных приложений, а также предоставляет необходимый минимальный набор alert'ов для Prometheus и dashboard Grafana.

Если используется StorageClass с поддержкой автоматического расширения (`allowVolumeExpansion: true`), при нехватке места на диске для данных Prometheus его емкость будет увеличена.

Ресурсы CPU и memory автоматически выставляются при пересоздании пода на основе истории потребления, благодаря модулю [Vertical Pod Autoscaler](../../modules/302-vertical-pod-autoscaler/). Также, благодаря кэшированию запросов к Prometheus с помощью [Trickster](https://github.com/trickstercache/trickster), потребление памяти Prometheus сильно сокращается.

Поддерживается как pull-, так и push-модель получения метрик.

### Мониторинг аппаратных ресурсов

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

### Мониторинг Kubernetes

Deckhouse настраивает мониторинг широкого набора параметров «здоровья» Kubernetes и его компонентов, в частности:
- общей утилизации кластера;
- связанности узлов Kubernetes между собой (измеряется rtt между всеми узлами);
- доступности и работоспособности компонентов control plane:
  - `etcd`;
  - `coredns` и `kube-dns`;
  - `kube-apiserver` и др.
- синхронизации времени на узлах и др.

### Мониторинг Ingress

Подробно описан [здесь](../../modules/402-ingress-nginx/#мониторинг-и-статистика)

### Режим расширенного мониторинга

В Deckhouse возможно использование [режима расширенного мониторинга](../340-extended-monitoring/), который предоставляет возможности алертов по дополнительным метрикам: свободному месту и inode на дисках узлов, утилизации узлов, доступности подов и образов контейнеров, истечении действия сертификатов, другим событиям кластера.

#### Алертинг в режиме расширенного мониторинга

Deckhouse позволяет гибко настроить алертинг на каждый из namespace'ов и указывать разную критичность в зависимости от порогового значения. Есть возможность указать множество пороговых значений отправки алертов в различные namespace'ы, например, для таких параметров, как:
- значения свободного места и inodes на диске;
- утилизация CPU узлов и контейнера;
- процент 5xx ошибок на `nginx-ingress`;
- количество возможных недоступных подов в `Deployment`, `StatefulSet`, `DaemonSet`.

### Алерты

Мониторинг в составе Deckhouse включает также и возможности уведомления о событиях. В стандартной поставке уже идет большой набор только необходимых алертов, покрывающих состояние кластера и его компонентов. При этом всегда остается возможность добавления кастомных алертов.

#### Отправка алертов во внешние системы

Deckhouse поддерживает отправку алертов с помощью `Alertmanager`:
- по протоколу SMTP;
- в PagerDuty;
- в Slack;
- в Telegram;
- посредством Webhook;
- по любым другим каналам, поддерживаемым в Alertmanager.

### Включенные модули

![Схема взаимодействия](../../images/300-prometheus/prometheus_monitoring_new.svg)

#### Компоненты, устанавливаемые Deckhouse

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

#### Внешние компоненты

Deckhouse может интегрироваться с большим количеством разнообразных решений следующими способами:

| Название                       | Описание|
|--------------------------------|--------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanager'ы могут быть подключены к Prometheus и Grafana и находиться как в кластере Deckhouse, так и за его пределами.|
| **Long-term metrics storages** | Используя протокол `remote write`, возможно отсылать метрики из Deckhouse в большое количество хранилищ, включающее [Cortex](https://www.cortex.io/), [Thanos](https://thanos.io/), [VictoriaMetrics](https://victoriametrics.com/products/open-source/).|

## Модуль cni-flannel

Обеспечивает работу сети в кластере с помощью модуля [flannel](https://github.com/flannel-io/flannel).

## Модуль dashboard

Устанавливает [Web UI](https://github.com/kubernetes/dashboard) Kubernetes Dashboard для ручного управления кластером, который интегрирован с модулями [user-authn](../../modules/150-user-authn/) и [user-authz](../../modules/140-user-authz/) (доступ в кластер осуществляется от имени пользователя и с учетом его прав).

Если модуль работает по HTTP, он использует минимальные права с ролью `User` из модуля [user-authz](../../modules/140-user-authz/).

> **Важно!** Для работы модуля `dashboard` необходим включенный модуль `user-authz`.

Kubernetes Dashboard среди прочего позволяет:
- управлять подами и другими высокоуровневыми ресурсами;
- получать доступ в контейнеры через веб-консоль для отладки;
- просматривать логи отдельных контейнеров.

## Модуль snapshot-controller

Этот модуль включает поддержку снапшотов для совместимых CSI-драйверов в кластере Kubernetes.

CSI-драйверы в Deckhouse, которые поддерживают снапшоты:
- [cloud-provider-openstack](../030-cloud-provider-openstack/);
- [cloud-provider-vsphere](../030-cloud-provider-vsphere/);
- [ceph-csi](../031-ceph-csi/);
- [cloud-provider-aws](../030-cloud-provider-aws/);
- [cloud-provider-azure](../030-cloud-provider-azure/);
- [cloud-provider-gcp](../030-cloud-provider-gcp/);
- [sds-replicated-volume](https://deckhouse.ru/modules/sds-replicated-volume/stable/)
- [csi-nfs](https://deckhouse.ru/modules/csi-nfs/stable/).
