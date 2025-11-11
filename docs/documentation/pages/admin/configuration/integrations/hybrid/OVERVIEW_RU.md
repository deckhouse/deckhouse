---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Deckhouse Kubernetes Platform (DKP) имеет возможность использовать ресурсы облачных провайдеров для расширения ресурсов статических кластеров. В данный момент поддерживается интеграция с облаками на базе [OpenStack](../public/openstack/connection-and-authorization.html) и [vSphere](../virtualization/vsphere/authorization.html).

Гибридный кластер представляет собой объединенные в один кластер bare-metal-узлы и узлы провайдера. Для создания такого кластера необходимо наличие L2-сети между всеми узлами кластера.

{% alert level="info" %}
В Deckhouse Kubernetes Platform есть возможность задавать префикс для имени CloudEphemeral-узлов, добавляемых в гибридный кластер c master-узлами типа Static.
Для этого используйте параметр [`instancePrefix`](/modules/node-manager/configuration.html#parameters-instanceprefix) модуля `node-manager`. Префикс, указанный в параметре, будет добавляться к имени всех добавляемых в кластер узлов типа CloudEphemeral. Задать префикс для определенной NodeGroup нельзя.
{% endalert %}

## Гибридный кластер с vSphere

Выполните следующие шаги:

1. Удалите `flannel` из `kube-system`:

   ```shell
   d8 k -n kube-system delete ds flannel-ds
   ```

1. Настройте интеграцию и пропишите необходимые для работы параметры.

{% alert level="warning" %}
`Cloud-controller-manager` синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (DKP прописывает `static://` на узлы в `.spec.providerID`, а `cloud-controller-manager` такие узлы игнорирует).
{% endalert %}

## Гибридный кластер с OpenStack

Выполните следующие шаги:

1. Удалите `flannel` из `kube-system`:

   ```shell
   d8 k -n kube-system delete ds flannel-ds
   ```

2. Настройте интеграцию и пропишите необходимые для работы параметры.
3. Создайте один или несколько кастомных ресурсов [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass).
4. Создайте один или несколько кастомных ресурсов [NodeGroup](/modules/node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

{% alert level="warning" %}
`Cloud-controller-manager` синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому, если узел Kubernetes запущен не с параметром `--cloud-provider=external`, он автоматически игнорируется (DKP прописывает `static://` на узлы в `.spec.providerID`, а `cloud-controller-manager` такие узлы игнорирует).
{% endalert %}

### Подключение storage

Если вам требуются PersistentVolumes на узлах, подключаемых к кластеру из OpenStack, необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно с помощью следующей команды:

```shell
openstack volume type list
```

Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # Обязательно должно быть так.
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```

## Гибридный кластер с Yandex Cloud

Для создания гибридного кластера, объединяющего статические узлы и узлы в Yandex Cloud, выполните описанные далее шаги.

### Предварительные требования

- Рабочий кластер с параметром `clusterType: Static`.
- Контроллер CNI переведён в режим VXLAN. Подробнее — [настройка `tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode).
- Настроенная сетевая связность между Yandex Cloud и сетью узлов статического кластера согласно [необходимым сетевым политикам для работы DKP](../../configuration/network/policy/).

### Шаги по настройке

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
   sshPublicKey: <ssh-rsa SSHKEY>
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

Перед началом убедитесь, что выполнены следующие условия:

- **Инфраструктура**:
  - Установлен bare-metal кластер DKP.
  - Настроен тенант в VCD [с выделенными ресурсами](../virtualization/vcd/connection-and-authorization.html).
  - Настроена сетевая связанность между сетью узлов статического кластера и VCD (на уровне L2, либо на уровне L3 с доступами по портам согласно [необходимым сетевым политикам для работы DKP](../../configuration/network/policy/)).
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
   - `mainNetwork` — имя сети, в которой будут размещаться облачные ноды в вашем VCD кластере.
   - `internalNetworkCIDR` — CIDR-адресация указанной сети.
   - `organization` — название вашей VCD организации.
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
