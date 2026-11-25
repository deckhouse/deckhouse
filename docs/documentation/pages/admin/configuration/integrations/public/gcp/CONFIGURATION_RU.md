---
title: Настройка интеграции с Google Cloud Platform
permalink: ru/admin/integrations/public/gcp/configuration.html
lang: ru
---

## Конфигурация

Интеграции с GCP осуществляется с помощью [ресурса GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration),
который описывает конфигурацию [облачного кластера](../../../../glossary.html#cloud-cluster) в GCP и используется
[облачным провайдером](../../../../glossary.html#cloud-provider), если управляющий слой (control plane) кластера размещён в облаке. Отвечающий
за интеграцию модуль Deckhouse Platform настраивается автоматически, исходя из выбранной [схемы размещения](../../../../glossary.html#layout).

Выполните следующую команду, чтобы изменить конфигурацию в работающем кластере:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
После изменения параметров узлов необходимо выполнить команду `dhctl converge`, чтобы изменения вступили в силу.
{% endalert %}

Пример конфигурации:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
sshKey: "<SSH_PUBLIC_KEY>"
subnetworkCIDR: 10.36.0.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
nodeGroups:
- name: static
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
    additionalNetworkTags:
    - tag1
    additionalLabels:
      kube-node: static
provider:
  region: europe-west3
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"
```
