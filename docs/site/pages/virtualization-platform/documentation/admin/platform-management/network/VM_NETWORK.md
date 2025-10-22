---
title: "VM Network"
permalink: en/virtualization-platform/documentation/admin/platform-management/network/vm-network.html
---

Each virtual machine is assigned an address from the ranges specified in the `.spec.settings.virtualMachineCIDRs` section of the ModuleConfig [virtualization parameters](/modules/virtualization/configuration.html).

To view the current configuration, run the following command:

```bash
d8 k get mc virtualization -oyaml
```

Example of the output:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  enabled: true
  settings:
    dvcr:
      storage:
        persistentVolumeClaim:
          size: 60G
          storageClassName: linstor-thin-r1
        type: PersistentVolumeClaim
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.66.30.0/24
  version: 1
```

To edit the subnet list, run the following command:

```bash
d8 k edit mc virtualization
```

Addresses are assigned sequentially from each specified range,
excluding only the first (network address) and the last (broadcast address).

When an IP address is assigned to a virtual machine, a corresponding cluster resource [VirtualMachineIPAddressLease](/modules/virtualization/cr.html#virtualmachineipaddresslease) is created.
This resource is linked to the project resource [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress), which in turn is linked to the virtual machine.

If the VirtualMachineIPAddress resource is deleted, the IP address is detached but remains reserved for the project for 10 minutes after deletion.
