---
title: "Cloud provider - VMware vSphere: Layouts"
description: "Schemes of placement and interaction of resources in VMware vSphere when working with the Deckhouse cloud provider."
---

One layout is supported.

## Standard

* To be able to process incoming traffic, DNAT rules must be configured manually.
* To be able to process outgoing traffic, SNAT rules must be configured manually.
* If `useNestedResourcePool: true` is set in the `VsphereClusterConfiguration`, a separate [resource pool](https://registry.terraform.io/providers/hashicorp/vsphere/latest/docs/data-sources/resource_pool) is created for the cluster.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vQW4nFq5MBnSGBbMaohNl7SPuU4NfjVoeH2O1W0bUbNlUg9kX0tt1gVPZo7ia7TFYXTRXFghKxSpqgS/pub?w=667&h=516)
<!--- Source: https://docs.google.com/drawings/d/16gL-oBQDps2uxlq-M8gZzBdRWHCSI5H64u5pRS5BHyI/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VsphereClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
vmFolderPath: dev
regionTagCategory: k8s-region
zoneTagCategory: k8s-zone
region: X1
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
nodeGroups:
- name: khm
  replicas: 1
  zones:
  - ru-central1-a
  instanceClass:
    numCPUs: 4
    memory: 8192
    template: dev/golden_image
    datastore: dev/lun_1
    mainNetwork: net3-k8s
sshPublicKey: "<SSH_PUBLIC_KEY>"
zones:
- ru-central1-a
- ru-central1-b
```
