---
title: Схемы размещения и настройка Yandex Cloud
permalink: ru/admin/integrations/public/yandex/layout.html
lang: ru
---

## Схемы размещения

Данный раздел описывает возможные схемы размещения узлов кластера в инфраструктуре Yandex Cloud и связанные с ними настройки. От выбора схемы зависят принципы сетевого взаимодействия, наличие публичных IP-адресов, маршрутизация исходящего трафика и способ подключения к узлам. Схема размещения задаётся параметром [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout) в ресурсе ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

### Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из [выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

{% alert level="warning" %}
Так как в данной схеме размещения узлы создаются без публичных IP-адресов, для установки кластера на master-узел должен быть SSH-доступ с машины, на которой запускается dhctl: напрямую по приватной сети или через bastion-хост.
Если master-узел недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host`, `--ssh-bastion-user` и, при необходимости, `--ssh-bastion-port`.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10422&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

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
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
          - ru-central1-b
          - ru-central1-d
        labels:
          billing: prod
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

### WithoutNAT

В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдается публичный IP-адрес.

{% alert level="warning" %}
Deckhouse Platform (DP) не назначает узлам группы безопасности, поэтому в этой схеме размещения узлы доступны извне без ограничений. Ограничить трафик можно правилами группы по умолчанию, как описано в разделе [«Настройка групп безопасности в Yandex Cloud»](#настройка-групп-безопасности-в-yandex-cloud).
{% endalert %}

![Схема размещения WithoutNAT в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10557&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

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
        layout: WithoutNAT
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
          - ru-central1-b
          - ru-central1-d
        externalIPAddresses:
          master:
            - Auto
            - Auto
            - Auto
          worker:
            - <ZONE_A_EXTERNAL_IP_WORKER_1>
            - Auto
        externalSubnetIDs:
          master:
            - <ZONE_A_SUBNET_ID>
            - <ZONE_B_SUBNET_ID>
            - <ZONE_D_SUBNET_ID>
          worker:
            - <ZONE_A_SUBNET_ID>
            - <ZONE_B_SUBNET_ID>
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

### WithNATInstance

В данной схеме размещения в отдельной подсети создается NAT-инстанс, а в таблицу маршрутизации подсетей зон добавляется правило с маршрутом на `0.0.0.0/0` с NAT-инстансом в качестве nexthop.
Подсеть выделяется для предотвращения петли маршрутизации и не должна пересекаться с другими сетями, используемыми в кластере.

Для размещения NAT-инстанса в существующей подсети используйте [параметр `withNATInstance.internalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetid). Инстанс создаётся в зоне, которой принадлежит эта подсеть.

