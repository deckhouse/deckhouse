---
title: Настройка сети в VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/network.html
lang: ru
---

## Настройка сети для WithNAT

При использовании [схемы WithNAT](layout.html#withnat) необходимо уточнить у администратора тип платформы сетевой виртуализации и указать его в [параметре `edgeGateway.type`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-type). Поддерживаются два варианта: `NSX-T` и `NSX-V`.

Для обеспечения административного доступа к узлам кластера разворачивается бастион. Параметры для его настройки описываются [в секции `bastion`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-bastion).

### DHCP

Если Edge Gateway работает на базе `NSX-T`, в созданной сети для узлов автоматически активируется DHCP-сервер. Он будет выделять IP-адреса, начиная с 30-го адреса в подсети и до предпоследнего (перед broadcast-адресом). Начальный адрес DHCP-пула можно изменить с помощью [параметра `internalNetworkDHCPPoolStartAddress`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-internalnetworkdhcppoolstartaddress).

Если используется `NSX-V`, DHCP необходимо настроить вручную. В противном случае узлы, ожидающие получение IP-адреса по DHCP, не смогут его получить.

{% alert level="warning" %}
Не рекомендуется использовать динамическую адресацию для первого [master-узла](../../../../glossary.html#master-node) совместно с `NSX-V`.
{% endalert %}

### Правила NAT

Схема размещения предполагает автоматическое создание следующих правил NAT:

- SNAT — трансляция адресов внутренней сети узлов во внешний адрес, указанный в [параметре `edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip).
- DNAT — трансляция внешнего адреса и порта, заданных в параметрах [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip) и [`edgeGateway.externalPort`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalport), на внутренний IP-адрес бастиона по порту 22 (протокол TCP) для обеспечения административного доступа по SSH.

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-V`, то для построения правил необходимо указать имя и тип сети, к которым правило будет привязано в свойствах [`edgeGateway.NSX-V.externalNetworkName`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworkname) и [`edgeGateway.NSX-V.externalNetworkType`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworktype) соответственно. Как правило, это сеть, подключённая к Edge Gateway в разделе `Gateway Interfaces` и имеющая внешний IP-адрес.
{% endalert %}

### Правила брандмауэра

Дополнительно возможно создание правил брандмауэра [отдельным свойством `createDefaultFirewallRules`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-createdefaultfirewallrules).

{% alert level="warning" %}
Если Edge Gateway обеспечивается средствами `NSX-T`, то существующие в Edge Gateway правила будут перезаписаны. Предполагается, что использование данной опции подразумевает размещение одного кластера на Edge Gateway.
{% endalert %}

Будут созданы следующие правила:

- Разрешение любого исходящего трафика;
- Разрешение входящего трафика по протоколу `TCP` и 22 порту для соединения с узлами кластера по SSH;
- Разрешение любого входящего трафика по протоколу `ICMP`;
- Разрешение входящего трафика по протоколам `TCP` и `UDP` и портам 30000–32767 для использования `NodePort`.
