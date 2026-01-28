---
title: "Cloud provider - VMware Cloud Director: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в VMware Cloud Director при работе облачного провайдера Deckhouse."
---

## Standard

![Схема размещения Standard](images/vcd-standard.png)
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

## WithNAT

![Схема размещения WithNAT](images/vcd-withnat.png)

При использовании данной схемы размещения необходимо уточнить у администратора тип платформы сетевой виртуализации и указать его в параметре `edgeGateway.type`.
Поддерживаются два варианта: `NSX-T` и `NSX-V`.

Для обеспечения административного доступа к узлам кластера разворачивается бастион. Параметры для его настройки описываются [в секции `bastion`](./cluster_configuration.html#vcdclusterconfiguration-bastion).

Если Edge Gateway работает на базе `NSX-T`, в созданной сети для узлов автоматически активируется DHCP-сервер. Он будет выделять IP-адреса, начиная с 30-го адреса в подсети и до предпоследнего (перед broadcast-адресом). Начальный адрес DHCP-пула можно изменить с помощью параметра `internalNetworkDHCPPoolStartAddress`.

Если используется `NSX-V`, DHCP необходимо настроить вручную. В противном случае узлы, ожидающие получение IP-адреса по DHCP, не смогут его получить.

{% alert level="warning" %}
Не рекомендуется использовать динамическую адресацию для первого master-узла совместно с `NSX-V`.
{% endalert %}

Схема размещения предполагает автоматическое создание следующих правил NAT:

- SNAT — трансляция адресов внутренней сети узлов во внешний адрес, указанный в параметре `edgeGateway.externalIP`.
- DNAT — трансляция внешнего адреса и порта, заданных в параметрах `edgeGateway.externalIP` и `edgeGateway.externalPort`, на внутренний IP-адрес бастиона по порту 22 (протокол TCP) для обеспечения административного доступа по SSH.

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-V`, то для построения правил необходимо указать имя и тип сети, к которым правило будет привязано в свойствах `edgeGateway.NSX-V.externalNetworkName` и `edgeGateway.NSX-V.externalNetworkType` соответственно. Как правило, это сеть, подключённая к Edge Gateway в разделе `Gateway Interfaces` и имеющая внешний IP-адрес.
{% endalert %}

Дополнительно возможно создание правил брандмауэра отдельным свойством `createDefaultFirewallRules`.

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-T`, то существующие в Edge Gateway правила будут перезаписаны. Предполагается, что использование данной опции подразумевает размещение одного кластера на Edge Gateway.
{% endalert %}

Будут созданы следующие правила:

- Разрешение любого исходящего трафика;
- Разрешение входящего трафика по протоколу `TCP` и 22 порту для соединения с узлами кластера по SSH;
- Разрешение любого входящего трафика по протоколу `ICMP`;
- Разрешение входящего трафика по протоколам `TCP` и `UDP` и портам 30000–32767 для использования `NodePort`.

Пример конфигурации схемы размещения с использованием `NSX-T`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: WithNAT
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
internalNetworkDNSServers:
  - 77.88.8.8
  - 1.1.1.1
mainNetwork: internal
bastion:
  instanceClass:
    rootDiskSizeGb: 30
    sizingPolicy: 2cpu1mem
    template: "catalog/Ubuntu 22.04 Server"
    storageProfile: Fast vHDD
    mainNetworkIPAddress: 10.1.4.10
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-T"
  externalIP: 10.0.0.1
  externalPort: 10022
createDefaultFirewallRules: false
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

Пример конфигурации схемы размещения с использованием `NSX-V`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: WithNAT
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
internalNetworkDNSServers:
  - 77.88.8.8
  - 1.1.1.1
mainNetwork: internal
bastion:
  instanceClass:
    rootDiskSizeGb: 30
    sizingPolicy: 2cpu1mem
    template: "catalog/Ubuntu 22.04 Server"
    storageProfile: Fast vHDD
    mainNetworkIPAddress: 10.1.4.10
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-V"
  externalIP: 10.0.0.1
  externalPort: 10022
  NSX-V:
    externalNetworkName: external
    externalNetworkType: ext
createDefaultFirewallRules: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```
