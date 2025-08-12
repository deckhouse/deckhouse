---
title: "Установка"
permalink: ru/installing/
description: |
 Информация по установке Deckhouse Kubernetes Platform, включая подготовку инфраструктуры, конфигурацию и запуск инсталлятора.
lang: ru
---

{% alert level="warning" %}
Страница находится в стадии активной разработки и может содержать неполные данные. Ниже представлена обзорная информация о процессе установки Deckhouse. Для более детального ознакомления рекомендуем перейти в раздел [Быстрый старт](/products/kubernetes-platform/gs/), где доступны пошаговые инструкции.
{% endalert %}

Инсталлятор Deckhouse доступен в виде образа контейнера и основан на утилите [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>), в задачи которой входят:

* Создание и настройка объектов в облачной инфраструктуре с помощью Terraform;
* Установка необходимых пакетов ОС на узлах (в том числе установка пакетов Kubernetes);
* Установка Deckhouse;
* Создание, настройка узлов кластера Kubernetes;
* Поддержание состояния кластера в соответствии с описанной конфигурацией.

Варианты установки Deckhouse:

- **В поддерживаемом облаке.** Утилита `dhctl` автоматически создает и настраивает все необходимые ресурсы, включая виртуальные машины, развертывает Kubernetes-кластер и устанавливает Deckhouse. Полный список поддерживаемых облачных провайдеров доступен в разделе [Кластер Kubernetes](../kubernetes.html).

- **На серверах bare-metal или в неподдерживаемых облаках**. В этом варианте `dhctl`  выполняет настройку сервера или виртуальной машины, развертывает Kubernetes-кластер с одним master-узлом и устанавливает Deckhouse. Для добавления дополнительных узлов в кластер можно воспользоваться готовыми скриптами настройки.

- **В существующем Kubernetes-кластере.** Если Kubernetes-кластер уже развернут, `dhctl` устанавливает Deckhouse, интегрируя его с текущей инфраструктурой.

## Подготовка инфраструктуры

Перед установкой убедитесь в следующем:

- **Для кластеров на bare-metal и в неподдерживаемых облаках**: сервер использует операционную систему из [списка поддерживаемых ОС](../supported_versions.html) или совместимую с ним, а также доступен по SSH через ключ.

- **Для поддерживаемых облаков**: имеются необходимые квоты для создания ресурсов и подготовлены параметры доступа к облачной инфраструктуре (зависят от конкретного провайдера).

- **Для всех вариантов установки**: настроен доступ к container registry с образами Deckhouse (`registry.deckhouse.io` или `registry.deckhouse.ru`).

## Подготовка конфигурации

