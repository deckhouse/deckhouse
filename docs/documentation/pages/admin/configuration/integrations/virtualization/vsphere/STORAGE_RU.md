---
title: Сети, хранилище и балансировка нагрузки в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/storage.html
lang: ru
---

## Сети

Узлы кластера подключаются к сети, путь к которой задаётся параметром [`mainNetwork`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetwork). Через эту сеть узлы обращаются к vCenter, к хранилищу образов контейнеров и друг к другу.

Сеть должна отвечать следующим требованиям:

- она доступна на всех хостах ESXi, на которых создаются виртуальные машины;
- в ней работает DHCP, если адреса узлов не заданы статически;
- из неё доступны vCenter и хранилище образов контейнеров.

### Адресация узлов

По умолчанию узлы получают адреса по DHCP. Для узлов, которые создаёт установщик, адреса задаются статически в параметре [`mainNetworkIPAddresses`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-mainnetworkipaddresses). Для узлов, которые заказываются по ресурсу [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass), статическая адресация не поддерживается, такие узлы получают адреса по DHCP.

Адреса назначаются узлам группы по порядку, поэтому задайте их столько же, сколько узлов в группе. Если адресов меньше, список используется повторно и один адрес достаётся нескольким узлам.

Пример статической адресации для трёх master-узлов:

```yaml
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: k8s-msk-178
    mainNetworkIPAddresses:
    - address: 10.1.14.20/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.21/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
    - address: 10.1.14.22/24
      gateway: 10.1.14.254
      nameservers:
        addresses:
        - 8.8.8.8
        - 8.8.4.4
```

### Несколько сетей

Дополнительные интерфейсы виртуальных машин подключаются к сетям из параметра [`additionalNetworks`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-additionalnetworks). Например, через дополнительный интерфейс подключают сеть для трафика BGP.

