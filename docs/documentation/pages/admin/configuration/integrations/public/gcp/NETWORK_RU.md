---
title: Настройка сети в Google Cloud Platform
permalink: ru/admin/integrations/public/gcp/network.html
lang: ru
---

## Правила межсетевого экрана

При создании кластера Deckhouse Platform создаёт в VPC кластера следующие правила файрвола:

- `<CLUSTER_PREFIX>-ssh-and-ping` — разрешение входящего трафика по протоколам ICMP и TCP (порт `22`) к узлам с network tag `<CLUSTER_PREFIX>` из CIDR, указанных в [`sshAllowList`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-sshallowlist) (по умолчанию `0.0.0.0/0`);
- `<CLUSTER_PREFIX>-intercommunication` — разрешение любого трафика между узлами с network tag `<CLUSTER_PREFIX>`, а также из подсети подов ([`podSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-podsubnetcidr)).

Собственные правила файрвола применяются к узлам через дополнительные network tags ([`additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags)):

- для master-узлов — в параметре [`masterNodeGroup.instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags) ресурса [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration);
- для [статических узлов](../../../../glossary.html#static-node) — в параметре [`nodeGroups[].instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-additionalnetworktags) ресурса [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration);
- для эфемерных узлов — в параметре [`spec.additionalNetworkTags`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-additionalnetworktags) ресурса [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass).
