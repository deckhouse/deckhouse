---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Гибридный кластер — это кластер DKP, в котором одновременно используются узлы, размещённые в собственной инфраструктуре, и узлы, созданные во внешнем облаке или среде виртуализации.

Обычно постоянная часть нагрузки размещается на собственных серверах (bare-metal или виртуальных машинах), а дополнительные ресурсы подключаются по мере необходимости через облачного провайдера. Такой подход позволяет объединить локальную инфраструктуру и облачные ресурсы в рамках одного кластера Kubernetes.

В DKP гибридная архитектура строится на сочетании разных типов групп узлов:

- Static — постоянно существующие узлы, управляемые пользователем;
- CloudEphemeral — узлы, создаваемые автоматически через API облачного провайдера.

Гибридный кластер может использоваться для следующих сценариев:

- масштабирование локальной инфраструктуры за счёт облачных ресурсов;
- постепенная миграция сервисов из собственного ЦОД в облако;
- временное увеличение вычислительных мощностей при пиковых нагрузках;
- размещение различных типов рабочих нагрузок в подходящей среде.

В этом разделе описаны общие требования к гибридным кластерам, особенности их архитектуры и настройка поддерживаемых провайдеров инфраструктуры.

## Общие требования

### Кластер и типы узлов

Для большинства гибридных сценариев исходный кластер создаётся с параметром `clusterType: Static` ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).

