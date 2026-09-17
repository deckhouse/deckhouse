---
title: Connection and authorization in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/authorization.html
---

{% alert level="info" %}
The Basis Dynamix integration is experimental. Compatibility with future versions isn't guaranteed and the behavior may change.
{% endalert %}

The integration provides the following capabilities:

- Ordering and removing virtual machines via the Basis Dynamix API;
- Using prepared cloud images to deploy nodes;
- Configuring VM parameters (CPU, RAM, disk, network);
- Support for external and internal networks, including CIDR and DNS settings;
- Support for multiple layouts — with an external or a combined network;
- Disk placement via a storage policy.

## Requirements

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

To integrate Deckhouse Platform (DP) with Basis Dynamix, you need the following:

- Basis Dynamix 4.6 or newer (on an older platform `dhctl bootstrap` refuses to deploy a cluster);
- Access to the API controller and to the Basis Dynamix SSO;
- The account name and the application parameters ([`appId`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-provider-appid) and [`appSecret`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-provider-appsecret));
- An OS cloud image uploaded to the cloud;
- An external network ([`externalNetwork`](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass-v1-spec-externalnetwork)) and, if required, the internal network parameters (CIDR, DNS);
- The name of a storage policy available to the account ([`storagePolicy`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-storagepolicy)); the storage endpoint and the pool within the policy are chosen by the platform itself;
- A public SSH key for accessing the cluster nodes.

## Preparing a cloud image

To deploy virtual machines, DP uses OS cloud images prepared by vendors for virtual environments. Such images usually include:

- `cloud-init`;
- Virtual hardware drivers;
- Guest agents.

We recommend using official cloud images distributed in the `.img`, `.qcow2`, and similar formats.

To add an image to Basis Dynamix:

1. Go to "Images" → "Template images".
1. Upload the prepared cloud image.

## Configuring access to the portal

If your infrastructure has no DNS server, add the IP addresses and domains of the Basis Dynamix portal to the `cloud-init` template manually, so that the system works correctly when the nodes start.

1. Edit the `hosts` file template located in the following directory:

   ```console
   /etc/cloud/templates/
   ```

1. Add the IP address and domain name entries specific to your operating system and configuration. This may include entries for the Basis Dynamix portal, such as:

   ```txt
   <IP address> <domain name>
   ```

1. After the changes, clean up the `cloud-init` state:

   ```console
   cloud-init clean
   ```

1. Create a template from the prepared virtual machine. This is required for the `/etc/hosts` file to be generated correctly on the first boot.
