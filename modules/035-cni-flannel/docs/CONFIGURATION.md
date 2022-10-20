---
title: "The cni-flannel module: configuration"
---

The module is **automatically** enabled for the following cloud providers:
- [OpenStack](../../modules/030-cloud-provider-openstack/);
- [VMware vSphere](../../modules/030-cloud-provider-vsphere/).

{% include module-enable.liquid moduleName="cni-flannel" %}

## Parameters

<!-- SCHEMA -->

## An example of the configuration

```yaml
cniFlannel: |
  podNetworkMode: VXLAN
```