Если необходимо создать новую подсеть, укажите [параметр `withNATInstance.internalSubnetCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-internalsubnetcidr). NAT-инстанс размещается в созданной подсети.

Задайте один из двух параметров, `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`.

Если также указан [`withNATInstance.externalSubnetID`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-externalsubnetid), NAT-инстанс будет подключён к этой подсети через вторичный сетевой интерфейс.

![Схема размещения WithNATInstance в Yandex Cloud](../../../../images/cloud-provider-yandex/yandex-withnatinstance.png)
<!--- Исходник: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

Пример конфигурации схемы размещения:

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
        layout: WithNATInstance
        sshPublicKey: <SSH_PUBLIC_KEY>
        nodeNetworkCIDR: 192.168.12.0/24
        existingNetworkID: <EXISTING_NETWORK_ID>
        zones:
          - ru-central1-a
        withNATInstance:
          natInstanceExternalAddress: <NAT_INSTANCE_EXTERNAL_ADDRESS>
          internalSubnetID: <INTERNAL_SUBNET_ID>
          externalSubnetID: <EXTERNAL_SUBNET_ID>
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - <DNS_SERVER_1>
            - <DNS_SERVER_2>
```

{% alert level="info" %}
В примере master и worker-узлы создаются без публичных IP-адресов и без дополнительных внешних сетевых интерфейсов. Исходящий трафик из подсетей кластера проходит через NAT-инстанс.

NAT-инстанс не используется инсталлятором `dhctl` автоматически в качестве bastion-хоста или jump-хоста. Для установки кластера на master-узел должен быть SSH-доступ с машины, на которой запускается `dhctl`: напрямую или через bastion-хост.

Если master-узел не имеет публичного IP-адреса или недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host` и `--ssh-bastion-user`.

Если необходимо подключаться к master-узлу напрямую по публичному IP-адресу, укажите для него параметры [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) и [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/). Учитывайте, что интерфейс из сети, указанной в `externalSubnetIDs`, будет использоваться как шлюз по умолчанию для узла.
{% endalert %}

## Конфигурация

Параметры облачного провайдера Yandex Cloud задаются в четырёх ресурсах:

- [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) задаёт [схему размещения](#схемы-размещения), [сетевые параметры](#сетевые-параметры-и-безопасность) и идентификаторы облака и каталога;
- секрет `d8-credentials` в неймспейсе `d8-cloud-provider-yandex` хранит ключ сервисного аккаунта, его формат описан в разделе [«Секрет с учётными данными»](authorization.html#секрет-с-учётными-данными);
- [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass) описывает параметры виртуальных машин;
- [NodeGroup](/modules/node-manager/cr.html#nodegroup) описывает группы узлов и ссылается на инстанс-класс. Группа master-узлов описывается такой же группой с типом [`CloudPermanent`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html).

Чтобы изменить настройки провайдера в работающем кластере, отредактируйте ModuleConfig:

```shell
d8 k edit mc cloud-provider-yandex
```

После изменения параметров узлов выполните команду:

```shell
dhctl converge
```

{% alert level="warning" %}
Если кластер был установлен со схемой [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration), необходима миграция на конфигурацию через ModuleConfig. Пока миграция не выполнена, срабатывает [алерт `D8CloudProviderYandexCloudMigrationPending`](../../../../reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending), а обновление Deckhouse Platform может блокироваться.

Инструкция приведена в разделе [«Как мигрировать облачный провайдер на конфигурацию через ModuleConfig?»](../../../../faq.html#как-мигрировать-облачный-провайдер-на-конфигурацию-через-modulec).

Если в настройках модуля был задан параметр `storageClass.default`, после миграции укажите StorageClass по умолчанию глобальным параметром [`defaultClusterStorageClass`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-defaultclusterstorageclass). Конверсия это значение не переносит.
{% endalert %}

### Пример конфигурации

Ниже приведён пример минимальной конфигурации кластера со схемой размещения Standard, одним master-узлом и одной группой рабочих узлов:

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
        nodeNetworkCIDR: 10.241.32.0/20
        sshPublicKey: <SSH_PUBLIC_KEY>
        labels:
          billing: prod
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: master
spec:
  cores: 4
  memory: 8192
  imageID: <IMAGE_ID>
  etcdDiskSizeGB: 10
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: master
    minPerZone: 1
    maxPerZone: 1
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: worker
spec:
  cores: 4
  memory: 8192
  coreFraction: 50
  imageID: <IMAGE_ID>
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: worker
    minPerZone: 1
    maxPerZone: 3
    zones:
      - ru-central1-a
```

Где:

- `CLOUD_ID` и `FOLDER_ID` — идентификаторы облака и каталога Yandex Cloud, задаваемые параметрами [`cloudID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-cloudid) и [`folderID`](/modules/cloud-provider-yandex/configuration.html#parameters-provider-parameters-folderid) в секции `provider.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/). Их получение описано в разделе [«Подключение и авторизация в Yandex Cloud»](authorization.html);
- `SSH_PUBLIC_KEY` — публичный SSH-ключ для доступа к узлам, задаваемый параметром [`sshPublicKey`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-sshpublickey) в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/);
- `IMAGE_ID` — идентификатор образа операционной системы, из которого создаются виртуальные машины, задаваемый параметром [`imageID`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-imageid) ресурса YandexInstanceClass. Если параметр не задан, используется образ из инстанс-класса группы `master`.

В ресурсе YandexInstanceClass для группы `master` необходимо указать параметр [`etcdDiskSizeGB`](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass-v1-spec-etcddisksizegb), который задаёт размер отдельного диска под etcd. Для остальных групп узлов этот параметр указывать нельзя.

Остальные параметры описаны в [настройках модуля](/modules/cloud-provider-yandex/configuration.html) и в описании ресурса [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).

Ключ сервисного аккаунта в ModuleConfig не хранится, он задаётся [отдельным секретом](authorization.html#секрет-с-учётными-данными).

## Сетевые параметры и безопасность

Далее описаны настройки, связанные с адресацией, маршрутизацией, внешним трафиком и безопасностью сети в кластере Deckhouse Platform, развернутом в Yandex Cloud.

### Внутренняя адресация узлов кластера

[Параметр `nodeNetworkCIDR`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-nodenetworkcidr) задаёт диапазон IP-адресов, который распределяется между зонами доступности и применяется к внутренним интерфейсам узлов. Параметр задаётся в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        nodeNetworkCIDR: 192.168.12.0/24
```

Платформа делит эту подсеть на четыре равные части и создаёт из них по одной внутренней подсети в каждой зоне доступности:

- ru-central1-a
- ru-central1-b
- ru-central1-e
- ru-central1-d

К созданной подсети подключаются узлы соответствующей зоны. Деление выполняется во всех схемах размещения. Если задан [параметр `existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap), DP подсети не создаёт и использует указанные в нём.

