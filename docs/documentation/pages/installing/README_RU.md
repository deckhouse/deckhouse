---
title: "Установка"
permalink: ru/installing/
lang: ru
---

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

## Подготовка инфраструктуры

Перед установкой проверьте следующее:
- *(для кластеров на bare metal и в неподдерживаемых облаках)* ОС сервера находится в [списке поддерживаемых ОС](../supported_versions.html) (или совместима с ними) и до сервера есть SSH-доступ по ключу;
- *(в поддерживаемых облаках)* наличие квот, необходимых для создания ресурсов, и параметров доступа к облаку (набор зависит от конкретной облачной инфраструктуры или облачного провайдера);
- доступ до container registry с образами Deckhouse (по умолчанию — `registry.deckhouse.io`).

## Подготовка конфигурации

Для установки Deckhouse нужно подготовить YAML-файл конфигурации установки и, при необходимости, YAML-файл ресурсов, которые нужно создать после успешной установки Deckhouse.

### Файл конфигурации установки

YAML-файл конфигурации установки содержит параметры нескольких ресурсов (манифесты):
- [InitConfiguration](configuration.html#initconfiguration) — начальные параметры [конфигурации Deckhouse](../#конфигурация-deckhouse). С этой конфигурацией Deckhouse запустится после установки.

  В этом ресурсе, в частности, указываются параметры, без которых Deckhouse не запустится или будет работать некорректно. Например, параметры [размещения компонентов Deckhouse](../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys), используемый [storageClass](../deckhouse-configure-global.html#parameters-storageclass), параметры доступа к [container registry](configuration.html#initconfiguration-deckhouse-registrydockercfg), [шаблон используемых DNS-имен](../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate) и другие.

  **Внимание!** Параметр [configOverrides](configuration.html#initconfiguration-deckhouse-configoverrides) устарел и будет удален. Для установки параметров **встроенных модулей** Deckhouse используйте набор ресурсов [ModuleConfig](../#настройка-модуля) в файле конфигурации установки. Для остальных модулей используйте [файл ресурсов установки](#файл-ресурсов-установки).
  
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
  configOverrides:
    global:
      modules:
        publicDomainTemplate: "%s.example.com"
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

### Файл ресурсов установки

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

### Post-bootstrap-скрипт

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

## Установка Deckhouse

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
- `<DECKHOUSE_REVISION>` — редакция Deckhouse: `ee` — для Enterprise Edition и `ce` — для Community Edition.
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
  --config=/config.yml --resources=/resources.yml
```

где:
- `/config.yml` — файл конфигурации установки;
- `/resources.yml` — файл манифестов ресурсов;
- `<SSH_USER>` — пользователь на сервере для подключения по SSH;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.

### Откат установки и удаление Deckhouse

При установке в поддерживаемом облаке в случае прерывания установки или возникновения проблем во время установки в облаке могут остаться созданные ресурсы. Для удаления созданных в облаке ресурсов используйте команду: `dhctl bootstrap-phase abort`. Обратите внимание, что файл конфигурации (передаваемый через параметр `--config`) должен быть тот же, с которым производилась установка.

Чтобы удалить работающий в поддерживаемом облаке кластер (развернутый после установки Deckhouse), используйте команду: `dhctl destroy`. В этом случае `dhctl` подключится к master-узлу, получит от него terraform state и корректно удалит созданные в облаке ресурсы.
