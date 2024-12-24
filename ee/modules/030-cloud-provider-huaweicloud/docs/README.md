---
title: "Cloud provider — HuaweiCloud"
---

The `cloud-provider-huaweicloud` module is responsible for interacting with the [HuaweiCloud](https://www.huaweicloud.com/intl/en-us/) cloud resources. It allows the [node manager](../../modules/040-node-manager/) module to use HuaweiCloud resources for provisioning nodes for the specified [node group](../../modules/040-node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

The `cloud-provider-huaweicloud` module:
- Manages HuaweiCloud resources using the `cloud-controller-manager` (CCM) module
- Provisions disks using the `CSI storage` component.
- Registers with the [node-manager](../../modules/040-node-manager/) module so that [HuaweicloudInstanceClasses](cr.html#huaweicloudinstanceclass) can be used when creating the [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup).