{% alert level="info" %}
К подсети привязывается только одна таблица маршрутизации, поэтому два кластера с модулем [`cni-simple-bridge`](/modules/cni-simple-bridge/) не могут использовать одни и те же подсети. Чтобы развернуть такие кластеры в общих подсетях, используйте модуль [`cni-cilium`](/modules/cni-cilium/).
{% endalert %}

### Назначение внешних IP-адресов и исходящего трафика

[Параметр `externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) задаётся в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) отдельно для каждой группы узлов:

- ключ — имя группы узлов;
- значение — список идентификаторов подсетей Yandex Cloud, в которые подключаются внешние сетевые интерфейсы узлов этой группы, в порядке зон размещения.

Параметр обязателен, если требуется:

- назначить публичный IP-адрес;
- определить маршрут по умолчанию для исходящего трафика с узлов;
- использовать значение `Auto` в параметре [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses).

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        externalSubnetIDs:
          worker:
            - <RU_CENTRAL1_A_SUBNET_ID>
            - <RU_CENTRAL1_B_SUBNET_ID>
            - <RU_CENTRAL1_D_SUBNET_ID>
```

Параметр [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) задаёт список внешних IP-адресов для каждой группы узлов в порядке зон размещения.

### Настройка DNS и DHCP для внутренних сетей

[Параметр `dhcpOptions`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-dhcpoptions) задаёт настройки DHCP-сервера, которые применяются ко всем подсетям, создаваемым в кластере Deckhouse Platform в Yandex Cloud. Параметр задаётся в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

Доступные поля:

- `domainName` — поисковый домен (search domain), который устанавливается в конфигурации сети.
- `domainNameServers` — массив IP-адресов DNS-серверов, которые используются как рекурсивные резолверы.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        dhcpOptions:
          domainName: test.local
          domainNameServers:
            - 192.168.0.2
            - 192.168.0.3
```

{% alert level="info" %}
Указанные DNS-серверы в `domainNameServers` обязательно должны разрешать внешние и внутренние доменные зоны, используемые в кластере. В противном случае возможны сбои в работе сервисов.
{% endalert %}

После внесения изменений в `dhcpOptions`:

- принудительно обновите DHCP lease на всех виртуальных машинах;
- перезапустите компоненты, работающие в сети узла (`hostNetwork: true`), в том числе `kube-dns`, чтобы пересчитать содержимое `resolv.conf`.

Для применения изменений можно использовать:

```shell
netplan apply
```

или другой способ, принятый в вашей системе, например `systemd-networkd` или `dhclient`.

### Использование заранее созданных подсетей

[Параметр `existingZoneToSubnetIDMap`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-existingzonetosubnetidmap) задаёт соответствие между зонами доступности и ранее созданными подсетями в Yandex Cloud. Укажите его, если DP должна использовать готовые подсети, а не создавать новые. Параметр задаётся в секции `nodes.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

Пример использования:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  settings:
    nodes:
      parameters:
        existingZoneToSubnetIDMap:
          ru-central1-a: e2lu8r1tbbtryhdpa9ro
          ru-central1-b: e2lu8r1tbbtryhdpa9ro
          ru-central1-d: e2lu8r1tbbtryhdpa9ro
```

{% alert level="info" %}
Платформа создаёт таблицу маршрутизации, но не привязывает её к подсетям. Привяжите таблицу вручную в интерфейсе Yandex Cloud.
{% endalert %}

### Дополнительные внешние сети

[Параметр `additionalExternalNetworkIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-ccm-parameters-additionalexternalnetworkids) задаёт список дополнительных внешних сетей, адреса из которых DP считает публичными. Параметр задаётся в секции `ccm.parameters` ресурса ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

Эта настройка полезна, если:

- имеются внешние подсети, не указанные в `externalSubnetIDs`, но требующие учёта как внешние;
- требуется точный контроль над интерпретацией IP-адресов при настройке балансировки, маршрутизации и экспорта статуса;
- используются нестандартные схемы подключения, например через внешние NAT-шлюзы или ручное резервирование IP.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-yandex
spec:
  version: 2
  enabled: true
  settings:
    ccm:
      parameters:
        additionalExternalNetworkIDs:
          - enp6t4sno
