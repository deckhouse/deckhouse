---
title: "Cloud provider — VMware vSphere: examples"
---

Below is an example configuration for a VMware vSphere cloud provider.

## An example of the configuration

The example is a module configuration named `cloud-provider-vsphere`, which is used with VMware vSphere. The module configuration contains connection settings, the path to the virtual machine folder, network names, security settings, and NSX settings that can be used to manage and monitor instances running on vSphere.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-vsphere
spec:
  version: 1
  enabled: true
  settings:
    host: vc-3.internal
    username: user
    password: password
    vmFolderPath: dev/test
    insecure: true
    region: moscow-x001
    sshKeys:
    - "ssh-rsa AAAAB3N....6xHJwwj"
    externalNetworkNames:
    - KUBE-3
    - devops-internal
    internalNetworkNames:
    - KUBE-3
    - devops-internal
    nsxt:
      defaultIpPoolName: "External IP Pool"
      tier1GatewayPath: flant_tier1
      user: guestuser1
      password: pass
      host: 1.2.3.4
      insecureFlag: true
      size: SMALL
```

## An example of the `VsphereInstanceClass` custom resource

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereInstanceClass
metadata:
  name: test
spec:
  numCPUs: 2
  memory: 2048
  rootDiskSize: 20
  template: dev/golden_image
  network: k8s-msk-178
  datastore: lun-1201
```
