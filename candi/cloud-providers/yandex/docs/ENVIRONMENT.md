---
title: "Cloud provider — Yandex Cloud: Preparing environment"
description: "Configuring Yandex Cloud for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

You need to create a service account with the editor role with the cloud provider so that Deckhouse can manage cloud resources. The detailed instructions for creating a service account with Yandex Cloud are available in the provider's [documentation](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Below, we will provide a brief overview of the necessary actions:

1. Create a user named `deckhouse`. The command response will contain its parameters:

   ```yaml
   yc iam service-account create --name deckhouse
   id: <userID>
   folder_id: <folderID>
   created_at: "YYYY-MM-DDTHH:MM:SSZ"
   name: deckhouse
   ```

1. Assign the required roles to the newly created user for your cloud:

   ```yaml
   yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
   yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
   ```

1. Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:

   ```yaml
   yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
   ```

> You may need to increase [quotas](#quotas).
>
> [Reserve](faq.html#how-to-reserve-a-public-ip-address) a public IP address if necessary.

## Quotas

> Note that you need to increase the quotas using the Yandex console when provisioning a new cluster.

Recommended quotas for a new cluster:

* The number of virtual processors: 64.
* The total volume of SSD disks: 2000 GB.
* The number of virtual machines: 25.
* The total amount of RAM of virtual machines: 256 GB.
