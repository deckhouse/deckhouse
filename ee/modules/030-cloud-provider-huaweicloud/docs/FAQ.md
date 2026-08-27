---
title: "Cloud provider — Huawei Cloud: FAQ"
---

## How do I set up security policies on cluster nodes?

### Default security groups

When [`internalNetworkSecurity: true`](cluster_configuration.html#huaweicloudclusterconfiguration-standard-internalnetworksecurity) (the default), the module creates a security group named after the cluster prefix and assigns it to the nodes.

The created group allows the following inbound connections by default:

* TCP/22 (SSH) from `0.0.0.0/0` by default;
* ICMP from `0.0.0.0/0`;
* TCP NodePort range `30000–32767` from `0.0.0.0/0` (UDP NodePorts are not opened by default).

Unlike OpenStack, the “all inbound traffic from nodes in the same security group” rule is not created by default in Huawei Cloud.

The module does not add HTTP/HTTPS or other application ports to the managed security group. Create such rules manually in a separate security group and attach it to the nodes.

### Attaching custom security groups

For CloudEphemeral nodes, additional security groups are set in the [`HuaweiCloudInstanceClass`](cr.html#huaweicloudinstanceclass) resource via [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). They are applied together with the security group created by the module.
