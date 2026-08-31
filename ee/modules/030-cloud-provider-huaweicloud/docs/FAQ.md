---
title: "Cloud provider — Huawei Cloud: FAQ"
---

## How do I set up security policies on cluster nodes?

Default security group rules are described in the [layouts](layouts.html) section.

For CloudEphemeral nodes, additional security groups are set in the [`HuaweiCloudInstanceClass`](cr.html#huaweicloudinstanceclass) resource via [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). They are applied together with the security group created by the module.
