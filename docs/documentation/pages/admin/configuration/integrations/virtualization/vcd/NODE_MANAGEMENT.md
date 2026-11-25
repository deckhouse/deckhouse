---
title: Node management in VMware Cloud Director
permalink: en/admin/integrations/virtualization/vcd/node-management.html
lang: en
---

The number of nodes to be provisioned and their parameters are defined in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource, where you also specify the name of the [instance class](../../../../glossary.html#instance-class) used for that group (the `cloudInstances.classReference` parameter). For the VCD [cloud provider](../../../../glossary.html#cloud-provider), the instance class is a custom resource called [VCDInstanceClass](/modules/cloud-provider-vcd/cr.html#vcdinstanceclass), which contains the specific configuration of the VMs.

The following is the example configuration of VCDInstanceClass for ephemeral nodes of the VMware Cloud Director cloud provider.

## Example configuration of VCDInstanceClass

```yaml
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: test
spec:
  rootDiskSizeGb: 90
  sizingPolicy: payg-4-8
  storageProfile: SSD-dc1-pub1-cl1
  template: MyOrg/Linux/ubuntu2204-cloud-ova
```
