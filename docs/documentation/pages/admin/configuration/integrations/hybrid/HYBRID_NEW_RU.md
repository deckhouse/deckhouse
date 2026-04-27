---
title: Гибридные кластера
permalink: ru/admin/integrations/hybrid/hybrid-new.html
lang: ru
description: |
  Обзор гибридных кластеров Deckhouse Kubernetes Platform, общие требования к сети и инфраструктуре,
  сценарии подключения облачных узлов для OpenStack, Yandex Cloud, VMware vCloud Director, VMware vSphere и zVirt.
search: гибридный кластер, hybrid, static, CloudEphemeral, OpenStack, Yandex, VCD, vSphere, zVirt
---

Deckhouse Kubernetes Platform (DKP) позволяет расширять **статический** кластер узлами в облаке или виртуализации. Такие узлы заказываются через API провайдера и в кластере представлены как **CloudEphemeral**. Ниже — общие правила и настройка по провайдерам.

{% alert level="info" %}
Чем гибридный кластер отличается от гибридной **группы** узлов, описано в разделе [Управление гибридными группами узлов и кластерами](../../../../architecture/cluster-and-infrastructure/node-management/hybrid-nodegroups-and-clusters.html).
{% endalert %}

## Когда использовать гибридный кластер

Гибрид удобен, если нужно:

- постепенно переносить нагрузку между ЦОД и облаком в рамках одного кластера;
- держать постоянную часть узлов у себя, а пик — заказывать в облаке;
- соблюдать требования к данным и при этом использовать облачные машины для вычислений.

В **закрытом контуре** возможны те же схемы, если доступны образы и артефакты установки, при необходимости — [зеркало registry](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D0%BA%D0%B0%D0%BA-%D0%B8%D1%81%D0%BF%D0%BE%D0%BB%D1%8C%D0%B7%D0%BE%D0%B2%D0%B0%D1%82%D1%8C-dkp-%D0%B2-%D0%B7%D0%B0%D0%BA%D1%80%D1%8B%D1%82%D0%BE%D0%BC-%D0%BA%D0%BE%D0%BD%D1%82%D1%83%D1%80%D0%B5). Сеть между площадками должна удовлетворять тем же условиям, что и в открытой среде.

## Поддерживаемые провайдеры

В гибридной схеме на статическом кластере поддерживаются провайдеры:

- **OpenStack** — модуль `cloud-provider-openstack`. Как подключить облако: [Подключение OpenStack](../public/openstack/connection-and-authorization.html).
- **Yandex Cloud** — модуль `cloud-provider-yandex`. [Авторизация в Yandex Cloud](../public/yandex/authorization.html).
- **VMware vCloud Director (VCD)** — модуль `cloud-provider-vcd`. [Подключение и авторизация в VCD](../virtualization/vcd/connection-and-authorization.html).
- **VMware vSphere** — модуль `cloud-provider-vsphere`. [Службы vSphere](../virtualization/vsphere/services.html), [схемы размещения](../virtualization/vsphere/layout.html).
- **zVirt** — модуль `cloud-provider-zvirt`. [Службы zVirt](../virtualization/zvirt/services.html), [схемы размещения](../virtualization/zvirt/layout.html).

## Общие требования

### Кластер и тип узлов

