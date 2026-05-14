---
title: Гибридный кластер с VCD
permalink: ru/admin/integrations/hybrid/vcd-hybrid.html
lang: ru
---

Далее описан процесс добавления worker-узлов из VMware Cloud Director (VCD) в существующий статический кластер DKP.

Для интеграции с VCD используется модуль [`cloud-provider-vcd`](/modules/cloud-provider-vcd/). Он обеспечивает взаимодействие DKP с VMware Cloud Director, создание и удаление виртуальных машин, получение информации об инфраструктуре VCD, а также интеграцию со StorageClass и другими возможностями провайдера.

В разделе описаны три способа добавления worker-узлов:

- **Автоматическое создание узлов в VCD**. DKP создаёт виртуальные машины через API VCD. Параметры ВМ задаются ресурсом [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), а требуемое количество узлов — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).
- **Подключение вручную созданных узлов через CAPS**. Виртуальная машина создаётся пользователем заранее, а DKP подключается к ней по SSH через Cluster API Provider Static. Для этого используются ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `Static`, а также ресурсы [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) и [StaticInstance](/modules/node-manager/cr.html#staticinstance).
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html).

## Предварительные требования для VCD

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин в VCD настроена сетевая связность.
- Узлы VCD, добавляемые в кластер, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подключение и авторизация в VMware vCloud Director](../virtualization/vcd/connection-and-authorization.html):
  - настроен тенант в VCD с выделенными ресурсами;
  - подготовлена учётная запись VCD со статичным паролем и правами администратора;
  - настроена рабочая сеть в VCD с включённым DHCP-сервером;
  - подготовлены необходимые ресурсы VCD: VDC, vApp, шаблоны, политики и другие параметры.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

## Добавление автоматически создаваемых узлов

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
   - `virtualApplicationName` — имя vApp, где будут создаваться узлы, например `dkp-vcd-app`;
   - `sshPublicKey` — публичный SSH-ключ для доступа к узлам;
   - `provider.server` — URL-адрес API VCD;
   - `provider.username` — имя пользователя VCD;
   - `provider.password` — пароль пользователя VCD;
   - `provider.insecure` — установите значение `true`, если VCD использует самоподписанный TLS-сертификат.

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

## Добавление вручную созданных узлов через CAPS

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

- Виртуальная машина подключена к сети VCD, используемой как основная сеть для облачных узлов кластера. Обычно это сеть, указанная в параметре [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) конфигурации `cloud-provider-vcd` или в используемом VCDInstanceClass.
- - На виртуальной машине есть административный SSH-доступ для первичной настройки пользователя, под которым CAPS будет подключаться к узлу, либо такой пользователь уже создан заранее.
- Пользователь для подключения по SSH может выполнять команды через `sudo` без ввода пароля.
- На виртуальной машине установлены необходимые базовые пакеты для поддерживаемой ОС. Для РЕД ОС заранее установите `which` и пакетный менеджер, если они отсутствуют.

1. На master-узле задайте переменные для создаваемой NodeGroup и подключаемой виртуальной машины:

   ```shell
   export NODE_GROUP="vcd-caps"
   export NODE_NAME="vcd-worker-caps"
   export NODE_SSH_IP="<NODE_IP>"
   export CAPS_USER="caps"
   ```

   Где:

   - `NODE_GROUP` — имя NodeGroup, в которую будет добавлен узел;
   - `NODE_NAME` — имя подключаемого узла. Оно должно совпадать с hostname внутри операционной системы и именем ВМ в VCD;
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
   NAME       TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE   SYNCED
   vcd-caps   Static   0       0       0                                                               1m    True
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
   NAME             STATUS   ROLES      AGE   VERSION    INTERNAL-IP      EXTERNAL-IP
   static-master-0  Ready    master     1h    v1.33.10   192.168.240.138  <none>
   vcd-worker-caps  Ready    vcd-caps   5m    v1.33.10   192.168.240.151  <none>
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

- Виртуальная машина подключена к сети VCD, используемой как основная сеть для облачных узлов кластера. Обычно это сеть, указанная в параметре [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) конфигурации `cloud-provider-vcd` или в используемом VCDInstanceClass.
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
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```