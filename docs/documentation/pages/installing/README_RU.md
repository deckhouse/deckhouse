---
title: "Установка"
permalink: ru/installing/
description: |
 Информация по установке Deckhouse Kubernetes Platform, включая подготовку инфраструктуры, конфигурацию и запуск инсталлятора.
lang: ru
search: deckhouse installation, kubernetes installation, platform setup, infrastructure preparation, installer configuration, установка Deckhouse, установка Kubernetes, настройка платформы, подготовка инфраструктуры, конфигурация инсталлятора
---

{% alert level="warning" %}
Страница находится в стадии активной разработки и может содержать неполные данные. Ниже представлена обзорная информация о процессе установки Deckhouse. Для более детального ознакомления рекомендуем перейти в раздел [Быстрый старт](/products/kubernetes-platform/gs/), где доступны пошаговые инструкции.
{% endalert %}

{% alert %}
Попробуйте [графический установщик Deckhouse Kubernetes Platform](/products/kubernetes-platform/gs/#gui-install)! <span class="beta-badge">Beta</span>
{% endalert %}

Инсталлятор Deckhouse доступен в виде образа контейнера и основан на утилите [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>), в задачи которой входят:

* Создание и настройка объектов в облачной инфраструктуре с помощью Terraform;
* Установка необходимых пакетов ОС на узлах (в том числе установка пакетов Kubernetes);
* Установка Deckhouse;
* Создание, настройка узлов кластера Kubernetes;
* Поддержание состояния кластера в соответствии с описанной конфигурацией.

Варианты установки Deckhouse:

- **В поддерживаемом облаке.** Утилита `dhctl` автоматически создает и настраивает все необходимые ресурсы, включая виртуальные машины, развертывает Kubernetes-кластер и устанавливает Deckhouse. Полный список поддерживаемых облачных провайдеров доступен в разделе [Интеграция платформы с инфраструктурой](../admin/integrations/integrations-overview.html).

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

1. [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) — начальные параметры [конфигурации Deckhouse](../#конфигурация-deckhouse), необходимые для корректного запуска Deckhouse после установки.

   Основные настройки, задаваемые в этом ресурсе:

   * [Параметры размещения компонентов](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys);
   * Используемый [StorageClass](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-storageclass);
   * Параметры доступа к [container registry](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg);
   * Шаблон [DNS-имен](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate);
   * Другие важные параметры, без которых Deckhouse не сможет корректно функционировать.

1. [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — общие параметры кластера, такие как версия control plane, сетевые настройки, параметры CRI и т. д.
    > Этот ресурс требуется использовать только в случае, если Deckhouse устанавливается с предварительным развертыванием кластера Kubernetes. Если Deckhouse устанавливается в уже существующий кластер, этот ресурс не нужен.

1. [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — параметры для кластеров Kubernetes, развертываемых на серверах bare-metal или виртуальных машинах в неподдерживаемых облаках.
   > Этот ресурс требуется использовать только в случае, если Deckhouse устанавливается с предварительным развертыванием кластера Kubernetes. Если Deckhouse устанавливается в уже существующий кластер, этот ресурс не нужен.

1. `<CLOUD_PROVIDER>ClusterConfiguration` — набор ресурсов, содержащих параметры конфигурации поддерживаемых облачных провайдеров. Включает такие параметры, как:

   * Настройки доступа к облачной инфраструктуре (параметры аутентификации);
   * Тип и параметры схемы размещения ресурсов;
   * Сетевые параметры;
   * Настройки создаваемых групп узлов.

   Список ресурсов конфигурации поддерживаемых облачных провайдеров:

   * [AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
   * [AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
   * [DynamixClusterConfiguration](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration) — Базис.DynamiX;
   * [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
   * [HuaweiCloudClusterConfiguration](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration) — Huawei Cloud;
   * [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack;
   * [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
   * [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) — VMware Cloud Director;
   * [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud;
   * [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) — zVirt.

1. [ModuleConfig](/products/kubernetes-platform/documentation/latest/reference/api/cr.html#moduleconfig) — набор ресурсов, содержащих параметры конфигурации встроенных модулей Deckhouse.

   Если кластер изначально создается с узлами, выделенными для определенных типов нагрузки (например, системные узлы или узлы для мониторинга), рекомендуется в конфигурации модулей, использующих тома постоянного хранилища явно задавать параметр `nodeSelector`.

   Например, для модуля `prometheus` настройка указывается в параметре [nodeSelector](/modules/prometheus/configuration.html#parameters-nodeselector).

1. [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) — развертывание Ingress-контроллера.

1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — создание дополнительных групп узлов.

1. InstanceClass — добавление конфигурационных ресурсов.

1. [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), [User](/modules/user-authn/cr.html#user) — настройка прав и пользователей.

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
1. `<RELEASE_CHANNEL>` — [канал обновлений](/modules/deckhouse/configuration.html#parameters-releasechannel) в формате kebab-case:
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
   - Значения параметров [PublicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) [clusterDomain](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) не совпадают.
   - Данные аутентификации для container registry, указанные в конфигурации установки, корректны.
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
   - Container registry доступен через прокси (если настройки прокси указаны в конфигурации установки).
   - На сервере (ВМ) для master-узла и в хосте инсталлятора свободны порты, необходимые для процесса установки.
   - DNS должен разрешать `localhost` в IP-адрес 127.0.0.1.
   - На сервере (ВМ) пользователю доступна команда `sudo`.
   - Открыты необходимые порты для установки:
     - между хостом запуска установщика и сервером — порт 22/TCP;
     - отсутствуют конфликты по портам, которые используются процессом установки.
   - На сервере (ВМ) установлено корректное время.
   - Адресное пространство подов (`podSubnetCIDR`), сервисов (`serviceSubnetCIRD`) и внутренней сети кластера (`internalNetworkCIDRs`) не пересекаются.
   - На сервере (ВМ) отсутствует пользователь `deckhouse`.

1. Проверки для установки облачного кластера:
   - Конфигурация виртуальной машины master-узла удовлетворяет минимальным требованиям.
   - API облачного провайдера доступно с узлов кластера.
   - Проверка конфигурации [Yandex Cloud с NAT Instance](/modules/cloud-provider-yandex/layouts.html#withnatinstance).

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

## Закрытое окружение, работа через proxy и сторонние registries

### Установка Deckhouse Kubernetes Platform из стороннего registry

{% alert level="warning" %}
Доступно в следующих редакциях: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

{% alert level="warning" %}
DKP поддерживает работу только с Bearer token-схемой авторизации в container registry.

Протестирована и гарантируется работа со следующими container registries:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.
{% endalert %}

При установке DKP можно настроить на работу со сторонним registry (например, проксирующий registry внутри закрытого контура).

Установите следующие параметры в ресурсе InitConfiguration:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа DKP EE в стороннем registry. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему registry, зашифрованные в Base64.

Если разрешен анонимный доступ к образам DKP в стороннем registry, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам DKP в стороннем registry необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

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

Для настройки нестандартных конфигураций сторонних registries в ресурсе InitConfiguration предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат registry (если registry использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к registry (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

### Особенности настройки Nexus

{% alert level="warning" %}
При взаимодействии с репозиторием типа `docker` расположенным в Nexus (например, при выполнении команд `docker pull`, `docker push`) требуется указывать адрес в формате `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Использование значения `URL` из параметров репозитория Nexus **недопустимо**
{% endalert %}

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Создан **проксирующий** репозиторий Docker («Administration» → «Repository» → «Repositories»):
  * Установлен в `0` параметр `Maximum metadata age` для репозитория.
* Настроен контроль доступа:
  * Создана роль **Nexus** («Administration» → «Security» → «Roles») со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`
    * `nx-repository-view-docker-<репозиторий>-read`
  * Создан пользователь («Administration» → «Security» → «Users») с ролью **Nexus**.

**Настройка**:

1. Создайте **проксирующий** репозиторий Docker («Administration» → «Repository» → «Repositories»), указывающий на [Deckhouse registry](https://registry.deckhouse.ru/):
   ![Создание проксирующего репозитория Docker](../images/registry/nexus/nexus-repository.png)

1. Заполните поля страницы создания репозитория следующим образом:
   * `Name` должно содержать имя создаваемого репозитория, например `d8-proxy`.
   * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например `8123` или иной.
   * `Remote storage` должно иметь значение `https://registry.deckhouse.ru/`.
   * `Auto blocking enabled` и `Not found cache enabled` могут быть выключены для отладки; в противном случае их следует включить.
   * `Maximum Metadata Age` должно быть равно `0`.
   * Если планируется использовать коммерческую редакцию Deckhouse Kubernetes Platform, флажок `Authentication` должен быть включен, а связанные поля должны быть заполнены следующим образом:
     * `Authentication Type` должно иметь значение `Username`.
     * `Username` должно иметь значение `license-token`.
     * `Password` должно содержать ключ лицензии Deckhouse Kubernetes Platform.

    ![Пример настроек репозитория 1](../images/registry/nexus/nexus-repo-example-1.png)
    ![Пример настроек репозитория 2](../images/registry/nexus/nexus-repo-example-2.png)
    ![Пример настроек репозитория 3](../images/registry/nexus/nexus-repo-example-3.png)

1. Настройте контроль доступа Nexus для доступа DKP к созданному репозиторию:
   * Создайте роль **Nexus** («Administration» → «Security» → «Roles») с полномочиями `nx-repository-view-docker-<репозиторий>-browse` и `nx-repository-view-docker-<репозиторий>-read`.

     ![Создание роли Nexus](../images/registry/nexus/nexus-role.png)

   * Создайте пользователя («Administration» → «Security» → «Users») с ролью, созданной выше.

     ![Создание пользователя Nexus](../images/registry/nexus/nexus-user.png)

В результате образы DKP будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Особенности настройки Harbor

Используйте функцию [Harbor Proxy Cache](https://github.com/goharbor/harbor).

* Настройте registry:
  * «Administration» → «Registries» → «New Endpoint».
  * «Provider: Docker Registry».
  * «Name» — укажите любое, на ваше усмотрение.
  * «Endpoint URL: `https://registry.deckhouse.ru`».
  * Укажите «Access ID» и «Access Secret» (лицензионный ключ для Deckhouse Kubernetes Platform).

    ![Настройка Registry](../images/registry/harbor/harbor1.png)

* Создайте новый проект:
  * «Projects → New Project».
  * «Project Name» будет частью URL. Используйте любой, например, `d8s`.
  * «Access Level: `Public`».
  * «Proxy Cache» — включите и выберите в списке registry, созданный на предыдущем шаге.

    ![Создание нового проекта](../images/registry/harbor/harbor2.png)

В результате настройки, образы DKP станут доступны, например, по следующему адресу: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Ручная загрузка образов Deckhouse Kubernetes Platform, БД сканера уязвимостей и модулей DKP в приватный registry

{% alert level="warning" %}
Утилита `d8 mirror` недоступна для использования с редакциями Community Edition (CE) и Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
О текущем статусе версий на каналах обновлений можно узнать на [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

1. [Скачайте и установите утилиту Deckhouse CLI](../cli/d8/).

1. Скачайте образы DKP в выделенную директорию, используя команду `d8 mirror pull`.

   По умолчанию `d8 mirror pull` скачивает только актуальные версии DKP, базы данных сканера уязвимостей (если они входят в редакцию DKP) и официально поставляемых модулей.
   Например, для Deckhouse Kubernetes Platform 1.59 будет скачана только версия 1.59.12, т. к. этого достаточно для обновления платформы с 1.58 до 1.59.

   Выполните следующую команду (укажите код редакции и лицензионный ключ), чтобы скачать образы актуальных версий:

   ```shell
   d8 mirror pull \
     --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
     --license='<LICENSE_KEY>' /home/user/d8-bundle
   ```

   где:

   - `--source` — адрес источника (container registry) Deckhouse Kubernetes Platform.
   - `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `se-plus`). По умолчанию параметр `--source` ссылается на редакцию Enterprise Edition (`ee`) и может быть опущен.
   - `--license` — параметр для указания лицензионного ключа Deckhouse Kubernetes Platform для аутентификации в официальном container registry
   - `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform.
   - `/home/user/d8-bundle` — директория, в которой будут расположены пакеты образов. Будет создана, если не существует.

   > Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента ее остановки прошло не более суток.

   {% offtopic title="Другие параметры команды, доступные для использования:" %}

   - `--no-pull-resume` — чтобы принудительно начать загрузку сначала;
   - `--no-platform` — для пропуска загрузки пакета образов Deckhouse Kubernetes Platform (platform.tar);
   - `--no-modules` — для пропуска загрузки пакетов модулей (module-*.tar);
   - `--no-security-db` — для пропуска загрузки пакета баз данных сканера уязвимостей (security.tar);
   - `--include-module` / `-i` = `name[@Major.Minor]` — для загрузки только определенного набора модулей по принципу белого списка (и, при необходимости, их минимальных версий). Укажите несколько раз, чтобы добавить в белый список больше модулей. Эти флаги игнорируются, если используются совместно с `--no-modules`.

     Поддерживаются следующие синтаксисы для указания версий модулей:
     - `module-name@1.3.0` — загрузка версий с semver ^ ограничением (^1.3.0), включая v1.3.0, v1.3.3, v1.4.1;
     - `module-name@~1.3.0` — загрузка версий с semver ~ ограничением (>=1.3.0 <1.4.0), включая только v1.3.0, v1.3.3;
     - `module-name@=v1.3.0` — загрузка точного соответствия тегу v1.3.0, публикация во все каналы релизов;
     - `module-name@=bobV1` — загрузка точного соответствия тегу "bobV1", публикация во все каналы релизов;
   - `--exclude-module` / `-e` = `name` — для пропуска загрузки определенного набора модулей по принципу черного списка. Укажите несколько раз, чтобы добавить в черный список больше модулей. Игнорируется, если используются `--no-modules` или `--include-module`.
   - `--modules-path-suffix` — для изменения суффикса пути к репозиторию модулей в основном репозитории DKP. По умолчанию используется суффикс `/modules` (так, например, полный путь к репозиторию с модулями будет выглядеть как `registry.deckhouse.ru/deckhouse/EDITION/modules`).
   - `--since-version=X.Y` — чтобы скачать все версии DKP, начиная с указанной минорной версии. Параметр будет проигнорирован, если указана версия выше чем версия находящаяся на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--deckhouse-tag`;
   - `--deckhouse-tag` — чтобы скачать только конкретную версию DKP (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--since-version`;
   - `--gost-digest` — для расчета контрольной суммы итогового набора образов DKP в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с tar-архивом, содержащим образы DKP;
   - Для аутентификации в стороннем container registry нужно использовать параметры `--source-login` и `--source-password`;
   - `--images-bundle-chunk-size=N` — для указания максимального размера файла (в ГБ), на которые нужно разбить архив образов. В результате работы вместо одного файла архива образов будет создан набор `.chunk`-файлов (например, `d8.tar.NNNN.chunk`). Чтобы загрузить образы из такого набора файлов, укажите в команде `d8 mirror push` имя файла без суффикса `.NNNN.chunk` (например, `d8.tar` для файлов `d8.tar.NNNN.chunk`);
   - `--tmp-dir` — путь к директории для временных файлов, который будет использоваться во время операций загрузки и выгрузки образов. Вся обработка выполняется в этом каталоге. Он должен иметь достаточное количество свободного дискового пространства, чтобы вместить весь загружаемый пакет образов. По умолчанию используется поддиректория `.tmp` в директории с пакетами образов.

   {% endofftopic %}

   Дополнительные параметры конфигурации для семейства команд `d8 mirror` доступны в виде переменных окружения.

   {% offtopic title="Подробнее:" %}

   - `HTTP_PROXY`/`HTTPS_PROXY` — URL прокси-сервера для запросов к HTTP(S) хостам, которые не указаны в списке хостов в переменной `$NO_PROXY`.
   - `NO_PROXY` — список хостов, разделенных запятыми, которые следует исключить из проксирования. Каждое значение может быть представлено в виде IP-адреса (`1.2.3.4`), CIDR (`1.2.3.4/8`), домена или символа (`*`). IP-адреса и домены также могут включать номер порта (`1.2.3.4:80`). Доменное имя соответствует как самому себе, так и всем поддоменам. Доменное имя, начинающееся с `.`, соответствует только поддоменам. Например, `foo.com` соответствует `foo.com` и `bar.foo.com`; `.y.com` соответствует `x.y.com`, но не соответствует `y.com`. Символ `*` отключает проксирование.
   - `SSL_CERT_FILE` — указывает путь до сертификата SSL. Если переменная установлена, системные сертификаты не используются.
   - `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы сертификатов SSL. Если переменная установлена, системные сертификаты не используются. [Подробнее...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html)
   - `MIRROR_BYPASS_ACCESS_CHECKS` — установите для этого параметра значение `1`, чтобы отключить проверку корректности переданных учетных данных для registry.

   {% endofftopic %}

   Пример команды для загрузки всех версий DKP EE начиная с версии 1.59 (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --since-version=1.59 /home/user/d8-bundle
   ```

   Пример команды для загрузки актуальных версий DKP SE (укажите лицензионный ключ):

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --source='registry.deckhouse.ru/deckhouse/se' \
   /home/user/d8-bundle
   ```

   Пример команды для загрузки образов DKP из стороннего container registry:

   ```shell
   d8 mirror pull \
   --source='corp.company.com:5000/sys/deckhouse' \
   --source-login='<USER>' --source-password='<PASSWORD>' /home/user/d8-bundle
   ```

   Пример команды для загрузки пакета баз данных сканера уязвимостей:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-modules /home/user/d8-bundle
   ```

   Пример команды для загрузки пакетов всех доступных дополнительных модулей:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db /home/user/d8-bundle
   ```

   Пример команды для загрузки пакетов модулей `stronghold` и `secrets-store-integration`:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module stronghold \
   --include-module secrets-store-integration \
   /home/user/d8-bundle
   ```

   Пример команды для загрузки модуля `stronghold` с semver `^` ограничением от версии 1.2.0:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module stronghold@1.2.0 \
   /home/user/d8-bundle
   ```

   Пример команды для загрузки модуля `secrets-store-integration` с semver `~` ограничением от версии 1.1.0:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module secrets-store-integration@~1.1.0 \
   /home/user/d8-bundle
   ```

   Пример команды для загрузки точной версии модуля `stronghold` 1.2.5 с публикацией во все каналы релизов:

   ```shell
   d8 mirror pull \
   --license='<LICENSE_KEY>' \
   --no-platform --no-security-db \
   --include-module stronghold@=v1.2.5 \
   /home/user/d8-bundle
   ```

1. На хост с доступом к container registry, куда нужно загрузить образы DKP, скопируйте загруженный пакет образов DKP и установите [Deckhouse CLI](../cli/d8/).

1. Загрузите образы DKP в container registry с помощью команды `d8 mirror push`.

   Команда `d8 mirror push` загружает в container registry образы из всех пакетов, которые присутствуют в переданной директории.
   При необходимости выгрузить в container registry только часть пакетов, вы можете либо выполнить команду для каждого необходимого пакета образов передав ей прямой путь до пакета tar вместо директории, либо убрав расширение `.tar` у ненужных пакетов или переместив их вне директории.

   Пример команды для загрузки пакетов образов из директории `/mnt/MEDIA/d8-images` (укажите данные для авторизации при необходимости):

   ```shell
   d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
     --registry-login='<USER>' --registry-password='<PASSWORD>'
   ```

   Перед загрузкой образов убедитесь, что путь для загрузки в container registry существует (в примере — `/sys/deckhouse`) и у используемой учетной записи есть права на запись.

   Если вы используете Harbor, вы не сможете выгрузить образы в корень проекта, используйте выделенный репозиторий в проекте для размещения образов DKP.

1. После загрузки образов в container registry можно переходить к установке DKP. Воспользуйтесь [руководством по быстрому старту](/products/kubernetes-platform/gs/bm-private/step2.html).

   При запуске установщика используйте не официальный публичный container registry DKP, а container registry в который ранее были загружены образы. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable`, вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

   В ресурсе [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) при установке также используйте адрес вашего container registry и данные авторизации (параметры [imagesRepo](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), [registryDockerCfg](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg) или [шаг 3]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm-private/step3.html) руководства по быстрому старту).

### Создание кластера и запуск DKP без использования каналов обновлений

{% alert level="warning" %}
Этот способ следует использовать только в случае, если в изолированном приватном registry нет образов, содержащих информацию о каналах обновлений.
{% endalert %}

Если необходимо установить DKP с отключенным автоматическим обновлением:

1. Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
1. Укажите соответствующий номер версии в параметре [deckhouse.devBranch](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-devbranch) в ресурсе [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).
    > **Не указывайте** параметр [deckhouse.releaseChannel](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#module-v1alpha1-properties-releasechannel) в ресурсе [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).

Если вы хотите отключить автоматические обновления у уже установленного Deckhouse (включая обновления patch-релизов), удалите параметр [releaseChannel](/modules/deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

### Использование proxy-сервера

{% alert level="warning" %}
Доступно в следующих редакциях: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

{% offtopic title="Пример шагов по настройке proxy-сервера на базе Squid..." %}

1. Подготовьте сервер (или виртуальную машину). Сервер должен быть доступен с необходимых узлов кластера, и у него должен быть выход в интернет.
1. Установите Squid (здесь и далее примеры для Ubuntu):

   ```shell
   apt-get install squid
   ```

1. Создайте файл конфигурации Squid:

   ```shell
   cat <<EOF > /etc/squid/squid.conf
   auth_param basic program /usr/lib/squid3/basic_ncsa_auth /etc/squid/passwords
   auth_param basic realm proxy
   acl authenticated proxy_auth REQUIRED
   http_access allow authenticated

   # Укажите необходимый порт. Порт 3128 используется по умолчанию.
   http_port 3128
   ```

1. Создайте пользователя и пароль для аутентификации на proxy-сервере:

   Пример для пользователя `test` с паролем `test` (обязательно измените):

   ```shell
   echo "test:$(openssl passwd -crypt test)" >> /etc/squid/passwords
   ```

1. Запустите Squid и включите его автоматический запуск при загрузке сервера:

   ```shell
   systemctl restart squid
   systemctl enable squid
   ```

{% endofftopic %}

Для настройки DKP на использование proxy используйте параметр [proxy](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-proxy) ресурса ClusterConfiguration.

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

{% raw %}

### Автозагрузка переменных proxy пользователям в CLI

Начиная с версии платформы DKP v1.67 больше не настраивается файл `/etc/profile.d/d8-system-proxy.sh`, который устанавливал переменные proxy для пользователей. Для автозагрузки переменных proxy пользователям в CLI используйте ресурс [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: profile-proxy.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 99
  content: |
    {{- if .proxy }}
      {{- if .proxy.httpProxy }}
    export HTTP_PROXY={{ .proxy.httpProxy | quote }}
    export http_proxy=${HTTP_PROXY}
      {{- end }}
      {{- if .proxy.httpsProxy }}
    export HTTPS_PROXY={{ .proxy.httpsProxy | quote }}
    export https_proxy=${HTTPS_PROXY}
      {{- end }}
      {{- if .proxy.noProxy }}
    export NO_PROXY={{ .proxy.noProxy | join "," | quote }}
    export no_proxy=${NO_PROXY}
      {{- end }}
    bb-sync-file /etc/profile.d/profile-proxy.sh - << EOF
    export HTTP_PROXY=${HTTP_PROXY}
    export http_proxy=${HTTP_PROXY}
    export HTTPS_PROXY=${HTTPS_PROXY}
    export https_proxy=${HTTPS_PROXY}
    export NO_PROXY=${NO_PROXY}
    export no_proxy=${NO_PROXY}
    EOF
    {{- else }}
    rm -rf /etc/profile.d/profile-proxy.sh
    {{- end }}
```

{% endraw %}