Облачные виртуальные машины описываются ресурсами `*InstanceClass` и [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов `CloudEphemeral`.

{% alert level="info" %}
В некоторых примерах и устаревших версиях API вместо `CloudEphemeral` может использоваться прежнее обозначение `Cloud`.
{% endalert %}

### Сеть

Между сетью статических узлов и сетью облачных виртуальных машин должна быть обеспечена связность на уровне L3. Также необходимо открыть сетевой доступ для компонентов DKP.

Полный перечень соединений приведён в разделе [Сетевое взаимодействие](../../../../reference/network_interaction.html), а рекомендации по ограничениям доступа — в разделе [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

Дополнительно рекомендуется проверить:

- одинаковое значение MTU на всём сетевом пути, особенно при использовании туннелей;
- доступность DNS-серверов и разрешённых внешних адресов;
- доступность Kubernetes API для подключаемых узлов;
- параметры инкапсуляции трафика при использовании Cilium, включая [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), если между площадками применяется фильтрация трафика.

Общая L2-сеть между статическими и облачными узлами не требуется. В большинстве случаев достаточно L3-маршрутизации, корректного MTU и открытых необходимых портов.

Требования к подсетям, шаблонам виртуальных машин, учётным данным и дополнительным параметрам зависят от используемого провайдера инфраструктуры и приведены в разделе **«Предварительные требования»** для соответствующего провайдера ниже.

## Гибридный кластер с Yandex Cloud

Для создания гибридного кластера, объединяющего статические узлы и узлы в Yandex Cloud, выполните описанные далее шаги.

### Предварительные требования

- Кластер с `clusterType: Static`, соответствующий [общим требованиям](#общие-требования) к сети, DNS и подготовке узлов.
- Сервисный аккаунт и каталог в Yandex Cloud, настроенные согласно разделу [Авторизация в Yandex Cloud](../public/yandex/authorization.html).
- Для туннелирования трафика подов при использовании Cilium — режим **VXLAN**. Подробнее см. параметр [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
- Сетевая связность между сетью статического кластера и VPC Yandex Cloud в соответствии с требованиями раздела [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

### Шаги по настройке

1. Создайте Service Account в нужном каталоге Yandex Cloud:

   - Назначьте роль `editor`.
   - Предоставьте доступ к используемой VPC с ролью `vpc.admin`.

   Пример создания через Yandex CLI:

   ```shell
   export FOLDER_ID=b1g...
   yc iam service-account create --name dkp-hybrid --folder-id "$FOLDER_ID"
   export SA_ID=$(yc iam service-account get --name dkp-hybrid --folder-id "$FOLDER_ID" --format json | jq -r .id)
   yc resource-manager folder add-access-binding "$FOLDER_ID" --role editor --subject "serviceAccount:${SA_ID}"
   yc vpc network list --folder-id "$FOLDER_ID"
   yc ia
   ```

   Подробнее в разделе [Авторизация в Yandex Cloud](../public/yandex/authorization.html)

1. Создайте секрет `d8-provider-cluster-configuration` с нужными данными. Пример содержимого `cloud-provider-cluster-configuration.yaml`:

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

   Значения параметров:
   - `nodeNetworkCIDR` — CIDR сети, который включает адреса всех используемых подсетей узлов в Yandex Cloud;
   - `cloudID` — ID вашего облака;
   - `folderID` — ID каталога;
   - `serviceAccountJSON` — service account в каталоге, выгруженный в формате JSON;
   - `sshPublicKey` — публичный ключ, который будет добавлен на разворачиваемые машины.

   Поля в `masterNodeGroup` в гибриде часто формальны: **master-узлы в Yandex не создаются**, если кластер изначально статический.

1. Заполните значения для файла `data.cloud-provider-discovery-data.json` в этом же секрете. Пример:

   ```yaml
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

    Значения параметров:
    - `internalNetworkIDs` — список ID сетей в Yandex Cloud, через которые обеспечивается внутренняя связность между узлами.
    - `zoneToSubnetIdMap` — отображение зон на соответствующие подсети внутри указанных сетей (по одной подсети на зону).
    - `shouldAssignPublicIPAddress: true` — указывает, требуется ли назначать публичные IP-адреса для создаваемых узлов. Для зон, в которых подсети отсутствуют, допустимо использовать значение `empty`.

1. Закодируйте полученные выше файлы YandexClusterConfiguration и YandexCloudDiscoveryData в формат Base64. Затем вставьте закодированные строки в поля `cloud-provider-cluster-configuration.yaml` и `cloud-provider-discovery-data.json` секрета, как показано в примере ниже:

   ```yaml
   apiVersion: v1
   data:
     cloud-provider-cluster-configuration.yaml: <YANDEXCLUSTERCONFIGURATION_BASE64_ENCODED>
     cloud-provider-discovery-data.json: <YANDEXCLOUDDISCOVERYDATA-BASE64-ENCODED>
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

1. Удалите объект ValidatingAdmissionPolicyBinding, чтобы избежать конфликтов:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

1. Примените два созданных на предыдущем шаге манифеста в кластере (секрет и ModuleConfig из шага 4, при необходимости — в одном файле).

1. После применения дождитесь активации модуля `cloud-provider-yandex` и появления ресурса YandexInstanceClass:

   ```shell
   d8 k get mc cloud-provider-yandex
   d8 k get crd yandexinstanceclass
   ```

1. Создайте YandexInstanceClass и NodeGroup. Пример:

   ```yaml
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: CloudEphemeral
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

   В `mainSubnet` укажите ID подсети в Yandex Cloud, из которой ВМ доступны сети статических узлов.

   Примените манифест в кластере:

   ```shell
   d8 k apply -f yandex-instanceclass-nodegroup.yaml
   ```

   После применения манифестов начнётся заказ виртуальных машин в Yandex Cloud, управляемых модулем `node-manager`.

1. Для диагностики состояния и поиска возможных проблем проверьте логи `machine-controller-manager`:

   ```shell
   d8 k -n d8-cloud-provider-yandex get machine
   d8 k -n d8-cloud-provider-yandex get machineset
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
   ```

## Гибридный кластер с VCD

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в VMware vCloud Director (VCD) с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования

Перед началом убедитесь, что выполнены следующие условия:

- **Инфраструктура**:
  - Установлен bare-metal кластер DKP.
  - Настроен тенант в VCD [с выделенными ресурсами](../virtualization/vcd/connection-and-authorization.html).
  - Настроена сетевая связанность между сетью узлов статического кластера и VCD (на уровне L2, либо на уровне L3 с доступами по портам согласно [необходимым сетевым политикам для работы DKP](../../configuration/network/policy/configuration.html)).
  - Настроена рабочая сеть в VCD с включённым DHCP-сервером.
  - Создан пользователь со статичным паролем и правами администратора VCD.

- **Настройки ПО**:
  - Контроллер CNI переведён в режим VXLAN. Подробнее — [настройка `tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
  - Подготовлен [список необходимых ресурсов VCD](../virtualization/vcd/connection-and-authorization.html) (VDC, VAPP, шаблоны, политики и т.д.).

### Настройка

1. Создайте файл конфигурации `cloud-provider-vcd-token.yml` со следующим содержимым:

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

   Где:
   - `mainNetwork` — имя сети, в которой будут размещаться облачные узлы в кластере VCD.
   - `internalNetworkCIDR` — CIDR-адресация указанной сети.
   - `organization` — название вашей организации в VCD.
   - `virtualApplicationName` — имя vApp, где будут создаваться узлы (например, `dkp-vcd-app`).
   - `virtualDataCenter` — имя виртуального датацентра.
   - `template` — шаблон ВМ для создания узлов.
   - `sizingPolicy` и `storageProfile` — соответствующие политики в VCD.
   - `provider.server` — URL-адрес API вашего VCD.
   - `provider.apiToken` — токен доступа (пароль) пользователя с правами администратора в VCD.
   - `provider.username` — имя статического пользователя, от имени которого будет происходить взаимодействие с VCD.
   - `mainNetworkIPAddresses` — список IP-адресов из указанной сети, которые будут выделены для master-узлов.
   - `storageProfile` — имя storage-профиля, определяющего хранилище для дисков создаваемых ВМ.

1. Закодируйте файл `cloud-provider-vcd-token.yml` в Base64:

   ```shell
   base64 -i $PWD/cloud-provider-vcd-token.yml
   ```

1. Создайте секрет со следующим содержимым:

   ```yaml
   apiVersion: v1
   data:
     cloud-provider-cluster-configuration.yaml: <BASE64_СТРОКА_ПОЛУЧЕННАЯ_НА_ПРЕДЫДУЩЕМ_ЭТАПЕ> 
     cloud-provider-discovery-data.json: eyJhcGlWZXJzaW9uIjoiZGVja2hvdXNlLmlvL3YxIiwia2luZCI6IlZDRENsb3VkUHJvdmlkZXJEaXNjb3ZlcnlEYXRhIiwiem9uZXMiOlsiZGVmYXVsdCJdfQo=
   kind: Secret
     metadata:
       labels:
         heritage: deckhouse
         name: d8-provider-cluster-configuration
       name: d8-provider-cluster-configuration
       namespace: kube-system
   type: Opaque
   ```

1. Включите модуль `cloud-provider-vcd`:

   ```shell
   d8 system module enable cloud-provider-vcd
   ```

1. Отредактируйте секрет `d8-cni-configuration`, чтобы значение параметра `mode` определялось из `mc cni-cilium` (измените `.data.cilium` на `.data.necilium` при необходимости).

1. Убедитесь, что все поды в пространстве имён `d8-cloud-provider-vcd` находятся в состоянии `Running`:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Перезагрузите master-узел и дождитесь завершения инициализации.

1. Создайте классы инстансов в VCD:

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
   ```  

1. Создайте ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup):

   ```yaml
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

1. Убедитесь, что в кластере появилось требуемое количество узлов:

   ```shell
   d8 k get nodes -o wide
   ```

## Гибридный кластер с vSphere

Для создания гибридного кластера, объединяющего статические узлы и узлы в VMware vSphere, выполните описанные далее шаги.

В таком сценарии исходный кластер DKP уже развёрнут как статический кластер, а новые worker-узлы создаются в vSphere через модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/). Параметры виртуальных машин задаются ресурсом [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), а количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).

### Предварительные требования

Перед началом создания убедитесь, что выполнены следующие условия:

- Кластер создан с параметром `clusterType: Static` ресурса [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) и соответствует [общим требованиям](#общие-требования): раздел [«Сеть»](#сеть), [Сетевое взаимодействие](../../../../reference/network_interaction.html), [настройка сетевых политик](../../network/policy/configuration.html), доступность Kubernetes API и DNS для новых узлов.
- Выполнены требования из раздела [Подключение и авторизация в VMware vSphere](../virtualization/vsphere/authorization.html):
  - доступ к vCenter из кластера, в первую очередь с master-узлов;
  - подготовлен шаблон виртуальной машины (поле `template` в [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), см. [схемы размещения](../virtualization/vsphere/layout.html));
  - настроены сети, datastore, теги регионов и зон (`mainNetwork`, `datastore` в [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass); `region`, `zones`, `regionTagCategory`, `zoneTagCategory` в [конфигурации модуля](/modules/cloud-provider-vsphere/configuration.html));
  - подготовлена учётная запись vSphere с необходимыми привилегиями.
- Инвентарь vSphere, теги регионов и зон, сети, datastore и путь к шаблону соответствуют выбранной схеме размещения. Подробнее — разделы [Схемы размещения и настройка VMware vSphere](../virtualization/vsphere/layout.html) и [Хранилище и балансировка нагрузки в VMware vSphere](../virtualization/vsphere/storage.html); интеграция со службами — [Интеграция со службами VMware vSphere](../virtualization/vsphere/services.html).
- При использовании модуля [`cni-cilium`](/modules/cni-cilium/) с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), согласованный с [сетевой связностью между площадками](#сеть).

### Шаги по настройке

Подключение к vCenter задают через [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) в `spec.settings` модуля `cloud-provider-vsphere` — так удобнее для уже работающего статического кластера. Пример и описание полей — в [примерах модуля](/modules/cloud-provider-vsphere/docs/examples.html) и в [конфигурации модуля](/modules/cloud-provider-vsphere/configuration.html).

Альтернатива — секрет `kube-system/d8-provider-cluster-configuration` с `VsphereClusterConfiguration` и `VsphereCloudDiscoveryData` в Base64 (часто при установке через dhctl); см. подраздел **«Конфигурация через секрет»** в конце раздела.

Ниже — настройка через `ModuleConfig`.

1. Создайте файл, например `vsphere-mc.yaml`, с `ModuleConfig` для модуля `cloud-provider-vsphere`. В `spec.version` укажите актуальную схему настроек модуля (значение `x-config-version` в OpenAPI настроек модуля).

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vsphere
   spec:
     version: 2
     enabled: true
     settings:
       host: "<VCENTER_FQDN>"
       username: "<USERNAME@DOMAIN.LOCAL>"
       password: "<PASSWORD>"
       insecure: true
       vmFolderPath: "<FOLDER_PATH_UNDER_DATACENTER>"
       regionTagCategory: "<TAG_CATEGORY_FOR_REGION>"
       zoneTagCategory: "<TAG_CATEGORY_FOR_ZONE>"
       region: "<REGION_TAG_NAME_ON_DATACENTER>"
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
       internalNetworkNames:
         - "<PORT_GROUP_NAME_FOR_INTERNAL_IP>"
       sshKeys:
         - "<SSH_PUBLIC_KEY_ONE_LINE>"
   ```

   Значения параметров:
   - `host`, `username`, `password`, `insecure` — доступ к API vCenter;
   - `vmFolderPath` — папка для клонируемых ВМ (см. [схемы размещения](../virtualization/vsphere/layout.html));
   - `regionTagCategory`, `zoneTagCategory`, `region`, `zones` — категории и теги региона/зоны в vSphere; в `NodeGroup` указывают те же имена зон, что в `zones`;
   - `internalNetworkNames` — портовые группы для InternalIP узла; при необходимости публичных адресов — [`externalNetworkNames`](/modules/cloud-provider-vsphere/configuration.html);
   - `sshKeys` — публичные SSH-ключи для создаваемых ВМ.

   {% alert level="warning" %}
   Учётные данные в `ModuleConfig` доступны при чтении объекта в API кластера; для постоянных сценариев используйте принятую у вас модель хранения секретов.
   {% endalert %}

1. Примените манифест и дождитесь готовности модуля:

   ```shell
   d8 k apply -f vsphere-mc.yaml
   d8 system module enable cloud-provider-vsphere
   d8 k get moduleconfig cloud-provider-vsphere
   d8 k get pods -n d8-cloud-provider-vsphere -o wide
   ```

1. Создайте [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup) с `nodeType: CloudEphemeral`, например в файле `vsphere-instance.yaml`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereInstanceClass
   metadata:
     name: ephemeral
   spec:
     numCPUs: 2
     memory: 4096
     rootDiskSize: 40
     template: "<PATH_TO_TEMPLATE_FROM_DATACENTER>"
     mainNetwork: "<PORT_GROUP_NAME>"
     datastore: "<DATASTORE_OR_FOLDER/DATASTORE>"
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ephemeral
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VsphereInstanceClass
         name: ephemeral
       maxPerZone: 1
       minPerZone: 1
       zones:
         - "<ZONE_TAG_NAME_ON_CLUSTER>"
     disruptions:
       approvalMode: Automatic
   ```

   В `cloudInstances.zones` укажите зоны из списка `zones` в `ModuleConfig`. Режимы [`disruptions.approvalMode`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode) — в справочнике NodeGroup.

   ```shell
   d8 k apply -f vsphere-instance.yaml
   ```

1. Проверьте узлы и при сбоях заказа ВМ — объекты `Machine` / `MachineSet` в `d8-cloud-provider-vsphere` и логи `machine-controller-manager` в `d8-cloud-instance-manager`:

   ```shell
   d8 k get nodes -o wide
   d8 k -n d8-cloud-provider-vsphere get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

   При необходимости: `d8 k queue list`. Дополнительные параметры модуля — в [документации `cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

#### Конфигурация через секрет

Если используется секрет `d8-provider-cluster-configuration`:

1. Подготовьте `VsphereClusterConfiguration` (например, layout `Standard`) и `cloud-provider-discovery-data.json` с `VsphereCloudDiscoveryData` — см. [документацию модуля](/modules/cloud-provider-vsphere/).
1. Закодируйте оба файла в Base64 и поместите в секрет `kube-system/d8-provider-cluster-configuration` в ключи `cloud-provider-cluster-configuration.yaml` и `cloud-provider-discovery-data.json`.
1. Примените секрет и `ModuleConfig` с `spec.enabled: true` и корректным `spec.version`.
1. Создайте `VsphereInstanceClass` и `NodeGroup`, как в шаге 3.

   Пример `VsphereClusterConfiguration` для `Standard`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VsphereClusterConfiguration
   sshPublicKey: "<SSH_PUBLIC_KEY>"
   layout: Standard
   vmFolderPath: folder/prefix
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
     server: "<SERVER>"
     username: "<USERNAME@DOMAIN.LOCAL>"
     password: "<PASSWORD>"
     insecure: true
   ```

   Для `VsphereCloudDiscoveryData` минимально укажите `apiVersion`, `kind` и `vmFolderPath`, совпадающий с конфигурацией кластера.

