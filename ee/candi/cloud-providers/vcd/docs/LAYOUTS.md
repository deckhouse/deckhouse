---
title: "Cloud provider - VMware Cloud Director: Layouts"
description: "Schemes of placement and interaction of resources in VMware Cloud Director when working with the Deckhouse cloud provider."
---

Before reading this document, make sure you are familiar with the [Cloud provider layout](/deckhouse/docs/documentation/pages/CLOUD-PROVIDER-LAYOUT.md).

One layout is supported.

## Standard

* To be able to process incoming traffic, DNAT rules must be configured manually.
* To be able to process outgoing traffic, SNAT rules must be configured manually.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vRGmMErKA7NCWKtZ6b0DTW6DfP9P3n4F4IhkK7CYae35cygF9npthYfbGp2KM1Mm75FpMIDfmTozU6i/pub?w=1000&h=774)
<!--- Source: https://docs.google.com/drawings/d/1aosnFD7AzBgHrQGvxxQHZPfV0PSaTM66A-EPMWgPEqw/edit --->

Example of the layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetwork: internal
    mainNetworkIPAddresses:
    - 192.168.199.10
```
