---
title: "Cloud provider — Huawei Cloud: configuration"
force_searchable: true
---

The module is automatically enabled for all cloud clusters deployed in Huawei Cloud.

## List of required Huawei Cloud services

A list of services required for Deckhouse Kubernetes Platform to work in Huawei Cloud:

| Service                         | API version |
|:--------------------------------|:-----------:|
| Identity                        | [v3](https://support.huaweicloud.com/intl/en-us/api-iam/iam_30_0001.html) |
| Compute                         | [v2.1](https://support.huaweicloud.com/intl/en-us/api-ecs/ecs_04_0001.html) |
| Network (VPC)                   | [v1/v2](https://support.huaweicloud.com/intl/en-us/api-vpc/vpc-api-pdf.pdf) |
| Network (OpenStack Neutron API) | [v2.0](https://support.huaweicloud.com/intl/en-us/api-vpc/vpc-api-pdf.pdf) |
| Block Storage                   | [v3](https://support.huaweicloud.com/intl/en-us/api-evs/evs_04_2065.html) |
| Load Balancing (ELB) *          | [v3](https://support.huaweicloud.com/intl/en-us/api-elb/CreateLoadBalancer.html) |

\* Required if load balancers need to be provisioned in the cluster.

{% include module-alerts.liquid %}

The module does not have any settings.
