---
title: "Установка"
permalink: ru/installing/
description: |
 Установка Deckhouse Kubernetes Platform (DKP), подготовка инфраструктуры установки, запуск установщика.
lang: ru
search: требования, системные требования, installation, platform setup, infrastructure preparation, installer configuration, настройка платформы, подготовка инфраструктуры, конфигурация инсталлятора, конфигурация установщика, dhctl, dhctl bootstrap
extractedLinksMax: 2
relatedLinks:
  - title: "Быстрый старт"
    url: /products/kubernetes-platform/gs/
  - title: "Поддерживаемые версии ОС и Kubernetes"
    url: ../reference/supported_versions.html
  - title: "Интеграция с инфраструктурой"
    url: ../admin/integrations/integrations-overview.html
  - title: "Установка DKP в закрытом окружении"
    url: /products/kubernetes-platform/guides/private-environment.html
  - title: "Подготовка к Production"
    url: /products/kubernetes-platform/guides/production.html   
---

{% alert %}
В разделе {% if site.mode == 'module' %}[«Быстрый старт»]({{ site.urls[page.lang] }}/products/kubernetes-platform/gs/){% else %}[Быстрый старт](/products/kubernetes-platform/gs/){% endif %} доступны пошаговые инструкции по установке Deckhouse Kubernetes Platform.

Попробуйте также [графический установщик Deckhouse Kubernetes Platform]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/#gui-install)! <span class="beta-badge">Beta</span>
{% endalert %}

На этой странице представлена обзорная информация по установке Deckhouse Kubernetes Platform (DKP).

## Способы установки

Установить DKP можно следующими способами:

- с помощью CLI-установщика (доступен в виде образа контейнера и основан на утилите [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>));
- с помощью [графического установщика]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/#gui-install) (находится в режиме бета-тестирования).

Далее рассмотрен процесс установки с помощью **CLI-установщика**.

## Варианты установки

Установить DKP можно в следующих вариантах:

- **В поддерживаемом облаке.** Установщик автоматически создает и настраивает все необходимые ресурсы (включая виртуальные машины, сетевые объекты и т.д.), разворачивает кластер Kubernetes и устанавливает DKP. Полный список поддерживаемых облачных провайдеров доступен в разделе [«Интеграция с IaaS»](../admin/integrations/public/overview.html).

- **На серверах bare metal (в том числе гибридные кластеры) или в неподдерживаемых облаках**. Установщик настраивает указанные в конфигурации серверы или виртуальные машины, разворачивает кластер Kubernetes и устанавливает DKP. Пошаговые инструкции по развертыванию на bare metal можно найти в разделе [«Быстрый старт» → «Deckhouse Kubernetes Platform на bare metal»]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/bm/step2.html).

- **В существующем кластере Kubernetes.** Установщик разворачивает DKP и интегрирует его с текущей инфраструктурой. Пошаговые инструкции по развертыванию в существующем кластере можно найти в разделе [«Быстрый старт» → «Deckhouse Kubernetes Platform в существующем кластере»]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/existing/step2.html).

## Требования к установке

Для оценки ресурсов, необходимых для установки Deckhouse Kubernetes Platform, вы можете ознакомиться со следующими руководствами:

- [Руководство по подбору ресурсов для кластера на bare metal](/products/kubernetes-platform/guides/hardware-requirements.html)
- [Руководство по разметке и объему дисков](/products/kubernetes-platform/guides/fs-requirements.html)
- [Руководство по подготовке к production](/products/kubernetes-platform/guides/production.html)

Перед установкой убедитесь в следующем:

- Для кластера на bare metal (в том числе гибридного кластера) и при установке в неподдерживаемых облаках: сервер использует операционную систему из [списка поддерживаемых ОС](../reference/supported_versions.html) или совместимую с ним, а также доступен по SSH через ключ.

- При настройке интеграции с поддерживаемыми облаками: имеются необходимые квоты для создания ресурсов и подготовлены параметры доступа к облачной инфраструктуре (зависят от конкретного провайдера).

- Есть доступ к хранилищу образов контейнеров Deckhouse (к публичному — `registry.deckhouse.io` или `registry.deckhouse.ru`, либо к зеркалу).

## Подготовка конфигурации

