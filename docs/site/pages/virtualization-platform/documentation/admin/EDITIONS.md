---
title: "Platform editions"
permalink: en/virtualization-platform/documentation/admin/editions.html
---

The Deckhouse Virtualization Platform is available in Community Edition (CE) and Enterprise Edition (EE). DVP editions differ in their set of features and the level of available support.

The table below provides brief a comparison of editions listing its main features and functions:

{% capture coming_soon %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.coming_soon[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture read_only %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.read_only[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture snapshot_ee %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.supported_storage[page.lang] }}" aria-expanded="false">{% endcapture %}
{% capture snapshot_ce %}<img src="/images/icons/intermediate_v2.svg" title="{{ site.data.i18n.common.restrictions_and_supported_storage[page.lang] }}" aria-expanded="false">{% endcapture %}
{% assign not_supported = '<img src="/images/icons/not_supported_v2.svg">' %}
{% assign supported = '<img src="/images/icons/supported_v2.svg">' %}

| Feature                                                              | CE                     | EE                  |
|----------------------------------------------------------------------|------------------------|---------------------|
| Declarative resources creation (GitOps ready)                        | {{ supported }}        | {{ supported }}     |
| Scaling up to 1000 nodes and 50000 VMs                               | {{ supported }}        | {{ supported }}     |
| Hypervisor maintenance mode                                          | {{ supported }}        | {{ supported }}     |
| High availability of VMs in case of hypervisor failure               | {{ supported }}        | {{ supported }}     |
| **Resource planning**                                                |                        |                     |
| Resource quotas at the project level                                 | {{ supported }}        | {{ supported }}     |
| Policies for sizing virtual machines (VirtualMachineClass)           | {{ supported }}        | {{ supported }}     |
| Unification of CPU instructions on hypervisors (VirtualMachineClass) | {{ supported }}        | {{ supported }}     |
| **Management capabilities**                                          |                        |                     |
| Administrator web interface                                          | {{ read_only }}      | {{ supported }}     |
| Management through CLI and access via API                            | {{ supported }}        | {{ supported }}     |
| Importing VM images and disks (qcow, vmdk, raw, vdi)                 | {{ supported }}        | {{ supported }}     |
| Public and project images for creating virtual machines              | {{ supported }}        | {{ supported }}     |
| Customization of the VM OS at first launch                           | {{ supported }}        | {{ supported }}     |
| Live VM migration without downtime                                   | {{ supported }}        | {{ supported }}     |
| Consistent disk snapshots                                            | {{ snapshot_ce }}      | {{ snapshot_ee }}     |
| Adding and changing VM disk sizes without rebooting                  | {{ supported }}        | {{ supported }}     |
| VM launch policies                                                   | {{ supported }}        | {{ supported }}     |
| VM placement management (affinity/antiaffinity)                      | {{ supported }}        | {{ supported }}     |
| **Data storage**                                                     |                        |                     |
| Built-in SDS                                                         | {{ supported }}        | {{ supported }}     |
| Support for hardware storage systems using API (Yadro, Huawei, HPE)  | {{ not_supported }}    | {{ supported }}     |
| Universal support for hardware storage systems (SCSI-generic)        | {{ not_supported }}    | {{ supported }}     |
| Support for NFS                                                      | {{ supported }}        | {{ supported }}     |
| Support for third-party SDS (Ceph)                                   | {{ supported }}        | {{ supported }}     |
| **Network capabilities (SDN)**                                       |                        |                     |
| Network policies (micro-segmentation)                                | {{ supported }}        | {{ supported }}     |
| Built-in load balancer                                               | {{ supported }}        | {{ supported }}     |
| External load balancer based on MetalLB                              | {{ not_supported }}    | {{ supported }}     |
| Active health check load balancer                                    | {{ not_supported }}    | {{ supported }}     |
| Static routing management                                            | {{ not_supported }}    | {{ supported }}     |
| Egress Gateway                                                       | {{ not_supported }}    | {{ supported }}     |
| **Security**                                                         |                        |                     |
| Multitenancy (projects)                                              | {{ supported }}        | {{ supported }}     |
| Flexible role-based access model                                     | {{ supported }}        | {{ supported }}     |
| Integration with external authentication providers (LDAP, OIDC)      | {{ supported }}        | {{ supported }}     |
| Data in Transit encryption                                           | {{ supported }}        | {{ supported }}     |
| Certificate management                                               | {{ supported }}        | {{ supported }}     |
| Deploying to an air-gapped environment                               | {{ not_supported       | {{ supported }}     |
| **Monitoring**                                                       |                        |                     |
| Built-in monitoring and logging of infrastructure and VMs            | {{ supported }}        | {{ supported }}     |
| Sending metrics and logs to external collectors                      | {{ supported }}        | {{ supported }}     |
| **Support**                                                          |                        |                     |
| Community support                                                    | {{ supported }}        | {{ supported }}     |
| [Extended technical support (8/5)](/tech-support/)                   | {{ not_supported }}    | {{ supported }}     |
| [Extended technical support (24/7)](/tech-support/)                  | {{ not_supported }}    | {{ supported }}     |