Исходный кластер должен быть со `clusterType: Static` ([ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration)). Облачные машины описываются ресурсами `*InstanceClass` и [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узла **CloudEphemeral** (в части примеров ниже для совместимости со старыми API может встречаться `Cloud`).

### Сеть

Между сетью статических узлов и сетью облачных ВМ нужна **связность по L3** и открыты порты для компонентов DKP. Полный перечень — в [списке сетевого взаимодействия](../../../../reference/network_interaction.html); ограничения на стороне фаервола — в разделе [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

Обычно проверяют также:

- одинаковый **MTU** на всём пути (особенно если трафик подов идёт в туннеле);
- доступность **DNS** и тех внешних адресов, которые разрешены вашей политикой;
- доступность **Kubernetes API** для новых узлов;
- при использовании Cilium — параметр [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), если трафик подов фильтруют на границе сетей.

Общая L2-сеть между статическими и облачными узлами **не обязательна**: для работы кластера обычно достаточно L3 и открытых портов. Нужна ли дополнительно общая L2, какие подсети и учётные данные использовать, какие шаблоны ВМ и классы хранения доступны — зависит от провайдера; это расписано в разделах **«Предварительные требования»** соответствующего облака ниже.

### Узлы

Нужны синхронизация времени (NTP), корректные hostname и маршруты до API кластера. SSH-ключи задаются при установке или в конфигурации провайдера.

### Как передаётся конфигурация облака

Способ задания параметров провайдера **разный**. Общая схема с секретом `d8-provider-cluster-configuration` в `kube-system` описана в разделе [«Общая схема подключения облака»](#общая-схема-подключения-облака). Для **OpenStack** в типовом гибриде достаточно [настройки модуля](/modules/cloud-provider-openstack/configuration.html) через `ModuleConfig` / конфигурацию Deckhouse, без обязательного секрета с `*ClusterConfiguration` — пошагово в разделе [«Гибридный кластер с OpenStack»](#гибридный-кластер-с-openstack); альтернатива с секретом — в подразделе **«Альтернатива: OpenStack через секрет»** там же.

{% alert level="info" %}
Префикс имён всех CloudEphemeral-узлов в гибриде со Static master задаётся параметром [`instancePrefix`](/modules/node-manager/configuration.html#parameters-instanceprefix) модуля `node-manager`. Для одной NodeGroup отдельный префикс задать нельзя.
{% endalert %}

## Общая схема подключения облака

Для любого провайдера порядок работ обычно такой:

1. Подготовить статический кластер и сеть по разделу «Общие требования».
2. Подготовить конфигурацию провайдера по инструкции выбранного раздела ниже: для **Yandex Cloud**, **VCD**, **vSphere** и **zVirt** — YAML ресурса `*ClusterConfiguration` (в гибриде часть полей, например `masterNodeGroup`, может быть формальной, если мастера не создаются в облаке). Для **OpenStack** в типовом сценарии — параметры в `ModuleConfig` (раздел [«Гибридный кластер с OpenStack»](#гибридный-кластер-с-openstack)); при необходимости — ещё `OpenStackClusterConfiguration` и discovery в секрете (подраздел **«Альтернатива: OpenStack через секрет»** в том же разделе).
3. Для провайдеров со секретом положить YAML и при необходимости JSON **discovery** в секрет `kube-system` с именем `d8-provider-cluster-configuration` (ключи `cloud-provider-cluster-configuration.yaml` и `cloud-provider-discovery-data.json`; часто содержимое передают в **Base64**).
4. Включить модуль **`cloud-provider-…`** командой [`d8 system module enable`](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#%D0%BA%D0%B0%D0%BA-%D0%B2%D0%BA%D0%BB%D1%8E%D1%87%D0%B8%D1%82%D1%8C-%D0%BC%D0%BE%D0%B4%D1%83%D0%BB%D1%8C) или через `ModuleConfig` и дождаться готовности подов в namespace модуля.
5. Создать ресурсы `*InstanceClass` и [NodeGroup](/modules/node-manager/cr.html#nodegroup) для заказа машин в облаке.
6. Проверить узлы командой `d8 k get nodes -o wide`. Если машины не появляются, смотреть логи `machine-controller-manager` в namespace провайдера.

Подробнее про добавление облачных узлов: [Добавление облачных узлов](../../platform-scaling/node/cloud-node.html).

## Гибридный кластер с OpenStack

Интеграция описана в модуле [cloud-provider-openstack](/modules/cloud-provider-openstack/).

{% alert level="warning" %}
`Cloud-controller-manager` синхронизирует OpenStack и Kubernetes и может удалять узлы, которых нет в облаке. Статические узлы DKP помечает так, что контроллер их **не трогает** (в `.spec.providerID` используется `static://`).
{% endalert %}

### Предварительные требования

- Кластер со `clusterType: Static` и выполненные [общие требования](#общие-требования) по сети и узлам.
- В OpenStack: проект, сеть, образы, flavor, квоты и доступ к API — по инструкции [подключение OpenStack](../public/openstack/connection-and-authorization.html).
- Учётная запись или приложение с правами на создание инстансов и сопутствующих ресурсов (по выбранному layout).
- В [FAQ модуля](/modules/cloud-provider-openstack/faq.html#как-поднять-гибридный-кластер) для гибрида указана **общая L2-сеть** между всеми узлами кластера. Если L2 между ЦОД и облаком недоступна, ориентируйтесь на L3 и настройки CNI из общих требований и убедитесь, что поды и сервисы между площадками достижимы.

### Настройка

Ниже — типовой путь для гибрида: **учётная запись и сеть в OpenStack**, **`ModuleConfig` модуля** (без секрета `d8-provider-cluster-configuration`), **`OpenStackInstanceClass`** и **`NodeGroup`**. Параметры подставьте свои; списки flavor, образов и сетей смотрите командами `openstack flavor list`, `openstack image list`, `openstack network list`.

1. **CNI.** Если в `kube-system` остался DaemonSet **Flannel** (`flannel-ds`) после перехода на **Cilium**, удалите его:

   ```shell
   d8 k -n kube-system delete ds flannel-ds
   ```

   Если используется только Cilium, шаг не нужен. При фильтрации туннельного трафика настройте [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) и [сетевые политики](../../configuration/network/policy/configuration.html).

2. **Учётная запись и ключ SSH в OpenStack.** Подготовьте пользователя с правами на проект (см. [подключение OpenStack](../public/openstack/connection-and-authorization.html)). Создайте **keypair** для заказа ВМ (имя должно совпасть с `instances.sshKeyPairName` в шаге 3):

   ```shell
   source ./openrc.sh
   openstack keypair create --public-key ~/.ssh/id_rsa.pub my-dkp-hybrid-keypair
   ```

3. **Включите модуль и задайте `ModuleConfig`.** Пример (аналогично [примерам модуля](/modules/cloud-provider-openstack/examples.html)):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-openstack
   spec:
     version: 1
     enabled: true
     settings:
       connection:
         authURL: https://openstack.example.com:5000/v3/
         domainName: default
         tenantName: my-project
         username: dkp-hybrid-user
         password: "<PASSWORD>"
         region: RegionOne
       externalNetworkNames:
         - public
       internalNetworkNames:
         - kube-internal
       instances:
         sshKeyPairName: my-dkp-hybrid-keypair
         securityGroups:
           - default
         imageName: debian-12-generic-amd64
         mainNetwork: kube-internal
       zones:
         - nova
       podNetworkMode: DirectRoutingWithPortSecurityEnabled
   ```

   Сохраните YAML в файл (например, `cloud-provider-openstack-mc.yaml`) и примените:

   ```shell
   d8 k apply -f cloud-provider-openstack-mc.yaml
   d8 k get pods -n d8-cloud-provider-openstack
   ```

   После включения модуля данные облака дополняет **cloud-data-discoverer**; отдельно заполнять `cloud-provider-discovery-data.json` для этого сценария обычно не требуется.

4. **`OpenStackInstanceClass`.** Пример:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: OpenStackInstanceClass
   metadata:
     name: worker
   spec:
     flavorName: m1.large
     imageName: debian-12-generic-amd64
     rootDiskSize: 50
   ```

5. **`NodeGroup`.** Пример с типом узла **CloudEphemeral**:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-os
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: OpenStackInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 3
       zones:
         - nova
   ```

   Сохраните ресурсы из шагов 4–5 в файл и примените:

   ```shell
   d8 k apply -f openstack-instanceclass-nodegroup.yaml
   ```

6. **Проверка.**

   ```shell
   d8 k get nodes -o wide
   d8 k -n d8-cloud-provider-openstack get machine
   ```

   Если машины не создаются, см. [«Устранение неполадок»](#устранение-неполадок).

### Альтернатива: OpenStack через секрет `d8-provider-cluster-configuration`

Используйте этот вариант, если конфигурацию провайдера нужно задавать так же, как при установке **облачного** кластера в OpenStack (ресурс `OpenStackClusterConfiguration` и discovery в секрете). Шаги ниже **самодостаточны** для OpenStack и не опираются на другие провайдеры в этом документе.

1. **Файл `openstack-cluster-configuration.yaml`.** Сохраните в нём ресурс [`OpenStackClusterConfiguration`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration). Пример для `layout: Standard` (блок `masterNodeGroup` при гибриде часто формален, если мастера не создаются в OpenStack):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: OpenStackClusterConfiguration
   layout: Standard
   sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAB..."
   zones:
     - nova
   standard:
     internalNetworkDNSServers:
       - 8.8.8.8
     internalNetworkCIDR: 192.168.195.0/24
     internalNetworkSecurity: false
     externalNetworkName: public
   provider:
     authURL: https://openstack.example.com:5000/v3/
     domainName: default
     tenantName: my-project
     username: dkp-user
     password: "<PASSWORD>"
     region: RegionOne
   masterNodeGroup:
     replicas: 1
     instanceClass:
       rootDiskSize: 50
       flavorName: m1.large
       imageName: debian-12-generic-amd64
     volumeTypeMap:
       nova: fast-ssd
   ```

2. **Файл `openstack-discovery.json`.** Сохраните в нём ресурс **`OpenStackCloudDiscoveryData`** для того же `layout: Standard` (должны быть заданы сети, зоны, режим сети подов, параметры инстансов и блок **`loadBalancer`** с `subnetID` и `floatingNetworkID`). Идентификаторы сетей и подсетей получите в OpenStack, например:

   ```shell
   openstack network list
   openstack subnet list
   ```

   Пример JSON (подставьте свои UUID и имена сетей, keypair и образ):

   ```json
   {
     "apiVersion": "deckhouse.io/v1",
     "kind": "OpenStackCloudDiscoveryData",
     "layout": "Standard",
     "internalNetworkNames": ["kube-internal"],
     "externalNetworkNames": ["public"],
     "podNetworkMode": "DirectRoutingWithPortSecurityEnabled",
     "zones": ["nova"],
     "instances": {
       "sshKeyPairName": "my-dkp-hybrid-keypair",
       "imageName": "debian-12-generic-amd64",
       "mainNetwork": "kube-internal",
       "securityGroups": ["default"]
     },
     "loadBalancer": {
       "subnetID": "<SUBNET_UUID>",
       "floatingNetworkID": "<EXTERNAL_NETWORK_UUID>"
     }
   }
   ```

   Для других `layout` (`StandardWithNoRouter`, `Simple`, …) набор полей в `OpenStackCloudDiscoveryData` другой — сверяйтесь со схемой в репозитории модуля `cloud-provider-openstack` (файл `cloud_discovery_data.yaml`) и с [документацией модуля](/modules/cloud-provider-openstack/).

3. **Закодируйте оба файла в Base64** (без переносов строк в выводе):

   ```shell
   base64 -w0 < openstack-cluster-configuration.yaml > openstack-cluster.b64
   base64 -w0 < openstack-discovery.json > openstack-discovery.b64
   ```

4. **Создайте манифест секрета** (например, `openstack-provider-secret.yaml`). Вставьте в `data` **одну строку** из каждого `.b64` файла:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: <СОДЕРЖИМОЕ_openstack-cluster.b64>
     cloud-provider-discovery-data.json: <СОДЕРЖИМОЕ_openstack-discovery.b64>
   ```

5. **Примените секрет:**

   ```shell
   d8 k apply -f openstack-provider-secret.yaml
   ```

6. **Включите модуль** `cloud-provider-openstack`, если он ещё не включён (любой из способов):

   ```shell
   d8 system module enable cloud-provider-openstack
   ```

   или примените `ModuleConfig`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-openstack
   spec:
     version: 1
     enabled: true
   ```

7. **Проверьте поды модуля:**

   ```shell
   d8 k get pods -n d8-cloud-provider-openstack
   ```

8. **`OpenStackInstanceClass` и `NodeGroup`.** Примеры (подставьте flavor, образ и зону из вашего OpenStack):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: OpenStackInstanceClass
   metadata:
     name: worker
   spec:
     flavorName: m1.large
     imageName: debian-12-generic-amd64
     rootDiskSize: 50
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-os
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: OpenStackInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 3
       zones:
         - nova
   ```

   Примените и проверьте узлы:

   ```shell
   d8 k apply -f openstack-instanceclass-nodegroup.yaml
   d8 k get nodes -o wide
   d8 k -n d8-cloud-provider-openstack get machine
   ```

### Подключение хранилища (OpenStack)

Если на узлах из OpenStack нужны тома Cinder, создайте `StorageClass` с `provisioner: csi-cinderplugin`. Список типов томов в облаке:

```shell
openstack volume type list
```

Пример для типа `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Должно быть именно так.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

Дополнительные вопросы по модулю — в [FAQ OpenStack](/modules/cloud-provider-openstack/faq.html).

## Гибридный кластер с Yandex Cloud

### Предварительные требования

- Кластер со `clusterType: Static` и [общие требования](#общие-требования) по сети, DNS и узлам.
- Сервисный аккаунт и каталог в Yandex Cloud — по [авторизации в Yandex Cloud](../public/yandex/authorization.html).
- Для туннелирования подов через Cilium — режим **VXLAN**, см. [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
- Связь между сетью статического кластера и VPC Yandex Cloud по правилам из раздела [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

### Настройка

Выполните шаги:

1. Создайте **Service Account** в каталоге Yandex Cloud с ролями **`editor`** на каталог и **`vpc.admin`** на VPC (подробнее — [авторизация в Yandex Cloud](../public/yandex/authorization.html)). Пример через CLI:

   ```shell
   export FOLDER_ID=b1g...
   yc iam service-account create --name dkp-hybrid --folder-id "$FOLDER_ID"
   export SA_ID=$(yc iam service-account get --name dkp-hybrid --folder-id "$FOLDER_ID" --format json | jq -r .id)
   yc resource-manager folder add-access-binding "$FOLDER_ID" --role editor --subject "serviceAccount:${SA_ID}"
   yc vpc network list --folder-id "$FOLDER_ID"
   yc iam key create --service-account-id "$SA_ID" --output sa-key.json
   ```

   Ключ `sa-key.json` вставьте в поле `provider.serviceAccountJSON` в шаге 2 (как одну строку JSON).

2. Подготовьте секрет `d8-provider-cluster-configuration`. В ключ `cloud-provider-cluster-configuration.yaml` поместите ресурс `YandexClusterConfiguration`. Поля в `masterNodeGroup` в гибриде часто формальны: **мастера в Yandex не создаются**, если кластер изначально статический.

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexClusterConfiguration
   layout: WithoutNAT
   masterNodeGroup:
     replicas: 1
     instanceClass:
       cores: 4
       memory: 8192
       imageID: fd80bm0rh4rkepi5ksdi
       diskSizeGB: 100
       platform: standard-v3
       externalIPAddresses:
       - "Auto"
   nodeNetworkCIDR: 10.160.0.0/16
   existingNetworkID: empty
   provider:
     cloudID: CLOUD_ID
     folderID: FOLDER_ID
     serviceAccountJSON: '{"id":"ajevk1dp8f9...--END PRIVATE KEY-----\n"}'
   sshPublicKey: <SSH_PUBLIC_KEY>
   ```

   Кратко по полям: `nodeNetworkCIDR` — CIDR, который покрывает подсети узлов в Yandex Cloud; `cloudID`, `folderID`, `serviceAccountJSON`, `sshPublicKey` — параметры облака и доступа.

3. В ключ `cloud-provider-discovery-data.json` того же секрета поместите JSON `YandexCloudDiscoveryData`, например:

   ```json
   {
     "apiVersion": "deckhouse.io/v1",
     "defaultLbTargetGroupNetworkId": "empty",
     "internalNetworkIDs": [
       "<NETWORK-ID>"
     ],
     "kind": "YandexCloudDiscoveryData",
     "monitoringAPIKey": "",
     "region": "ru-central1",
     "routeTableID": "empty",
     "shouldAssignPublicIPAddress": false,
     "zoneToSubnetIdMap": {
       "ru-central1-a": "<A-SUBNET-ID>",
       "ru-central1-b": "<B-SUBNET-ID>",
       "ru-central1-d": "<D-SUBNET-ID>"
     },
     "zones": [
       "ru-central1-a",
       "ru-central1-b",
       "ru-central1-d"
     ]
   }
   ```

   Здесь задаются сети для связности узлов (`internalNetworkIDs`), соответствие зон и подсетей (`zoneToSubnetIdMap`), нужны ли публичные IP на ВМ (`shouldAssignPublicIPAddress`).

4. Закодируйте YAML и JSON в **Base64** (подставьте имена файлов из шагов 2–3):

   ```shell
   base64 -w0 < yandex-cluster-configuration.yaml > yandex-cluster.b64
   base64 -w0 < yandex-discovery.json > yandex-discovery.b64
   ```

   Вставьте содержимое `yandex-cluster.b64` и `yandex-discovery.b64` в поля секрета ниже. Пример секрета **вместе** с включением модуля:

   ```yaml
   apiVersion: v1
   data:
     cloud-provider-cluster-configuration.yaml: <YANDEXCLUSTERCONFIGURATION_BASE64>
     cloud-provider-discovery-data.json: <YANDEXCLOUDDISCOVERYDATA_BASE64>
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-yandex
   spec:
     version: 1
     enabled: true
     settings:
       storageClass:
         default: network-ssd
   ```

5. Удалите объект **ValidatingAdmissionPolicyBinding**, если это требуется для вашей версии:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

6. Примените манифесты (секрет и `ModuleConfig` из шага 4, при необходимости — в одном файле) и дождитесь активации модуля и появления CRD:

   ```shell
   d8 k apply -f yandex-hybrid-manifests.yaml
   d8 k get mc cloud-provider-yandex
   d8 k get crd yandexinstanceclasses
   ```

7. Создайте `YandexInstanceClass` и `NodeGroup`. В `mainSubnet` укажите ID подсети в Yandex Cloud, из которой ВМ доступны сети статических узлов.

   ```yaml
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Cloud
     cloudInstances:
       classReference:
         kind: YandexInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 3
       zones:
         - ru-central1-d
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: YandexInstanceClass
   metadata:
     name: worker
   spec:
     cores: 4
     memory: 8192
     diskSizeGB: 50
     diskType: network-ssd
     mainSubnet: <YOUR-SUBNET-ID>
   ```

   Примените:

   ```shell
   d8 k apply -f yandex-instanceclass-nodegroup.yaml
   ```

8. Проверьте узлы:

   ```shell
   d8 k get nodes -o wide
   ```

   Если машины не появились, см. раздел [«Устранение неполадок»](#устранение-неполадок).

## Гибридный кластер с VMware vCloud Director (VCD)

### Предварительные требования

- Кластер со `clusterType: Static` и [общие требования](#общие-требования) по сети и узлам.
- Настроенный тенант VCD по [инструкции](../virtualization/vcd/connection-and-authorization.html).
- Связь между сетью статических узлов и VCD (L2 или L3 с нужными портами — см. [сетевые политики](../../configuration/network/policy/configuration.html)).
- В VCD — рабочая сеть с **DHCP**, если выбран сценарий с DHCP.
- Учётная запись VCD со статическим паролем и достаточными правами.
- При туннелировании подов через Cilium — [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).

### Настройка

Выполните шаги:

1. **Учётная запись и API VCD.** Создайте пользователя в организации и получите URL API (часто `https://<vcd-host>/api`). Пример проверки доступности:

   ```shell
   curl -skI "https://vcd.example.com/api/versions" | head -n1
   ```

   Сохраните логин, пароль (или токен) и параметры vApp / VDC из [инструкции по подключению](../virtualization/vcd/connection-and-authorization.html). Далее — файл `cloud-provider-vcd-token.yml` с ресурсом `VCDClusterConfiguration`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDClusterConfiguration
   layout: Standard
   mainNetwork: <NETWORK_NAME>
   internalNetworkCIDR: <NETWORK_CIDR>
   organization: <ORGANIZATION>
   virtualApplicationName: <VAPP_NAME>
   virtualDataCenter: <VDC_NAME>
   provider:
     server: <API_URL>
     apiToken: <PASSWORD>
     username: <USER_NAME>
     insecure: false
   masterNodeGroup:
     instanceClass:
       etcdDiskSizeGb: 10
       mainNetworkIPAddresses:
       - 192.168.199.2
       rootDiskSizeGb: 50
       sizingPolicy: <SIZING_POLICY>
       storageProfile: <STORAGE_PROFILE>
       template: <VAPP_TEMPLATE>
     replicas: 1
   sshPublicKey: <SSH_PUBLIC_KEY>
   ```

   Поля задают сеть и CIDR для узлов, организацию, vApp, VDC, шаблон ВМ, политики sizing и storage, URL API VCD и учётные данные. Блок `masterNodeGroup` при гибриде может быть формальным, если мастера не создаются в VCD.

2. Подготовьте **`VCDCloudProviderDiscoveryData`** (файл `cloud-provider-discovery-data.json`). Минимальный вариант — одна зона `default`:

   ```json
   {
     "apiVersion": "deckhouse.io/v1",
     "kind": "VCDCloudProviderDiscoveryData",
     "zones": [
       "default"
     ]
   }
   ```

3. Закодируйте оба файла в Base64:

   ```shell
   base64 -w0 < cloud-provider-vcd-token.yml > cluster.b64
   base64 -w0 < cloud-provider-discovery-data.json > discovery.b64
   ```

4. Создайте секрет. Пример: подставьте содержимое `cluster.b64` и `discovery.b64` вместо плейсхолдеров (или вставьте вывод команд выше в одну строку):

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: <BASE64_YAML_ИЗ_ШАГА_1>
     cloud-provider-discovery-data.json: <BASE64_JSON_ИЗ_ШАГА_2>
   ```

   Эквивалент одной строки для discovery из шага 2 (уже в Base64):

   ```text
   eyJhcGlWZXJzaW9uIjoiZGVja2hvdXNlLmlvL3YxIiwia2luZCI6IlZDRENsb3VkUHJvdmlkZXJEaXNjb3ZlcnlEYXRhIiwiem9uZXMiOlsiZGVmYXVsdCJdfQo=
   ```

5. **Включите модуль.** Сохраните секрет (шаг 4) и при необходимости `ModuleConfig` в один манифест и примените:

   ```shell
   d8 k apply -f vcd-provider-secret-and-mc.yaml
   ```

   Либо только CLI:

   ```shell
   d8 system module enable cloud-provider-vcd
   ```

   Пример `ModuleConfig` (если не объединяете с секретом в одном файле):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vcd
   spec:
     version: 1
     enabled: true
   ```

6. При необходимости приведите секрет **`d8-cni-configuration`** в соответствие с [`ModuleConfig` модуля `cni-cilium`](/modules/cni-cilium/configuration.html) (в отдельных сценариях упоминается правка полей вроде `.data.cilium` / `.data.necilium`).

7. Убедитесь, что поды в `d8-cloud-provider-vcd` в состоянии `Running`:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

8. В ряде сценариев после первичной настройки требуется **перезагрузка master** — ориентируйтесь на рекомендации для вашей версии платформы.

9. Создайте [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     rootDiskSizeGb: 50
     sizingPolicy: <SIZING_POLICY>
     storageProfile: <STORAGE_PROFILE>
     template: <VAPP_TEMPLATE>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     cloudInstances:
       classReference:
         kind: VCDInstanceClass
         name: worker
       maxPerZone: 2
       minPerZone: 1
     nodeTemplate:
       labels:
         node-role/worker: ""
     nodeType: CloudEphemeral
   ```

   Примените манифест:

   ```shell
   d8 k apply -f vcd-instanceclass-nodegroup.yaml
   ```

10. Проверьте узлы:

   ```shell
   d8 k get nodes -o wide
   ```

## Гибридный кластер с VMware vSphere

Узлы в vSphere описываются [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass). Для гибрида со **статическим** control plane конфигурация облака задаётся ресурсом [VsphereClusterConfiguration](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration) в секрете **`d8-provider-cluster-configuration`** (см. [общую схему](#общая-схема-подключения-облака)). Поля layout, `provider`, datastore, сети и зоны разобраны в [схемах размещения vSphere](../virtualization/vsphere/layout.html); учётная запись vCenter — в [подключении и привилегиях](../virtualization/vsphere/authorization.html).

### Предварительные требования

- Кластер со `clusterType: Static` и [общие требования](#общие-требования) по сети (L3 до сети ВМ vSphere, MTU, DNS, доступ к API кластера с новых узлов).
- Доступ с узлов Deckhouse (или с мастеров, в зависимости от политики) к **API vCenter** по HTTPS.
- Выбранный **layout**, портгруппы, datastore или кластер datastore, шаблон или клон ВМ — по документации размещения.
- При туннелировании подов через Cilium — согласованный [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) и политики на границе сетей.

### Настройка

Выполните шаги (подставьте FQDN vCenter, пути к сетям, шаблону и datastore из [схем размещения](../virtualization/vsphere/layout.html) и вашего инвентаря).

1. **Учётная запись vCenter.** Создайте пользователя и выдайте роли по [подключению и привилегиям](../virtualization/vsphere/authorization.html). Пример проверки доступности API с машины, где выполняете команды:

   ```shell
   curl -skI "https://vcenter.example.com/" | head -n1
   ```

2. **`VsphereClusterConfiguration`.** Сохраните YAML (пример для `layout: Standard`; блок `masterNodeGroup` при гибриде часто **формальный**, если мастера не создаются в vSphere):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereClusterConfiguration
   sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAB..."
   layout: Standard
   vmFolderPath: kubernetes/hybrid
   regionTagCategory: k8s-region
   zoneTagCategory: k8s-zone
   region: region2
   zones:
     - region2-a
   externalNetworkNames:
     - net3-k8s
   internalNetworkNames:
     - K8S_3
   internalNetworkCIDR: 172.16.2.0/24
   baseResourcePool: kubernetes/cloud
   masterNodeGroup:
     replicas: 1
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: Templates/ubuntu-focal-20.04
       mainNetwork: net3-k8s
       additionalNetworks:
         - K8S_3
       datastore: lun10
       rootDiskSize: 50
       runtimeOptions:
         nestedHardwareVirtualization: false
   provider:
     server: "vcenter.example.com"
     username: "administrator@vsphere.local"
     password: "<PASSWORD>"
     insecure: true
   ```

3. **`VsphereCloudDiscoveryData`.** Файл `cloud-provider-discovery-data.json` (минимально достаточный вариант; детали зон и datastore discoverer может дополнить сам):

   ```json
   {
     "apiVersion": "deckhouse.io/v1",
     "kind": "VsphereCloudDiscoveryData",
     "vmFolderPath": "kubernetes/hybrid"
   }
   ```

4. **Base64** для секрета:

   ```shell
   base64 -w0 < vsphere-cluster-configuration.yaml > vsphere-cluster.b64
   base64 -w0 < cloud-provider-discovery-data.json > vsphere-discovery.b64
   ```

5. **Секрет** `kube-system/d8-provider-cluster-configuration`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: <СОДЕРЖИМОЕ_vsphere-cluster.b64>
     cloud-provider-discovery-data.json: <СОДЕРЖИМОЕ_vsphere-discovery.b64>
   ```

6. **Включите модуль** `cloud-provider-vsphere`. Секрет из шага 5 можно применить так:

   ```shell
   d8 k apply -f vsphere-provider-secret.yaml
   d8 system module enable cloud-provider-vsphere
   ```

   либо только `ModuleConfig`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vsphere
   spec:
     version: 2
     enabled: true
   ```

7. **Готовность подов:**

   ```shell
   d8 k get pods -n d8-cloud-provider-vsphere
   ```

8. **`VsphereInstanceClass` и `NodeGroup`:**

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereInstanceClass
   metadata:
     name: worker
   spec:
     numCPUs: 4
     memory: 8192
     rootDiskSize: 50
     template: Templates/ubuntu-focal-20.04
     mainNetwork: net3-k8s
     datastore: lun10
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-vsphere
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VsphereInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 3
       zones:
         - region2-a
   ```

   Примените манифест:

   ```shell
   d8 k apply -f vsphere-instanceclass-nodegroup.yaml
   ```

9. **Проверка узлов и хранилище:**

   ```shell
   d8 k get nodes -o wide
   ```

   Балансировку, теги зон и **StorageClass** настройте по [документации модуля](/modules/cloud-provider-vsphere/) и разделу [«Хранилище»](#хранилище). При сетевых сбоях на новых узлах см. [«Устранение неполадок»](#устранение-неполадок).

## Гибридный кластер с zVirt

Для заказа ВМ используются [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) и [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass). Примеры полей `clusterID`, `template`, `vnicProfileID`, `storageDomainID` — в [схемах размещения zVirt](../virtualization/zvirt/layout.html).

### Предварительные требования

- Кластер со `clusterType: Static` и [общие требования](#общие-требования) по сети до виртуальной среды zVirt.
- Доступ к **API oVirt** / zVirt, учётная запись по [инструкции подключения](../virtualization/zvirt/authorization.html).
- Шаблон ВМ, профиль vNIC, домен хранения — в соответствии с выбранным layout.
- Требования к [хранилищу](../virtualization/zvirt/storage.html) для дисков узлов и CSI.
- При использовании Cilium с туннелированием подов — [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).

### Настройка

Выполните шаги. Идентификаторы `clusterID`, `vnicProfileID`, `storageDomainID` и имя шаблона возьмите из админки zVirt/oVirt или [схем размещения](../virtualization/zvirt/layout.html).

1. **Доступ к API.** Проверка TLS до менеджера (подставьте хост):

   ```shell
   curl -skI "https://zvirt-engine.example.com/ovirt-engine/" | head -n1
   ```

2. **`ZvirtClusterConfiguration`.** Файл `zvirt-cluster-configuration.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ZvirtClusterConfiguration
   layout: Standard
   sshPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAB..."
   clusterID: "b46372e7-0d52-40c7-9bbf-fda31e187088"
   masterNodeGroup:
     replicas: 1
     instanceClass:
       numCPUs: 4
       memory: 8192
       template: debian-bookworm
       vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
       storageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424"
   provider:
     server: "zvirt-engine.example.com"
     username: "admin@internal"
     password: "<PASSWORD>"
     insecure: true
   ```

3. **`ZvirtCloudProviderDiscoveryData`.** Файл `cloud-provider-discovery-data.json`:

   ```json
   {
     "apiVersion": "deckhouse.io/v1",
     "kind": "ZvirtCloudProviderDiscoveryData",
     "zones": [
       "default"
     ]
   }
   ```

4. **Base64:**

   ```shell
   base64 -w0 < zvirt-cluster-configuration.yaml > zvirt-cluster.b64
   base64 -w0 < cloud-provider-discovery-data.json > zvirt-discovery.b64
   ```

5. **Секрет** `kube-system/d8-provider-cluster-configuration`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     labels:
       heritage: deckhouse
       name: d8-provider-cluster-configuration
     name: d8-provider-cluster-configuration
     namespace: kube-system
   type: Opaque
   data:
     cloud-provider-cluster-configuration.yaml: <СОДЕРЖИМОЕ_zvirt-cluster.b64>
     cloud-provider-discovery-data.json: <СОДЕРЖИМОЕ_zvirt-discovery.b64>
   ```

6. **Включите модуль:**

   ```shell
   d8 k apply -f zvirt-provider-secret.yaml
   d8 system module enable cloud-provider-zvirt
   ```

   или только `ModuleConfig` (если секрет уже создан):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-zvirt
   spec:
     version: 1
     enabled: true
   ```

7. **Поды модуля:**

   ```shell
   d8 k get pods -n d8-cloud-provider-zvirt
   ```

8. **`ZvirtInstanceClass` и `NodeGroup`:**

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ZvirtInstanceClass
   metadata:
     name: worker
   spec:
     numCPUs: 4
     memory: 8192
     template: debian-bookworm
     vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker-zvirt
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: ZvirtInstanceClass
         name: worker
       minPerZone: 1
       maxPerZone: 2
       zones:
         - default
   ```

   Примените манифест:

   ```shell
   d8 k apply -f zvirt-instanceclass-nodegroup.yaml
   ```

9. **Проверка:**

   ```shell
   d8 k get nodes -o wide
   d8 k -n d8-cloud-provider-zvirt get machine
   ```

   При проблемах с заказом машин см. [«Устранение неполадок»](#устранение-неполадок).

## Размещение приложений на разных площадках

Чтобы направлять поды на статические или облачные узлы, используйте **affinity** и **anti-affinity**, **taints** и **tolerations**, а также распределение по топологии (**topology spread**), если это поддерживает ваше приложение.

Учитывайте задержку и пропускную способность канала между ЦОД и облаком для сервисов с большим трафиком между подами.

Для входа трафика заранее решите, на каких узлах будут **Ingress** или **LoadBalancer** и как маршрутизировать трафик до них.

## Хранилище

В гибриде часто нужны **несколько StorageClass**: отдельно для томов на статических узлах и отдельно для томов на облачных — провайдер и provisioner должны совпадать с площадкой, где запущен под.

По провайдерам:

- **OpenStack** — тома Cinder, `provisioner: csi-cinderplugin`; пошаговый пример и команда `openstack volume type list` — в подразделе **«Подключение хранилища (OpenStack)»** в разделе про OpenStack выше.
- **Yandex Cloud** — типы дисков и CSI в [модуле `cloud-provider-yandex`](/modules/cloud-provider-yandex/); в примере `ModuleConfig` на странице выше может задаваться `storageClass.default` (например, `network-ssd`).
- **VCD** — см. [модуль `cloud-provider-vcd`](/modules/cloud-provider-vcd/) и раздел virtualization про VCD.
- **vSphere** — CSI и автоматически создаваемые StorageClass по datastore: [модуль `cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).
- **zVirt** — [хранилище zVirt](../virtualization/zvirt/storage.html) и документация модуля [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/).

## Отказоустойчивость

Если недоступно облако, остаются статические узлы: приложения должны переносить потерю части реплик (число реплик, **PDB**).

При обрыве канала между площадками страдает связность всего кластера; для etcd и мастеров важен **кворум** и согласованность данных.

В типовом гибриде **мастера** остаются на статике, в облаке заказываются в основном **рабочие** узлы.

## Устранение неполадок

### Общие симптомы

- **Узел в `NotReady`.** Проверьте сеть до API Kubernetes, kubelet, маршруты, MTU и фаервол между площадками.
- **Поды не доходят до облачной площадки или обратно.** Проверьте [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode) и [сетевые политики](../../configuration/network/policy/configuration.html), не режется ли VXLAN.
- **Под не стартует на облачном узле.** Проверьте taints, доступ к registry и образам, наличие нужного **StorageClass** на этой площадке (см. [«Хранилище»](#хранилище)).
- **Проблемы с DNS.** Проверьте доступность DNS и сетевые политики на egress с узла.
- **Обрывы или нестабильная сеть.** Сверьте MTU на всём пути между статикой и облаком.

### Заказ машин и Machine Controller Manager

Логи **`machine-controller-manager`** всегда смотрите в `d8-cloud-instance-manager`. Объекты `Machine` / `MachineSet` — в namespace **своего** модуля провайдера.

**OpenStack:**

```shell
d8 k -n d8-cloud-provider-openstack get machine
d8 k -n d8-cloud-provider-openstack get machineset
d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
```

**Yandex Cloud:**

```shell
d8 k -n d8-cloud-provider-yandex get machine
d8 k -n d8-cloud-provider-yandex get machineset
d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
```

**VCD:**

```shell
d8 k -n d8-cloud-provider-vcd get machine
d8 k -n d8-cloud-provider-vcd get machineset
d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
```

**VMware vSphere:**

```shell
d8 k -n d8-cloud-provider-vsphere get machine
d8 k -n d8-cloud-provider-vsphere get machineset
d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
```

**zVirt:**

```shell
d8 k -n d8-cloud-provider-zvirt get machine
d8 k -n d8-cloud-provider-zvirt get machineset
d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
```

### VMware vSphere: нет IP узла, bashible `000_discover_node_ip.sh`, ошибка `jq`

Такое встречалось после добавления **новых worker-узлов** из vSphere (в том числе на **AlmaLinux 9**): узлы в `Ready`, часть подов работает, часть — нет.

**Со стороны Kubernetes** может появляться ошибка вида `no preferred addresses found; known addresses: []`. В выводе `d8 k get pods -A -o wide` у части подов в колонке адреса узла стоит `<none>`. Системные DaemonSet'ы (например, `chrony`, `node-exporter`, компоненты SDS или виртуализации) могут уходить в `CreateContainerConfigError`, `CrashLoopBackOff` или зависать в `Init`, тогда как на старых узлах всё в порядке.

**На узле** в логах bashible может повторяться ошибка шага `000_discover_node_ip.sh`, например:

```text
Failed to execute step /var/lib/bashible/bundle_steps/000_discover_node_ip.sh ... retry in 10 seconds.
jq: error (at <stdin>:674): Cannot iterate over null (null)
```

Bashible пытается определить **основной IP** узла из данных Kubernetes. Если структура адресов не та, скрипт падает, шаг перезапускается, а у подов не оказывается ожидаемого адреса для конфигурации.

**Что проверить:**

1. Вывод `d8 k get node <имя-узла> -o yaml` — секция `status.addresses`, в первую очередь **InternalIP**; он должен совпадать с реальной сетью стыка с кластером.
2. Поды и логи модуля **`cloud-provider-vsphere`**: нет ли ошибок при синхронизации узла; совпадают ли IP ВМ в vSphere с подсетью из **`internalNetworkCIDR`** и сетью **`mainNetwork`** в `VsphereClusterConfiguration`.
3. Совпадают ли **hostname** ВМ, имя Node в Kubernetes и идентификаторы в облаке. Сверьте версию DKP с известными изменениями в шагах обнаружения IP на узле.

Если `status.addresses` пустой или не сходится с сетью, без разбора конфигурации vSphere, Node и логов bashible на вашей ревизии DKP обычно не обойтись — обращайтесь в сопровождение.

---

*Черновик документа. Детали зависят от версии DKP; краткий обзор см. на странице [Гибридная интеграция](./overview.html).*
