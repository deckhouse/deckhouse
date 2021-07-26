---
title: "Cloud provider — Yandex.Cloud: Preparing environment"
---

You need to create a service account with the editor role with the cloud provider so that Deckhouse can manage cloud resources. The detailed instructions for creating a service account with Yandex.Cloud are available in the provider's [documentation](https://cloud.yandex.com/en/docs/resource-manager/operations/cloud/set-access-bindings). Below, we will provide a brief overview of the necessary actions:

> You may need to increase [quotas](#quotas).

> [Reserve](faq.html#reserving-a-public-ip-address) a public IP address if necessary.

- Create a user named `deckhouse`. The command response will contain its parameters:
  ```yaml
yc iam service-account create --name deckhouse
id: <userId>
folder_id: <folderId>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: candi
```
- Assign the `editor` role to the newly created user:
  ```yaml
  yc resource-manager folder add-access-binding <cloudname> --role editor --subject serviceAccount:<userId>
  ```
- Create a JSON file containing the parameters for user authorization in the cloud. These parameters will be used to log in to the cloud:
  ```yaml
  yc iam key create --service-account-name deckhouse --output candi-sa-key.json
  ```

## Quotas

Note that you need to increase the quotas using the Yandex console when provisioning a new cluster. Recommended parameters:
* The number of virtual processors: 64.
* The total volume of SSD disks: 2000 GB.
* The number of virtual machines: 25.
* The total amount of RAM of virtual machines: 256 GB.

## Permissions

It is recommended to create a service account key using the CLI's `yc` command (instead of Terraform or the web interface) since only this command generates a correctly formatted JSON with a key.

```shell
$ yc iam service-account create --name deckhouse
id: ajee8jv6lj8t7eg381id
folder_id: b1g1oe1s72nr8b95qkgn
created_at: "2020-08-17T08:50:38Z"
name: deckhouse

$ yc resource-manager folder add-access-binding prod --role editor --subject serviceAccount:ajee8jv6lj8t7eg381id

$ yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```
