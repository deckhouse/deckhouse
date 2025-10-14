---
title: Layouts and configuration
permalink: en/admin/integrations/virtualization/zvirt/layout.html
---

## Standard

The Standard layout is used to integrate Deckhouse Kubernetes Platform with a zVirt virtual infrastructure.
This layout assumes that all nodes are deployed within a single zVirt cluster
with centralized management of templates, storage, and networking.

Key features:

- Use of a single zVirt cluster ([`clusterID`](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration-clusterid)).
- A storage domain accessible to all hosts in the cluster.
- A virtual machine template created from a cloud image.
- Assignment of a vNIC network profile when provisioning VMs.
- Full automation of node provisioning and removal via the zVirt API.

![Standard layout in zVirt](../../../../images/cloud-provider-zvirt/zvirt-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11447&t=Qb5yyWumzPiTBtfL-0 --->

Configuration example:

```yaml
apiVersion: deckhouse.io/v1
kind: ZvirtClusterConfiguration
layout: Standard
sshPublicKey: "ssh-rsa AAAABBBB"
clusterID: "b46372e7-0d52-40c7-9bbf-fda31e187088"
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
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

Required parameters for the [ZvirtClusterConfiguration](/modules/cloud-provider-zvirt/cluster_configuration.html#zvirtclusterconfiguration) resource:

- `clusterID`: UUID of the zVirt cluster where virtual machines will be deployed.
- `sshPublicKey`: Public SSH key used to access the nodes.
- `template`: Name of the prepared VM template.
- `vnicProfileID`: UUID of the vNIC network profile.
- `storageDomainID`: UUID of the storage domain where disks will be placed.

{% alert level="info" %}
UUID values (`clusterID`, `vnicProfileID`, and `storageDomainID`) can be obtained via the zVirt API or administrator interface.
{% endalert %}
