---
title: Гибридный кластер с vSphere
permalink: ru/admin/integrations/hybrid/vsphere-hybrid.html
lang: ru
search: гибрид с vSphere
description: Подготовка к гибридной интеграции с VMware vSphere в Deckhouse Kubernetes Platform.
---

Далее описан процесс добавления узлов из vSphere в существующий статический кластер Deckhouse Kubernetes Platform (DKP).

Для интеграции с vSphere используется модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/). Он обеспечивает взаимодействие DKP с vCenter, получение информации о виртуальных машинах, работу с параметрами размещения и интеграцию с инфраструктурными возможностями vSphere.

В разделе описаны два способа добавления узлов:

- **Автоматическое создание узлов в vSphere**. DKP создаёт виртуальные машины через API vSphere. Параметры ВМ задаются ресурсом [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), а требуемое количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html).

## Предварительные требования для vSphere

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин во vSphere настроена [сетевая связность](./overview.html#общие-сетевые-требования). Узлы vSphere, добавляемые в кластер, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [«Сетевое взаимодействие»](../../../../reference/network_interaction.html) и [«Настройка сетевых политик»](../../configuration/network/policy/configuration.html). При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.
- Выполнены требования из раздела [«Подключение и авторизация в VMware vSphere»](../virtualization/vsphere/authorization.html):
  - настроен доступ к vCenter;
  - подготовлена учётная запись vSphere с необходимыми привилегиями;
  - подготовлен шаблон виртуальной машины;
  - настроены сети, Datastore, теги регионов и зон.

## Добавление автоматически создаваемых узлов

Для подключения уже работающего статического кластера к vCenter используйте ресурс [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/).

В параметре `spec.settings` укажите параметры доступа к vCenter, сетевые настройки, теги региона и зоны, а также SSH-ключи, которые будут добавлены на создаваемые виртуальные машины.

Пример конфигурации и описание доступных параметров приведены в [примерах модуля](/modules/cloud-provider-vsphere/examples.html) и в разделе [с описанием настроек модуля](/modules/cloud-provider-vsphere/configuration.html).

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
   d8 k get module cloud-provider-vsphere -o wide
   ```

1. Создайте файл c ресурсами [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup) со значением `nodeType: CloudEphemeral`. Например, `vsphere-instance.yaml`:

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

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                             STATUS   ROLES                  AGE   VERSION
   static-master-0                  Ready    control-plane,master   1h    v1.33.10
   ephemeral-1ca02a5b-7588b-k89dc   Ready    ephemeral              10m   v1.33.10
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. При сбоях создания ВМ проверьте объекты Machine, MachineSet и логи machine-controller-manager:

   ```shell
   d8 k -n d8-cloud-instance-manager get machinesets,machines -o wide
   d8 k -n d8-cloud-instance-manager logs deploy/machine-controller-manager --tail=200
   ```

   Также проверьте события в кластере:

   ```shell
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

## Добавление вручную созданных узлов через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/) включён:
  
  ```shell
  d8 k get module cloud-provider-vsphere -o wide
  ```

- Компоненты модуля `cloud-provider-vsphere` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vsphere get pods -o wide
  ```

- В кластере созданы StorageClass для vSphere:
  
  ```shell
  d8 k get sc
  ```

- В vSphere создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в vSphere совпадает с именем хоста (hostname) внутри операционной системы.
- В дополнительных параметрах ВМ в vSphere заданы параметры:

  ```text
  disk.EnableUUID = TRUE
  guestinfo.metadata = <BASE64_ENCODED_METADATA>
  guestinfo.metadata.encoding = base64
  ```

  Параметр `guestinfo.metadata` должен содержать конфигурацию метаданных, закодированных в Base64. Пример файла `metadata.json`:

  ```json
  {
     "instance-id": "cloud-static-worker-0",
     "local-hostname": "cloud-static-worker-0",
     "public-keys-data": "<SSH_PUBLIC_KEY>",
     "network": {
       "version": 2,
       "ethernets": {
         "id0": {
           "match": {
             "driver": "vmxnet3"
           },
           "set-name": "ens192",
           "dhcp4": true
         }
       }
     }
   }
  ```

  Где:

  - `instance-id` — идентификатор виртуальной машины;
  - `local-hostname` — имя хоста (hostname) узла внутри операционной системы;
  - `public-keys-data` — публичный SSH-ключ для доступа к виртуальной машине;
  - `network` — сетевые настройки, которые будут применены внутри виртуальной машины.

  Чтобы получить значение для параметра `guestinfo.metadata`, выполните:

  ```shell
  METADATA_B64="$(base64 -w0 metadata.json)"
  echo "$METADATA_B64"
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) конфигурации модуля `cloud-provider-vsphere`.
- На виртуальной машине установлен один из пакетных менеджеров (`apt`/`apt-get`, `yum` или `rpm`) для поддерживаемой ОС.  В РЕД ОС по умолчанию могут отсутствовать `yum` и `which`, поэтому их необходимо заранее установить.

1. Создайте файл с ресурсом NodeGroup и типом узлов `CloudStatic`. Например, `cloud-static-nodegroup.yaml`:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: cloud-static
   spec:
     nodeType: CloudStatic
   ```

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup cloud-static
   ```

   Пример ожидаемого результата:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME           TYPE          READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   cloud-static   CloudStatic   0       0       0                                                               1m    True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

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

1. На виртуальной машине назначьте права и запустите bootstrap-скрипт:

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

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.135
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.152
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
