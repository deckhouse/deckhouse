---
title: Гибридный кластер с vSphere
permalink: ru/admin/integrations/hybrid/vsphere-hybrid.html
lang: ru
---

Далее описан процесс добавления worker-узлов из vSphere в существующий статический кластер DKP.

Для интеграции с vSphere используется модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/). Он обеспечивает взаимодействие DKP с vCenter, получение информации о виртуальных машинах, работу с параметрами размещения и интеграцию с инфраструктурными возможностями vSphere.

В разделе описаны три способа добавления worker-узлов:

- **Автоматическое создание узлов в vSphere**. DKP создаёт виртуальные машины через API vSphere. Параметры ВМ задаются ресурсом [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), а требуемое количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).
- **Подключение вручную созданных узлов через CAPS**. Виртуальная машина создаётся пользователем заранее, а DKP подключается к ней по SSH через Cluster API Provider Static. Для этого используются ресурсы NodeGroup с типом `Static`, а также ресурсы SSHCredentials и StaticInstance.
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html).

## Предварительные требования для vSphere

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин во vSphere настроена сетевая связность.
- Узлы vSphere, добавляемые в кластер, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vSphere](../virtualization/vsphere/authorization.html):
  - настроен доступ к vCenter;
  - подготовлена учётная запись vSphere с необходимыми привилегиями;
  - подготовлен шаблон виртуальной машины;
  - настроены сети, Datastore, теги регионов и зон.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

## Добавление автоматически создаваемых узлов

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

## Добавление вручную созданных узлов через CAPS

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
- Имя виртуальной машины в vSphere, значение local-hostname в метаданных и hostname внутри операционной системы совпадают.
- В дополнительных параметрах ВМ в vSphere задан параметр:

  ```text
  disk.EnableUUID = TRUE
  ```

- Виртуальная машина подключена к сети, указанной в параметре [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) конфигурации модуля `cloud-provider-vsphere`.
- - На виртуальной машине есть административный SSH-доступ для первичной настройки пользователя, под которым CAPS будет подключаться к узлу, либо такой пользователь уже создан заранее.
- Пользователь для подключения по SSH может выполнять команды через `sudo` без ввода пароля.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

{% offtopic title="Передача метаданных в ВМ через vSphere..." %}
Для предварительной настройки виртуальной машины можно использовать cloud-init через метаданные vSphere. Например, метаданные можно использовать для настройки hostname, сети и SSH-ключей.

В этом случае задайте в дополнительных параметрах ВМ:

```text
guestinfo.metadata = <BASE64_ENCODED_METADATA>
guestinfo.metadata.encoding = base64
```

Пример файла `metadata.json`:

```json
{
  "instance-id": "vsphere-worker-caps",
  "local-hostname": "vsphere-worker-caps",
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
- `local-hostname` — hostname узла внутри операционной системы;
- `public-keys-data` — публичный SSH-ключ для доступа к виртуальной машине;
- `network` — сетевые настройки, которые будут применены внутри виртуальной машины.

Чтобы получить значение для параметра `guestinfo.metadata`, выполните:

```shell
METADATA_B64="$(base64 -w0 metadata.json)"
echo "$METADATA_B64"
```

Использование `guestinfo.metadata` не является обязательным требованием CAPS. Главное, чтобы к моменту создания ресурса StaticInstance виртуальная машина была доступна по SSH, имела корректный hostname и пользователь для подключения мог выполнять команды через `sudo` без пароля.
{% endofftopic %}

1. На master-узле задайте переменные для создаваемой NodeGroup и подключаемой виртуальной машины:

   ```shell
   export NODE_GROUP="vsphere-caps"
   export NODE_NAME="vsphere-worker-caps"
   export NODE_SSH_IP="<NODE_IP>"
   export CAPS_USER="caps"
   ```

   Где:

   - `NODE_GROUP` — имя NodeGroup, в которую будет добавлен узел;
   - `NODE_NAME` — имя подключаемого узла. Оно должно совпадать с hostname внутри операционной системы и именем ВМ в vSphere;
   - `NODE_SSH_IP` — IP-адрес виртуальной машины, доступный по SSH;
   - `CAPS_USER` — пользователь, под которым CAPS будет подключаться к виртуальной машине.

1. На master-узле создайте NodeGroup:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: ${NODE_GROUP}
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: ${NODE_GROUP}
   EOF
   ```

   В этом сценарии используется `nodeType: Static`, потому что виртуальная машина уже создана вручную, а CAPS будет только подключать и настраивать её по SSH.

1. Убедитесь, что NodeGroup создана и синхронизирована:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}
   ```

   Пример ожидаемого результата:

   ```console
   NAME           TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   vsphere-caps   Static   0       0       0                                                               1m    True
   ```

1. На master-узле сгенерируйте SSH-ключ, который CAPS будет использовать для подключения к виртуальной машине:

   ```shell
   ssh-keygen -t ed25519 \
     -f /dev/shm/${NODE_GROUP}-id \
     -C "" \
     -N ""
   ```

   {% alert level="info" %}
   Ключ создаётся с пустой парольной фразой, так как CAPS должен использовать его автоматически.
   {% endalert %}

1. На master-узле создайте ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials):

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: SSHCredentials
   metadata:
     name: ${NODE_GROUP}
   spec:
     user: ${CAPS_USER}
     privateSSHKey: "$(base64 -w0 /dev/shm/${NODE_GROUP}-id)"
   EOF
   ```

   Ресурс SSHCredentials хранит имя пользователя и приватный SSH-ключ, с помощью которых CAPS будет подключаться к виртуальной машине.

