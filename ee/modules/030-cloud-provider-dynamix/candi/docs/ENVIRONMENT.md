---
title: "Cloud provider — Basis Dynamix: Preparing environment"
description: "Configuring Basis Dynamix for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

### Platform version

The module requires **Basis Dynamix 4.6 or newer**.

Starting with 4.6, the platform requires a storage policy for every disk and every virtual machine it creates: `storage_policy_id` is a mandatory field of `disks/create` and `kvmx86/create`. The module resolves a policy by name and discovers the policies available to the account through the `/restmachine/cloudapi/storage_policy/list` endpoint, which does not answer on earlier versions. A preflight check probes that endpoint, so `dhctl bootstrap` refuses to deploy a cluster on an older platform before any resource is created.

### Account permissions

The module operates entirely within the user API of Basis Dynamix (`/restmachine/cloudapi/*`). An account with administrative (`/restmachine/cloudbroker/*`) permissions is **not** required and should not be used: it widens the visibility scope beyond the cluster owner's account and breaks tenant isolation.

The account specified in `DynamixClusterConfiguration.provider` must be allowed to use the following user API groups:

| Group | Used by | Purpose |
| --- | --- | --- |
| `account`, `locations`, `rg` | all components | resolving the account, the location and the resource group by name |
| `extnet`, `vins` | terraform, CAPD | external and internal networks |
| `image` | terraform, CAPD | OS images for the nodes |
| `storage_policy` | terraform, CAPD, CSI, cloud-data-discoverer | listing the storage policies available to the account and resolving a policy by name |
| `disks` | terraform, CSI | disks of the master nodes and persistent volumes |
| `compute`, `kvmx86` | terraform, CCM, CAPD, CSI | virtual machines and disk attachment |
| `lb` | CCM | load balancers for `LoadBalancer` services |

{% alert level="info" %}
The module never chooses a storage endpoint or a pool for a disk. It only names a storage policy, and Basis Dynamix picks the placement within that policy itself.
{% endalert %}

### Prepare an operating system image

Operating system vendors typically provide special cloud builds of their operating systems for use in virtualization environments. These builds typically contain virtual hardware drivers, cloud-init, virtualization guest agents, and are distributed as IMG or QCOW2 disk images. We recommend that you use these cloud images as the OS on the nodes in your clusters.

The cloud image of the operating system must be placed in the "Images" → "Template Images" section of the Basis Dynamix portal. Follow these steps to upload the OS image to the storage:

If the infrastructure does not have a DNS server, access to the Basis Dynamix portal from the cluster.
Basis Dynamix portal from the cluster can be organized by adding the IP address and domains associated with the Basis Dynamix portal to the `cloud-init` template for generating the hosts file.
This template is located in the `/etc/cloud/templates/` folder. The name of the template depends on the OS.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

{% alert level="warning" %}
After adding data to hosts and before creating the template from the virtual machine, you must run the `cloud-init clean` command.
{% endalert %}
