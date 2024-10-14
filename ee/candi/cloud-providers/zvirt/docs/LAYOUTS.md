---
title: "Cloud provider - zVirt: Layouts"
description: "Schemes of placement and interaction of resources in VMware Cloud Director when working with the Deckhouse cloud provider."
---

## Standard

![resources](../../images/030-cloud-provider-zvirt/network/zvirt-standard.svg)
<!--- Исходник: https://docs.google.com/drawings/d/1aosnFD7AzBgHrQGvxxQHZPfV0PSaTM66A-EPMWgPEqw/edit --->

Example of the layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAABBBB"
clusterID: "b46372e7-0d52-40c7-9bbf-fda31e187088"
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 4
    memory: 8192
    rootDiskSizeGb: 40
    template: ALT-p10
    vnicProfileID: "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
    storageDomainID: "c4bf82a5-b803-40c3-9f6c-b9398378f424"
```
