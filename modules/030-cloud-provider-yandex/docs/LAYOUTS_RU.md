---
title: "Cloud provider — Yandex Cloud: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Yandex Cloud при работе облачного провайдера Deckhouse Platform."
---

Поддерживаются три схемы размещения. Схема размещения задаётся параметром [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout) в секции ресурса ModuleConfig модуля. Ниже подробнее о каждой их них.

## Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из [выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

{% alert level="warning" %}
Так как в данной схеме размещения узлы создаются без публичных IP-адресов, для установки кластера на master-узел должен быть SSH-доступ с машины, на которой запускается dhctl: напрямую по приватной сети или через bastion-хост.
Если master-узел недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host`, `--ssh-bastion-user` и, при необходимости, `--ssh-bastion-port`.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](images/yandex-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10422&t=IvETjbByf1MSQzcm-0 --->

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

## WithoutNAT

В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдается публичный IP-адрес.

{% alert level="warning" %}
Модуль `cloud-provider-yandex` не назначает узлам группы безопасности, поэтому в этой схеме размещения узлы доступны извне без ограничений. Ограничить трафик можно правилами группы по умолчанию, как описано в разделе [«Настройка групп безопасности»](environment.html#настройка-групп-безопасности).
{% endalert %}

![Схема размещения WithoutNAT в Yandex Cloud](images/yandex-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10557&t=IvETjbByf1MSQzcm-0 --->

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

## WithNATInstance

В данной схеме размещения в отдельной подсети создаётся NAT-инстанс, а в таблицу маршрутизации подсетей зон добавляется правило с маршрутом на `0.0.0.0/0` с NAT-инстансом в качестве nexthop.
Подсеть выделяется для предотвращения петли маршрутизации и не должна пересекаться с другими сетями, используемыми в кластере.

Для размещения NAT-инстанса в существующей подсети используйте параметр `withNATInstance.internalSubnetID` — инстанс будет создан в зоне, соответствующей этой подсети.

Если необходимо создать новую подсеть, укажите параметр `withNATInstance.internalSubnetCIDR` — в ней будет размещён NAT-инстанс.

Обязателен один из параметров: `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`.

При указании параметра `withNATInstance.internalSubnetCIDR` NAT-инстанс будет создан в единственном экземпляре в зоне `a`.

При указании параметра `withNATInstance.internalSubnetID` NAT-инстанс будет создан в единственном экземпляре в зоне, указанной в настройках обозначенной сети.

Если `withNATInstance.externalSubnetID` указан в дополнение к предыдущим, NAT-инстанс будет подключен к нему через вторичный интерфейс.

![Схема размещения WithNATInstance в Yandex Cloud](images/yandex-withnatinstance.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10034&t=IvETjbByf1MSQzcm-0 --->

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

NAT-инстанс не используется инсталлятором `dhctl` автоматически в качестве bastion-хоста или jump-хоста. Для установки кластера у master-узла должен быть SSH-доступ с машины, на которой запускается `dhctl`: напрямую или через bastion-хост.

Если master-узел не имеет публичного IP-адреса или недоступен напрямую, запускайте установку с параметрами `--ssh-bastion-host` и `--ssh-bastion-user`.

Если необходимо подключаться к master-узлу напрямую по публичному IP-адресу, укажите для него параметры [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses) и [`externalSubnetIDs`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalsubnetids) в секции `nodes.parameters` ресурса ModuleConfig модуля `cloud-provider-yandex`. Учитывайте, что интерфейс из сети, указанной в `externalSubnetIDs`, будет использоваться как шлюз по умолчанию для узла.
{% endalert %}
