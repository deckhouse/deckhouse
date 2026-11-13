---
title: Как заменить образ ОС в облачном кластере?
subsystems:
  - cluster_infrastructure
lang: ru
---

Deckhouse Platform создаёт узлы типов `CloudEphemeral` и `CloudPermanent`, включая master-узлы, на основе образа операционной системы (ОС), который указан в конфигурации группы узлов. Чтобы перевести узлы на новый образ, укажите его в этой конфигурации вместо прежнего. Узлы типа `CloudEphemeral` Deckhouse Platform пересоздаёт из нового образа автоматически. Узлы типа `CloudPermanent` и master-узлы пересоздаются, когда вы применяете конфигурацию командой `dhctl converge`.

Для узлов типа `CloudStatic` виртуальные машины создаёт и обслуживает администратор. Чтобы заменить образ такого узла, самостоятельно подготовьте виртуальную машину с нужной ОС и подключите её по [инструкции добавления CloudStatic-узлов](admin/configuration/platform-scaling/node/cloud-node.html#добавление-cloudstatic-узла-в-кластер).

Ресурс, в котором указывается образ, зависит от типа узлов и способа настройки облачного провайдера:

| Тип узлов | Способ настройки провайдера | Где изменить образ |
| --- | --- | --- |
| `CloudEphemeral` | Через ModuleConfig или `<PROVIDER>ClusterConfiguration` | В ресурсе `<PROVIDER>InstanceClass`, на который ссылается группа узлов |
| `CloudPermanent` и master-узлы | Через ModuleConfig | В ресурсе `<PROVIDER>InstanceClass`, на который ссылается группа узлов |
| `CloudPermanent` и master-узлы | Через `<PROVIDER>ClusterConfiguration` | В секции `instanceClass` нужной группы узлов этого ресурса |

{% alert level="info" %}
Переход с `<PROVIDER>ClusterConfiguration` на новую модель конфигурации через ModuleConfig поддерживается только для встроенной виртуализации (DVP), Yandex Cloud и zVirt.
{% endalert %}

Параметры образа зависят от ресурса, в котором вы меняете конфигурацию. Выберите нужную вкладку.

{% tabs os_image_parameters %}
{% tab "InstanceClass" %}

Для узлов типа `CloudEphemeral` используйте параметры инстанс-класса при любом способе настройки провайдера. При настройке через ModuleConfig эти параметры также задают образ узлов типа `CloudPermanent` и master-узлов.

| Облачный провайдер | Параметр в секции `spec` | Ресурс инстанс-класса |
| --- | --- | --- |
| [Amazon Web Services](/modules/cloud-provider-aws/) | [`ami`](/modules/cloud-provider-aws/cr.html#awsinstanceclass-v1-spec-ami) | [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass) |
| [Microsoft Azure](/modules/cloud-provider-azure/) | [`urn`](/modules/cloud-provider-azure/cr.html#azureinstanceclass-v1-spec-urn) | [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass) |
| [Google Cloud Platform](/modules/cloud-provider-gcp/) | [`image`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-image) | [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) |
| [Yandex Cloud](/modules/cloud-provider-yandex/) | [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) | [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) |
| [OpenStack](/modules/cloud-provider-openstack/) | [`imageName`](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass-v1-spec-imagename) | [OpenStackInstanceClass](/modules/cloud-provider-openstack/cr.html#openstackinstanceclass) |
| [Huawei Cloud](/modules/cloud-provider-huaweicloud/) | [`imageName`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-imagename) | [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) |
| [Basis Dynamix](/modules/cloud-provider-dynamix/) | [`imageName`](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass-v1-spec-imagename) | [DynamixInstanceClass](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) |
| [VMware Cloud Director](/modules/cloud-provider-vcd/) | [`template`](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass-v1-spec-template) | [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass) |
| [VMware vSphere](/modules/cloud-provider-vsphere/) | [`template`](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass-v1-spec-template) | [VsphereInstanceClass](/modules/cloud-provider-vsphere/cr.html#vsphereinstanceclass) |
| [zVirt](/modules/cloud-provider-zvirt/) | [`template`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-template) | [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) |
| [Встроенная виртуализация](/modules/cloud-provider-dvp/) | [`rootDisk.image`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image) | [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass) |

{% endtab %}
{% tab "ClusterConfiguration" %}

При настройке провайдера через `<PROVIDER>ClusterConfiguration` образ master-узлов задаётся в секции `masterNodeGroup.instanceClass`, а образ остальных узлов типа `CloudPermanent` в секции `nodeGroups[].instanceClass`.

| Облачный провайдер | Master-узлы | Рабочие узлы `CloudPermanent` |
| --- | --- | --- |
| [Amazon Web Services](/modules/cloud-provider-aws/) | [`masterNodeGroup.instanceClass.ami`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-masternodegroup-instanceclass-ami) | [`nodeGroups[].instanceClass.ami`](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration-nodegroups-instanceclass-ami) |
| [Microsoft Azure](/modules/cloud-provider-azure/) | [`masterNodeGroup.instanceClass.urn`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-masternodegroup-instanceclass-urn) | [`nodeGroups[].instanceClass.urn`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-nodegroups-instanceclass-urn) |
| [Google Cloud Platform](/modules/cloud-provider-gcp/) | [`masterNodeGroup.instanceClass.image`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-image) | [`nodeGroups[].instanceClass.image`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-image) |
| [Yandex Cloud](/modules/cloud-provider-yandex/) | [`masterNodeGroup.instanceClass.imageID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-masternodegroup-instanceclass-imageid) | [`nodeGroups[].instanceClass.imageID`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-imageid) |
| [OpenStack](/modules/cloud-provider-openstack/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-openstack/cluster_configuration.html#openstackclusterconfiguration-nodegroups-instanceclass-imagename) |
| [Huawei Cloud](/modules/cloud-provider-huaweicloud/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-nodegroups-instanceclass-imagename) |
| [Basis Dynamix](/modules/cloud-provider-dynamix/) | [`masterNodeGroup.instanceClass.imageName`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-imagename) | [`nodeGroups[].instanceClass.imageName`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-nodegroups-instanceclass-imagename) |
| [VMware Cloud Director](/modules/cloud-provider-vcd/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-nodegroups-instanceclass-template) |
| [VMware vSphere](/modules/cloud-provider-vsphere/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-vsphere/cluster_configuration.html#vsphereclusterconfiguration-nodegroups-instanceclass-template) |
| [zVirt](/modules/cloud-provider-zvirt/) | [`masterNodeGroup.instanceClass.template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-masternodegroup-instanceclass-template) | [`nodeGroups[].instanceClass.template`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-nodegroups-instanceclass-template) |
| [Встроенная виртуализация](/modules/cloud-provider-dvp/) | [`masterNodeGroup.instanceClass.rootDisk.image`](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration-masternodegroup-instanceclass-rootdisk-image) | [`nodeGroups[].instanceClass.rootDisk.image`](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration-nodegroups-instanceclass-rootdisk-image) |

{% endtab %}
{% endtabs %}

У облачных провайдеров, кроме [VMware Cloud Director](/modules/cloud-provider-vcd/), [VMware vSphere](/modules/cloud-provider-vsphere/), [zVirt](/modules/cloud-provider-zvirt/) и [встроенной виртуализации](/modules/cloud-provider-dvp/), значение параметра задаётся строкой с именем или идентификатором образа ОС. В VMware Cloud Director, VMware vSphere и zVirt параметр `template` задаёт шаблон виртуальной машины, подготовленный на основе образа ОС. Чтобы перевести узлы этих провайдеров на новый образ, подготовьте на его основе новый шаблон виртуальной машины.

{% alert level="info" %}
Во встроенной виртуализации образ задаётся объектом. В параметре [`rootDisk.image.kind`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image-kind) укажите тип ресурса с образом (`ClusterVirtualImage`, `VirtualImage` или `VirtualDisk`), а в параметре [`rootDisk.image.name`](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass-v1alpha1-spec-rootdisk-image-name) укажите имя этого ресурса.
{% endalert %}

{% offtopic title="Пример параметра с образом во встроенной виртуализации" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: worker
spec:
  rootDisk:
    # Остальные параметры инстанс-класса не показаны.
    image:
      kind: ClusterVirtualImage
      name: ubuntu-24-04-20260204
```

{% endofftopic %}

#### Требования к новому образу

Образ ОС или подготовленный на его основе шаблон виртуальной машины должен соответствовать требованиям
модуля выбранного облачного провайдера.

Перед заменой убедитесь, что новый образ или шаблон виртуальной машины соответствует следующим общим требованиям:

- Значение параметра с образом отличается от прежнего. Если образ или шаблон задаётся именем, включите в имя версию или дату, например `ubuntu-24-04-20260204`.
- Образ или подготовленный на его основе шаблон существует в облаке к моменту пересоздания узлов.
- Образ содержит `cloud-init` и операционную систему, которую поддерживает Deckhouse Platform. Список поддерживаемых систем приведён в разделе [«Поддерживаемые версии ОС»](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#linux).

Для некоторых провайдеров подготовка образа или шаблона включает дополнительные шаги.
Выполните их по инструкциям соответствующего модуля:

- Для OpenStack используйте раздел [«Как загрузить образ в OpenStack?»](/modules/cloud-provider-openstack/faq.html#как-загрузить-образ-в-openstack).
- Для Basis Dynamix используйте раздел [«Подготовка образа операционной системы»](/modules/cloud-provider-dynamix/environment.html#подготовка-образа-операционной-системы).
- Для VMware Cloud Director используйте раздел [«Шаблон виртуальной машины»](/modules/cloud-provider-vcd/environment.html#шаблон-виртуальной-машины).
- Для VMware vSphere используйте раздел [«Требования к образу виртуальной машины»](/modules/cloud-provider-vsphere/environment.html#требования-к-образу-виртуальной-машины), который включает инструкцию по подготовке образа.
- Для zVirt используйте разделы [«Подготовка образа операционной системы»](/modules/cloud-provider-zvirt/environment.html#подготовка-образа-операционной-системы) и [«Подготовка шаблона виртуальной машины»](/modules/cloud-provider-zvirt/environment.html#подготовка-шаблона-виртуальной-машины).

{% alert level="warning" %}
Deckhouse Platform сравнивает значения параметров в конфигурации, а не содержимое образа в облаке. Если заменить образ или шаблон в облаке, оставив прежнее имя, узлы не пересоздадутся.
{% endalert %}

#### Изменение образа в конфигурации

Создайте в облаке новый образ, который соответствует требованиям выше.
В VMware Cloud Director, VMware vSphere и zVirt подготовьте на его основе новый шаблон виртуальной машины.
Затем выберите способ настройки облачного провайдера.

{% alert level="warning" %}
В Yandex Cloud группы типа `CloudEphemeral`, в инстанс-классах которых не задан `imageID`, используют образ master-узлов по умолчанию. При смене образа master-узлов существующие узлы таких групп не пересоздаются. Новые узлы могут создаваться из нового образа, поэтому в одной группе могут оказаться узлы с разными образами.

Чтобы сохранить прежний образ группы, явно укажите его `imageID` в YandexInstanceClass. Чтобы пересоздать всю группу из нового образа, явно укажите новый `imageID`.
{% endalert %}

Для DVP, Yandex Cloud и zVirt перед выбором вкладки проверьте конфигурацию группы узлов `master`:

```shell
d8 k get nodegroup master -o yaml
```

Если заданы `spec.cloudInstances.classReference.kind` и `spec.cloudInstances.classReference.name` и миграция завершена, выберите вкладку «Через ModuleConfig».
Убедитесь, что не срабатывает алерт миграции вашего провайдера:

- Для DVP это [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending).
- Для Yandex Cloud это [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending).
- Для zVirt это [`D8CloudProviderZvirtMigrationPending`](reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending).

Если алерт срабатывает, сначала [завершите миграцию](#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).
Если у группы узлов `master` не задан `classReference` или используется другой провайдер, выберите вкладку «Через ClusterConfiguration».

{% alert level="info" %}
После завершения миграции на ModuleConfig изменять образ необходимо в инстанс-классе нужной группы узлов.

В кластере может остаться секрет `d8-provider-cluster-configuration` со старой конфигурацией провайдера. Команда `d8 system edit provider-cluster-configuration` позволяет редактировать эту конфигурацию, но платформа уже не применяет изменения образа в ней.
{% endalert %}

Выберите нужную вкладку, в зависимости от способа настройки облачного провайдера:

{% tabs os_image_configuration %}
{% tab "Через ModuleConfig" %}

<a id="замена-образа-при-настройке-через-moduleconfig"></a>

При настройке провайдера [через ModuleConfig](#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec) образ для узлов типов `CloudEphemeral` и `CloudPermanent`, включая master-узлы, задаётся в инстанс-классе.
Чтобы заменить образ, измените инстанс-класс, на который ссылается нужная группа узлов:

1. Откройте конфигурацию группы узлов:

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o yaml
   ```

   Вместо `<NODE_GROUP_NAME>` укажите имя группы узлов. Для master-узлов укажите `master`.

   Определите тип узлов по полю `spec.nodeType`, а тип и имя инстанс-класса по полям `spec.cloudInstances.classReference.kind` и `spec.cloudInstances.classReference.name`.
   Если инстанс-класс используется несколькими группами, изменение образа затронет все эти группы.

1. Укажите новый образ или шаблон виртуальной машины в инстанс-классе группы узлов и сохраните файл:

   ```shell
   d8 k edit <PROVIDER>instanceclass <INSTANCE_CLASS_NAME>
   ```

   Вместо `<PROVIDER>` укажите имя облачного провайдера строчными буквами, например `yandex`, а вместо `<INSTANCE_CLASS_NAME>` укажите имя инстанс-класса.

   Пример для Yandex Cloud:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: YandexInstanceClass
   metadata:
     name: worker
   spec:
     cores: 4
     memory: 8192
     imageID: fd83ergat2e815oohe7o
   ```

{% offtopic title="Пример конфигурации master-узлов во встроенной виртуализации" %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DVPInstanceClass
metadata:
  name: master
spec:
  rootDisk:
    # Остальные параметры инстанс-класса не показаны.
    image:
      kind: ClusterVirtualImage
      name: ubuntu-24-04-20260204
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: DVPInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
```

{% endofftopic %}

После изменения инстанс-класса перейдите к [применению изменений](#применение-изменений).

{% endtab %}
{% tab "Через ClusterConfiguration" %}

При настройке провайдера через `<PROVIDER>ClusterConfiguration` ресурс для изменения образа зависит от типа узлов.
Выберите тип узлов группы.

{% tabs os_image_legacy_node_type %}
{% tab "CloudEphemeral" %}

Для узлов типа `CloudEphemeral` образ задаётся в инстанс-классе, на который ссылается группа узлов:

1. Получите тип и имя инстанс-класса из полей `spec.cloudInstances.classReference.kind` и `spec.cloudInstances.classReference.name` ресурса NodeGroup:

   ```shell
   d8 k get nodegroup <NODE_GROUP_NAME> -o yaml
   ```

   Вместо `<NODE_GROUP_NAME>` укажите имя группы узлов.

1. Откройте инстанс-класс и измените параметр образа из таблицы выше:

   ```shell
   d8 k edit <PROVIDER>instanceclass <INSTANCE_CLASS_NAME>
   ```

   Вместо `<PROVIDER>` укажите имя облачного провайдера строчными буквами, например `yandex`, а вместо `<INSTANCE_CLASS_NAME>` укажите имя инстанс-класса.

{% endtab %}
{% tab "CloudPermanent" %}

Для узлов типа `CloudPermanent` измените образ в конфигурации провайдера:

1. Откройте конфигурацию облачного провайдера:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Измените параметр с образом в секции `instanceClass` нужной группы в списке `nodeGroups` и сохраните файл. Пример для Yandex Cloud:

   ```yaml
   nodeGroups:
   - name: front
     replicas: 2
     instanceClass:
       cores: 4
       memory: 8192
       imageID: fd83ergat2e815oohe7o
   ```

{% endtab %}
{% tab "Master-узлы" %}

Для master-узлов измените образ в конфигурации провайдера:

1. Откройте конфигурацию облачного провайдера:

   ```shell
   d8 system edit provider-cluster-configuration
   ```

1. Измените параметр образа в секции `masterNodeGroup.instanceClass` и сохраните файл.

{% endtab %}
{% endtabs %}
{% endtab %}
{% endtabs %}

После сохранения конфигурации примените изменения в соответствии с типом узлов.

#### Применение изменений

Способ применения изменений зависит от типа узлов и одинаков для обоих способов настройки провайдера.
Выберите нужную вкладку.

{% tabs os_image_apply %}
{% tab "CloudEphemeral" %}

<a id="замена-образа-у-узлов-cloudephemeral"></a>

Deckhouse Platform автоматически пересоздаёт узлы типа `CloudEphemeral` после изменения образа в инстанс-классе.
Чтобы проверить завершение замены, выполните следующие шаги:

1. Дождитесь, пока Deckhouse Platform пересоздаст узлы группы:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

   Вместо `<NODE_GROUP_NAME>` укажите имя группы узлов.

   Во время замены в списке могут одновременно присутствовать прежние и новые узлы. Новые узлы отличаются меньшим значением в колонке `AGE`. Пример вывода во время замены:

   ```console
   NAME                                     STATUS     ROLES    AGE     VERSION
   cloud-demo-worker-92f9049e-gm2lw-6hkm2   NotReady   worker   37s     v1.34.1
   cloud-demo-worker-92f9049e-xn29v-66rm9   Ready      worker   8m21s   v1.34.1
   ```

   Замена завершена, когда в списке остаются только новые узлы в статусе `Ready`.

{% endtab %}
{% tab "CloudPermanent и master-узлы" %}

<a id="замена-образа-у-узлов-cloudpermanent"></a>
<a id="замена-образа-у-master-узлов"></a>

Чтобы применить новый образ к узлам типа `CloudPermanent` и master-узлам, запустите `dhctl converge`.
Команда читает актуальную конфигурацию из кластера, включая ModuleConfig, NodeGroup и инстанс-классы при настройке провайдера через ModuleConfig.

При замене master-узлов предварительно создайте [резервную копию etcd](admin/configuration/backup/backup-and-restore.html#резервное-копирование-etcd).
Также сохраните директорию `/etc/kubernetes` и скопируйте обе резервные копии за пределы кластера.
Master-узлы пересоздаются по одному. В кластере с одним master-узлом команда может предложить временный переход к трём master-узлам и обратный переход после замены.

1. На локальной машине получите редакцию и версию Deckhouse Platform в кластере:

   ```shell
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}')
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]')
   ```

1. На локальной машине запустите контейнер установщика Deckhouse Platform той же редакции и версии:

   ```shell
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.ru/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

   Если кластер использует другое хранилище образов контейнеров, укажите его адрес вместо `registry.deckhouse.ru`.

1. В контейнере установщика проверьте, какие изменения будут внесены в облако:

   ```shell
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host=<MASTER_IP>
   ```

   Вместо `<SSH_KEY_FILENAME>` укажите имя файла закрытого SSH-ключа, вместо `<USERNAME>` укажите пользователя для подключения к master-узлу, а вместо `<MASTER_IP>` укажите адрес master-узла.

   При замене master-узлов укажите адреса всех master-узлов, повторив параметр `--ssh-host` для каждого адреса.

   В выводе `dhctl terraform check` проверьте, что изменения узлов и остальных облачных ресурсов соответствуют ожидаемым. В итоговом списке `nodes` узлы, для которых требуется пересоздание, получают статус `destructively_changed`.
   Если один инстанс-класс используется несколькими группами, проверьте изменения для каждой из них.

1. В контейнере установщика примените изменения с теми же параметрами подключения по SSH, что и при проверке:

   ```shell
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host=<MASTER_IP>
   ```

   Команда выводит план изменений и перед заменой каждого узла запрашивает подтверждение `Do you want to CHANGE objects state in the cloud?`. Чтобы заменить узел, введите `y`. Также команда просит подтвердить, что имя master-узла соответствует его адресу.

   Перед подтверждением замены следующего master-узла проверьте состояние etcd и control plane по [инструкции проверки master-узлов](admin/configuration/platform-scaling/control-plane/scaling-and-changing-master-nodes.html#изменение-образа-ос-master-узлов-в-мультимастерном-кластере).

1. Проверьте, что узлы группы пересозданы:

   ```shell
   d8 k get nodes -l node.deckhouse.io/group=<NODE_GROUP_NAME>
   ```

   Вместо `<NODE_GROUP_NAME>` укажите имя группы узлов. Для master-узлов укажите `master`.
   Дождитесь, пока новые узлы перейдут в статус `Ready`.

{% endtab %}
{% endtabs %}