Перед началом установки необходимо подготовить [файл конфигурации установки](#файл-конфигурации-установки), а также, при необходимости, [post-bootstrap-скрипт](#post-bootstrap-скрипт).

### Файл конфигурации установки

Файл конфигурации установки состоит из YAML-секций (документов) и содержит настройки DKP, а также описание (манифесты) объектов и ресурсов кластера, которые будут созданы после установки. Файл конфигурации установки используется в CLI-установщике и передается с помощью параметра `--config` (см. далее).

Список обязательных и опциональных объектов и ресурсов кластера, которые могут понадобиться в файле конфигурации установки:

1. [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) (**обязательный**) — начальные [параметры конфигурации](../admin/configuration/), необходимые для запуска DKP.

   > Начиная с версии DKP 1.75, используйте ModuleConfig `deckhouse` для настройки доступа к хранилищу образов DKP. Настройка доступа с помощью InitConfiguration (параметры `imagesRepo`, `registryDockerCfg`, `registryScheme`, `registryCA`) считается устаревшим способом.

1. [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) — общие параметры кластера, такие как версия Kubernetes (компонентов control plane кластера), сетевые настройки, параметры CRI и т. д. Является **обязательным**, кроме случая, когда DKP устанавливается в уже существующий кластер Kubernetes.

1. [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration) — параметры кластера, развертываемого на серверах bare-metal (в том числе гибридного кластера) или виртуальных машинах в неподдерживаемых облаках. Является **обязательным**, кроме случая, когда DKP устанавливается в уже существующий кластер Kubernetes.

   Для добавления группы узлов (объект [NodeGroup](/modules/node-manager/cr.html#nodegroup)) под рабочую нагрузку в кластер могут понадобиться также объекты [StaticInstance](/modules/node-manager/cr.html#staticinstance) и [SSHCredentials](/modules/node-manager/cr.html#sshcredentials).

1. &lt;PROVIDER&gt;ClusterConfiguration — параметры интеграции с облачным провайдером. Является **обязательным** при интеграции DKP с [поддерживаемой облачной инфраструктурой](../admin/integrations/public/overview.html).

   Примеры ресурсов, настраивающих интеграцию DKP с облачным провайдером:

   * [AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) — Amazon Web Services;
   * [AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) — Microsoft Azure;
   * [DynamixClusterConfiguration](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration) — Базис.DynamiX;
   * [DVPClusterConfiguration](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration) — Deckhouse Virtualization Platform;
   * [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) — Google Cloud Platform;
   * [HuaweiCloudClusterConfiguration](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration) — Huawei Cloud;
   * [OpenStackClusterConfiguration](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration) — OpenStack, OVHcloud, Selectel, VK Cloud;
   * [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) — VMware vSphere;
   * [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) — VMware Cloud Director;
   * [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) — Yandex Cloud;
   * [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) — zVirt.

   Для добавления облачных узлов в кластер также понадобятся объекты &lt;PROVIDER&gt;InstanceClass (например [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) для Yandex Cloud), которые описывают конфигурацию виртуальных машин в группе узлов (объект [NodeGroup](/modules/node-manager/cr.html#nodegroup)).

1. Конфигурации модулей DKP.

   Каждый модуль настраивается (а также может быть включен или отключен) с помощью собственного объекта [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) с именем модуля (например, ModuleConfig `user-authn` для [модуля `user-authn`](/modules/user-authn/)). Допустимые параметры, которые можно указывать в объекте ModuleConfig, можно найти в документации соответствующего модуля в разделе «Настройки» (например, [настройки модуля `user-authn`](/modules/user-authn/configuration.html)).

   Список всех модулей Deckhouse Kubernetes Platform доступен в разделе [«Модули»](/modules/) документации.

   Некоторые модули могут быть включены и предварительно настроены автоматически, в зависимости от выбранного варианта установки и конфигурации кластера (например, модули, обеспечивающие работу control plane кластера и сети).

   Модули, часто настраиваемые при установке:

   * [`global`](/products/kubernetes-platform/documentation/v1/reference/api/global.html) — глобальные настройки DKP для указания параметров, которые используются по умолчанию всеми модулями и компонентами (шаблон DNS-имен, StorageClass, настройки расположения компонентов модулей и т.д.);
   * [`deckhouse`](/modules/deckhouse/configuration.html) — настройки доступа к хранилищу образов, желаемый канал обновлений и другие параметры;
   * [`user-authn`](/modules/user-authn/configuration.html) — отвечает за единую систему аутентификации;
   * [`cni-cilium`](/modules/cni-cilium/configuration.html) — отвечает за работу сети в кластере (например, используется при установке DKP на bare metal, в закрытом окружении, на РЕД-виртуализации и на SpaceVM).

   Если кластер изначально создается с узлами, выделенными для определенных типов нагрузки (например, системные узлы или узлы для мониторинга), рекомендуется в конфигурации модулей, использующих тома постоянного хранилища, явно задавать параметр `nodeSelector` (например, в [параметре `nodeSelector`](/modules/prometheus/configuration.html#parameters-nodeselector) ModuleConfig `prometheus` для модуля `prometheus`).

1. [IngressNginxController](/modules/ingress-nginx/cr.html#ingressnginxcontroller) — параметры создаваемого балансировщика HTTP/HTTPS-трафика (Ingress-контроллера).

1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — параметры группы узлов. Необходим для добавления узлов под рабочую нагрузку в кластер.

1. Объекты для настройки аутентификации и авторизации, такие как [ClusterAuthorizationRule](/modules/user-authz/cr.html#clusterauthorizationrule), [AuthorizationRule](/modules/user-authz/cr.html#authorizationrule), [User](/modules/user-authn/cr.html#user), [Group](/modules/user-authn/cr.html#group), [DexProvider](/modules/user-authn/cr.html#dexprovider).

   Читайте подробнее в документации о настройке [аутентификации](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authentication/) и [авторизации](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/).

{% offtopic title="Пример файла конфигурации установки..." %}

<div class="tabs">
  <a id='tab_variant_new_config'
     href="javascript:void(0)"
     class="tabs__btn tabs__btn_variant active"
     onclick="openTabAndSaveStatus(event,'tabs__btn_variant','tabs__content_variant','block_variant_new_config');">
     Конфигурация, применимая с версии 1.75 DKP
  </a>
  <a id='tab_variant_legacy_config'
     href="javascript:void(0)"
     class="tabs__btn tabs__btn_variant"
     onclick="openTabAndSaveStatus(event,'tabs__btn_variant','tabs__content_variant','block_variant_legacy_config');">
     Устаревший вариант конфигурации
  </a>
</div>

<div id='block_variant_new_config' class="tabs__content tabs__content_variant active" markdown="1">
В этом примере доступ к хранилищу образов DKP настраивается с помощью ModuleConfig `deckhouse`.

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
  name: deckhouse
spec:
  enabled: true
  settings:
    releaseChannel: Stable
    bundle: Default
    logLevel: Info
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: test-registry.io/some/path
        scheme: HTTPS
        username: <username>
        password: <password>
        ca: <CA>
  version: 1
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 2
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
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
  # Укажите в случае использования выделенных узлов для мониторинга.
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
```

</div>

<div id='block_variant_legacy_config' class="tabs__content tabs__content_variant" markdown="1">

В этом примере доступ к хранилищу образов DKP настраивается с помощью InitConfiguration.

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
  imagesRepo: registry.deckhouse.ru/deckhouse/ee
  registryDockerCfg: eyJhdXRocyI6IHsgInJlZ2zzzmRlY2tob3Vxxcxxxc5ydSI6IsssfX0K
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
  settings:
    modules:
      publicDomainTemplate: "%s.k8s.example.com"
  version: 2
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: user-authn
spec:
  version: 2
  enabled: true
  settings:
    controlPlaneConfigurator:
      dexCAMode: DoNotNeed
    publishAPI:
      enabled: true
      https:
        mode: Global
        global:
          kubeconfigGeneratorMasterCA: ""
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
  # Укажите в случае использования выделенных узлов для мониторинга.
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
```

</div>

{% endofftopic %}

### Post-bootstrap-скрипт

Установщик позволяет выполнить пользовательский скрипт на одном из master-узлов после завершения установки (post-bootstrap-скрипт). Такой скрипт может использоваться для:

* дополнительной настройки кластера;
* сбора диагностической информации;
* интеграции с внешними системами и других задач.

Указать путь к post-bootstrap-скрипту можно с помощью параметра `--post-bootstrap-script-path` при запуске CLI-установщика.

{% offtopic title="Пример скрипта, выводящего IP-адрес балансировщика..." %}
Пример скрипта, который выводит IP-адрес балансировщика после установки DKP:

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

## Установка

{% alert level="info" %}
При установке коммерческой редакции Deckhouse Kubernetes Platform из публичного хранилища образов `registry.deckhouse.ru` необходимо предварительно пройти аутентификацию с использованием лицензионного ключа:

```shell
docker login -u license-token registry.deckhouse.ru
```

{% endalert %}

Команда для запуска контейнера с установщиком из публичного хранилища образов контейнеров Deckhouse:

```shell
docker run --pull=always -it [<MOUNT_OPTIONS>] registry.deckhouse.ru/deckhouse/<DECKHOUSE_REVISION>/install:<RELEASE_CHANNEL> bash
```

Где:

1. `<DECKHOUSE_REVISION>` — [редакция DKP](../reference/revision-comparison.html). Например, `ee` — для Enterprise Edition, `ce` — для Community Edition и т. д.
1. `<MOUNT_OPTIONS>` — параметры монтирования файлов в контейнер установщика, таких как:
   - SSH-ключи доступа;
   - файл конфигурации;
   - файл ресурсов и т. д.
1. `<RELEASE_CHANNEL>` — [канал обновлений](/modules/deckhouse/configuration.html#parameters-releasechannel) в формате kebab-case:
   - `alpha` — для канала обновлений Alpha;
   - `beta` — для канала обновлений Beta;
   - `early-access` — для канала обновлений Early Access;
   - `stable` — для канала обновлений Stable;
   - `rock-solid` — для канала обновлений Rock Solid.

Пример команды для запуска контейнера с установщиком DKP Community Edition из канала обновлений Stable:

```shell
docker run -it --pull=always \
  -v "$PWD/config.yaml:/config.yaml" \
  -v "$PWD/dhctl-tmp:/tmp/dhctl" \
  -v "$HOME/.ssh/:/tmp/.ssh/" registry.deckhouse.ru/deckhouse/ce/install:stable bash
```

Установка DKP осуществляется в контейнере установщика с помощью команды `dhctl`:

* Для запуска установки DKP с развертыванием нового кластера (все случаи, кроме установки в существующий кластер) используйте команду `dhctl bootstrap`.
* Для установки DKP в уже существующий кластер используйте команду `dhctl bootstrap-phase install-deckhouse`.

{% alert level="info" %}
Для получения подробной справки по параметрам команды выполните `dhctl bootstrap -h`.
{% endalert %}

Пример запуска установки DKP с развертыванием кластера в облаке:

```shell
dhctl bootstrap \
  --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
  --config=/config.yml
```

Где:

- `/config.yml` — файл конфигурации установки;
- `<SSH_USER>` — имя пользователя для подключения по SSH к серверу;
- `--ssh-agent-private-keys` — файл приватного SSH-ключа для подключения по SSH.
- `<SSH_PRIVATE_KEY_FILE>` — имя приватного ключа. Например, для ключа с RSA-шифрованием это может быть `id_rsa`, а для ключа с ED25519-шифрованием — `id_ed25519`.

### Проверки перед началом установки

{% alert level="info" %}
Начиная с версии 1.74, в DKP встроен механизм контроля целостности модулей, который защищает их от подмены и изменения. Этот механизм включается автоматически при поддержке модуля ядра `erofs` операционной системой на узлах кластера. При отсутствии этой поддержки механизм контроля целостности модулей будет отключен, и в системе мониторинга появится соответствующий алерт.
{% endalert %}

{% offtopic title="Схема выполнения проверок, выполняемых установщиком перед началом установки..." %}
![Схема выполнения проверок, выполняемых установщиком перед началом установки Deckhouse Kubernetes Platform](../images/installing/preflight-checks.png)
{% endofftopic %}

Список проверок, выполняемых установщиком перед началом установки Deckhouse Kubernetes Platform:

1. Общие проверки:
   - Значения параметров [`publicDomainTemplate`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-publicdomaintemplate) и [`clusterDomain`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clusterdomain) не совпадают.
   - Данные аутентификации для хранилища образов, указанные в конфигурации установки, корректны.
   - Имя хоста соответствует следующим требованиям:
     - длина не более 63 символов;
     - состоит только из строчных букв;
     - не содержит спецсимволов (допускаются символы `-` (дефис) и `.` (точка), при этом они не могут быть в начале или в конце имени).
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
     - в случае с РЕД ОС — убедитесь, что установлены `yum` и `which` (по умолчанию могут отсутствовать);
     - **в случае использования `ContainerdV2`** в качестве container runtime по умолчанию на узлах кластера:
       - поддержка `CgroupsV2`;
       - systemd версии `244`;
       - поддержка модуля ядра `erofs`.
   - На сервере (ВМ) для master-узла установлен Python.
   - Хранилище образов доступно через прокси (если настройки прокси указаны в конфигурации установки).
   - На сервере (ВМ) для master-узла и в хосте, на котором запущен установщик, свободны порты, необходимые для процесса установки.
   - DNS должен разрешать `localhost` в IP-адрес `127.0.0.1`.
   - На сервере (ВМ) пользователю доступна команда `sudo`.
   - Открыты необходимые порты для установки:
     - между хостом запуска установщика и сервером — порт `22/TCP`;
     - отсутствуют конфликты по портам, которые используются процессом установки.
   - На сервере (ВМ) установлено корректное время.
   - Адресное пространство подов (`podSubnetCIDR`), сервисов (`serviceSubnetCIRD`) и внутренней сети кластера (`internalNetworkCIDRs`) не пересекаются.
   - На сервере (ВМ) отсутствует пользователь `deckhouse`.

1. Проверки для установки облачного кластера:
   - Конфигурация виртуальной машины master-узла удовлетворяет минимальным требованиям.
   - API облачного провайдера доступно с узлов кластера.
   - Проверка конфигурации [Yandex Cloud с NAT Instance](/modules/cloud-provider-yandex/layouts.html#withnatinstance).

{% offtopic title="Список флагов пропуска проверок..." %}

- `--preflight-skip-all-checks` — пропуск всех предварительных проверок;
- `--preflight-skip-ssh-forward-check` — пропуск проверки проброса SSH;
- `--preflight-skip-availability-ports-check` — пропуск проверки доступности необходимых портов;
- `--preflight-skip-resolving-localhost-check` — пропуск проверки разрешения `localhost`;
- `--preflight-skip-deckhouse-version-check` — пропуск проверки версии DKP;
- `--preflight-skip-registry-through-proxy` — пропуск проверки доступа к хранилищу образов через прокси-сервер;
- `--preflight-skip-public-domain-template-check` — пропуск проверки шаблона `publicDomain`;
- `--preflight-skip-ssh-credentials-check` — пропуск проверки учетных данных SSH-пользователя;
- `--preflight-skip-registry-credential` — пропуск проверки учетных данных для доступа к хранилищу образов;
- `--preflight-skip-containerd-exist` — пропуск проверки наличия containerd;
- `--preflight-skip-python-checks` — пропуск проверки наличия Python;
- `--preflight-skip-sudo-allowed` — пропуск проверки прав доступа для выполнения команды `sudo`;
- `--preflight-skip-system-requirements-check` — пропуск проверки соответствия системным требованиям;
- `--preflight-skip-one-ssh-host` — пропуск проверки количества указанных SSH-хостов;
- `--preflight-cloud-api-accesibility-check` — пропуск проверки доступности Cloud API;
- `--preflight-time-drift-check` — пропуск проверки отсутствия рассинхронизации времени (time drift);
- `--preflight-skip-cidr-intersection` — пропуск проверки пересечения CIDR;
- `--preflight-skip-deckhouse-user-check` — пропуск проверки наличия пользователя `deckhouse`;
- `--preflight-skip-yandex-with-nat-instance-check` — пропуск проверки конфигурации Yandex Cloud с WithNatInstance;
- `--preflight-skip-dvp-kubeconfig` — пропуск проверки DVP kubeconfig.
- `--preflight-skip-staticinstances-with-ssh-credentials` — пропуск проверки доступности StaticInstances с SSHCredentials.

Пример применения флага пропуска:

```shell
    dhctl bootstrap \
    --ssh-user=<SSH_USER> --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> \
    --config=/config.yml \
    --preflight-skip-all-checks
```

> Замените здесь `<SSH_PRIVATE_KEY_FILE>` на имя вашего приватного ключа. Например, для ключа с RSA-шифрованием это может быть `id_rsa`, а для ключа с ED25519-шифрованием — `id_ed25519`.

{% endofftopic %}

### Откат установки

Если установка была прервана или возникли проблемы во время установки в поддерживаемом облаке, то могут остаться ресурсы, созданные в процессе установки. Для их удаления выполните следующую команду в контейнере с установщиком:

```shell
dhctl bootstrap-phase abort
```

{% alert level="warning" %}
Файл конфигурации, передаваемый через параметр `--config` при запуске установщика, должен быть тем же, который использовался для первоначальной установки.
{% endalert %}

<div id="#закрытое-окружение-работа-через-proxy-и-сторонние-registries"></div>

## Закрытое окружение, работа через прокси-сервер и стороннее хранилище образов контейнеров

<div id="установка-deckhouse-kubernetes-platform-из-стороннего-registry"></div>

{% alert level="info" %}
Подробнее с установкой и обновлением DKP в закрытом окружении можно ознакомиться в руководствах [«Установка DKP в закрытом окружении»](/products/kubernetes-platform/guides/private-environment.html) и [«Обновление DKP в закрытом окружении»](/products/kubernetes-platform/guides/airgapped-update.html).
{% endalert %}

### Установка из стороннего хранилища образов контейнеров

{% alert level="warning" %}
Доступно в следующих редакциях: SE, SE+, EE, CSE Lite, CSE Pro.
{% endalert %}

DKP можно установить из стороннего хранилища образов или через проксирующий сервер внутри закрытого контура.

{% alert level="warning" %}
DKP поддерживает аутентификацию в хранилище образов только по схеме Bearer token.

Протестирована и гарантируется работа со следующими хранилищами образов:
{%- for registry in site.data.supported_versions.registries %}
[{{- registry[1].shortname }}]({{- registry[1].url }})
{%- unless forloop.last %}, {% endunless %}
{%- endfor %}.

При работе со сторонним хранилищем образов не используйте учетную запись администратора для доступа к нему со стороны DKP. Используйте отдельную учетную запись с правами только на чтение и только в пределах нужного раздела в хранилище образов. Ознакомьтесь с [примером создания](#особенности-настройки-nexus) такой учетной записи.
{% endalert %}

Варианты настройки работы со сторонними хранилищами образов при установке кластера:

- начиная с версии DKP 1.75 — с помощью ModuleConfig `deckhouse`;
- до версии DKP 1.75 — с помощью InitConfiguration (устаревший способ, пример приведен ниже).

Для настройки с помощью ModuleConfig `deckhouse` укажите параметры доступа к стороннему хранилищу образов в [секции `settings.registry`](/modules/deckhouse/configuration.html#parameters-registry).

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    registry:
      mode: Direct
      direct:
        imagesRepo: test-registry.io/some/path
        scheme: HTTPS
        username: <username>
        password: <password>
        ca: <CA>
```

{% offtopic title="Настройка работы со сторонним хранилищем образов через InitConfiguration **(устаревший способ)**" %}

Установите следующие параметры в InitConfiguration:

* `imagesRepo: <PROXY_REGISTRY>/<DECKHOUSE_REPO_PATH>/ee` — адрес образа DKP EE в стороннем хранилище образов. Пример: `imagesRepo: registry.deckhouse.ru/deckhouse/ee`;
* `registryDockerCfg: <BASE64>` — права доступа к стороннему хранилищу образов, зашифрованные в Base64.

Если разрешен анонимный доступ к образам DKP в стороннем хранилище образов, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {}}}
```

Приведенное значение должно быть закодировано в Base64.

Если для доступа к образам DKP в стороннем хранилище образов необходима аутентификация, `registryDockerCfg` должен выглядеть следующим образом:

```json
{"auths": { "<PROXY_REGISTRY>": {"username":"<PROXY_USERNAME>","password":"<PROXY_PASSWORD>","auth":"<AUTH_BASE64>"}}}
```

где:

* `<PROXY_USERNAME>` — имя пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_PASSWORD>` — пароль пользователя для аутентификации на `<PROXY_REGISTRY>`;
* `<PROXY_REGISTRY>` — адрес стороннего хранилища образов в виде `<HOSTNAME>[:PORT]`;
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

Для настройки нестандартных конфигураций сторонних хранилищ образов в InitConfiguration предусмотрены еще два параметра:

* `registryCA` — корневой сертификат, которым можно проверить сертификат хранилища образов (если хранилище образов использует самоподписанные сертификаты);
* `registryScheme` — протокол доступа к хранилищу образов (`HTTP` или `HTTPS`). По умолчанию — `HTTPS`.
{% endofftopic %}

<div markdown="0" style="height: 0;" id="особенности-настройки-сторонних-registry"></div>

### Особенности настройки Nexus

{% alert level="warning" %}
При взаимодействии с репозиторием типа `docker`, расположенным в Nexus (например, при выполнении команд `docker pull`, `docker push`), требуется указывать адрес в формате `<NEXUS_URL>:<REPOSITORY_PORT>/<PATH>`.

Использование значения `URL` из параметров репозитория Nexus **недопустимо**.
{% endalert %}

При использовании менеджера репозиториев [Nexus](https://github.com/sonatype/nexus-public) должны быть выполнены следующие требования:

* Создан **проксирующий** репозиторий Docker («Administration» → «Repository» → «Repositories»):
  * установлен в `0` параметр `Maximum metadata age` для репозитория.
* Настроен контроль доступа:
  * создана роль **Nexus** («Administration» → «Security» → «Roles») со следующими полномочиями:
    * `nx-repository-view-docker-<репозиторий>-browse`;
    * `nx-repository-view-docker-<репозиторий>-read`;
  * создан пользователь («Administration» → «Security» → «Users») с ролью **Nexus**.

Чтобы настроить Nexus, выполните следующие шаги:

1. Создайте проксирующий репозиторий Docker («Administration» → «Repository» → «Repositories»), указывающий на [публичное хранилище образов Deckhouse](https://registry.deckhouse.ru/).
   ![Создание проксирующего репозитория Docker](../images/registry/nexus/nexus-repository.png)

1. Заполните поля страницы создания репозитория следующим образом:
   * `Name` должно содержать имя создаваемого репозитория, например, `d8-proxy`.
   * `Repository Connectors / HTTP` или `Repository Connectors / HTTPS` должно содержать выделенный порт для создаваемого репозитория, например, `8123` или иной.
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

   * Включите **Docker Bearer Token Realm** («Administration» → «Security» → «Realms»):
     * **Docker Bearer Token Realm** должен быть в списке **Active** (справа), а не в **Available** (слева).
     * Если его нет в **Active**:
       1. Найдите в списке **Available**.
       1. Переместите стрелкой в **Active**.
       1. Нажмите **Save**.
       1. **Перезапустите Nexus** (это обязательно, для применения изменений).

     ![Настройка Docker Bearer Token Realm](../images/registry/nexus/nexus-realms.png)

В результате образы DKP будут доступны, например, по следующему адресу: `https://<NEXUS_HOST>:<REPOSITORY_PORT>/deckhouse/ee:<d8s-version>`.

### Особенности настройки Harbor

Используйте функцию [Harbor Proxy Cache](https://github.com/goharbor/harbor).

1. Настройте доступ к хранилищу образов:
   * в боковом меню перейдите в раздел «Administration» → «Registries»
     и нажмите «New Endpoint», чтобы добавить эндпоинт для хранилища образов;
   * в выпадающем списке «Provider» выберите «Docker Registry»;
   * в поле «Name» укажите имя эндпоинта на свое усмотрение;
   * в поле «Endpoint URL» укажите `https://registry.deckhouse.ru`;
   * в поле «Access ID» укажите `license-token`;
   * в поле «Access Secret» укажите свой лицензионный ключ Deckhouse Kubernetes Platform;
   * задайте остальные параметры по своему усмотрению;
   * нажмите «ОК», чтобы подтвердить создание эндпоинта для хранилища образов.

   ![Настройка доступа к хранилищу образов](../images/registry/harbor/harbor1.png)

1. Создайте новый проект:
   * в боковом меню перейдите в раздел «Projects» и нажмите «New Project», чтобы добавить проект;
   * в поле «Project Name» укажите любое имя проекта на свое усмотрение (например, `d8s`).
     Указанное имя будет частью URL-адреса;
   * в поле «Access Level» выберите «Public»;
   * включите «Proxy Cache» и в выпадающем списке выберите хранилище образов, созданное ранее;
   * задайте остальные параметры по своему усмотрению;
   * нажмите «ОК», чтобы подтвердить создание проекта.

   ![Создание нового проекта](../images/registry/harbor/harbor2.png)

После настройки Harbor образы DKP станут доступны по адресу следующего вида: `https://your-harbor.com/d8s/deckhouse/ee:{d8s-version}`.

### Ручная загрузка образов DKP и БД уязвимостей в приватное хранилище образов контейнеров

{% alert level="warning" %}
Утилита `d8 mirror` недоступна для использования с редакциями Community Edition (CE) и Basic Edition (BE).
{% endalert %}

{% alert level="info" %}
О текущем статусе версий на каналах обновлений можно узнать на [releases.deckhouse.ru](https://releases.deckhouse.ru).
{% endalert %}

- [Скачайте и установите утилиту Deckhouse CLI](../cli/d8/).

- Скачайте образы DKP в выделенную директорию, используя команду `d8 mirror pull`.

  По умолчанию `d8 mirror pull` скачивает только актуальные версии DKP, базы данных сканера уязвимостей (если они входят в редакцию DKP) и официально поставляемых модулей.
  Например, для Deckhouse Kubernetes Platform 1.59 будет скачана только версия 1.59.12, т. к. этого достаточно для обновления платформы с 1.58 до 1.59.

  Выполните следующую команду (укажите код редакции и лицензионный ключ), чтобы скачать образы актуальных версий:

  ```shell
  d8 mirror pull \
    --source='registry.deckhouse.ru/deckhouse/<EDITION>' \
    --license='<LICENSE_KEY>' /home/user/d8-bundle
  ```

  где:

  - `--source` — адрес хранилища образов Deckhouse;
  - `<EDITION>` — код редакции Deckhouse Kubernetes Platform (например, `ee`, `se`, `se-plus`). По умолчанию параметр `--source` ссылается на редакцию Enterprise Edition (`ee`) и может быть опущен;
  - `--license` — параметр для указания лицензионного ключа Deckhouse Kubernetes Platform для аутентификации в официальном хранилище образов;
  - `<LICENSE_KEY>` — лицензионный ключ Deckhouse Kubernetes Platform;
  - `/home/user/d8-bundle` — директория, в которой будут расположены пакеты образов. Будет создана, если не существует.

  > Если загрузка образов будет прервана, повторный вызов команды продолжит загрузку, если с момента ее остановки прошло не более суток.

  Пример команды для загрузки всех версий DKP EE, начиная с версии 1.59 (укажите лицензионный ключ):

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

  Пример команды для загрузки образов DKP из стороннего хранилища образов:

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

{% offtopic title="Другие параметры команды, доступные для использования:" %}

- `--no-pull-resume` — принудительно начать загрузку сначала;
- `--no-platform` — пропустить загрузку пакета образов Deckhouse Kubernetes Platform (`platform.tar`);
- `--no-modules` — пропустить загрузку пакетов модулей (`module-*.tar`);
- `--no-security-db` — пропустить загрузку пакета баз данных сканера уязвимостей (`security.tar`);
- `--include-module` / `-i` = `name[@Major.Minor]` — загрузить определенный набор модулей по принципу белого списка (и, при необходимости, их минимальных версий). Укажите несколько раз, чтобы добавить в белый список больше модулей. Эти флаги игнорируются, если используются совместно с `--no-modules`.

  Поддерживаются следующие синтаксисы для указания версий модулей:
  - `module-name@1.3.0` — загрузка версий с semver ^ ограничением (^1.3.0), включая v1.3.0, v1.3.3, v1.4.1;
  - `module-name@~1.3.0` — загрузка версий с semver ~ ограничением (>=1.3.0 <1.4.0), включая только v1.3.0, v1.3.3;
  - `module-name@=v1.3.0` — загрузка точного соответствия тегу v1.3.0, публикация во все каналы релизов;
  - `module-name@=bobV1` — загрузка точного соответствия тегу "bobV1", публикация во все каналы релизов.
- `--exclude-module` / `-e` = `name` — пропустить загрузку определенного набора модулей по принципу черного списка. Укажите несколько раз, чтобы добавить в черный список больше модулей. Игнорируется, если используются `--no-modules` или `--include-module`.
- `--modules-path-suffix` — изменить суффикс пути к репозиторию модулей в основном репозитории DKP. По умолчанию используется суффикс `/modules` (так, например, полный путь к репозиторию с модулями будет выглядеть как `registry.deckhouse.ru/deckhouse/EDITION/modules`);
- `--since-version=X.Y` — скачать все версии DKP, начиная с указанной минорной версии. Параметр будет проигнорирован, если указанная версия выше, чем версия на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--deckhouse-tag`;
- `--deckhouse-tag` — скачать только конкретную версию DKP (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--since-version`;
- `--gost-digest` — рассчитать контрольную сумму итогового набора образов DKP в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с TAR-архивом, содержащим образы DKP;
- `--source-login` и `--source-password` — данные для аутентификации в стороннем хранилище образов;
- `--images-bundle-chunk-size=N` — максимальный размер файлов (в ГБ), на которые нужно разбить архив образов. В результате работы вместо одного файла архива образов будет создан набор CHUNK-файлов (например, `d8.tar.NNNN.chunk`). Чтобы загрузить образы из такого набора файлов, укажите в команде `d8 mirror push` имя файла без суффикса `.NNNN.chunk` (например, `d8.tar` для файлов `d8.tar.NNNN.chunk`);
- `--tmp-dir` — путь к директории для временных файлов, который будет использоваться во время операций загрузки и выгрузки образов. Вся обработка выполняется в этом каталоге. Он должен иметь достаточный объем свободного дискового пространства, чтобы вместить весь загружаемый пакет образов. По умолчанию используется поддиректория `.tmp` в директории с пакетами образов.

Дополнительные параметры конфигурации для семейства команд `d8 mirror` доступны в виде переменных окружения:

- `HTTP_PROXY`/`HTTPS_PROXY` — URL-адрес прокси-сервера для запросов к HTTP(S)-хостам, которые не указаны в списке хостов в переменной `$NO_PROXY`;
- `NO_PROXY` — список хостов, разделенных запятыми, которые следует исключить из проксирования. Каждое значение может быть представлено в виде IP-адреса (`1.2.3.4`), CIDR (`1.2.3.4/8`), домена или символа (`*`). IP-адреса и домены также могут включать номер порта (`1.2.3.4:80`). Доменное имя соответствует как самому себе, так и всем поддоменам. Доменное имя, начинающееся с `.`, соответствует только поддоменам. Например, `foo.com` соответствует `foo.com` и `bar.foo.com`; `.y.com` соответствует `x.y.com`, но не соответствует `y.com`. Символ `*` отключает проксирование;
- `SSL_CERT_FILE` — путь до SSL-сертификата. Если переменная установлена, системные сертификаты не используются;
- `SSL_CERT_DIR` — список каталогов, разделенный двоеточиями. Определяет, в каких каталогах искать файлы SSL-сертификатов. Если переменная установлена, системные сертификаты не используются. [Подробнее...](https://www.openssl.org/docs/man1.0.2/man1/c_rehash.html)
- `MIRROR_BYPASS_ACCESS_CHECKS` — установите для этого параметра значение `1`, чтобы отключить проверку корректности переданных учетных данных для хранилища образов.
{% endofftopic %}

- На хост с доступом к хранилищу образов, куда нужно загрузить образы DKP, скопируйте загруженный пакет образов DKP и установите [Deckhouse CLI](../cli/d8/).

- Загрузите образы DKP в хранилище образов с помощью команды `d8 mirror push`.

  Команда `d8 mirror push` загружает в хранилище образов образы из всех пакетов, которые присутствуют в переданной директории.
  При необходимости выгрузить в хранилище образов только часть пакетов вы можете либо выполнить команду для каждого необходимого пакета образов, передав ей прямой путь до TAR-пакета вместо директории, либо убрав расширение `.tar` у ненужных пакетов или переместив их вне директории.

  Пример команды для загрузки пакетов образов из директории `/mnt/MEDIA/d8-images` (укажите данные для авторизации при необходимости):

  ```shell
  d8 mirror push /mnt/MEDIA/d8-images 'corp.company.com:5000/sys/deckhouse' \
    --registry-login='<USER>' --registry-password='<PASSWORD>'
  ```

  Перед загрузкой образов убедитесь, что путь для загрузки в хранилище образов существует (в примере — `/sys/deckhouse`) и у используемой учетной записи есть права на запись.

  Если вы используете Harbor, вы не сможете выгрузить образы в корень проекта. Используйте выделенный репозиторий в проекте для размещения образов DKP.

- После загрузки образов в хранилище образов можно переходить к установке DKP. Воспользуйтесь [руководством по быстрому старту](/products/kubernetes-platform/gs/bm-private/step2.html).

  При запуске установщика используйте хранилище образов, в которое ранее были загружены образы, а не официальное публичное хранилище образов DKP. Для примера выше адрес запуска установщика будет иметь вид `corp.company.com:5000/sys/deckhouse/install:stable` вместо `registry.deckhouse.ru/deckhouse/ee/install:stable`.

  В [секции параметров `registry`](/modules/deckhouse/configuration.html#parameters-registry) ModuleConfig `deckhouse` при установке также используйте адрес вашего хранилища образов и данные авторизации (с версии DKP 1.75). Устаревший способ — использование [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration) (параметры [`imagesRepo`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), [`registryDockerCfg`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-registrydockercfg)).

### Создание кластера и запуск DKP без использования каналов обновлений

{% alert level="warning" %}
Этот способ следует использовать только в случае, если в приватном хранилище нет образов, содержащих информацию о каналах обновлений.
{% endalert %}

Если необходимо установить DKP с отключенным автоматическим обновлением:

1. Используйте тег образа установщика соответствующей версии. Например, если вы хотите установить релиз `v1.44.3`, используйте образ `your.private.registry.com/deckhouse/install:v1.44.3`.
1. Укажите соответствующий номер версии в [параметре `deckhouse.devBranch`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-devbranch) в [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).
   > **Не указывайте** [параметр `deckhouse.releaseChannel`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#module-v1alpha1-properties-releasechannel) в [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration).

Если вы хотите отключить автоматические обновления для уже установленного DKP (включая обновления patch-релизов), удалите [параметр `releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel) из конфигурации модуля `deckhouse`.

### Использование прокси-сервера

{% alert level="warning" %}
Доступно в следующих редакциях: BE, SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).
{% endalert %}

{% offtopic title="Пример шагов по настройке прокси-сервера на базе Squid..." %}

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

1. Создайте пользователя и пароль для аутентификации на прокси-сервере:

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

Для настройки DKP на работу с прокси-сервером используйте [параметр `proxy`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-proxy) ресурса ClusterConfiguration.

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

### Автозагрузка прокси-переменных пользователям в CLI

Начиная с версии 1.67, в DKP больше не настраивается файл `/etc/profile.d/d8-system-proxy.sh`, который ранее устанавливал прокси-переменные для пользователей. Для автозагрузки прокси-переменных пользователям в CLI используйте [ресурс NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

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
