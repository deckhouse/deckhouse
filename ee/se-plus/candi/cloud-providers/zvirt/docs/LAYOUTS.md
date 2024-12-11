---
title: "Cloud provider - zVirt: Layouts"
description: "Schemes of placement and interaction of resources in zVirt when working with the Deckhouse cloud provider."
---

One layout is supported.

## Standard

* Network infrastructure should be configured manually.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vT38mXMMBEoVwOyq0yicOIukzVeAP_uxmOC0Kpz3LSVuP7Q-tq2NioZNfkKf2u6-Jsk_dzHsaaWA27S/pub?w=667&h=516)
<!--- Исходник: https://docs.google.com/drawings/d/1xeM2JZtnlfTmP44MzjvKmXroSIbrJhK4AYyVVs5HA1Y/edit --->

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
