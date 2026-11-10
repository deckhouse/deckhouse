---
title: Схемы размещения и настройка zVirt
permalink: ru/admin/integrations/virtualization/zvirt/layout.html
lang: ru
---

## Схемы размещения

[Схема размещения](../../../../glossary.html#layout) определяет, как узлы кластера размещаются в инфраструктуре zVirt. Поддерживается только схема размещения Standard. Схема размещения задаётся параметром [`layout`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-layout) в ресурсе ModuleConfig модуля [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/).

### Standard

В схеме размещения Standard все узлы кластера создаются в одном кластере zVirt. Особенности схемы:

- виртуальные машины создаются в кластере zVirt, заданном параметром [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid);
- в кластере zVirt должны быть общие домены хранения, доступные всем его хостам, и процессоры одного типа;
- виртуальные машины клонируются из шаблона, подготовленного на основе cloud-образа;
- сетевой интерфейс виртуальной машины создаётся на основе профиля vNIC, заданного параметром [`vnicProfileID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-vnicprofileid) ресурса ZvirtInstanceClass;
- Deckhouse Platform создаёт и удаляет виртуальные машины узлов через API zVirt.

![Схема размещения Standard в zVirt](../../../../images/cloud-provider-zvirt/zvirt-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11447&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        server: <ZVIRT_API_URL>
        clusterID: <CLUSTER_ID>
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: <SSH_PUBLIC_KEY>
```

## Конфигурация

Параметры облачного провайдера zVirt задаются в четырёх ресурсах:

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-zvirt`](/modules/cloud-provider-zvirt/) задаёт [схему размещения](#схемы-размещения), параметры подключения к API zVirt и кластер zVirt, в котором создаются виртуальные машины;
- секрет `d8-credentials` в неймспейсе `d8-cloud-provider-zvirt` хранит логин и пароль пользователя zVirt в формате, описанном [в разделе «Секрет с учётными данными»](authorization.html#секрет-с-учётными-данными);
- [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass) описывает параметры виртуальных машин;
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает группы узлов и ссылается на инстанс-класс. Master-узлы описываются группой узлов `master` с типом [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html).

Чтобы изменить настройки провайдера в работающем кластере, отредактируйте ModuleConfig:

```shell
d8 k edit mc cloud-provider-zvirt
```

Чтобы применить изменения к master-узлам и узлам CloudPermanent-групп, выполните команду:

```shell
dhctl converge
```

{% alert level="warning" %}
Если кластер был установлен со схемой [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration), необходима миграция на конфигурацию через ModuleConfig.
Пока миграция не выполнена, может срабатывать [алерт `D8CloudProviderZvirtMigrationPending`](../../../../reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending) и блокироваться обновление Deckhouse Platform.

Порядок миграции описан [в разделе «Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?»](../../../../faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).
{% endalert %}

### Пример конфигурации

Ниже приведён пример минимальной конфигурации кластера со схемой размещения Standard, одним master-узлом и одной группой рабочих узлов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  enabled: true
  settings:
    provider:
      parameters:
        server: <ZVIRT_API_URL>
        clusterID: <CLUSTER_ID>
    nodes:
      parameters:
        layout: Standard
        sshPublicKey: <SSH_PUBLIC_KEY>
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-zvirt
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: userPassword
  identity: <USERNAME>
  secret: <PASSWORD>
---
apiVersion: deckhouse.io/v1
kind: ZvirtInstanceClass
metadata:
  name: master
spec:
  numCPUs: 4
  memory: 8192
  rootDiskSizeGb: 50
  etcdDiskSizeGb: 10
  template: <TEMPLATE_NAME>
  vnicProfileID: <VNIC_PROFILE_ID>
  storageDomainID: <STORAGE_DOMAIN_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: ZvirtInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: ZvirtInstanceClass
metadata:
  name: worker
spec:
  numCPUs: 4
  memory: 8192
  template: <TEMPLATE_NAME>
  vnicProfileID: <VNIC_PROFILE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: ZvirtInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
```

Где:

- `ZVIRT_API_URL` — URL-адрес эндпоинта API zVirt (параметр [`server`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-server)), например `https://zvirt.example.com/ovirt-engine/api`;
- `CLUSTER_ID` — идентификатор кластера zVirt, в котором создаются виртуальные машины (параметр [`clusterID`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-clusterid));
- `SSH_PUBLIC_KEY` — публичный SSH-ключ для доступа к узлам (параметр [`sshPublicKey`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-sshpublickey));
- `USERNAME` и `PASSWORD` — логин и пароль пользователя zVirt [для секрета с учётными данными](authorization.html#секрет-с-учётными-данными);
- `TEMPLATE_NAME` — имя шаблона виртуальной машины, который подготавливается по инструкции [из раздела «Подготовка шаблона виртуальной машины»](authorization.html#подготовка-шаблона-виртуальной-машины);
- `VNIC_PROFILE_ID` — идентификатор профиля vNIC, на основе которого создаётся сетевой интерфейс виртуальной машины;
- `STORAGE_DOMAIN_ID` — идентификатор домена хранения zVirt, в котором создаётся диск для etcd.

Количество master-узлов задаётся параметром [`minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone) группы узлов `master`. В ресурсе ZvirtInstanceClass группы `master` задайте параметры [`etcdDiskSizeGb`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-etcddisksizegb) и [`storageDomainID`](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass-v1-spec-storagedomainid), которые задают размер и домен хранения отдельного диска для etcd. В инстанс-классах других CloudPermanent-групп параметр `etcdDiskSizeGb` задавать нельзя.

Если API zVirt использует самоподписанный сертификат, передайте сертификат CA в параметре [`caBundle`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-cabundle) или установите для параметра [`insecure`](/modules/cloud-provider-zvirt/configuration.html#parameters-provider-parameters-insecure) значение `true`.

Остальные параметры описаны [в настройках модуля](/modules/cloud-provider-zvirt/configuration.html) и в описании ресурса [ZvirtInstanceClass](/modules/cloud-provider-zvirt/cr.html#zvirtinstanceclass).

{% alert level="info" %}
Идентификаторы `clusterID`, `vnicProfileID` и `storageDomainID` можно получить через API zVirt или портал администрирования. Получение идентификатора профиля vNIC через API описано [в разделе «Как получить идентификатор профиля vNIC?»](/modules/cloud-provider-zvirt/faq.html#как-получить-идентификатор-профиля-vnic).
{% endalert %}

### Статическая настройка сети узлов

Узлы, для группы которых не задана статическая настройка сети, получают сетевые настройки по DHCP. Чтобы назначить узлам CloudPermanent-групп статические IP-адреса, задайте параметр [`customNetworkConfigs`](/modules/cloud-provider-zvirt/configuration.html#parameters-nodes-parameters-customnetworkconfigs) в секции `nodes.parameters` ресурса ModuleConfig. Ключ параметра задаёт имя группы узлов, а значение задаёт настройку сетевого интерфейса узлов этой группы.

Пример статической настройки сети для группы `master` из трёх узлов:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-zvirt
spec:
  version: 2
  settings:
    nodes:
      parameters:
        customNetworkConfigs:
          master:
            networkInterfaceName: enp1s0
            networkInterfaceAddresses:
              - 192.168.1.10
              - 192.168.1.11
              - 192.168.1.12
            networkInterfaceNetmask: 255.255.255.0
            networkInterfaceGateway: 192.168.1.1
            dnsServers:
              - 192.168.1.2
```

Адреса из списка `networkInterfaceAddresses` назначаются узлам группы по порядку, по одному адресу на узел, поэтому укажите в списке адрес для каждого узла группы. Повторяющиеся адреса в списке не допускаются.

{% alert level="warning" %}
Если изменить сетевые настройки существующих узлов в параметре `customNetworkConfigs`, команда `dhctl converge` пересоздаёт виртуальные машины этих узлов.
{% endalert %}
