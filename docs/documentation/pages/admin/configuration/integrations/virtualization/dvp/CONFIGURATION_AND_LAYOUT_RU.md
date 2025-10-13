---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/virtualization/dvp/configuration-and-layout-scheme.html
lang: ru
---

![Схема размещения Standard](../../../../images/cloud-provider-dvp/dvp-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=1314-7740&t=5VUUyoMpasR1vVxZ-4 --->

Пример конфигурации схемы размещения:

```yaml
---
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
sshPublicKey: ssh-rsa AAAABBBB
masterNodeGroup:
  replicas: 1
  instanceClass:
    virtualMachine:
      cpu:
        cores: 4
        coreFraction: 100%
      memory:
        size: 8Gi
      ipAddresses:
        - Auto
      virtualMachineClassName: generic
    rootDisk:
      size: 50Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
    etcdDisk:
      size: 15Gi
      storageClass: ceph-pool-r2-csi-rbd-immediate
provider:
  kubeconfigDataBase64: <KUBE_CONFIG>
  namespace: demo
```

## Конфигурация

Deckhouse использует ресурс [DVPClusterConfiguration](/modules/cloud-provider-dvp/cluster_configuration.html#dvpclusterconfiguration) для описания конфигурации кластера в DVP.

Для изменения конфигурации в работающем кластере выполните:

```shell
d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

После изменения параметров узлов выполните команду:

```shell
dhctl converge
```

Пример конфигурации с множеством параметров:

```yaml
apiVersion: deckhouse.io/v1
kind: DVPClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
- zone-a
- zone-b
- zone-c
region: r1
masterNodeGroup:
  replicas: 3
  zones:
  - zone-a
  - zone-b
  - zone-c
  instanceClass:
    virtualMachine:
      cpu:
        cores: 1
        coreFraction: 100%
      memory:
        size: 4Gi
      virtualMachineClassName: generic
      ipAddresses:
      - 10.66.30.100
      - 10.66.30.101
      - 10.66.30.102
      additionalLabels:
        additional-vm-label: label-value
      additionalAnnotations:
        additional-vm-annotation: annotation-value
      tolerations:
      - key: dedicated.deckhouse.io
        operator: Equal
        value: system
      nodeSelector:
        beta.kubernetes.io/os: linux
    rootDisk:
      size: 10Gi
      storageClass: linstor-thin-r1
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
    etcdDisk:
      size: 10Gi
      storageClass: linstor-thin-r1
nodeGroups:
- name: worker
  zones:
  - zone-a
  - zone-b
  - zone-c
  replicas: 1
  instanceClass:
    virtualMachine:
      cpu:
        cores: 4
        coreFraction: 100%
      memory:
        size: 8Gi
      virtualMachineClassName: generic
    rootDisk:
      size: 10Gi
      image:
        kind: ClusterVirtualImage
        name: ubuntu-2204
provider:
  kubeconfigDataBase64: ZXhhbXBsZQo=
  namespace: default
```