Когда сетей несколько, укажите, какие из них платформа считает внутренними, а какие внешними. По именам сетей из параметров [`internalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworknames) и [`externalNetworkNames`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-externalnetworknames) компонент `vsphere-cloud-controller-manager` проставляет адреса InternalIP и ExternalIP в объекте Node. В этих параметрах указывается имя сети, а не путь к ней.

Параметр [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) задаёт подсеть, из которой master-узлы получают адреса во внутренней сети. Адреса выделяются начиная с десятого. Параметр обязателен, если в конфигурации есть секция [`nodeGroups`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups).

Пример конфигурации с двумя сетями:

```yaml
externalNetworkNames:
- net3-k8s
internalNetworkNames:
- K8S_3
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 3
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: lun_1
    mainNetwork: net3-k8s
    additionalNetworks:
    - K8S_3
```

## Хранилище

Для хранения данных кластера в VMware vSphere используются:

- Datastore — для размещения root-дисков виртуальных машин;
- CNS-диски (Container Native Storage) — для автоматического создания PersistentVolume через CSI.

Deckhouse Kubernetes Platform автоматически создаёт StorageClass для каждого Datastore, размеченного тегом зоны.

Чтобы платформа не создавала StorageClass для отдельных Datastore, перечислите их в параметре [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude). Он принимает список имён или регулярных выражений, каждое из которых должно совпадать с именем Datastore целиком. Частичное совпадение не учитывается. Например, для Datastore `vsanDatastore` выражение `vsan` не исключает ни одного StorageClass, а `vsan.*` исключает все.

Пример настройки через ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 2
  enabled: true
  settings:
    storageClass:
      exclude:
        - ".*-lun101-.*"
        - slow-lun103
```

Чтобы задать StorageClass по умолчанию, используйте глобальный параметр [`global.defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass).

### Настройка Datastore

Платформа создаёт StorageClass только для тех Datastore, которым назначены теги региона и зоны. Datastore без тегов платформе не виден, и заказать в нём PersistentVolume нельзя. Datastore назначают тот же тег региона, что и Datacenter, и тот же тег зоны, что и Cluster, узлы которого используют это хранилище.

{% alert %}
Разметку **Datastore** также можно сделать через **vSphere Client** по инструкции [«Настройка через vSphere Client»](authorization.html#настройка-datastore-с-использованием-vsphere-client). Ниже описана настройка через **`govc`**.
{% endalert %}

{% alert level="warning" %}
Для динамического заказа `PersistentVolume` необходимо, чтобы `Datastore` был доступен на **каждом** хосте ESXi (shared datastore).
{% endalert %}

Назначьте теги. В примере ниже два Datastore размечаются для разных зон одного региона:

```shell
govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>
govc tags.attach -c k8s-zone test-zone-1 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_1>

govc tags.attach -c k8s-region test-region /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
govc tags.attach -c k8s-zone test-zone-2 /<DATACENTER_NAME>/datastore/<DATASTORE_NAME_2>
```

### Политики хранения SPBM

Если в vSphere настроены политики хранения SPBM (Storage Policy Based Management), DKP обнаруживает их и дополнительно создаёт StorageClass для каждого сочетания Datastore и политики. При заказе PersistentVolume через такой StorageClass vSphere применяет к тому соответствующую политику.

Имя StorageClass складывается из имени Datastore и имени политики. DKP приводит его к нижнему регистру, заменяет пробелы на дефисы и удаляет остальные символы, кроме дефиса и точки. Например, для Datastore `lun_1` и политики `Gold Policy` получается StorageClass `lun1-gold-policy`.

Чтобы DKP обнаружил политики, учётной записи vSphere нужна привилегия `StorageProfile.View`. Она входит в [список необходимых привилегий](layout.html#список-необходимых-привилегий), а создание роли описано в разделе [«Создание и назначение роли с использованием vSphere Client»](authorization.html#создание-и-назначение-роли-с-использованием-vsphere-client).

Политики применяются к томам в любом сценарии, где работает модуль [`cloud-provider-vsphere`](/modules/cloud-provider-vsphere/), включая гибридный кластер. Выбрать политику для отдельного StorageClass вручную нельзя, набор StorageClass формируется автоматически.

Ограничения:

- для DatastoreCluster не создаются ни базовые StorageClass, ни StorageClass с политиками. Такие StorageClass создаются только в legacy-режиме с дисками FCD, который включается параметром [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag);
- параметр [`exclude`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-exclude) сопоставляется с именем Datastore, преобразованным по описанным выше правилам. Исключение убирает и базовый StorageClass, и StorageClass с политиками для этого Datastore. Отдельный StorageClass с политикой по его собственному имени параметр не исключает.

### Политика хранения для дисков узлов

Диски виртуальных машин, которые создаёт установщик, получают политику хранения из параметра [`storagePolicyID`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-storagepolicyid) ресурса VsphereClusterConfiguration. В параметре указывается идентификатор политики SPBM.

Параметр действует на master-узлы и на статические узлы, которые создаёт установщик. Узлы, заказываемые по VsphereInstanceClass, политику из этого параметра не получают.

Идентификатор политики можно посмотреть в vSphere Client. Откройте «Menu» → «Policies and Profiles» → «VM Storage Policies» и выберите политику по имени. В адресной строке браузера найдите параметр `lvSelectedItemId`. Идентификатор расположен между `PbmRequirementStorageProfile:` и первым символом `%`.

![Список политик хранения](../../../../images/cloud-provider-vsphere/storage-policy-setup/vm-storage-policies.png)

Идентификатор политики также выводит команда `govc`:

```shell
govc storage.policy.ls "<POLICY_NAME>"
```

Замените `<POLICY_NAME>` на имя политики, как оно показано в списке «VM Storage Policies», например `vSAN Default Storage Policy`.

### Режим работы CSI

По умолчанию подсистема хранения использует диски CNS с возможностью изменения размера без отключения тома от узла (online resize). Также поддерживается работа в legacy-режиме с дисками FCD, в котором изменение размера без отключения тома недоступно. Режим выбирается параметром [`compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag).

### Увеличение размера PersistentVolumeClaim

Платформа поддерживает изменение размера PersistentVolume без отключения тома от узла (online resize), начиная с версии vSphere 7.0U2.

Чтобы увеличить том, измените запрошенный размер в PersistentVolumeClaim:

```shell
d8 k -n <NAMESPACE> patch pvc <PVC_NAME> -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

Дополнительные действия не требуются. Платформа расширяет том в vSphere, затем kubelet расширяет файловую систему на узле, к которому том подключён. Рабочая нагрузка при этом не перезапускается.

Пока расширение выполняется, в статусе PersistentVolumeClaim присутствуют condition `Resizing` и `FileSystemResizePending`. После того как kubelet расширит файловую систему, оба condition удаляются, а поле `status.capacity` содержит новый размер. Команда ниже выводит текущий размер тома и список condition:

```shell
d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
  -o jsonpath='{.status.capacity.storage}{"\n"}{range .status.conditions[*]}{.type}={.status} {end}'
```

Пример вывода для тома, расширение которого завершилось. В первой строке размер, вторая строка пустая, потому что condition в статусе не осталось:

```console
2Gi

```

Если condition `Resizing` остаётся в статусе, расширение без отключения тома в этой конфигурации недоступно, например в legacy-режиме с дисками FCD. Ограничение описано в [issue проекта external-resizer](https://github.com/kubernetes-csi/external-resizer/issues/44). Чтобы расширить такой том, отключите его от узла:

1. Запретите планирование новых нагрузок на узел, к которому подключён том:

   ```shell
   d8 k cordon <NODE_NAME>
   ```

   Замените `<NODE_NAME>` на имя узла, где работает рабочая нагрузка, использующая PersistentVolumeClaim.

1. Удалите рабочую нагрузку, использующую PersistentVolumeClaim, чтобы том отключился от узла.

1. Дождитесь удаления condition `Resizing` из статуса PersistentVolumeClaim.

1. Разрешите планирование на узел:

   ```shell
   d8 k uncordon <NODE_NAME>
   ```

### Просмотр томов в vSphere Client

Тома, заказанные через CSI, отображаются в vSphere Client. Откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите объект Cluster, перейдите на вкладку «Monitor» и в разделе «Cloud Native Storage» выберите «Container Volumes». Для каждого тома приводятся имя, лейблы, Datastore, соответствие политике хранения («Compliance Status»), доступность («Health Status») и размер.

![Список CNS-томов](../../../../images/cloud-provider-vsphere/cns-volumes/container-volumes.png)

Имя тома в vSphere совпадает с именем PersistentVolume в кластере.

По значку слева от имени тома открывается панель с подробностями. На вкладке «Kubernetes objects» приводятся неймспейс, имя и лейблы PersistentVolumeClaim, а также рабочая нагрузка, которая использует том.

![Подробности CNS-тома](../../../../images/cloud-provider-vsphere/cns-volumes/container-volume-details.png)

## Балансировка нагрузки

Входящий трафик балансируется одним из трёх способов.

1. **Через внешний балансировщик.** Балансировщик, который уже есть в инфраструктуре, направляет трафик на frontend-узлы кластера. Со стороны vSphere требуется только сеть, в которой frontend-узлы доступны балансировщику. Трафик в кластере принимает Ingress-контроллер, его настройка описана в разделе [«ALB средствами Ingress NGINX Controller»](../../../configuration/network/ingress/alb/nginx.html).

1. **Через MetalLB.** Способ подходит, когда внешнего балансировщика нет. MetalLB выдаёт адреса сервисам с типом LoadBalancer и работает в режимах L2 и BGP. Требования к сети, доступность режимов в редакциях и настройка описаны в разделе [«Балансировка средствами MetalLB»](../../../configuration/network/ingress/nlb/metallb.html). Для режима BGP со стороны vSphere потребуются:

   - отдельная сеть, в которой frontend-узлы обмениваются трафиком с BGP-роутерами;
   - второй сетевой интерфейс у frontend-узлов, подключённый к этой сети параметром [`additionalNetworks`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-additionalnetworks);
   - DHCP в этой сети. Адрес на дополнительном интерфейсе платформа задаёт только master-узлам, из подсети [`internalNetworkCIDR`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-internalnetworkcidr) начиная с десятого адреса. Узлам групп адрес в этой сети платформа не назначает;
   - IP-адреса BGP-роутеров, номер автономной системы (ASN) роутеров и номер ASN кластера, а также диапазон адресов, который кластер анонсирует.

1. **Через NSX-T.** Если в инфраструктуре развёрнут NSX-T, `cloud-controller-manager` заказывает в нём балансировщик для каждого сервиса с типом LoadBalancer. Имя пула адресов, путь к шлюзу Tier-1 и учётные данные задаются в секции [`nsxt`](/modules/cloud-provider-vsphere/configuration.html#parameters-nsxt).
