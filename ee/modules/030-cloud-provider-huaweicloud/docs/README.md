---
title: "Cloud provider — Huawei Cloud"
description: "Cloud resource management in Deckhouse Kubernetes Platform using Huawei Cloud."
---

The `cloud-provider-huaweicloud` module is responsible for interacting with the [Huawei Cloud](https://www.huaweicloud.com/intl/en-us/) cloud resources. It allows the [node manager module](/modules/node-manager/) to use Huawei Cloud resources for provisioning nodes for the specified [node group](/modules/node-manager/cr.html#nodegroup) (a group of nodes that are acted upon as if they were a single entity).

Key features of the `cloud-provider-huaweicloud` module:

- Manages Huawei Cloud resources using the `cloud-controller-manager` (CCM) module
- Provisions disks using the `CSI storage` component
- Registers with the [node-manager](/modules/node-manager/) module so that [HuaweiCloudInstanceClasses](cr.html#huaweicloudinstanceclass) can be used when creating the [NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference)
