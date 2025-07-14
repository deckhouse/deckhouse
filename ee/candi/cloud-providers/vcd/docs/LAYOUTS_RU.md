---
title: "Cloud provider - VMware Cloud Director: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в VMware Cloud Director при работе облачного провайдера Deckhouse."
---

## Standard

![Схема размещения Standard](../../images/cloud-provider-vcd/vcd-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11247&t=IvETjbByf1MSQzcm-0 --->

Пример конфигурации схемы размещения:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

## StandardWithNetwork

При выборе этой схемы размещения необходимо узнать у администратора тип платформы сетевой виртуализации и указать ее в свойстве `edgeGateway.type`. Схема размещения поддерживает `NSX-T` и `NSX-V`.

Если Edge Gateway работает на базе `NSX-T`, в созданной сети для узлов будет автоматически активирован DHCP-сервер, предоставляющий IP-адреса начиная с 30‑го адреса сети и до предпоследнего адреса (перед broadcast). Если Edge Gateway обеспечивается средствами `NSX-V`, то необходимо настроить DHCP для планируемой сети узлов вручную. В противном случае узлы, предполагающие получение адреса динамически, не смогут получить адрес.

Схема размещения предполагает автоматизированное создание правил NAT:

- Правило SNAT для трансляции адресов внутренней сети узлов во внешний адрес, указанный в свойстве `edgeGateway.externalIP`
- Правило DNAT для трансляции внешних адреса и порта, указанных в свойствах `edgeGateway.externalIP` и `edgeGateway.externalPort` соответственно, во внутренний адрес первого master-узла на порт 22 по протоколу `TCP` для административного доступа к узлам по SSH.

{% alert level="warning" %}
Правило DNAT будет создано только в том случае, если IP-адрес первого master-узла задан статически. В противном случае потребуется ручное создание правила.
{% endalert %}

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-V`, то для построения правил необходимо указать имя и тип сети, к которым правило будет привязано в свойствах `edgeGateway.NSX-V.externalNetworkName` и `edgeGateway.NSX-V.externalNetworkType` соответственно. Как правило, это сеть, подключённая к Edge Gateway в разделе `Gateway Interfaces` и имеющая внешний IP-адрес.
{% endalert %}

Дополнительно возможно создание правил брандмауэра отдельным свойством `createDefaultFirewallRules`.

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-T`, то существующие в Edge Gateway правила будут перезаписаны. Предполагается, что использование данной опции подразумевает размещение одного кластера на Edge Gateway.
{% endalert %}

Будут созданы следующие правила:

- Разрешение любого исходящего трафика
- Разрешение входящего трафика по протоколу `TCP` и 22 порту для соединения с узлами кластера по SSH
- Разрешение любого входящего трафика по протоколу `ICMP`
- Разрешение входящего трафика по протоколам `TCP` и `UDP` и портам 30000–32767 для использования `NodePort`

Пример конфигурации схемы размещения с использованием NSX-T:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-T"
  externalIP: 10.0.0.1
  externalPort: 10022
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

Пример конфигурации схемы размещения с использованием NSX-V:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-V"
  externalIP: 10.0.0.1
  externalPort: 10022
  NSX-V:
    externalNetworkName: external
    externalNetworkType: ext
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```
