---
title: Хранилище и балансировка нагрузки в Yandex Cloud
permalink: ru/admin/integrations/public/yandex/storage.html
lang: ru
---

Этот раздел охватывает дополнительные аспекты интеграции Deckhouse Platform (DP) с Yandex Cloud:

- подключение облачных дисков через CSI;
- автоматическое создание StorageClass;
- использование балансировщиков нагрузки;
- особенности применения изменений;
- работу с CloudStatic-узлами и [bastion-хостами](../../../../glossary.html#bastion-host).

## Хранилище (CSI и StorageClass)

DP обеспечивает интеграцию с блочным хранилищем Yandex Cloud через компонент Container Storage Interface (CSI). Это даёт возможность кластерам DP автоматически заказывать и подключать диски, а также использовать стандартные Kubernetes-ресурсы PersistentVolumeClaim для работы с хранилищем.

DP автоматически создает ресурсы StorageClass для всех поддерживаемых типов дисков Yandex Cloud. Это делает возможным для всех пользователей сразу использовать хранилище, не создавая вручную описания классов.

Поддерживаются следующие типы дисков:

| Тип диска                 | Имя StorageClass          | Комментарии              |
|--------------------------|---------------------------|--------------------------|
| `network-hdd`            | `network-hdd`             | —                        |
| `network-ssd`            | `network-ssd`             | —                        |
| `network-ssd-nonreplicated` | `network-ssd-nonreplicated` | Размер кратен 93 ГБ      |
| `network-ssd-io-m3`      | `network-ssd-io-m3`       | Размер кратен 93 ГБ      |

{% alert level="info" %}
Размеры дисков `network-ssd-nonreplicated` и `network-ssd-io-m3` должны быть кратны 93 ГБ, иначе произойдёт ошибка при заказе тома.
{% endalert %}

### Исключение ненужных StorageClass

Если в кластере не планируется использовать определённые типы дисков, можно отключить автоматическое создание соответствующих StorageClass. Это делается с помощью [параметра `settings.storage.parameters.excludedStorageClasses`](/modules/cloud-provider-yandex/configuration.html#parameters-storage-parameters-excludedstorageclasses) в ресурсе ModuleConfig:

```yaml
settings:
  storage:
    parameters:
      excludedStorageClasses:
        - network-ssd-.*
        - network-hdd
```

В приведённом примере DP не создаст StorageClass для всех `network-ssd` дисков и для `network-hdd`.

### Создание дополнительных StorageClass и размер блока

Параметр [`settings.storage.parameters.provisionedStorageClasses`](/modules/cloud-provider-yandex/configuration.html#parameters-storage-parameters-provisionedstorageclasses) позволяет создавать дополнительные StorageClass или переопределять параметры StorageClass, создаваемых DP по умолчанию.

С помощью параметра [`blockSize`](/modules/cloud-provider-yandex/configuration.html#parameters-storage-parameters-provisionedstorageclasses-blocksize) можно задать [размер блока](https://cloud.yandex.ru/docs/compute/operations/disk-create/empty-disk-blocksize) для создаваемых дисков. От размера блока зависит максимальный размер диска. Для значения `4Ki` он составляет `8Ti` и удваивается с каждым следующим размером блока, вплоть до `256Ti` при `128Ki`.

Пример конфигурации модуля `cloud-provider-yandex` с дополнительным StorageClass, для которого задан размер блока `64Ki`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        cloudID: <CLOUD_ID>
        folderID: <FOLDER_ID>
    nodes:
      parameters:
        layout: Standard
        nodeNetworkCIDR: 192.168.12.0/24
        sshPublicKey: <SSH_PUBLIC_KEY>
    storage:
      parameters:
        provisionedStorageClasses:
          - name: network-ssd-64k
            type: network-ssd
            blockSize: 64Ki
```

{% alert level="warning" %}
После создания диска изменить размер его блока нельзя. Изменение параметра `blockSize` приводит к пересозданию StorageClass, но не изменяет размер блока у ранее созданных томов.
{% endalert %}

### Назначение StorageClass по умолчанию

По умолчанию DP выбирает StorageClass на основе аннотации `storageclass.kubernetes.io/is-default-class=true`.

Чтобы задать другой StorageClass по умолчанию, необходимо использовать [глобальный параметр DP `global.defaultClusterStorageClass`](../../../../reference/api/global.html#parameters-defaultclusterstorageclass). Изменить его можно следующей командой:

```shell
d8 k edit mc global
```

Если параметр `defaultClusterStorageClass` не указан, DP определяет StorageClass, используемый по умолчанию, в следующем порядке:

- StorageClass с аннотацией `storageclass.kubernetes.io/is-default-class='true'` (если такой имеется в кластере).
- Первый StorageClass по алфавиту среди тех, что автоматически создаются [облачным провайдером](../../../../glossary.html#cloud-provider).
- По умолчанию значение параметра `defaultClusterStorageClass` — пустая строка (`""`).

### Изменение размера PVC

Размер существующего PVC можно увеличить путем изменения значения параметра `spec.resources.requests.storage` без остановки и пересоздания использующего его пода.

После изменения значения `spec.resources.requests.storage` [CSI-драйвер](../../../../glossary.html#csi-driver) последовательно:

- увеличивает размер диска в Yandex Cloud;
- обновляет размер связанного PersistentVolume;
- выполняет расширение файловой системы на узле, к которому подключён том.

Во время операции под продолжает работать, а смонтированный том остаётся доступным приложению. После завершения увеличения новый размер файловой системы становится доступен внутри контейнера без перезапуска пода.

{% alert level="info" %}
Уменьшение размера PVC не поддерживается.
{% endalert %}

Чтобы увеличить PVC, выполните следующие действия:

1. Получите имя StorageClass, используемого PVC:

   ```shell
   d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
     -o jsonpath='{.spec.storageClassName}{"\n"}'
   ```

   Где:

   - `<NAMESPACE>` — неймспейс, в котором находится PVC;
   - `<PVC_NAME>` — имя PVC, размер которого необходимо увеличить.

   Например:

   ```shell
   d8 k -n production get pvc application-data \
     -o jsonpath='{.spec.storageClassName}{"\n"}'
   ```

   Пример вывода команды:

   ```console
   network-ssd
   ```

   В этом примере PVC `application-data` использует StorageClass `network-ssd`.

1. Убедитесь, что StorageClass разрешает увеличение томов:

   ```shell
   d8 k get storageclass <STORAGECLASS_NAME> \
     -o jsonpath='{.allowVolumeExpansion}{"\n"}'
   ```

   Где `<STORAGECLASS_NAME>` — имя StorageClass, полученное на предыдущем шаге.

   Например:

   ```shell
   d8 k get storageclass network-ssd \
     -o jsonpath='{.allowVolumeExpansion}{"\n"}'
   ```

   Пример вывода команды:

   ```console
   true
   ```

1. Проверьте текущее состояние и размер PVC:

   ```shell
   d8 k -n <NAMESPACE> get pvc <PVC_NAME>
   ```

   Например:

   ```shell
   d8 k -n production get pvc application-data
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   20Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Убедитесь, что:

   - PVC находится в состоянии `Bound`;
   - в поле `CAPACITY` указан текущий размер PVC;
   - в поле `STORAGECLASS` указан StorageClass, проверенный на предыдущем шаге.

1. Увеличьте размер PVC:

   ```shell
   d8 k -n <NAMESPACE> edit pvc <PVC_NAME>
   ```

   Например:

   ```shell
   d8 k -n production edit pvc application-data
   ```

   В поле `spec.resources.requests.storage` укажите новый размер PVC:

   ```yaml
   spec:
     resources:
       requests:
         storage: 30Gi
   ```

   В этом примере размер PVC увеличивается до 30Gi.

   Сохраните изменения и закройте редактор.

   {% alert level="warning" %}
   Для StorageClass `network-ssd-nonreplicated` и `network-ssd-io-m3` [размер должен быть кратен 93Gi](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-disktype).
   {% endalert %}

1. Дождитесь увеличения PVC:

   ```shell
   d8 k -n <NAMESPACE> get pvc <PVC_NAME> --watch
   ```

   Где:

   - `<NAMESPACE>` — неймспейс, в котором находится PVC;
   - `<PVC_NAME>` — имя PVC, размер которого увеличивается.

   Например:

   ```shell
   d8 k -n production get pvc application-data --watch
   ```

   Во время увеличения в поле `CAPACITY` может отображаться прежний размер:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   20Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Операция завершена, когда в поле `CAPACITY` отображается новый размер PVC:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME               STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS
   application-data   Bound    pvc-65e92674-077c-4b4f-b65d-19e92f04e103   30Gi       RWO            network-ssd
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте события PVC:

   ```shell
   d8 k -n <NAMESPACE> describe pvc <PVC_NAME>
   ```

   Например:

   ```shell
   d8 k -n production describe pvc application-data
   ```

   Во время увеличения могут появиться следующие события:

   ```console
   ExternalExpanding
   Resizing
   FileSystemResizeRequired
   ```

   Об успешном увеличении файловой системы свидетельствует событие:

   ```console
   FileSystemResizeSuccessful
   ```

   Например:

   ```console
   Normal  FileSystemResizeSuccessful  kubelet  MountVolume.NodeExpandVolume succeeded for volume "pvc-65e92674-077c-4b4f-b65d-19e92f04e103"
   ```

1. Проверьте размер файловой системы внутри пода:

   ```shell
   d8 k -n <NAMESPACE> exec <POD_NAME> -- \
     df -hT <MOUNT_PATH>
   ```

   Где:

   - `<NAMESPACE>` — неймспейс, в котором находится под;
   - `<POD_NAME>` — имя пода, использующего PVC;
   - `<MOUNT_PATH>` — путь внутри контейнера, в который смонтирован PVC.

   Например:

   ```shell
   d8 k -n production exec application-0 -- \
     df -hT /data
   ```

   Пример вывода:

   ```console
   Filesystem   Type   Size    Used   Avail   Use%   Mounted on
   /dev/vde     ext4   29.4G   22M    29.4G    1%    /data
   ```

   {% alert level="info" %}
   Фактический размер файловой системы может быть немного меньше размера PVC из-за служебных данных файловой системы.
   {% endalert %}

## Балансировка нагрузки

### Внешний LoadBalancer

DP автоматически подписывается на Kubernetes-объекты Service с типом LoadBalancer. При их создании в кластере, он создаёт соответствующие ресурсы:

- **NetworkLoadBalancer** — сетевой балансировщик нагрузки в Yandex Cloud;
- **TargetGroup** — группа конечных точек для балансировки трафика.

Эти ресурсы предоставляют Kubernetes-сервисам с типом LoadBalancer возможность принимать входящий трафик из интернета или внутренних сетей в зависимости от настроек.

Подробнее об архитектуре см. в [в документации Kubernetes Cloud Controller Manager for Yandex Cloud](https://github.com/flant/yandex-cloud-controller-manager).

### Внутренний LoadBalancer

Чтобы создать внутренний балансировщик (INTERNAL LoadBalancer), укажите подсеть, в которой должен быть создан listener:

Для этого добавьте следующую аннотацию в объект Service:

```yaml
metadata:
  annotations:
    yandex.cpi.flant.com/listener-subnet-id: <SUBNET_ID>
```

Значение `SubnetID` — это ID подсети, в которой будет создан внутренний слушатель (listener) Yandex LoadBalancer. Использование этой аннотации даёт возможность контролировать сетевую доступность балансировщика, ограничивая его только внутренними адресами.

> Поведение по умолчанию (внешний или внутренний LB) зависит от конфигурации кластера. Для явного выбора типа используйте аннотацию `yandex.cpi.flant.com/loadbalancer-external`.

#### Аннотации объекта Service

В кластере заданы значения по умолчанию для размещения ресурсов балансировщиков (сеть для целевой группы и подсеть для Listener). Эти значения выставляются автоматически во время развёртывания кластера и могут быть переопределены аннотациями на уровне конкретного Service.

В Yandex Cloud Controller Manager поддерживаются следующие аннотации:

1. `yandex.cpi.flant.com/target-group-network-id` — указывает NetworkID, в котором будет создана целевая группа для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-subnet-id` — задаёт SubnetID для слушателей создаваемого балансировщика для данного Service. Переопределяет соответствующее значение по умолчанию.
1. `yandex.cpi.flant.com/listener-address-ipv4` — задаёт предопределённый IPv4-адрес для слушателей (поддерживаются и внутренние, и внешние LB).
1. `yandex.cpi.flant.com/loadbalancer-external` — включает создание внешнего (external) LB для данного Service (используйте, если нужно явно создать внешний балансировщик). Переопределяет поведение по умолчанию.
1. `yandex.cpi.flant.com/target-group-name-prefix` — задаёт префикс имени целевой группы, которую использует LoadBalancer. Аннотация на Service определяет целевую группу, но не выбирает узлы. Чтобы включить узлы в целевую группу, задайте аннотацию с тем же значением в параметре [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations) ресурса NodeGroup. Yandex Cloud Controller Manager включает в целевую группу все подходящие узлы с таким значением аннотации независимо от их NodeGroup. Имя целевой группы формируется как `<ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>`.

Если для control plane или [master-узлов](../../../../glossary.html#master-node) создаются отдельные целевые группы, добавьте на master-узлы лейбл `node.kubernetes.io/exclude-from-external-load-balancers: ""`. Это предотвратит попытки контроллера автоматически добавлять master-узлы в новые целевые группы для балансировщиков.
Если вы создаёте собственный балансировщик для master-узлов и хотите, чтобы YCC также мог размещать свои балансировщики на master-узлах, заранее создайте целевую группу с именем по маске `${CLUSTER-NAME}${VPC.ID}`.

#### Использование отдельной целевой группы для NodeGroup

По умолчанию Yandex Cloud Controller Manager добавляет все подходящие узлы кластера в целевую группу по умолчанию. Чтобы назначить узлам отдельную целевую группу, используйте аннотацию `yandex.cpi.flant.com/target-group-name-prefix`.

1. В NodeGroup, узлы которой должны входить в отдельную целевую группу, задайте аннотацию `yandex.cpi.flant.com/target-group-name-prefix` в параметре [`spec.nodeTemplate.annotations`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate-annotations). Например:

   ```yaml
   spec:
     nodeTemplate:
       annotations:
         yandex.cpi.flant.com/target-group-name-prefix: frontend-
   ```

   Аннотация распространяется на объекты Node. Yandex Cloud Controller Manager добавляет в целевую группу все подходящие узлы с таким значением аннотации независимо от их NodeGroup.

1. В объекте [Service](/modules/cloud-provider-yandex/examples.html#аннотации-объекта-service) типа LoadBalancer задайте ту же аннотацию с тем же значением. Например:

   ```yaml
   metadata:
     annotations:
       yandex.cpi.flant.com/target-group-name-prefix: frontend-
   ```

   Аннотация на Service определяет целевую группу, которую использует LoadBalancer, но не выбирает узлы.

1. Убедитесь, что значения `yandex.cpi.flant.com/target-group-name-prefix` в NodeGroup и Service совпадают. В примере выше в обоих ресурсах используется значение `frontend-`.

1. Целевая группа будет создана с именем, сформированным по шаблону:

   ```shell
   <ANNOTATION_VALUE><YANDEX_CLOUD_CLUSTER_NAME><NETWORK_ID>
   ```

   Например, если значение аннотации — `frontend-`, имя кластера — `my-cluster-`, а идентификатор сети — `enp123456789`, имя целевой группы будет следующим:

   ```shell
   frontend-my-cluster-enp123456789
   ```

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: frontend
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    annotations:
      yandex.cpi.flant.com/target-group-name-prefix: frontend-
---
apiVersion: v1
kind: Service
metadata:
  name: nginx-frontend
  annotations:
    yandex.cpi.flant.com/target-group-name-prefix: frontend-
spec:
  type: LoadBalancer
```

{% alert level="warning" %}
Yandex Cloud не позволяет одному целевому ресурсу (target), определяемому парой `(SubnetID, IP)`, одновременно находиться в нескольких целевых группах.

При изменении целевой группы Yandex Cloud Controller Manager сначала удаляет ресурс из текущей группы, а затем добавляет в новую. Во время перемещения возможен кратковременный перерыв в обработке трафика.
{% endalert %}

Узлы, для которых задан отдельный префикс целевой группы, исключаются из целевой группы по умолчанию. Поэтому балансировщики, использующие её, перестают направлять трафик на такие узлы.

Такое автоматическое перемещение выполняется только между целевыми группами, которыми управляет Yandex Cloud Controller Manager. Если ресурс уже находится в другой целевой группе, не управляемой Yandex Cloud Controller Manager, удалите ресурс из неё перед использованием аннотации.

После изменения или удаления префикса прежняя целевая группа может остаться пустой. Если она больше не используется балансировщиками, удалите её вручную.

#### Проверки состояния целевой группы

Параметры проверок состояния для создаваемых целевых групп балансировщика:

1. `yandex.cpi.flant.com/healthcheck-interval-seconds` — как часто запускать проверку, в секундах (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-timeout-seconds` — сколько ждать ответа от эндпоинта, в секундах. Если за это время ответ не получен, проверка считается неуспешной (по умолчанию 1).
1. `yandex.cpi.flant.com/healthcheck-unhealthy-threshold` — сколько подряд неуспешных проверок нужно, чтобы пометить эндпоинт как неработоспособный (unhealthy) и исключить его из балансировки (по умолчанию 2).
1. `yandex.cpi.flant.com/healthcheck-healthy-threshold` — сколько подряд успешных проверок нужно, чтобы вернуть эндпоинт в статус работоспособный (healthy) и снова включить его в балансировку (по умолчанию 2).

## Особенности применения изменений

DP не пересоздаёт уже существующие объекты Machine при изменении параметров.
Пересоздание узлов происходит только при изменении:

- параметров в [секции NodeGroup](/modules/node-manager/cr.html#nodegroup);
- [параметров YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).

Это поведение помогает избежать лишних операций и простоя существующих узлов, однако требует ручного вмешательства при необходимости пересоздать машины.

Если вы изменили [настройки провайдера](/modules/cloud-provider-yandex/configuration.html) в [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) `cloud-provider-yandex` (например, параметры провайдера, [схемы размещения](../../../../glossary.html#layout) или подсетей), чтобы изменения вступили в силу, выполните команду:

```shell
dhctl converge
```

Команда инициирует пересчёт конфигурации и приводит текущее состояние кластера в соответствие с описанным в ресурсах.

## Изменение `networkType` не пересоздаёт CloudEphemeral-узлы

В кластерах, использующих Machine Controller Manager (MCM), изменение параметра [`networkType`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-networktype) в ресурсе YandexInstanceClass не приводит к автоматическому пересозданию существующих CloudEphemeral-узлов. Хотя MachineClass обновляется, тип сетевого ускорения у уже созданных виртуальных машин в Yandex Cloud не изменяется.

DP намеренно не учитывает параметр `networkType` при определении необходимости пересоздания CloudEphemeral-узлов в MCM. Если бы он учитывался, обновление DP привело бы к пересозданию CloudEphemeral-узлов во всех кластерах, где `networkType` уже указан, даже если его значение не изменялось.

В Cluster API параметр `networkType` учитывается при определении необходимости пересоздания узлов, поэтому его изменение запускает их обновление автоматически. Новые [CloudEphemeral](../../../../glossary.html#cloud-ephemeral-node) NodeGroup в Yandex по умолчанию используют CAPI.

Если вы изменили `networkType` в кластере на MCM и хотите, чтобы новое значение применилось к уже существующим CloudEphemeral-узлам, пересоздайте их вручную — инструкция в [документации](/modules/node-manager/faq.html#как-пересоздать-эфемерные-машины-в-облаке-с-новой-конфигурацией).

## Интеграция вручную созданных ВМ

DP позволяет подключать существующие виртуальные машины в Yandex Cloud к Kubernetes-кластеру в качестве узлов. Такие узлы называются [CloudStatic](../../../../glossary.html#cloud-static-node), поскольку они не управляются напрямую [модулем `node-manager`](/modules/node-manager/), но могут использоваться в составе кластера.

Чтобы вручную подключить виртуальную машину в качестве CloudStatic-узла, необходимо:

1. Узнать актуальное значение `nodeNetworkCIDR` из кластера:

   ```shell
   d8 k get mc cloud-provider-yandex -o jsonpath='{.spec.settings.nodes.parameters.nodeNetworkCIDR}{"\n"}'
   ```

   Результатом будет строка вида:

   ```console
   192.168.12.0/24
   ```

   Это значение необходимо скопировать и указать как `value` в метаданных виртуальной машины.

1. Задать параметр `node-network-cidr` в метаданных ВМ:

   ```yaml
   key: node-network-cidr
   value: <NODE_NETWORK_CIDR>
   ```

   Параметр `node-network-cidr` должен совпадать со значением параметра [`nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) в ModuleConfig.
