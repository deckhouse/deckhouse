---
title: Гибридный кластер с DVP
permalink: ru/admin/integrations/hybrid/dvp-hybrid.html
lang: ru
---

Далее описан процесс добавления worker-узлов из Deckhouse Virtualization Platform (DVP) в существующий статический кластер DKP.

Для интеграции с DVP используется модуль [`cloud-provider-dvp`](/modules/cloud-provider-dvp/). Он обеспечивает взаимодействие DKP с API кластера DVP, создание виртуальных машин, подключение созданных ВМ к существующему Kubernetes-кластеру и управление жизненным циклом worker-узлов через механизмы Cluster API.

В этом разделе описано автоматическое создание worker-узлов в DVP. В этом сценарии DKP самостоятельно создаёт виртуальные машины в указанном неймспейсе DVP и подключает их к существующему кластеру через механизмы Cluster API. Параметры ВМ задаются ресурсом [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass), а требуемое количество узлов и зоны размещения — ресурсом [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `CloudEphemeral`.

## Предварительные требования

Перед началом убедитесь, что выполнены следующие условия:

- Кластер создан с параметром [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype).
- Между сетью статических узлов кластера DKP и сетью виртуальных машин в DVP настроена сетевая связность.
- Создаваемые в DVP worker-узлы имеют доступ к Kubernetes API подключаемого DKP-кластера, DNS и необходимым адресам согласно разделам [Сетевое взаимодействие](../../../../reference/network_interaction.html) и [Настройка сетевых политик](../../configuration/network/policy/configuration.html).
- Выполнены требования из раздела [Подготовка окружения](/modules/cloud-provider-dvp/environment.html):
  - создан ServiceAccount для доступа к API DVP;
  - назначены необходимые права доступа;
  - сгенерирован kubeconfig для подключения к API DVP;
  - подготовлен неймспейс, в котором будут создаваться виртуальные машины и диски.
- В DVP доступен образ ОС Linux с поддержкой `cloud-init`, например `ubuntu-24-04-lts`.
- В DVP доступен подходящий VirtualMachineClass, например `amd-epyc-gen-3`.
- В DVP доступен StorageClass для корневых дисков виртуальных машин, например `replicated`.
- Если используется шаблон виртуальной машины, убедитесь, что он содержит только один диск.
- Из кластера DKP доступен Kubernetes API кластера DVP.
- При использовании Cilium с туннелированием трафика подов выбран режим [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), соответствующий сетевой связности между площадками.

{% alert level="warning" %}
В параметрах DVPInstanceClass используются ресурсы кластера DVP: VirtualMachineClass, ClusterVirtualImage, VirtualImage, VirtualDisk и StorageClass из DVP, а не из подключаемого DKP-кластера.
{% endalert %}

## Добавление автоматически создаваемых узлов

1. На машине администратора, где настроен доступ к кластеру DVP, подготовьте kubeconfig для доступа модуля `cloud-provider-dvp` к API DVP.

   Выполните шаги из раздела [Подготовка окружения](/modules/cloud-provider-dvp/environment.html) и закодируйте полученный kubeconfig в Base64:

   ```shell
   export DVP_PROVIDER_KUBECONFIG="./kubeconfig"
   export DVP_KUBECONFIG_B64="$(base64 -w0 ${DVP_PROVIDER_KUBECONFIG})"
   ```

1. Задайте пространство имён DVP, в котором будут создаваться виртуальные машины и диски:

   ```shell
   export DVP_NAMESPACE="<DVP_NAMESPACE>"
   ```

1. Определите зону DVP, в которой будут создаваться worker-узлы.

   Выполните команду в кластере DVP под пользователем, у которого есть права на просмотр узлов:

   ```shell
   d8 k get nodes -L topology.kubernetes.io/region,topology.kubernetes.io/zone
   ```

   Если в DVP не настроены отдельные топологические зоны, используйте зону `default`.

   {% alert level="warning" %}
   Значение зоны в ModuleConfig и NodeGroup должно совпадать с доступной зоной DVP. Не указывайте произвольные значения, например `zone-a` или `zone-b`, если такие зоны не настроены в DVP.
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
         - <ZONE_NAME>
   EOF
   ```

   Где:

   - `provider.kubeconfigDataBase64` — kubeconfig для доступа к API DVP в кодировке Base64;
   - `provider.namespace` — неймспейс DVP, в котором будут создаваться виртуальные машины и диски;
   - `zones` — список зон DVP, в которых разрешено создавать worker-узлы.

1. Примените ModuleConfig:

   ```shell
   d8 k apply -f cloud-provider-dvp-mc.yaml
   ```

1. Дождитесь включения модуля `cloud-provider-dvp`:

   ```shell
   d8 k get moduleconfig cloud-provider-dvp
   d8 k get module cloud-provider-dvp -o wide
   d8 k -n d8-cloud-provider-dvp get pods -o wide
   ```

   Модуль должен перейти в состояние `Ready`, а поды в неймспейсе `d8-cloud-provider-dvp` — в состояние `Running`.

1. Убедитесь, что модуль `node-manager` находится в состоянии `Ready`:

   ```shell
   d8 k get module node-manager -o wide
   ```

   Если модуль находится в состоянии Error, проверьте, что в ModuleConfig и NodeGroup указаны доступные зоны DVP.

1. Убедитесь, что в кластере появился ресурс DVPInstanceClass:

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
   - `cloudInstances.zones` — зона DVP, в которой будет создан worker-узел. Значение должно входить в список `zones` из ModuleConfig.

1. Примените манифест:

   ```shell
   d8 k apply -f dvp-instanceclass-nodegroup.yaml
   ```

   После применения DKP начнёт создавать виртуальную машину в DVP и подключать её к кластеру как worker-узел.

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

   ```console
   NAME                              STATUS   ROLES                  AGE   VERSION    INTERNAL-IP
   dvp-hybrid-master-0               Ready    control-plane,master   1h    v1.33.10   10.12.0.69
   dvp-worker-c75a75c1-twqp4-bjpvl   Ready    dvp-worker             10m   v1.33.10   10.12.3.15
   ```
