![Standard layout](/images/gs/cloud-provider-dvp/dvp-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=1314-7740&t=5VUUyoMpasR1vVxZ-4 --->

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
      virtualMachineClassName: <VIRTUAL_MACHINE_CLASS_NAME>
    rootDisk:
      size: 50Gi
      storageClass: <STORAGE_CLASS>
      image:
        kind: ClusterVirtualImage
        name: <CLUSTER_VIRTUAL_IMAGE_NAME>
    etcdDisk:
      size: 15Gi
      storageClass: <STORAGE_CLASS>
provider:
  kubeconfigDataBase64: <KUBE_CONFIG>
  namespace: demo
```
