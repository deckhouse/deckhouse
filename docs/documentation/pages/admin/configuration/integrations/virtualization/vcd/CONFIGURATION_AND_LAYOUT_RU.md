---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/virtualization/vcd/configuration-and-layout-scheme.html
lang: ru
---

## Схемы размещения

Deckhouse Kubernetes Platform поддерживает две схемы размещения ресурсов в VCD.

### Standard

![Схема размещения Standard](../../../../images/cloud-provider-vcd/vcd-standard.png)
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
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetwork: internal
    mainNetworkIPAddresses:
    - 192.168.199.2
    mainNetwork: internal
```

### WithNAT

![Схема размещения WithNAT](../../../../images/cloud-provider-vcd/vcd-withnat.png)

При использовании данной схемы размещения необходимо уточнить у администратора тип платформы сетевой виртуализации и указать его в [параметре `edgeGateway.type`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-type). Поддерживаются два варианта: `NSX-T` и `NSX-V`.

Для обеспечения административного доступа к узлам кластера разворачивается бастион. Параметры для его настройки описываются [в секции `bastion`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-bastion).

Если Edge Gateway работает на базе `NSX-T`, в созданной сети для узлов автоматически активируется DHCP-сервер. Он будет выделять IP-адреса, начиная с 30-го адреса в подсети и до предпоследнего (перед broadcast-адресом). Начальный адрес DHCP-пула можно изменить с помощью [параметра `internalNetworkDHCPPoolStartAddress`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-internalnetworkdhcppoolstartaddress).

Если используется `NSX-V`, DHCP необходимо настроить вручную. В противном случае узлы, ожидающие получение IP-адреса по DHCP, не смогут его получить.

{% alert level="warning" %}
Не рекомендуется использовать динамическую адресацию для первого master-узла совместно с `NSX-V`.
{% endalert %}

Схема размещения предполагает автоматическое создание следующих правил NAT:

- SNAT — трансляция адресов внутренней сети узлов во внешний адрес, указанный в [параметре `edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip).
- DNAT — трансляция внешнего адреса и порта, заданных в параметрах [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip) и [`edgeGateway.externalPort`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalport), на внутренний IP-адрес бастиона по порту 22 (протокол TCP) для обеспечения административного доступа по SSH.

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-V`, то для построения правил необходимо указать имя и тип сети, к которым правило будет привязано в свойствах [`edgeGateway.NSX-V.externalNetworkName`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworkname) и [`edgeGateway.NSX-V.externalNetworkType`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworktype) соответственно. Как правило, это сеть, подключённая к Edge Gateway в разделе `Gateway Interfaces` и имеющая внешний IP-адрес.
{% endalert %}

Дополнительно возможно создание правил брандмауэра [отдельным свойством `createDefaultFirewallRules`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-createdefaultfirewallrules).

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

## Конфигурация

Интеграция осуществляется с помощью [ресурса VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration), который описывает конфигурацию облачного кластера в VCD и используется системой виртуализации, если управляющий слой (control plane) кластера размещён в системе. Отвечающий за интеграцию модуль DKP настраивается автоматически, исходя из выбранной схемы размещения.

Чтобы изменить конфигурацию в запущенном кластере, выполните следующую команду:

```shell
d8 platform edit provider-cluster-configuration
```

{% alert level="info" %}
После изменения параметров узлов необходимо выполнить команду `dhctl converge`, чтобы изменения вступили в силу.
{% endalert %}

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: Cloud
mainNetwork: internal
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      template: Org/Templates/ubuntu-focal-20.04
      sizingPolicy: 16cpu32ram
      storageProfile: ssd
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
```

Количество и параметры процесса заказа машин в облаке настраиваются в кастомном ресурсе [NodeGroup](/modules/node-manager/cr.html#nodegroup), в котором также указывается название используемого для этой группы узлов инстанс-класса (параметр `cloudInstances.classReference`). Инстанс-класс для cloud-провайдера VCD — это кастомный ресурс [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), в котором указываются конкретные параметры самих машин.

Ниже представлен пример конфигурации VCDInstanceClass для эфемерных узлов cloud-провайдера VMware Cloud Director.

### Пример конфигурации кастомного ресурса VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```

### Storage

Для каждого Datastore и DatastoreCluster из зон (зоны) автоматически создаётся StorageClass.

Имя StorageClass'а, который будет использоваться в кластере по умолчанию, можно настроить (параметр [default](#parameters-storageclass-default)) и отфильтровать ненужные StorageClass'ы (параметр [exclude](#parameters-storageclass-exclude)).

#### CSI

Подсистема хранения по умолчанию использует CNS-диски с возможностью изменения их размера на лету. Но также поддерживается работа и в legacy-режиме с использованием FCD-дисков. Поведение подсистемы устанавливается с помощью параметра [compatibilityFlag](#parameters-storageclass-compatibilityflag).
