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

Попробуйте также [графический установщик Deckhouse Kubernetes Platform]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/installer/).
{% endalert %}

На этой странице представлена обзорная информация по установке Deckhouse Kubernetes Platform (DKP).

{% alert level="info" %}
Администрирование платформы подробно разобрано в курсе [«Администрирование Deckhouse Kubernetes Platform»](https://deckhouse.ru/courses/basics-administration-deckhouse-kubernetes-platform/) в [Deckhouse Академии](https://deckhouse.ru/academy/).
{% endalert %}

## Способы установки

Установить DKP можно следующими способами:

- с помощью CLI-установщика (доступен в виде образа контейнера и основан на утилите [dhctl](<https://github.com{{ site.github_repo_path }}/tree/main/dhctl/>));
- с помощью [графического установщика]({% if site.mode == 'module' %}{{ site.urls[page.lang] }}{% endif %}/products/kubernetes-platform/gs/installer/).

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

- При наличии ограничений сетевого взаимодействия на уровне инфраструктуры соблюдены требования, описанные в разделе [«Сетевое взаимодействие компонентов платформы»](../reference/network_interaction.html).

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
   * [DynamixClusterConfiguration](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration) — Basis Dynamix;
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

{% tabs variant %}
{% tab "Конфигурация, применимая с версии 1.75 DKP" %}
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
  settings: {}
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

{% endtab %}
{% tab "Устаревший вариант конфигурации" %}
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
  settings: {}
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

{% endtab %}
{% endtabs %}

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

### Установка статического кластера на машинах с неизменяемой ОС

Группа узлов ([NodeGroup](/modules/node-manager/cr.html#nodegroup)) с параметром `systemType: Immutable` описывает машины с неизменяемой операционной системой: на них нет доступа по SSH и нет bashible, поэтому установщик не создаёт их и не настраивает по SSH. Установка проходит так:

1. Вы записываете образ ОС на диск каждой машины и включаете машины. Машина, у которой ещё нет конфигурации, ожидает её на порту `50000/TCP`.
1. Установщик передаёт каждой машине её конфигурацию через этот порт. Всё остальное — разметку дисков, kubelet, компоненты control plane — машина настраивает сама по этому документу.
1. Машина, указанная первой, создаёт кластер. Остальным конфигурация передаётся после установки DKP, и они присоединяются по одной, поскольку etcd принимает новых участников по одному.

{% alert level="warning" %}
Сеть, в которой идёт установка, должна быть изолированной. Эндпоинт на порту `50000/TCP` никого не проверяет: у машины без конфигурации ещё нет секрета, которым она могла бы подтвердить себя. Любой, кто может обратиться к этому порту, установит машину как узел чужого кластера. Инвентарь, который машина отдаёт на том же порту, тоже открыт без проверки подлинности: тот, кто до него дотянется, увидит диски машины, их серийные номера и её интерфейсы. Держите машины в отдельном сегменте сети до окончания установки.
{% endalert %}

Машина, ожидающая конфигурацию, сама рассказывает о себе на том же порту — гадать, что в ней стоит, не нужно. Одни и те же сведения она отдаёт в трёх видах, каждый по своему адресу:

```shell
curl http://<адрес>:50000/inventory          # документ, который предстоит заполнить
curl http://<адрес>:50000/inventory.pretty   # таблица, чтобы прочитать глазами
curl http://<адрес>:50000/inventory.json     # всё целиком, чтобы разобрать скриптом
```

`/inventory` отвечает в YAML, в форме того самого `NodeConfig`, который машина ждёт, — чтобы вы объединили ответ со своим документом, а не перепечатывали его. Каждый интерфейс, у которого есть адрес, уже заполнен: со статическими адресами, если они статические, и с `dhcp: true` и текущей арендой в комментарии, если нет. Каждый диск приведён отдельным блоком `spec.storage`, и все такие блоки закомментированы: в каждом — `diskSelector`, который называет только этот диск, и точка монтирования `kubernetes-data`. Вам остаётся раскомментировать блок того диска, на который ставится узел. Если машина ничем не выделяет диск, вместо селектора так и написано.

`/inventory.pretty` — выровненная таблица: её читают глазами, а не объединяют с документом. По каждому диску она даёт имя в ядре, размер, состояние (`blank`, `formatted` или `system-layout`), ссылку в `/dev/disk/by-path`, модель, тип подключения, серийный номер, `wwid` и вращается ли диск, затем все селекторы, которые называют этот диск, с пометкой, скольким дискам подходит каждый, и разделы с их файловыми системами и метками. Для диска, который выделяется только именем в ядре, добавляется отдельная предупреждающая строка: это имя меняется между загрузками.

`/inventory.json` — те же сведения в JSON, и именно его читает сам установщик; сверх таблицы он отдаёт производителя, все ссылки `/dev/disk/by-id` и путь на шине. Таблица и JSON дают интерфейсам ещё и MAC-адреса и состояние линка, а `/inventory` называет интерфейсы и их адреса — это всё, что принимает `NodeConfig`. Вот как выглядит таблица:

```text
Disks:
  sda    30G  blank          pci-0000:0d:00.0-scsi-0:0:0:0
        QEMU HARDDISK · scsi · serial S3Z8NB0K700001
        diskSelector:
          serial: "S3Z8NB0K700001"
        diskSelector:
          busPath: "pci-0000:0d:00.0-scsi-0:0:0:0"
        diskSelector:
          model: "QEMU HARDDISK"   # matches 2 disks
        diskSelector:
          size: "=32212254720"
        diskSelector:
          name: "sda"   # kernel name, changes between boots
  sdb     8G  system-layout  pci-0000:0d:00.0-scsi-0:0:0:1
        QEMU HARDDISK · scsi · serial S3Z8NB0K700002
        diskSelector:
          serial: "S3Z8NB0K700002"
        diskSelector:
          busPath: "pci-0000:0d:00.0-scsi-0:0:0:1"
        diskSelector:
          model: "QEMU HARDDISK"   # matches 2 disks
        diskSelector:
          size: "=8589934592"
        diskSelector:
          name: "sdb"   # kernel name, changes between boots
        sdb1     1G vfat   "BOOT"
        sdb2   256M ext4   "CONFIG"
        sdb3   6.8G ext4   "DATA"
Interfaces:
  eth0 f2:4e:c6:60:03:72 up   192.168.199.11/24 gw 192.168.199.1 (dhcp)
```

Все три адреса отвечают, только пока машина ждёт конфигурацию: как только она её примет, сервер, который их обслуживал, закрывается.

Кроме [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) с `clusterType: Static` и [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration), файл конфигурации установки должен содержать:

- NodeGroup с именем `master`, с параметрами `nodeType: Static` и `systemType: Immutable`. Именно он сообщает установщику, что машины неизменяемые.
- По одному документу `NodeConfig` на каждую машину — с тем, чего кластер о ней знать не может.

Доступ к хранилищу образов должен быть настроен в режиме `Unmanaged`: неизменяемый узел забирает образы control plane и системные расширения из хранилища напрямую, без кластерного прокси, которого во время установки ещё нет.

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "Automatic"
clusterDomain: cluster.local
---
apiVersion: deckhouse.io/v1
kind: StaticClusterConfiguration
internalNetworkCIDRs:
- 192.168.199.0/24
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    releaseChannel: Stable
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: registry.deckhouse.ru/deckhouse/ee
        scheme: HTTPS
        username: license-token
        password: <LICENSE_KEY>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: Static
  systemType: Immutable
---
apiVersion: internal.deckhouse.io/v1alpha1
kind: NodeConfig
metadata:
  name: master-0
spec:
  # Машина с одним диском, поэтому spec.storage не задан; см. примечание ниже.
  network:
    interfaces:
    - name: eth0
      dhcp: false
      addresses:
      - 192.168.199.11/24
      gateway: 192.168.199.1
    dns:
      servers:
      - 192.168.199.1
  kubelet:
    nodeIP: 192.168.199.11
```

Для каждой машины нужен свой документ: `master-1` и `master-2` отличаются от приведённого только именем и адресами.

Документ `NodeConfig` задаёт только те три вещи о машине, которых кластер знать не может:

- `spec.network` — интерфейсы, DNS-серверы и статические маршруты;
- `spec.storage` — на какой диск устанавливается ОС (`diskSelector` или `device`) и дополнительные точки монтирования;
- `spec.kubelet.nodeIP` — адрес, с которым узел регистрируется.

Машине, которая берёт адрес по DHCP, раздел `spec.network` не нужен вовсе — как бы ни назывался её сетевой интерфейс: установщик по умолчанию подставляет `eth0` с `dhcp: true`, а это догадка, а не знание о машине, и DHCP машина поднимет на том интерфейсе, который у неё есть. Дело ограничится одним предупреждением о том, что угаданного имени на машине нет. Со статическим адресом наоборот: называйте интерфейс, который на машине действительно есть (`enp3s0`, `eno1` или `ens18` встречаются куда чаще, чем `eth0`), — адрес, повешенный на отсутствующее имя, отклоняется ещё до передачи конфигурации, и в отказе перечислены интерфейсы самой машины. Где это имя посмотреть, отвечает инвентарь.

Любое другое поле будет отклонено с объяснением: больше ничего из этого документа установщик в полезную нагрузку не кладёт, а остальные настройки узла берутся из его NodeGroup. Параметр `spec.storage.wipe` не принимается вовсе — на машине с одним диском очистка диска уничтожает установочный носитель, с которого копируется ОС.

Не задавать `spec.storage` можно только на машине с одним диском. Без него для системного диска подставляется `diskSelector` с `size: ">=20Gi"`, а для точки монтирования etcd — `partitionSelector`, под который подходит пустой диск от 10 ГБ.

При этом `size: ">=20Gi"` — не выбор диска, а фильтр: если под него подходит больше одного диска, ничего установлено не будет. Прежде чем что-либо передать машине, установщик читает её инвентарь и отклоняет конфигурацию, которую машина выполнить не может: несуществующий диск, селектор, под который подходит несколько дисков, интерфейс, которого у машины нет. В отказе названо поле, перечислено то, что на машине есть на самом деле, и напечатана строка, которую следует написать вместо текущей; установка на этом прекращается, а не уходит в повторные попытки — сколько ни жди, лишний диск у машины не появится.

Если проверку выполнить нельзя, установщик предупреждает и всё равно передаёт конфигурацию: машина, чей образ слишком стар, чтобы отдавать инвентарь, или машина, которая вовсе не отвечает, не должна ронять установку. Тогда отказ остаётся за самой машиной: она ничего не устанавливает, печатает подошедшие диски вместе со своим инвентарём и уходит в аварийную оболочку на консоли — прочитать это сообщение больше негде, потому что машина, ещё не ставшая узлом, никуда не отправляет журналы. Пока не было ни того ни другого отказа, молча брался первый подошедший диск: именно так однажды ОС записалась на диск с данными, узел зарегистрировался как обычно, а отказ проявился только при следующей загрузке.

Поэтому на машине с несколькими дисками называйте диск явно. `spec.storage.diskSelector` сопоставляет `serial`, `wwid`, `busPath`, `name` и `model` — все пять как шаблоны в стиле командной оболочки — а также `size`, `type` (`nvme`, `ssd`, `hdd`, `sd`) и `rotational`; совпасть должны все заданные поля. Вместо селектора диск можно назвать путём в `spec.storage.device` — берите устойчивый, `/dev/disk/by-path/…` или `/dev/disk/by-id/…`, потому что `/dev/sda` выдаётся в том порядке, в каком ядро нашло диски, и между загрузками может измениться. Если заданы оба, машина читает `diskSelector` и не смотрит на `device`.

Две особенности, о которых стоит знать, когда пишете селектор. Поле `busPath` сопоставляется с обеими формами, в которых машина сообщает путь: с путём на шине, каким его даёт ядро, и с именем ссылки в `/dev/disk/by-path`. А на гипервизоре у дисков часто нет ни собственного серийного номера, ни `wwid` — для диска `virtio-scsi` ядро не публикует ни того, ни другого, — и машина читает оба из ссылок в `/dev/disk/by-id`. По чему именно эту машину вообще можно выбрать, отвечает её инвентарь; поэтому его стоит прочитать до того, как писать документ.

`spec.storage.diskSelector` задаёт только диск под ОС. Подставленная точка монтирования etcd проверяется точно так же, поэтому машина, на которой под неё подходит больше одного устройства — например, второй пустой диск от 10 ГБ помимо системного, — тоже будет отклонена, с перечислением этих устройств. Назовите и эту точку монтирования: добавьте в `spec.storage.mounts` запись с именем `kubernetes-data` и своим `partitionSelector` или `device`. Поля `bindTo` и `mode` в этой записи задавать нельзя — с ними установщик отклонит документ: `/var/lib/etcd` статический под etcd держит как hostPath, поэтому собственный `bindTo` оставил бы узел без работающего control plane.

{% alert level="warning" %}
Не копируйте `NodeConfig` из работающего кластера: вывод `kubectl get nodeconfig <имя> -o yaml` не разбирается. Установщик читает эти документы строго и отклоняет поля, которые добавляет к ним API-сервер (`creationTimestamp`, `uid`, `resourceVersion`, `status`). Напишите минимальный документ, как в примере выше.
{% endalert %}

Пример команды установки:

```shell
dhctl bootstrap \
  --config=/config.yml \
  --master-host master-0=192.168.199.11 \
  --master-host master-1=192.168.199.12 \
  --master-host master-2=192.168.199.13
```

Где:

- `--master-host` — машина control plane в виде `<имя-узла>=<адрес>`, указывается по одному разу на машину. Имя узла должно совпадать с `metadata.name` документа `NodeConfig` этой машины; с этим именем узел и регистрируется. Машина, указанная первой, создаёт кластер.
- `--ssh-user` и `--ssh-host` не используются: машины не отвечают по SSH. Указанный `--ssh-host` (или ресурс `SSHHost` в `--connection-config`) отклоняется до начала установки: включаемые им проверки полезут по SSH на машину, где нет sshd.

Адрес в `--master-host` — это адрес, на котором машина ждёт свою конфигурацию; он вполне может быть получен по DHCP. Если `NodeConfig` первого мастера назначает интерфейсу статический адрес, установщик после установки переходит на этот адрес — на `spec.kubelet.nodeIP`, если документ его задаёт, иначе на первый статический адрес из документа: bootstrap-канал и API-сервер он ищет уже там, а не по адресу, на который передавал конфигурацию. К адресам в `spec.network` применяются два правила:

- У адреса должна быть длина префикса, в виде `192.168.0.101/24`. Без неё документ отклоняется: машина настроила бы интерфейс на единственный хост и оказалась бы вне собственной подсети, никак об этом не сообщив.
- Адреса на интерфейсе с `dhcp: true` машина игнорирует — адрес она берёт из аренды DHCP. Установщик их не отклоняет, а предупреждает о них: чтобы они применились, напишите `dhcp: false`.

При повторном запуске установщик пропускает машины, которым уже передал конфигурацию, — машина, ставшая узлом, вторую конфигурацию не примет. Эта отметка хранится по файлу на машину: `immutable-control-plane-pushed-payload-<имя-узла>` в каталоге состояния, который установщик печатает при старте (`State cache directory: …`; это подкаталог с хешем в имени внутри `--cache-dir`, по умолчанию `/tmp/dhctl`). Если между попытками переустановили одну машину, удалите файл только этой машины. Повторный запуск на кластере, где DKP уже установлен, больше не останавливается на пространстве имён `d8-system`: установщик видит его на месте и оставляет как есть.

{% alert level="warning" %}
Не используйте для этого `--yes-i-want-to-drop-cache`: он стирает это состояние целиком, вместе с отметками остальных машин и путём к учётным данным, собранным с первого мастера. Первому мастеру уже сообщили, что учётные данные сохранены, и он закрыл свой bootstrap-канал — собрать их второй раз нельзя. Сбрасывать состояние целиком уместно, только если заново установлены все машины.
{% endalert %}

Рабочие узлы установщик пока не создаёт. Чтобы добавить такой узел, подготовьте его `NodeConfig` из файла `/config/nodeconfig.yaml` уже работающего узла: измените `metadata.name`, `spec.nodeName` и `spec.network`, укажите группу узла и в `metadata.labels[node.deckhouse.io/group]`, и в `spec.kubelet.nodeLabels[node.deckhouse.io/group]` (в документе, скопированном с мастера, в обоих местах стоит `master`), удалите точку монтирования `kubernetes-data` для control plane и секцию `status`, а в `spec.kubelet.bootstrapToken` подставьте свежий `<token-id>.<token-secret>` из секрета `bootstrap-token-*` пространства имён `kube-system` (такой токен живёт недолго). Затем передайте документ ожидающей машине:

```shell
curl -X PUT --data-binary @nodeconfig.yaml http://<адрес>:50000/config
```

И установщик, и написанный руками `curl` передают документ по пути `/config` и ни по какому другому. Если машина отвечает на нём `404`, значит на ней не тот образ ОС; отличить это от машины, которая ещё загружается, установщик не может — он повторяет попытку всё отведённое время (120 попыток с интервалом в пять секунд) и только потом завершается с ошибкой, называя адрес и путь, по которому обращался. Запишите на такую машину актуальный образ и включите её снова.

### Проверки перед началом установки

{% alert level="info" %}
Начиная с версии 1.74, модули DKP устанавливаются в виде образов в формате EROFS, подключаемых только для чтения, что защищает их от изменения после установки. Этот механизм включается автоматически, если на узле, где работает контроллер DKP (по умолчанию — master-узел), в ядре зарегистрирована файловая система `erofs`. DKP загружает этот модуль ядра только на узлах с containerd v2, поэтому при использовании containerd v1 на master-узлах загрузку `erofs` нужно обеспечить средствами операционной системы. Иначе DKP будет устанавливать модули обычным способом, без защиты их целостности и без отдельного алерта. Подробнее — в разделе [«Защита целостности модулей DKP»](../architecture/security/integrity-control.html#защита-целостности-модулей-dkp).
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
   - На сервере (ВМ) для master-узла должен быть доступен Python (Python 3 или Python 2). Также должны быть доступны стандартные модули Python, используемые установщиком:
     - для работы с HTTP-запросами: `urllib.request` (Python 3) или `urllib2` (Python 2);
     - для обработки HTTP-ошибок: `urllib.error` (Python 3) или `urllib2` (Python 2);
     - для работы с конфигурацией: `configparser` (Python 3) или `ConfigParser` (Python 2);
     - для запуска HTTP-сервера: `http.server` (Python 3) или `SimpleHTTPServer` (Python 2);
     - для работы с TCP-сервером: `http.server` (Python 3) или `SocketServer` (Python 2).
   - Хранилище образов доступно через прокси (если настройки прокси указаны в конфигурации установки).
   - На сервере (ВМ) для master-узла и на хосте, с которого запускается установщик, должен быть доступен порт `22/TCP` (сетевой доступ от персонального компьютера), необходимый для процесса установки. Подробнее в разделе [«Сетевое взаимодействие компонентов платформы»](../reference/network_interaction.html).
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

Для пропуска отдельной проверки используйте флаг `--preflight-skip-check`, передав в качестве аргумента имя preflight-чека. Флаг можно указывать несколько раз.

- `--preflight-skip-all-checks` — пропуск всех предварительных проверок;
- `--preflight-skip-check=static-ssh-tunnel` — пропуск проверки проброса SSH;
- `--preflight-skip-check=ports-availability` — пропуск проверки доступности необходимых портов;
- `--preflight-skip-check=resolve-localhost` — пропуск проверки разрешения `localhost`;
- `--preflight-skip-check=dhctl-edition` — пропуск проверки версии DKP;
- `--preflight-skip-check=registry-access-through-proxy` — пропуск проверки доступа к хранилищу образов через прокси-сервер;
- `--preflight-skip-check=public-domain-template` — пропуск проверки шаблона `publicDomain`;
- `--preflight-skip-check=static-ssh-credential` — пропуск проверки учетных данных SSH-пользователя;
- `--preflight-skip-check=registry-credentials` — пропуск проверки учетных данных для доступа к хранилищу образов;
- `--preflight-skip-check=python-modules` — пропуск проверки наличия Python;
- `--preflight-skip-check=sudo-allowed` — пропуск проверки прав доступа для выполнения команды `sudo`;
- `--preflight-skip-check=static-system-requirements` — пропуск проверки соответствия системным требованиям;
- `--preflight-skip-check=static-single-ssh-host` — пропуск проверки количества указанных SSH-хостов;
- `--preflight-skip-check=cloud-api-accessibility` — пропуск проверки доступности Cloud API;
- `--preflight-skip-check=time-drift` — пропуск проверки отсутствия рассинхронизации времени (time drift);
- `--preflight-skip-check=cidr-intersection` — пропуск проверки пересечения CIDR;
- `--preflight-skip-check=deckhouse-user` — пропуск проверки наличия пользователя `deckhouse`;
- `--preflight-skip-check=yandex-cloud-config` — пропуск проверки конфигурации Yandex Cloud с WithNatInstance;
- `--preflight-skip-check=dvp-kubeconfig` — пропуск проверки DVP kubeconfig.
- `--preflight-skip-check=static-instances-ssh-credentials` — пропуск проверки доступности StaticInstances с SSHCredentials.

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

DKP можно установить из стороннего хранилища образов или через проксирующий сервер внутри закрытого контура.

{% alert level="warning" %}
DKP поддерживает аутентификацию в хранилище образов по схемам Basic и Bearer token (сначала проверяется Basic, при неуспехе — Bearer).

Если перед хранилищем стоит прокси-сервер, он должен корректно проксировать заголовок Registry API v2 `Docker-Distribution-API-Version: registry/2.0`, иначе проверка Basic может завершиться ошибкой, а последующая попытка Bearer сообщением с ошибкой `couldn't find bearer realm parameter`.

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

  Пример команды для загрузки модуля `stronghold` с semver-ограничением `^` (разрешает обновления, которые не меняют первую ненулевую цифру слева) от версии 1.2.0:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module stronghold@1.2.0 \
  /home/user/d8-bundle
  ```

  Пример команды для загрузки модуля `secrets-store-integration` с semver-ограничением `~` (разрешает обновления, которые не меняют разряд последней указанной цифры) от версии 1.1.0:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module secrets-store-integration@~1.1.0 \
  /home/user/d8-bundle
  ```

  Заключайте в кавычки значение флага `--include-module`, если оно содержит `>=` или `<=`. Без кавычек shell обработает это как перенаправление ввода/вывода.

  Пример команды для загрузки модуля `console` с semver-ограничением `>=` (любая версия, начиная с указанной и выше) от версии 1.43.2:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module "console@>=1.43.2" \
  /home/user/d8-bundle
  ```

  Пример команды для загрузки точной версии модуля `stronghold` 1.2.5 с публикацией во все каналы обновлений:

  ```shell
  d8 mirror pull \
  --license='<LICENSE_KEY>' \
  --no-platform --no-security-db \
  --include-module stronghold@=v1.2.5 \
  /home/user/d8-bundle
  ```

{% offtopic title="Другие параметры команды, доступные для использования:" %}

- `--no-pull-resume` — принудительно начать загрузку сначала;
- `--force` — перезаписать существующие пакеты, если они конфликтуют с текущей операцией загрузки;
- `--ignore-suspend` — игнорировать приостановленные каналы релизов и продолжить зеркалирование. Используйте с осторожностью;
- `--no-platform` — пропустить загрузку пакета образов Deckhouse Kubernetes Platform (`platform.tar`);
- `--no-modules` — пропустить загрузку пакетов модулей (`module-*.tar`);
- `--no-security-db` — пропустить загрузку пакета баз данных сканера уязвимостей (`security.tar`);
- `--no-packages` — пропустить загрузку пакетов Deckhouse;
- `--no-installer` — пропустить загрузку образов инсталлятора Deckhouse;
- `--only-extra-images` — загрузить только дополнительные образы модулей без загрузки основных образов модулей;
- `--skip-vex-images` — пропустить загрузку VEX-образов;
- `--include-platform` = `CONSTRAINT` — загрузить релизы Deckhouse Kubernetes Platform по semver-ограничению. Параметр нельзя использовать одновременно с `--since-version` и `--deckhouse-tag`. Значение ограничения всегда заключайте в кавычки: `>` и `<` являются shell-перенаправлениями. Примеры: `--include-platform ">=1.64 <=1.68"`, `--include-platform "~1.65.0"`, `--include-platform "^1.65.0"`, `--include-platform "1.65.0"`, `--include-platform "=v1.65.3"` или `--include-platform "=v1.65.3+stable"`;
- `--include-module` / `-i` = `name[@Major.Minor]` — загрузить определенный набор модулей по принципу белого списка (и, при необходимости, их минимальных версий). Укажите несколько раз, чтобы добавить в белый список больше модулей. Эти флаги игнорируются, если используются совместно с `--no-modules`.

  Поддерживаются следующие синтаксисы для указания версий модулей. Если используются операторы `>=` или `<=`, заключайте в кавычки все значение флага:
  - `module-name@1.3.0` — загрузка версий с semver ^ ограничением (^1.3.0), включая v1.3.0, v1.3.3, v1.4.1;
  - `module-name@~1.3.0` — загрузка версий с semver ~ ограничением (>=1.3.0 <1.4.0), включая только v1.3.0, v1.3.3;
  - `"module-name@>=1.3.0"` — загрузка версий с semver-ограничением `>=`, включая явно указанную версию и более новые версии, подходящие под ограничение;
  - `"module-name@>=1.3.0 <=1.4.0"` — загрузка версий в диапазоне с учетом нижней и верхней границы;
  - `module-name@=v1.3.0` — загрузка точного соответствия тегу v1.3.0, публикация во все каналы релизов;
  - `module-name@=v1.3.0+stable` — загрузка точного соответствия тегу v1.3.0, публикация в канал релизов stable;
  - `module-name@=bobV1` — загрузка точного соответствия тегу "bobV1", публикация во все каналы релизов.
- `--exclude-module` / `-e` = `name` — пропустить загрузку определенного набора модулей по принципу черного списка. Укажите несколько раз, чтобы добавить в черный список больше модулей. Игнорируется, если используются `--no-modules` или `--include-module`.
- `--include-package` = `name[@version]` — загрузить определенный набор пакетов по принципу белого списка. Для указания версий и semver-ограничений используется тот же синтаксис, что и у `--include-module`, включая правила использования кавычек.
- `--exclude-package` = `name[@version]` — пропустить загрузку определенного набора пакетов по принципу черного списка. Игнорируется, если используется `--include-package`.
- `--modules-path-suffix` — изменить суффикс пути к репозиторию модулей в основном репозитории DKP. По умолчанию используется суффикс `/modules` (так, например, полный путь к репозиторию с модулями будет выглядеть как `registry.deckhouse.ru/deckhouse/EDITION/modules`);
- `--since-version=X.Y` — скачать все версии DKP, начиная с указанной минорной версии. Параметр будет проигнорирован, если указанная версия выше, чем версия на канале обновлений Rock Solid. Параметр не может быть использован одновременно с параметром `--deckhouse-tag`;
- `--deckhouse-tag` — скачать только конкретную версию DKP (без учета каналов обновлений). Параметр не может быть использован одновременно с параметром `--since-version`;
- `--installer-tag=TAG` — скачать конкретный тег инсталлятора Deckhouse. Если параметр не указан, используется тег `latest`;
- `--proxy-registry` — использовать proxy/cache-хранилище, которое не поддерживает registry catalog API. Требует `--include-platform`, если платформа не пропущена через `--no-platform`, и хотя бы один `--include-module`, если модули не пропущены через `--no-modules`. Нельзя использовать вместе с `--deckhouse-tag` или `--since-version`;
- `--dry-run` — вывести список того, что будет загружено, без скачивания образов;
- `--verbose-summary` — вывести подробную сводку по всем модулям и пакетам с разрешенными версиями;
- `--gost-digest` — рассчитать контрольную сумму итогового набора образов DKP в формате ГОСТ Р 34.11-2012 (Стрибог). Контрольная сумма будет отображена и записана в файл с расширением `.tar.gostsum` в папке с TAR-архивом, содержащим образы DKP;
- `--source-login` и `--source-password` — данные для аутентификации в стороннем хранилище образов;
- `--tls-skip-verify` — отключить проверку TLS-сертификата;
- `--insecure` — обращаться к хранилищам образов по HTTP;
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
