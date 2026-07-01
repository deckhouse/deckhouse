---
title: Гибридный кластер с DVP
permalink: ru/admin/integrations/hybrid/dvp-hybrid.html
lang: ru
search: гибрид с DVP
description: Подготовка к гибридной интеграции с DVP в Deckhouse Kubernetes Platform.
---

Далее описан процесс добавления узлов из Deckhouse Virtualization Platform (DVP) в существующий статический кластер DKP.

Для интеграции с DVP используется модуль [`cloud-provider-dvp`](/modules/cloud-provider-dvp/). Он обеспечивает взаимодействие DKP с API кластера DVP, создание виртуальных машин, подключение созданных ВМ к существующему Kubernetes-кластеру и управление жизненным циклом узлов через механизмы Cluster API.

В разделе описаны два способа добавления узлов:

- **Автоматическое создание узлов в DVP**. DKP создаёт виртуальные машины через API DVP. Параметры ВМ задаются ресурсом [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass), а требуемое количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html).
- **Подключение вручную созданных узлов через bootstrap-скрипт**. Виртуальная машина создаётся пользователем заранее и подключается к кластеру с помощью bootstrap-скрипта DKP. Для такого сценария используется [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html).

## Предварительные требования

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов кластера DKP и сетью виртуальных машин в DVP настроена [сетевая связность](./overview.html#общие-сетевые-требования). Создаваемые в DVP узлы имеют доступ к Kubernetes API подключаемого DKP-кластера, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [«Настройка сетевых политик»](../../configuration/network/policy/configuration.html). При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками. Из кластера DKP доступен Kubernetes API кластера DVP.
- Выполнены требования из раздела [«Подготовка окружения»](/modules/cloud-provider-dvp/environment.html):
  - создан [ServiceAccount](/modules/cloud-provider-dvp/environment.html#создание-пользователя) для доступа к API DVP;
  - сгенерирован kubeconfig для подключения к API DVP;
  - подготовлен неймспейс, в котором будут создаваться виртуальные машины и диски.
- В DVP доступен образ ОС Linux с поддержкой `cloud-init`, например `ubuntu-24-04-lts`.
- В DVP доступен подходящий [VirtualMachineClass](/modules/virtualization/stable/cr.html#virtualmachineclass), например `amd-epyc-gen-3`.
- В DVP доступен StorageClass для корневых дисков виртуальных машин, например `replicated`.
- Если используется шаблон виртуальной машины, убедитесь, что он содержит только один диск.

{% alert level="warning" %}
В параметрах [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) используются ресурсы кластера DVP: VirtualMachineClass, ClusterVirtualImage, VirtualImage, VirtualDisk и StorageClass из DVP, а не из подключаемого DKP-кластера.
{% endalert %}

## Добавление автоматически создаваемых узлов

1. На машине администратора, где настроен доступ к кластеру DVP, подготовьте kubeconfig для доступа модуля `cloud-provider-dvp` к API DVP.

   Выполните шаги из раздела [«Подготовка окружения»](/modules/cloud-provider-dvp/environment.html) и закодируйте полученный kubeconfig в Base64:

   ```shell
   export DVP_PROVIDER_KUBECONFIG="./kubeconfig"
   export DVP_KUBECONFIG_B64="$(base64 -w0 ${DVP_PROVIDER_KUBECONFIG})"
   ```

1. Задайте неймспейс DVP, в котором будут создаваться виртуальные машины и диски:

   ```shell
   export DVP_NAMESPACE="<DVP_NAMESPACE>"
   ```

1. Укажите зону DVP, в которой будут создаваться узлы.

   На данный момент зонирование в DVP находится в разработке, поэтому для параметров `zones` в ModuleConfig и NodeGroup используйте значение `default`:

   ```shell
   export DVP_ZONE="default"
   ```

   При необходимости можно проверить топологические метки узлов в кластере DVP:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/region,topology.kubernetes.io/zone
   ```

   {% alert level="warning" %}
   Значение зоны в ModuleConfig и NodeGroup должно совпадать. Пока в DVP доступно только значение `default`.
   {% endalert %}

1. Создайте файл, например `cloud-provider-dvp-mc.yaml`, с конфигурацией модуля `cloud-provider-dvp`:

   ```shell
   cat > cloud-provider-dvp-mc.yaml <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: cloud-provider-dvp
   spec:
     enabled: true
     version: 1
     settings:
       provider:
         kubeconfigDataBase64: ${DVP_KUBECONFIG_B64}
         namespace: ${DVP_NAMESPACE}
       zones:
         - ${DVP_ZONE}
   EOF
   ```

   В манифесте используются значения переменных окружения, заданных на предыдущих шагах: `DVP_KUBECONFIG_B64`, `DVP_NAMESPACE` и `DVP_ZONE`.

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-dvp-mc.yaml
   ```

1. Дождитесь включения модуля `cloud-provider-dvp`:

   ```shell
   d8 k get module cloud-provider-dvp -o wide
   ```

   Модуль должен перейти в состояние `Ready`, а поды в неймспейсе `d8-cloud-provider-dvp` — в состояние `Running`.

1. Убедитесь, что модуль `node-manager` находится в состоянии `Ready`:

   ```shell
   d8 k get module node-manager -o wide
   ```

   Если модуль находится в состоянии `Error`, проверьте, что в ModuleConfig и NodeGroup указаны доступные зоны DVP.

1. Убедитесь, что в кластере появился ресурс [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass):

   ```shell
   d8 k get crd dvpinstanceclasses.deckhouse.io
   ```

1. На машине администратора, где настроен доступ к кластеру DVP, проверьте доступные классы виртуальных машин, образы и StorageClass:

   ```shell
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get virtualmachineclasses
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get clustervirtualimages
   d8 k --kubeconfig ${DVP_PROVIDER_KUBECONFIG} get storageclasses
   ```

   Используйте полученные значения при создании DVPInstanceClass.

1. Создайте файл, например `dvp-instanceclass-nodegroup.yaml`, с ресурсами DVPInstanceClass и NodeGroup:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: DVPInstanceClass
   metadata:
     name: dvp-worker
   spec:
     virtualMachine:
       cpu:
         cores: 3
         coreFraction: 20%
       memory:
         size: 6Gi
       virtualMachineClassName: <VIRTUAL_MACHINE_CLASS_NAME>
       bootloader: EFI
     rootDisk:
       size: 15Gi
       storageClass: <STORAGE_CLASS_NAME>
       image:
         kind: ClusterVirtualImage
         name: <CLUSTER_VIRTUAL_IMAGE_NAME>
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: dvp-worker
   spec:
     nodeType: CloudEphemeral
     cloudInstances:
       classReference:
         kind: DVPInstanceClass
         name: dvp-worker
       minPerZone: 1
       maxPerZone: 1
       zones:
         - <ZONE_NAME>
   ```

   Где:

   - `virtualMachineClassName` — имя VirtualMachineClass в DVP, например `amd-epyc-gen-3`;
   - `rootDisk.storageClass` — имя StorageClass в DVP, например `replicated`;
   - `rootDisk.image.kind` — тип источника образа. Для кластерного образа используйте `ClusterVirtualImage`;
   - `rootDisk.image.name` — имя образа ОС в DVP, например `ubuntu-24-04-lts`;
   - `cloudInstances.zones` — зона DVP, в которой будет создан узел. Значение должно входить в список `zones` из ModuleConfig.

1. Примените манифест:

   ```shell
   d8 k apply -f dvp-instanceclass-nodegroup.yaml
   ```

   После применения DKP начнёт создавать виртуальную машину в DVP и подключать её к кластеру как узел.

1. Проверьте состояние NodeGroup:

   ```shell
   d8 k get nodegroup dvp-worker -o wide
   d8 k describe nodegroup dvp-worker
   ```

1. Проверьте появление нового узла в DKP-кластере:

   ```shell
   d8 k get nodes -o wide
   ```

   Пример ожидаемого результата:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                              STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   dvp-hybrid-master-0               Ready    control-plane,master   1h    v1.33.10   10.12.0.69
   dvp-worker-c75a75c1-twqp4-bjpvl   Ready    dvp-worker             10m   v1.33.10   10.12.3.15
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

## Добавление вручную созданных узлов через bootstrap-скрипт

Перед началом убедитесь, что выполнены следующие условия:

- Модуль [`cloud-provider-dvp`](/modules/cloud-provider-dvp/) включён:

  ```shell
  d8 k get module cloud-provider-dvp -o wide
  ```

- Компоненты модуля `cloud-provider-dvp` находятся в состоянии `Running`:

  ```shell
  d8 k -n d8-cloud-provider-dvp get pods -o wide
  ```

- В DVP создана виртуальная машина, которая будет подключена к кластеру.
- Виртуальная машина подключена к сети DVP, используемой для гибридной интеграции с кластером.
- IP-адрес виртуальной машины входит в диапазон, указанный в [`internalNetworkCIDRs`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration-internalnetworkcidrs).
- Имя виртуальной машины в DVP совпадает с hostname внутри операционной системы.
- На виртуальной машине есть SSH-доступ для копирования и запуска bootstrap-скрипта.
- Пользователь для подключения по SSH может выполнять команды через `sudo` без ввода пароля.
- На виртуальной машине установлен один из пакетных менеджеров (`apt`/`apt-get`, `yum` или `rpm`) для поддерживаемой ОС.  В РЕД ОС по умолчанию могут отсутствовать `yum` и `which`, поэтому их необходимо заранее установить.

1. Создайте файл, например `cloud-static-nodegroup.yaml`, с ресурсом NodeGroup и типом узлов `CloudStatic`:

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

1. На виртуальной машине декодируйте bootstrap-скрипт, назначьте права и запустите его:

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
   NAME                   STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   dvp-hybrid-master-0    Ready    control-plane,master   1h    v1.33.12   10.12.0.69
   cloud-static-worker-0  Ready    cloud-static           5m    v1.33.12   10.12.3.88
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. При сбоях подключения проверьте состояние NodeGroup, события и логи bootstrap на подключаемой виртуальной машине:

   ```shell
   d8 k get nodegroup cloud-static
   d8 k describe nodegroup cloud-static
   d8 k get events -A --sort-by=.lastTimestamp | tail -n 100
   ```

   На подключаемой виртуальной машине:

   ```shell
   sudo tail -n 120 /var/log/d8/bashible/bootstrap.log
   ```

   Если в логах есть ошибка `Failed to discover node_ip that matches internalNetworkCIDRs`, проверьте, что IP-адрес виртуальной машины входит в `internalNetworkCIDRs`.
