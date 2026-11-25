---
title: Схемы размещения Yandex Cloud
permalink: ru/admin/integrations/public/yandex/layout.html
lang: ru
---

## Схемы размещения

Данный раздел описывает возможные [схемы размещения](../../../../glossary.html#layout) узлов кластера в инфраструктуре Yandex Cloud. От выбора схемы зависят принципы сетевого взаимодействия, наличие публичных IP-адресов, маршрутизация исходящего трафика
и способ подключения к узлам. Схема размещения задаётся параметром [`layout`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-layout)
в ресурсе ModuleConfig модуля [`cloud-provider-yandex`](/modules/cloud-provider-yandex/).

### Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из
[выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно
добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

{% alert level="warning" %}
Так как в данной схеме размещения узлы создаются без публичных IP-адресов, для установки кластера на [master-узел](../../../../glossary.html#master-node)
должен быть SSH-доступ с машины, на которой запускается dhctl: напрямую по приватной сети или через [bastion-хост](../../../../glossary.html#bastion-host).
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
Deckhouse Platform не назначает узлам группы безопасности, поэтому в этой схеме размещения узлы доступны извне без ограничений. Ограничить трафик можно
правилами группы по умолчанию, как описано в разделе [«Настройка групп безопасности в Yandex Cloud»](network.html#настройка-групп-безопасности-в-yandex-cloud).
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
В примере master и [worker-узлы](../../../../glossary.html#worker-node) создаются без публичных IP-адресов и без дополнительных внешних сетевых интерфейсов. Исходящий трафик из подсетей кластера проходит через NAT-инстанс.
{% endalert %}

[Настройка интеграции](configuration.html).
[Настройка сети](network.html).
