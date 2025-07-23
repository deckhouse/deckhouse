---
title: Integration with zVirt services
permalink: en/admin/integrations/virtualization/zvirt/zvirt-services.html
---

{% alert level="info" %}
Integration with zVirt is in experimental status.
Interfaces and functionality may change in the future.
{% endalert %}

Deckhouse Kubernetes Platform supports integration with zVirt infrastructure,
enabling the provisioning, management, and removal of virtual machines using definitions in the ZvirtInstanceClass resource.

## Key features

- Provisioning of virtual machines in zVirt during NodeGroup creation or scaling.
- Use of preconfigured virtual machine templates.
- Automatic configuration of network interfaces and cluster connection.
- Support for both dynamic and static IP address allocation.
- Disk placement in a specified storage domain.
- Support for cloud images with `cloud-init` for proper node initialization.
