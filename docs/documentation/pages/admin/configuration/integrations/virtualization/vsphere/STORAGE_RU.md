---
title: Хранилище и балансировка нагрузки в VMware vSphere
permalink: ru/admin/integrations/virtualization/vsphere/storage.html
lang: ru
---

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

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения размера без отключения тома от узла (online resize). Также поддерживается работа в legacy-режиме с FCD-дисками, в котором изменение размера без отключения тома недоступно. Поведение подсистемы устанавливается с помощью [параметра `compatibilityFlag`](/modules/cloud-provider-vsphere/configuration.html#parameters-storageclass-compatibilityflag).

### Изменение размера тома (PVC)

Deckhouse Kubernetes Platform поддерживает изменение размера PersistentVolume без отключения тома от узла (online resize), начиная с версии vSphere 7.0U2.

Чтобы увеличить том, измените запрошенный размер в PVC:

```shell
d8 k -n <NAMESPACE> patch pvc <PVC_NAME> -p '{"spec":{"resources":{"requests":{"storage":"2Gi"}}}}'
```

Дополнительные действия не требуются. DKP расширяет том в vSphere, затем kubelet расширяет файловую систему на узле, к которому том подключён. Рабочая нагрузка при этом не перезапускается.

Пока расширение выполняется, в статусе PVC присутствуют condition `Resizing` и `FileSystemResizePending`. После того как kubelet расширит файловую систему, оба condition удаляются, а поле `status.capacity` содержит новый размер:

```shell
d8 k -n <NAMESPACE> get pvc <PVC_NAME> \
  -o jsonpath='{.status.capacity.storage}{"\n"}{range .status.conditions[*]}{.type}={.status} {end}'
```

Если condition `Resizing` остаётся в статусе, расширение без отключения тома в этой конфигурации недоступно, например в legacy-режиме с дисками FCD. Ограничение описано в [issue проекта external-resizer](https://github.com/kubernetes-csi/external-resizer/issues/44). Чтобы расширить такой том, отключите его от узла:

1. Запретите планирование новых нагрузок на узел, к которому подключён том:

   ```shell
   d8 k cordon <NODE_NAME>
   ```

   Замените `<NODE_NAME>` на имя узла, где работает под, использующий PVC.

1. Удалите под, использующий PVC, чтобы том отключился от узла.

1. Дождитесь удаления condition `Resizing` из статуса PVC.

1. Разрешите планирование на узел:

   ```shell
   d8 k uncordon <NODE_NAME>
   ```

### Просмотр томов в vSphere Client

Тома, заказанные через CSI, отображаются в vSphere Client. Откройте «Menu» → «Inventory» → «Hosts and Clusters», выберите объект Cluster, перейдите на вкладку «Monitor» и в разделе «Cloud Native Storage» выберите «Container Volumes». Тот же список доступен на объектах vCenter, Datacenter и Datastore, но не на виртуальной машине. Для каждого тома приводятся имя, метки, Datastore, соответствие политике хранения («Compliance Status»), доступность («Health Status») и размер.

![Список CNS-томов](../../../../images/cloud-provider-vsphere/cns-volumes/container-volumes.png)

Имя тома в vSphere совпадает с именем PersistentVolume в кластере.

По значку слева от имени тома открывается панель с подробностями. На вкладке «Kubernetes objects» приводятся пространство имён, имя и метки PersistentVolumeClaim, а также рабочая нагрузка, которая использует том.

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

{% alert level="warning" %}
Балансировщики NSX-T реализованы в `cloud-controller-manager` как альфа-возможность. Платформа включает её переменной `ENABLE_ALPHA_NSXT_LB`, когда в конфигурации задана секция `nsxt`.
{% endalert %}
