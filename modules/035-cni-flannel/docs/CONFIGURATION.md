---
title: "The cni-flannel module: configuration"
---

The module is **automatically** enabled for the following cloud providers:
- [OpenStack](../../modules/030-cloud-provider-openstack/);
- [VMware vSphere](../../modules/030-cloud-provider-vsphere/).

To enable it for the bare metal machines, add the following parameter to the `deckhouse` configMap:
```
cniFlannelEnabled: "true"
```

## Parameters

<!-- SCHEMA -->

## An example of the configuration
```yaml
cniFlannel: |
  podNetworkMode: VXLAN
```
