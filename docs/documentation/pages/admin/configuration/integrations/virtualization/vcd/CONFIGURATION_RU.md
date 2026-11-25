---
title: Настройка интеграции с VMware Cloud Director
permalink: ru/admin/integrations/virtualization/vcd/configuration.html
lang: ru
---

## Конфигурация

Интеграция осуществляется с помощью [ресурса VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration), который описывает конфигурацию [облачного кластера](../../../../glossary.html#cloud-cluster) в VCD и используется системой виртуализации, если управляющий слой (control plane) кластера размещён в системе. Отвечающий за интеграцию модуль Deckhouse Platform настраивается автоматически, исходя из выбранной [схемы размещения](../../../../glossary.html#layout).

Чтобы изменить конфигурацию в запущенном кластере, выполните следующую команду:

```shell
d8 system edit provider-cluster-configuration
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
