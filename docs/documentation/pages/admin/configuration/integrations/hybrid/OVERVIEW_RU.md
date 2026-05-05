---
title: Гибридная интеграция
permalink: ru/admin/integrations/hybrid/overview.html
lang: ru
---

Гибридный кластер — это кластер DKP, в котором базовая часть размещается в собственной инфраструктуре, а дополнительные worker-узлы подключаются из внешнего облака или среды виртуализации, например из Yandex Cloud, VCD или vSphere.

Такой подход позволяет увеличивать вычислительные мощности, размещать часть нагрузки во внешней инфраструктуре или постепенно переносить туда сервисы без создания отдельного Kubernetes-кластера. Для приложений при этом сохраняется единая плоскость управления Kubernetes: общий API, единые ресурсы, единые механизмы планирования, мониторинга, обновления и эксплуатации.

В типовом сценарии сначала разворачивается кластер с [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype). В таком кластере control plane и базовые worker-узлы размещаются на собственных серверах или заранее подготовленных виртуальных машинах. Затем включается модуль соответствующего cloud provider, через который DKP получает информацию о внешней инфраструктуре и может работать с размещёнными в ней узлами.

В DKP гибридная архитектура строится на сочетании разных типов групп узлов:

- [`Static`](../../../../architecture/cluster-and-infrastructure/node-management/static-nodes.html) — постоянно существующие узлы, которые создаются и обслуживаются пользователем;
- [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) — узлы, которые DKP создаёт и удаляет автоматически через API провайдера;
- [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) — узлы, которые создаются вручную во внешней инфраструктуре и затем подключаются к кластеру.

Подключение узлов из внешней инфраструктуры может выполняться двумя способами:

- **Автоматическое создание узлов**. Используется тип узлов `CloudEphemeral`. Параметры виртуальных машин описываются ресурсом `*InstanceClass` (например, [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass)), а количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup). После применения этих ресурсов DKP обращается к API провайдера, создаёт виртуальные машины, подготавливает их и подключает к существующему кластеру как worker-узлы.
- **Подключение вручную созданных узлов**. Используется тип узлов `CloudStatic`. Виртуальные машины создаются пользователем вручную во внешней инфраструктуре, после чего подключаются к кластеру с помощью bootstrap-скрипта DKP как worker-узлы.

В этом разделе описаны общие требования к гибридным кластерам, предварительная подготовка инфраструктуры и добавление узлов через поддерживаемых провайдеров.

## Общие сетевые требования

Между статическими узлами кластера и узлами, размещёнными во внешней инфраструктуре, должна быть настроена сетевая связность, достаточная для работы компонентов DKP.

Подключаемые узлы должны иметь доступ к Kubernetes API, DNS и необходимым адресам внешних сервисов, включая container registry. Компоненты DKP, взаимодействующие с внешней инфраструктурой, должны иметь доступ к API соответствующего провайдера.

Полный перечень соединений приведён в разделе [Сетевое взаимодействие](../../../../reference/network_interaction.html), а рекомендации по ограничениям доступа — в разделе [Настройка сетевых политик](../../configuration/network/policy/configuration.html).

Дополнительно рекомендуется проверить:

- маршрутизацию между сетями статических и подключаемых узлов;
- одинаковое значение MTU на всём сетевом пути, особенно при использовании туннелей;
- доступность DNS-серверов и разрешённых внешних адресов;
- доступность Kubernetes API для подключаемых узлов;
- параметры инкапсуляции трафика при использовании Cilium, включая [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), если между площадками применяется фильтрация трафика.

Конкретные требования к сетям, подсетям, шаблонам виртуальных машин, учётным данным и дополнительным параметрам зависят от используемого провайдера инфраструктуры и приведены в разделе «Предварительные требования» для соответствующего провайдера.

## Гибридный кластер с Yandex Cloud

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в Yandex Cloud с использованием Deckhouse Kubernetes Platform (DKP).

### Предварительные требования для Yandex Cloud

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и VPC Yandex Cloud настроена сетевая связность.
- Узлы, создаваемые в Yandex Cloud, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Авторизация в Yandex Cloud](../public/yandex/authorization.html):
  - подготовлен сервисный аккаунт;
  - выбран каталог, в котором будут создаваться ресурсы;
  - настроены необходимые роли и доступ к используемой VPC.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в Yandex Cloud

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
   yc iam
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

### Предварительные требования для VCD

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин в VCD настроена сетевая связность.
- Узлы, создаваемые в VCD, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vCloud Director](../virtualization/vcd/connection-and-authorization.html):
  - настроен тенант в VCD с выделенными ресурсами;
  - подготовлена учётная запись VCD со статичным паролем и правами администратора;
  - настроена рабочая сеть в VCD с включённым DHCP-сервером;
  - подготовлены необходимые ресурсы VCD: VDC, vApp, шаблоны, политики и другие параметры.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в VCD

