---
title: "Cloud provider — DVP: layouts"
description: "Schemes of placement and interaction of resources in Deckhouse when working with the DVP cloud provider"
---

## Standard

![Standard layout](images/dvp-standard.png)


Example of the layout configuration:

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
