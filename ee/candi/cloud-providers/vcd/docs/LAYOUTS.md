---
title: "Cloud provider - VMware Cloud Director: Layouts"
description: "Schemes of placement and interaction of resources in VMware Cloud Director when working with the Deckhouse cloud provider."
---

## Standard

![resources](../../images/030-cloud-provider-vcd/VCD-Standard.svg)
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