Перед началом установки Deckhouse необходимо выполнить подготовку [YAML-файла конфигурации установки](#файл-конфигурации-установки). Этот файл содержит основные параметры для настройки Deckhouse, включая информацию о кластерных компонентах, сетевых настройках и интеграциях, а также описание ресурсов для создания после установки (настройки узлов и Ingress-контроллера).

Убедитесь, что конфигурационный файл соответствует требованиям вашей инфраструктуры и включает все необходимые параметры для корректного развертывания.

### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):

1. [InitConfiguration](configuration.html#initconfiguration) — начальные параметры [конфигурации Deckhouse](../#конфигурация-deckhouse), необходимые для корректного запуска Deckhouse после установки.

   Основные настройки, задаваемые в этом ресурсе:

   * [Параметры размещения компонентов](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys);
   * Используемый [StorageClass](../deckhouse-configure-global.html#parameters-storageclass) (параметры хранилища);
   * Параметры доступа к [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg);
   * Шаблон [DNS-имен](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate);
   * Другие важные параметры, без которых Deckhouse не сможет корректно функционировать.

1. [ClusterConfiguration](configuration.html#clusterconfiguration) — общие параметры кластера, такие как версия control plane, сетевые настройки, параметры CRI и т. д.
    > Этот ресурс требуется использовать только в случае, если Deckhouse устанавливается с предварительным развертыванием кластера Kubernetes. Если Deckhouse устанавливается в уже существующий кластер, этот ресурс не нужен.

1. [StaticClusterConfiguration](configuration.html#staticclusterconfiguration) — параметры для кластеров Kubernetes, развертываемых на серверах bare-metal или виртуальных машинах в неподдерживаемых облаках.
   > Этот ресурс требуется использовать только в случае, если Deckhouse устанавливается с предварительным развертыванием кластера Kubernetes. Если Deckhouse устанавливается в уже существующий кластер, этот ресурс не нужен.

1. `<CLOUD_PROVIDER>ClusterConfiguration` — набор ресурсов, содержащих параметры конфигурации поддерживаемых облачных провайдеров. Включает такие параметры, как:

   * Настройки доступа к облачной инфраструктуре (параметры аутентификации);
   * Тип и параметры схемы размещения ресурсов;
   * Сетевые параметры;
   * Настройки создаваемых групп узлов.

   Список ресурсов конфигурации поддерживаемых облачных провайдеров:

   * [AWSClusterConfiguration](../modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
   * [AzureClusterConfiguration](../modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
   * [GCPClusterConfiguration](../modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
   * [OpenStackClusterConfiguration](../modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack;
   * [VsphereClusterConfiguration](../modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
   * [VCDClusterConfiguration](../modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) — VMware Cloud Director;
   * [YandexClusterConfiguration](../modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud;
   * [ZvirtClusterConfiguration](../modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) — zVirt.

1. `ModuleConfig` — набор ресурсов, содержащих параметры конфигурации [встроенных модулей Deckhouse](../).

   Если кластер изначально создается с узлами, выделенными для определенных типов нагрузки (например, системные узлы или узлы для мониторинга), рекомендуется в конфигурации модулей, использующих тома постоянного хранилища явно задавать параметр `nodeSelector`.

   Например, для модуля `prometheus` настройка указывается в параметре [nodeSelector](../modules/prometheus/configuration.html#parameters-nodeselector).

1. `IngressNginxController` — развертывание Ingress-контроллера.

1. `NodeGroup` — создание дополнительных групп узлов.

1. `InstanceClass` — добавление конфигурационных ресурсов.

1. `ClusterAuthorizationRule`, `User` — настройка прав и пользователей.

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
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: Stable
    bundle: Default
    logLevel: Info
  version: 1
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
    earlyOomEnabled: false
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  # Укажите, в случае использования выделенных узлов для мониторинга.
  # settings:
  #   nodeSelector:
  #     node.deckhouse.io/group: monitoring
---
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

### Post-bootstrap-скрипт

После завершения установки Deckhouse инсталлятор предоставляет возможность выполнить пользовательский скрипт на одном из master-узлов. Такой скрипт может использоваться для:

* Выполнения дополнительной настройки кластера;
* Сбора диагностической информации;
* Интеграции с внешними системами и других задач.

Указать путь к post-bootstrap-скрипту можно с помощью параметра `--post-bootstrap-script-path` при запуске процесса инсталляции.

{% offtopic title="Пример скрипта, выводящего IP-адрес балансировщика..." %}
Пример скрипта, который выводит IP-адрес балансировщика после развертывания кластера в облаке и установки Deckhouse:

```shell
#!/usr/bin/env bash

set -e
set -o pipefail


INGRESS_NAME="nginx"


echo_err() { echo "$@" 1>&2; }

# Объявление переменной.
lb_ip=""

# Получение IP-адреса балансировщика нагрузки.
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

## Установка Deckhouse

{% alert level="info" %}
При установке коммерческой редакции Deckhouse Kubernetes Platform из официального container registry `registry.deckhouse.ru`, необходимо предварительно пройти аутентификацию с использованием лицензионного ключа:

```shell
docker login -u license-token registry.deckhouse.ru
```

{% endalert %}

Запуск контейнера инсталлятора Deckhouse из публичного container registry выглядит следующим образом:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

Где:

1. `<DECKHOUSE_REVISION>` — [редакция Deckhouse](../revision-comparison.html), например `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.
1. `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер инсталлятора, такие как:
   - SSH-ключи доступа;
   - Файл конфигурации;
   - Файл ресурсов и т. д.
1. `<RELEASE_CHANNEL>` — [канал обновлений](../modules/deckhouse/configuration.html#parameters-releasechannel) в формате kebab-case:
   - `alpha` — для канала обновлений Alpha;
   - `beta` — для канала обновлений Beta;
   - `early-access` — для канала обновлений Early Access;
   - `stable` — для канала обновлений Stable;
   - `rock-solid` — для канала обновлений Rock Solid.

Пример команды для запуска инсталлятора Deckhouse в редакции CE:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
```

Установка Deckhouse осуществляется в контейнере инсталлятора с помощью утилиты `dhctl`:

* Для запуска установки Deckhouse с развертыванием нового кластера (все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
* Для установки Deckhouse в уже существующий кластер используйте команду `dhctl bootstrap-phase install-deckhouse`.

{% alert level="info" %}
Для получения подробной справки по параметрам команды выполните `dhctl bootstrap -h`.
{% endalert %}

Пример запуска установки Deckhouse с развертыванием кластера в облаке:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
  --config=/config.yml
```

Где:

- `/config.yml` — файл конфигурации установки;
- `<SSH_USER>` — имя пользователя для подключения по SSH к серверу;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.

### Проверки перед началом установки

{% offtopic title="Схема выполнения проверок перед началом установки Deckhouse..." %}
![Схема выполнения проверок перед началом установки Deckhouse](../images/installing/preflight-checks.png)
{% endofftopic %}

Список проверок, выполняемых инсталлятором перед началом установки Deckhouse:

1. Общие проверки:
   - Значения параметров [PublicDomainTemplate](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) [clusterDomain](configuration.html#clusterconfiguration-clusterdomain) не совпадают.
   - Данные аутентификации для хранилища образов контейнеров, указанные в конфигурации установки, корректны.
   - Имя хоста соответствует следующим требованиям:
     - Длина не более 63 символов;
     - Состоит только из строчных букв;
     - Не содержит спецсимволов (допускаются символы `-` и `.`, при этом они не могут быть в начале или в конце имени).
   - На сервере (ВМ) установлен поддерживаемый container runtime (`containerd`).
   - Имя хоста уникально в пределах кластера.
   - На сервере установлено корректное время.
   - Адресное пространство подов (`podSubnetCIDR`) и сервисов (`serviceSubnetCIRD`) кластера не пересекаются.

1. Проверки для установки статического и гибридного кластера:
   - Указан только один параметр `--ssh-host`. Для статической конфигурации кластера можно задать только один IP-адрес для настройки первого master-узла.
   - Должна быть возможность подключения по SSH с использованием указанных данных аутентификации.
   - Должна быть возможность установки SSH-туннеля до сервера (или виртуальной машины) master-узла.
   - Сервер (ВМ), выбранный для установки master-узла, должен соответствовать [минимальным системным требованиям](/products/kubernetes-platform/guides/hardware-requirements.html):
     - не менее 4 CPU;
     - не менее 8 ГБ RAM;
     - не менее 60 ГБ диска с производительностью 400+ IOPS;
     - ядро Linux версии 5.8 или новее;
     - установлен один из пакетных менеджеров: `apt`, `apt-get`, `yum` или `rpm`;
     - доступ к стандартным системным репозиториям для установки зависимостей;
     - в случае с РЕД ОС — убедитесь, что установлены `yum` и `which` (по умолчанию могут отсутствовать).
   - На сервере (ВМ) для master-узла установлен Python.
   - Хранилище образов контейнеров доступно через прокси (если настройки прокси указаны в конфигурации установки).
   - На сервере (ВМ) для master-узла и в хосте инсталлятора свободны порты, необходимые для процесса установки.
   - DNS должен разрешать `localhost` в IP-адрес 127.0.0.1.
   - На сервере (ВМ) пользователю доступна команда `sudo`.
   - Открыты необходимые порты для установки:
     - между хостом запуска установщика и сервером — порт 22322/TCP;
     - отсутствуют конфликты по портам, которые используются процессом установки.
   - На сервере (ВМ) установлено корректное время.
   - Адресное пространство подов (`podSubnetCIDR`), сервисов (`serviceSubnetCIRD`) и внутренней сети кластера (`internalNetworkCIDRs`) не пересекаются.
   - На сервере (ВМ) отсутствует пользователь `deckhouse`.

1. Проверки для установки облачного кластера:
   - Конфигурация виртуальной машины master-узла удовлетворяет минимальным требованиям.
   - API облачного провайдера доступно с узлов кластера.
   - Проверка конфигурации [Yandex Cloud с NAT Instance](../modules/cloud-provider-yandex/layouts.html#withnatinstance).

{% offtopic title="Список флагов пропуска проверок..." %}

- `--preflight-skip-all-checks` — пропуск всех предварительных проверок.
- `--preflight-skip-ssh-forward-check`  — пропуск проверки проброса SSH.
- `--preflight-skip-availability-ports-check` — пропуск проверки доступности необходимых портов.
- `--preflight-skip-resolving-localhost-check` — пропуск проверки `localhost`.
- `--preflight-skip-deckhouse-version-check` — пропуск проверки версии Deckhouse.
- `--preflight-skip-registry-through-proxy` — пропуск проверки доступа к registry через прокси-сервер.
- `--preflight-skip-public-domain-template-check`  — пропуск проверки шаблона `publicDomain`.
- `--preflight-skip-ssh-credentials-check`   — пропуск проверки учетных данных SSH-пользователя.
- `--preflight-skip-registry-credential` — пропуск проверки учетных данных для доступа к registry.
- `--preflight-skip-containerd-exist` — пропуск проверки наличия containerd.
- `--preflight-skip-python-checks` — пропуск проверки наличия Python.
- `--preflight-skip-sudo-allowed` — пропуск проверки прав доступа для выполнения команды `sudo`.
- `--preflight-skip-system-requirements-check` — пропуск проверки системных требований.
- `--preflight-skip-one-ssh-host` — пропуск проверки количества указанных SSH-хостов.
- `--preflight-cloud-api-accesibility-check` — пропуск проверки доступности Cloud API.
- `--preflight-time-drift-check` — пропуск проверки рассинхронизации времени (time drift).
- `--preflight-skip-cidr-intersection` — пропуск проверки пересечения CIDR.
- `--preflight-skip-deckhouse-user-check` — пропуск проверки наличия пользователя Deckhouse.
- `--preflight-skip-yandex-with-nat-instance-check` — пропуск проверки конфигурации Yandex Cloud с WithNatInstance.

Пример применения флага пропуска:

```shell
    dhctl bootstrap \
    --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/id_rsa \
    --config=/config.yml \
    --preflight-skip-all-checks
```

{% endofftopic %}

### Откат установки

Если установка была прервана или возникли проблемы во время установки в поддерживаемом облаке, то могут остаться ресурсы, созданные в процессе установки.  Для их удаления используйте команду `dhctl bootstrap-phase abort`, выполнив ее в контейнере инсталлятора.

{% alert level="warning" %}
Файл конфигурации, передаваемый через параметр `--config` при запуске инсталлятора,  должен быть тем же, с которым проводилась первоначальная установка.
{% endalert %}
