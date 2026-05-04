---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) имеет возможность использовать ресурсы облачных провайдеров для расширения ресурсов статических кластеров. В данный момент поддерживается интеграция с облаками на базе [OpenStack](../public/openstack/connection-and-authorization.html), [Yandex Cloud](../public/yandex/authorization.html) и [VMware vCloud Director (VCD)](../virtualization/vcd/connection-and-authorization.html).

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы провайдера. Для создания такого кластера необходимо наличие L2-сети между всеми узлами кластера.

{% alert level="info" %}
В Deckhouse Kubernetes Platform есть возможность задавать префикс для имени CloudEphemeral-узлов, добавляемых в гибридный кластер c master-узлами типа Static.
Для этого используйте параметр [`instancePrefix`](/modules/node-manager/configuration.html#parameters-instanceprefix) модуля `node-manager`. Префикс, указанный в параметре, будет добавляться к имени всех добавляемых в кластер узлов типа CloudEphemeral. Задать префикс для определенной NodeGroup нельзя.
{% endalert %}

## Гибридный кластер с Yandex Cloud

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в Yandex Cloud с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования для Yandex Cloud

- Рабочий кластер с параметром `clusterType: Static`.
- Контроллер CNI переведён в режим VXLAN. Подробнее — [настройка `tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
- Настроенная сетевая связность между Yandex Cloud и сетью узлов статического кластера согласно [необходимым сетевым политикам для работы DKP](../../configuration/network/policy/configuration.html).

### Добавление автоматически создаваемых узлов в Yandex Cloud

1. Создайте Service Account в нужном каталоге Yandex Cloud:

   - Назначьте роль `editor`.
   - Предоставьте доступ к используемой VPC с ролью `vpc.admin`.

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

     Значения в `masterNodeGroup` не имеют значения, так как master-узлы не разворачиваются.

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

1. Удалите объект `ValidatingAdmissionPolicyBinding`, чтобы избежать конфликтов:

   ```shell
   d8 k delete validatingadmissionpolicybindings.admissionregistration.k8s.io heritage-label-objects.deckhouse.io
   ```

1. Примените два созданных на предыдущем шаге манифеста в кластере.

1. После применения дождитесь активации модуля `cloud-provider-yandex` и появления CRD yandexinstanceclasses:

   ```shell
   d8 k get mc cloud-provider-yandex
   d8 k get crd yandexinstanceclasses
   ```

1. Внесите необходимые значения в приведённые ниже манифесты и примените их в кластере. Пример:

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

   Параметр `mainSubnet` должен содержать ID подсети из Yandex Cloud, которая используется для связи с вашей инфраструктурой (L2-связность с группами статических узлов).

   После применения манифестов начнётся заказ виртуальных машин в Yandex Cloud, управляемых модулем `node-manager`.

1. Для диагностики состояния и поиска возможных проблем проверьте логи `machine-controller-manager`:

   ```shell
   d8 k -n d8-cloud-provider-yandex get machine
   d8 k -n d8-cloud-provider-yandex get machineset
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager
   ```

## Гибридный кластер с VCD

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в VMware vCloud Director (VCD) с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования для VCD

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

### Добавление автоматически создаваемых узлов в VCD

1. Создайте файл `cloud-provider-vcd-mc.yaml` со следующим содержимым:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-vcd
   spec:
     version: 1
     enabled: true
     settings:
       mainNetwork: <NETWORK_NAME>
       organization: <ORGANIZATION>
       virtualDataCenter: <VDC_NAME>
       virtualApplicationName: <VAPP_NAME>
       sshPublicKey: <SSH_PUBLIC_KEY>
       provider:
         server: <API_URL>
         username: <USER_NAME>
         password: <PASSWORD>
         apiToken: <API_TOKEN>
         insecure: false
   ```

   Где:
   - `mainNetwork` — имя сети, в которой будут размещаться облачные узлы в кластере VCD.
   - `organization` — название вашей организации в VCD.
   - `virtualDataCenter` — имя виртуального дата-центра.
   - `virtualApplicationName` — имя vApp, где будут создаваться узлы (например, `dkp-vcd-app`).
   - `sshPublicKey` — публичный SSH-ключ для доступа к узлам.
   - `provider.server` — URL-адрес API вашего VCD.
   - `provider.apiToken` — токен доступа пользователя с правами администратора в VCD.
   - `provider.username` — имя статического пользователя, от имени которого будет происходить взаимодействие с VCD.
   - `provider.password` — пароль пользователя с правами администратора в VCD.
   - `provider.insecure` — установите значение `true`, если VCD использует самоподписанный TLS-сертификат.

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-vcd-mc.yaml
   d8 k get mc cloud-provider-vcd
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
