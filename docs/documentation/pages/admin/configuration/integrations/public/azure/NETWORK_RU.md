---
title: Настройка сети в Microsoft Azure
permalink: ru/admin/integrations/public/azure/network.html
lang: ru
---

## Сетевые параметры

- Поддерживается использование NAT Gateway — можно явно задать количество публичных IP-адресов для SNAT через
  [параметр `natGatewayPublicIpCount`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-standard-natgatewaypublicipcount).
- Возможна настройка пиринга VNet, в том числе с [bastion-хостом](../../../../glossary.html#bastion-host) или другими VNet в облаке.

## Группы безопасности

При создании кластера Deckhouse Platform создаёт Network Security Group (NSG) с именем префикса кластера и привязывает её к подсети узлов.

Будут созданы следующие правила:

- `AllowIcmp` — разрешение входящего трафика по протоколу ICMP из любого источника;
- `AllowSsh` — разрешение входящего трафика по протоколу TCP и порту `22` из CIDR, указанных в [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist) (если список не задан — из любого источника).

Подключить заранее созданную собственную NSG к узлам через параметры Deckhouse Platform нельзя. Для ограничения SSH используйте [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist).