1. Убедитесь, что ресурс SSHCredentials создан:

   ```shell
   d8 k get sshcredentials
   d8 k describe sshcredentials ${NODE_GROUP}
   ```

1. На master-узле выведите публичную часть SSH-ключа:

   ```shell
   cat /dev/shm/${NODE_GROUP}-id.pub
   ```

   Этот ключ понадобится на следующем шаге для настройки пользователя на подключаемой виртуальной машине.

1. На подключаемой виртуальной машине создайте пользователя, под которым CAPS будет выполнять настройку узла. Выполните команды на подключаемой виртуальной машине, указав публичный SSH-ключ, полученный на предыдущем шаге:

   ```shell
   export CAPS_USER="caps"
   export KEY='<SSH_PUBLIC_KEY>'

   useradd -m -s /bin/bash ${CAPS_USER}
   usermod -aG sudo ${CAPS_USER}

   echo "${CAPS_USER} ALL=(ALL) NOPASSWD: ALL" | EDITOR='tee -a' visudo

   mkdir -p /home/${CAPS_USER}/.ssh
   echo "${KEY}" > /home/${CAPS_USER}/.ssh/authorized_keys

   chown -R ${CAPS_USER}:${CAPS_USER} /home/${CAPS_USER}
   chmod 700 /home/${CAPS_USER}/.ssh
   chmod 600 /home/${CAPS_USER}/.ssh/authorized_keys
   ```

   {% alert level="info" %}
   Значение `KEY` необходимо указывать в кавычках, так как публичный SSH-ключ содержит пробелы.
   {% endalert %}

   {% alert level="info" %}
   Для операционных систем семейства Astra Linux при использовании модуля мандатного контроля целостности Parsec дополнительно задайте максимальный уровень целостности для пользователя:

   ```shell
   pdpl-user -i 63 ${CAPS_USER}
   ```

   {% endalert %}

1. На master-узле проверьте, что CAPS-пользователь может подключиться к виртуальной машине по SSH и выполнять команды через `sudo` без пароля:

   ```shell
   ssh -i /dev/shm/${NODE_GROUP}-id ${CAPS_USER}@${NODE_SSH_IP} \
     'hostname; sudo -n true; echo OK'
   ```

   В выводе должно быть имя узла и строка `OK`.

1. На master-узле создайте ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance) для подключаемой виртуальной машины:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha2
   kind: StaticInstance
   metadata:
     name: ${NODE_NAME}
     labels:
       role: ${NODE_GROUP}
   spec:
     address: "${NODE_SSH_IP}"
     credentialsRef:
       kind: SSHCredentials
       name: ${NODE_GROUP}
   EOF
   ```

   Где:

   - `metadata.name` — имя подключаемого узла;
   - `metadata.labels.role` — лейбл, по которому NodeGroup выбирает этот StaticInstance;
   - `spec.address` — IP-адрес виртуальной машины, доступный по SSH;
   - `spec.credentialsRef.name` — имя созданного ранее ресурса SSHCredentials.

1. Проверьте состояние StaticInstance:

   ```shell
   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}
   ```

1. Дождитесь подключения узла и проверьте его состояние:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   ```console
   NAME                    STATUS   ROLES          AGE   VERSION    INTERNAL-IP      EXTERNAL-IP
   static-master-0         Ready    master         1h    v1.33.10   192.168.240.135  <none>
   vsphere-worker-caps     Ready    vsphere-caps   5m    v1.33.10   192.168.240.152  <none>
   ```

1. При сбоях подключения проверьте состояние NodeGroup, StaticInstance, Machine и события в кластере:

   ```shell
   d8 k get nodegroup ${NODE_GROUP}
   d8 k describe nodegroup ${NODE_GROUP}

   d8 k get staticinstances
   d8 k describe staticinstance ${NODE_NAME}

   d8 k -n d8-cloud-instance-manager get machines,machinesets,machinedeployments -o wide
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

## Добавление вручную созданных узлов через bootstrap-скрипт

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
  - `local-hostname` — hostname узла внутри операционной системы;
  - `public-keys-data` — публичный SSH-ключ для доступа к виртуальной машине;
  - `network` — сетевые настройки, которые будут применены внутри виртуальной машины.

  Чтобы получить значение для параметра `guestinfo.metadata`, выполните:

  ```shell
  METADATA_B64="$(base64 -w0 metadata.json)"
  echo "$METADATA_B64"
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

   ```console
   NAME                       STATUS   ROLES          AGE   VERSION    INTERNAL-IP
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.135
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.152
   ```
