---
title: Гибридный кластер с VCD
permalink: ru/admin/integrations/hybrid/vcd-hybrid.html
lang: ru
search: гибрид с VCD
description: Подготовка к гибридной интеграции с VMware Cloud Director в Deckhouse Kubernetes Platform.
---

Далее описан процесс добавления узлов из VMware Cloud Director (VCD) в существующий статический кластер Deckhouse Kubernetes Platform (DKP).

Для интеграции с VCD используется модуль [`cloud-provider-vcd`](/modules/cloud-provider-vcd/). Он обеспечивает взаимодействие DKP с VMware Cloud Director, создание и удаление виртуальных машин, получение информации об инфраструктуре VCD, а также интеграцию со StorageClass и другими возможностями провайдера.

В разделе описаны два способа добавления узлов:

- **Автоматическое создание узлов в VCD**. DKP создаёт виртуальные машины через API VCD. Параметры ВМ задаются ресурсом [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), а требуемое количество узлов — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html).

## Предварительные требования для VCD

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов и сетью виртуальных машин в VCD настроена [сетевая связность](./overview.html#общие-сетевые-требования). Узлы VCD, добавляемые в кластер, имеют доступ к Kubernetes API, DNS и необходимым адресам согласно разделам [«Сетевое взаимодействие»](../../../../reference/network_interaction.html) и [«Настройка сетевых политик»](../../configuration/network/policy/configuration.html). При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.
- Выполнены требования из раздела [«Подключение и авторизация в VMware vCloud Director»](../virtualization/vcd/connection-and-authorization.html):
  - настроен тенант в VCD с выделенными ресурсами;
  - подготовлена учётная запись VCD со статичным паролем и правами администратора;
  - настроена рабочая сеть в VCD с включённым DHCP-сервером;
  - подготовлены необходимые ресурсы VCD: VDC, vApp, шаблоны, политики и другие параметры.

## Добавление автоматически создаваемых узлов

1. Создайте файл с ресурсом ModuleConfig. Например, `cloud-provider-vcd-mc.yaml`:

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
     insecure: false
   ```

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-vcd-mc.yaml
   d8 k get mc cloud-provider-vcd
   ```

1. Убедитесь, что все поды в неймспейсе `d8-cloud-provider-vcd` находятся в состоянии `Running`:

   ```shell
   d8 k get pods -n d8-cloud-provider-vcd
   ```

1. Убедитесь, что в кластере созданы StorageClass для VCD:

   ```shell
   d8 k get sc
   ```

1. Создайте файл с ресурсами [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) и [NodeGroup](/modules/node-manager/cr.html#nodegroup). Например, `vcd-instanceclass-nodegroup.yaml`:

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

## Добавление вручную созданных узлов через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-vcd`](/modules/cloud-provider-vcd/) включён:
  
  ```shell
  d8 k get module cloud-provider-vcd -o wide
  ```

- Компоненты модуля `cloud-provider-vcd` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-vcd get pods -o wide
  ```

- В кластере созданы StorageClass для VCD:
  
  ```shell
  d8 k get sc
  ```

- В VCD создана виртуальная машина, которая будет подключена к кластеру.
- Имя виртуальной машины в VCD совпадает с именем хоста (hostname) внутри операционной системы.
- В дополнительных параметрах ВМ в VCD задано значение:

  ```text
  disk.EnableUUID = TRUE
  ```

- Виртуальная машина подключена к сети VCD, используемой как основная сеть для облачных узлов кластера. Обычно это сеть, указанная в параметре [`mainNetwork`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-mainnetwork) конфигурации `cloud-provider-vcd` или в используемом VCDInstanceClass.
- На виртуальной машине установлен один из пакетных менеджеров (`apt`/`apt-get`, `yum` или `rpm`) для поддерживаемой ОС.  В РЕД ОС по умолчанию могут отсутствовать `yum` и `which`, поэтому их необходимо заранее установить.

1. Создайте файл с ресурсом NodeGroup и типом узлов CloudStatic. Например, `cloud-static-nodegroup.yaml`:

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
   static-master-0            Ready    master         1h    v1.33.10   192.168.240.138
   cloud-static-worker-0      Ready    cloud-static   5m    v1.33.10   192.168.240.151
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->