```

Если параметр не задан, DP будет использовать только те подсети, что явно указаны в настройках модуля (например, через `externalSubnetIDs`), чтобы определять публичность IP.

## Настройка групп безопасности в Yandex Cloud

Для каждой [облачной сети](https://cloud.yandex.ru/ru/docs/vpc/concepts/network#network) Yandex Cloud создаёт [группу безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups) по умолчанию, в том числе для сети кластера. Эта группа разрешает любой входящий и исходящий трафик и применяется к тем сетевым интерфейсам виртуальных машин, которым не назначена другая группа.

Ограничить трафик кластера можно правилами этой группы. Назначить узлам другую группу безопасности нельзя, параметров для этого нет ни в настройках модуля, ни в ресурсе YandexInstanceClass.

{% alert level="warning" %}
Не назначайте группы безопасности узлам вручную в консоли Yandex Cloud. Команда `dhctl converge` приводит виртуальные машины к состоянию, описанному в конфигурации кластера, и может снять такую привязку.
{% endalert %}

{% alert level="danger" %}
Не удаляйте правила, разрешающие любой трафик, пока не настроите остальные правила группы. Иначе группа заблокирует трафик между узлами и компоненты кластера потеряют связь друг с другом.
{% endalert %}

Ошибки в правилах нарушают работу кластера. Перед настройкой правил в production-кластере изучите [особенности работы групп безопасности](https://cloud.yandex.ru/ru/docs/vpc/concepts/security-groups#security-groups-notes) в Yandex Cloud.

Если в кластере используется Cilium с туннелированием трафика подов через VXLAN, разрешите UDP-трафик между узлами на портах, которые перечислены в разделе [«Сетевое взаимодействие компонентов платформы»](../../../../reference/network_interaction.html).

Чтобы настроить правила группы безопасности:

1. Определите облачную сеть, в которой работает кластер Deckhouse Platform.

   Название сети совпадает с параметром [`prefix`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-prefix) ModuleConfig `global`.
   Его можно узнать с помощью команды:

   ```bash
   d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
   ```

   Если значение пустое, возьмите его из устаревшего параметра [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) ресурса ClusterConfiguration (префикс перенесён в ModuleConfig `global`):

   ```bash
   d8 k get secrets -n kube-system d8-cluster-configuration -ojson | \
     jq -r '.data."cluster-configuration.yaml"' | base64 -d | grep prefix | cut -d: -f2
   ```

1. В консоли Yandex Cloud выберите сервис Virtual Private Cloud и перейдите в раздел «Группы безопасности». У вас должна отображаться одна группа безопасности с пометкой `Default`.

   ![Группа безопасности по умолчанию](../../../../images/cloud-provider-yandex/sg-ru-default.png)

1. Создайте правила согласно [инструкции Yandex Cloud](https://cloud.yandex.ru/ru/docs/managed-kubernetes/operations/connect/security-groups#rules-internal).

   ![Правила для группы безопасности](../../../../images/cloud-provider-yandex/sg-ru-rules.png)

1. Удалите правило, разрешающее любой **входящий** трафик (на скриншоте выше оно уже удалено), и сохраните изменения.

## Настройка доступа через bastion-хост

Для подключения к узлам, находящимся в приватных подсетях (например, при использовании схемы размещения Standard или WithNATInstance), используется bastion-хост, промежуточная машина с публичным IP-адресом, через которую выполняется SSH-доступ к узлам.

Для настройки доступа выполните следующие шаги:

1. Выполните bootstrap базовой инфраструктуры. Перед созданием bastion-хоста необходимо выполнить начальную фазу установки DP, которая подготовит сетевую инфраструктуру:

   ```shell
   dhctl bootstrap-phase base-infra --config config.yml
   ```

1. Создайте bastion-хост в Yandex Cloud:

   ```shell
   yc compute instance create \
     --name bastion \
     --hostname bastion \
     --create-boot-disk image-family=ubuntu-2204-lts,image-folder-id=standard-images,size=20,type=network-hdd \
     --memory 2 \
     --cores 2 \
     --core-fraction 100 \
     --ssh-key ~/.ssh/<SSH_PUBLIC_KEY_FILE> \
     --zone ru-central1-a \
     --public-address 178.154.226.159
   ```

   Замените `<SSH_PUBLIC_KEY_FILE>` на имя публичного ключа, например `id_rsa.pub` для ключа RSA или `id_ed25519.pub` для ключа ED25519.

   Убедитесь, что IP-адрес из параметра `--public-address` доступен из вашей сети и указан корректно.

1. Запустите основной bootstrap DP через bastion-хост:

   ```shell
   dhctl bootstrap --ssh-bastion-host=178.154.226.159 --ssh-bastion-user=yc-user \
     --ssh-user=ubuntu --ssh-agent-private-keys=/tmp/.ssh/<SSH_PRIVATE_KEY_FILE> --config=/config.yml
   ```

   Замените `<SSH_PRIVATE_KEY_FILE>` на имя приватного ключа, например `id_rsa` для ключа RSA или `id_ed25519` для ключа ED25519.

   Здесь:

   - `--ssh-bastion-user` — пользователь для подключения к bastion-хосту;
   - `--ssh-user` — пользователь на целевых узлах кластера;
   - `--ssh-agent-private-keys` — путь до приватного SSH-ключа;
   - `--config` — путь до конфигурационного файла DP.