1. Создайте файл, например, `cloud-provider-vcd-mc.yaml` с ресурсом ModuleConfig:

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
         insecure: false
   ```

   Где:
   - `mainNetwork` — имя сети, в которой будут размещаться облачные узлы в VCD;
   - `organization` — имя Organization в VCD;
   - `virtualDataCenter` — имя Virtual Data Center в VCD;
   - `virtualApplicationName` — имя vApp, где будут создаваться узлы, например dkp-vcd-app;
   - `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
   - `provider.server` — URL-адрес API VCD;
   - `provider.username` — имя пользователя VCD;
   - `provider.password` — пароль пользователя VCD;
   - `provider.insecure` — установите значение true, если VCD использует самоподписанный TLS-сертификат.

   Если для аутентификации используется токен, вместо `username` и `password` укажите `apiToken`:

   ```yaml
   provider:
     server: <API_URL>
     apiToken: <API_TOKEN>
     username: ""
     password: ""
     insecure: false
   ```

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-vcd-mc.yaml
   d8 k get mc cloud-provider-vcd
   ```

1. Убедитесь, что все поды в пространстве имён `d8-cloud-provider-vcd` находятся в состоянии `Running`:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Убедитесь, что в кластере созданы StorageClass для VCD:

   ```shell
   d8 k get sc
   ```

1. Создайте файл, например, `vcd-instanceclass-nodegroup.yaml` с ресурсами [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup):

   ```yaml
   ---
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
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: VCDInstanceClass
         name: worker
       maxPerZone: 2
       minPerZone: 1
     nodeTemplate:
       labels:
         node-role/worker: ""
    ```

1. Примените манифест:

   ```shell
   d8 k apply -f vcd-instanceclass-nodegroup.yaml
   ```

   После применения манифеста DKP начнёт создавать виртуальные машины в VCD, управляемые модулем `node-manager`.

1. Убедитесь, что в кластере появилось требуемое количество узлов:

   ```shell
   d8 k get nodes -o wide
   ```

1. При сбоях создания ВМ проверьте объекты Machine, MachineSet и логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

### Добавление вручную созданных узлов в VCD

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) включён и настроен.
- Компоненты модуля `cloud-provider-vcd` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vcd get pods -o wide
  ```

- В кластере созданы StorageClass для VCD:
  
  ```shell
  d8 k get sc
  ```

- В VCD создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в VCD совпадает с hostname внутри операционной системы.
- В дополнительных параметрах ВМ в VCD задано значение:

  ```text
  disk.EnableUUID = 1
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) модуля `cloud-provider-vcd`.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. Создайте файл, например `cloud-static-nodegroup.yaml`, с ресурсом NodeGroup и типом узлов CloudStatic:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Примените манифест:

   ```shell
   d8 k apply -f cloud-static-nodegroup.yaml
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Пример ожидаемого результата:

   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```

1. Получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=cloud-static

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > bootstrap.b64
   ```

1. Скопируйте bootstrap-скрипт на подключаемую виртуальную машину:

   ```shell
   scp bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Подключитесь к виртуальной машине по SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. На виртуальной машине наначьте права и запустите bootstrap-скрипт:

   ```shell
   base64 -d /tmp/bootstrap.b64 > /tmp/bootstrap.sh
   chmod +x /tmp/bootstrap.sh

   sudo bash /tmp/bootstrap.sh
   ```

   После запуска bootstrap-скрипт установит необходимые компоненты, настроит container runtime, kubelet и подключит узел к кластеру.

1. На master-узле проверьте появление нового узла:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```

## Гибридный кластер с vSphere

Далее описан процесс создания гибридного кластера, объединяющего статические (bare-metal) узлы и облачные узлы в vSphere с использованием Deckhouse Kubernetes Platform (DKP).

В этом сценарии исходный кластер DKP уже развёрнут как статический кластер. Control-plane остаётся на статических узлах, а новые worker-узлы создаются во vSphere через модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

Параметры виртуальных машин задаются ресурсом [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), а количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).

### Предварительные требования для vSphere

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин во vSphere настроена сетевая связность.
- Узлы, создаваемые во vSphere, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vSphere](../virtualization/vsphere/authorization.html):
  - настроен доступ к vCenter;
  - подготовлена учётная запись vSphere с необходимыми привилегиями;
  - подготовлен шаблон виртуальной машины;
  - настроены сети, Datastore, теги регионов и зон.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

### Добавление автоматически создаваемых узлов в vSphere

