---
title: Настройка сети в Huawei Cloud
permalink: ru/admin/integrations/private/huaweicloud/network.html
lang: ru
---

## Группы безопасности

Параметр `internalNetworkSecurity` для схем [Standard](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-standard-internalnetworksecurity) и [VpcPeering](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-vpcpeering-internalnetworksecurity) (по умолчанию `true`) включает создание группы безопасности (SecurityGroup) при создании кластера. Deckhouse Platform создаёт группу с именем префикса кластера и назначает её узлам.

Будут созданы следующие правила входящего трафика:

- разрешение входящего трафика по протоколу TCP и порту `22` из `0.0.0.0/0`;
- разрешение входящего трафика по протоколу ICMP из `0.0.0.0/0`;
- разрешение входящего трафика по протоколу TCP и портам `30000`–`32767` для использования сервисов типа `NodePort`. Входящий трафик по протоколу UDP на порты `NodePort` по умолчанию не разрешается.

Собственные группы безопасности для CloudEphemeral-узлов задаются в ресурсе [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) параметром [`spec.securityGroups`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-securitygroups) и применяются вместе с группой, созданной Deckhouse Platform.
