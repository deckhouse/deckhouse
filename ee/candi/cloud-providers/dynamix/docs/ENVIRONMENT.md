---
title: "Cloud provider — Dynamix: Preparing environment"
description: "Configuring Dynamix for Deckhouse cloud provider operation."
---

<!-- AUTHOR! Don't forget to update getting started if necessary -->

### Prepare an operating system image

Operating system vendors typically provide special cloud builds of their operating systems for use in virtualization environments. These builds typically contain virtual hardware drivers, cloud-init, virtualization guest agents, and are distributed as IMG or QCOW2 disk images. We recommend that you use these cloud images as the OS on the nodes in your clusters.

The cloud image of the operating system must be placed in the "Images" → "Template Images" section of the Dynamix portal. Follow these steps to upload the OS image to the storage:

If the infrastructure does not have a DNS server, access to the Dynamix portal from the cluster.
Dynamix portal from the cluster can be organized by adding the IP address and domains associated with the Dynamix portal to the `cloud-init` template for generating the hosts file.
This template is located in the `/etc/cloud/templates/` folder. The name of the template depends on the OS.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

{% alert level="warning" %}
After adding data to hosts and before creating the template from the virtual machine, you must run the `cloud-init clean` command.
{% endalert %}