Для подключения уже работающего статического кластера к vCenter используйте ресурс [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

В параметре `spec.settings` укажите параметры доступа к vCenter, сетевые настройки, теги региона и зоны, а также SSH-ключи, которые будут добавлены на создаваемые виртуальные машины.

Пример конфигурации и описание доступных параметров приведены в [примерах модуля](/modules/cloud-provider-vsphere/examples.html) и в разделе [Конфигурация модуля `cloud-provider-vsphere`](/modules/cloud-provider-vsphere/configuration.html).

1. Создайте файл, например `vsphere-mc.yaml`, с ModuleConfig для модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/):

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

   - `host` — адрес vCenter;
   - `username`, `password` — учётные данные пользователя vSphere;
   - `insecure` — отключение проверки TLS-сертификата vCenter;
   - `vmFolderPath` — папка, в которой будут создаваться виртуальные машины;
   - `regionTagCategory`, `zoneTagCategory` — категории тегов региона и зоны;
   - `region` — тег региона;
   - `zones` — список зон, в которых можно создавать узлы;
   - `internalNetworkNames` — список сетей vSphere для подключения создаваемых узлов;
   - `sshKeys` — публичные SSH-ключи, которые будут добавлены на создаваемые виртуальные машины.

1. Примените конфигурацию модуля:

   ```shell
   d8 k apply -f vsphere-mc.yaml
   ```

1. Дождитесь готовности модуля `cloud-provider-vsphere`:

   ```shell
   d8 k get moduleconfig cloud-provider-vsphere
   d8 k get pods -n d8-cloud-provider-vsphere -o wide
   ```

1. Создайте файл, например `vsphere-instance.yaml`, c ресурсами [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup) со значением `nodeType: CloudEphemeral`:

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

   Где:

   - [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) описывает параметры виртуальной машины, которая будет создана во vSphere;
   - [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает группу узлов, которую DKP должен поддерживать в кластере;
   - `nodeType: CloudEphemeral` означает, что узлы будут создаваться автоматически через облачный провайдер;
   - `cloudInstances.classReference` указывает на VsphereInstanceClass;
   - `cloudInstances.zones` должен содержать зоны из списка `zones` в ModuleConfig.

1. Примените манифест:

   ```shell
   d8 k apply -f vsphere-instance.yaml
   ```

   После применения манифеста DKP начнёт создавать виртуальную машину во vSphere. После загрузки ВМ kubelet подключится к Kubernetes API, и новый узел появится в кластере.

1. Проверьте состояние узлов:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                             STATUS   ROLES                  AGE   VERSION
   static-master-0                  Ready    control-plane,master   1h    v1.33.10
   ephemeral-1ca02a5b-7588b-k89dc   Ready    ephemeral              10m   v1.33.10
   ```

1. При сбоях создания ВМ проверьте объекты Machine, MachineSet и логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

   Также проверьте события в кластере:

   ```shell
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

### Добавление вручную созданных узлов в vSphere

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) включён и настроен.
- Компоненты модуля `cloud-provider-vsphere` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vsphere get pods -o wide
  ```

- В кластере созданы StorageClass для vSphere:
  
  ```shell
  d8 k get sc
  ```

- В vSphere создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в vSphere совпадает с hostname внутри операционной системы.
- В дополнительных параметрах ВМ в vSphere задано значение:

  ```text
  disk.EnableUUID = TRUE
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) конфигурации модуля `cloud-provider-vsphere`.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. Создайте файл, например `cloud-static-nodegroup.yaml`, с ресурсом NodeGroup и типом узлов CloudStatic:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Примените манифест:

   ```shell
   d8 k apply -f cloud-static-nodegroup.yaml
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Пример ожидаемого результата:

   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```

1. Получите bootstrap-скрипт для созданной NodeGroup:

   ```shell
   NODE_GROUP=cloud-static

   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} \
     -o jsonpath='{.data.bootstrap\.sh}' > bootstrap.b64
   ```

1. Скопируйте bootstrap-скрипт на подключаемую виртуальную машину:

   ```shell
   scp bootstrap.b64 <USER>@<NODE_IP>:/tmp/bootstrap.b64
   ```

1. Подключитесь к виртуальной машине по SSH:

   ```shell
   ssh <USER>@<NODE_IP>
   ```

1. На виртуальной машине наначьте права и запустите bootstrap-скрипт:

   ```shell
   base64 -d /tmp/bootstrap.b64 > /tmp/bootstrap.sh
   chmod +x /tmp/bootstrap.sh

   sudo bash /tmp/bootstrap.sh
   ```

   После запуска bootstrap-скрипт установит необходимые компоненты, настроит container runtime, kubelet и подключит узел к кластеру.

1. На master-узле проверьте появление нового узла:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.135
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.152
   ```
