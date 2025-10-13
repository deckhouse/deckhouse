---
title: Connection and authorization
permalink: en/admin/integrations/public/yandex/authorization.html
---

To allow Deckhouse Kubernetes Platform (DKP) to manage resources in Yandex Cloud, you need to:

- Create a service account.
- Assign the required IAM roles to it.
- Generate an authorization key.
- Optionally reserve a public IP address.
- Ensure sufficient resource quotas in the cloud.

The steps below guide you through setting up the connection.

## Preparing the environment

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

The `cloud-init` package must be installed on virtual machines (VM).
After a VM is started, the following services associated with this package must be running:

- `cloud-config.service`
- `cloud-final.service`
- `cloud-init.service`

To verify that the services are running, use the following commands:

```shell
systemctl status cloud-config.service
systemctl status cloud-final.service
systemctl status cloud-init.service
```

## Creating a service account

To enable DKP to manage Yandex Cloud resources, create a service account and assign it editing permissions.
You can find detailed instructions on service account creation in the [Yandex Cloud documentation](https://yandex.cloud/en/docs/resource-manager/operations/cloud/set-access-bindings).

To create the service account, run the following command:

```shell
yc iam service-account create --name deckhouse
```

The command will return information about the newly created service account:

```console
id: <userID>
folder_id: <folderID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

{% alert level="warning" %}
Save the `userID` and `folderID` as they will be needed in the following steps.
{% endalert %}

## Assigning IAM roles

To allow DKP to work with cloud resources, assign the following roles to the service account:

```shell
yc resource-manager folder add-access-binding --id <folderID> --role compute.editor --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role vpc.admin --subject serviceAccount:<userID>
yc resource-manager folder add-access-binding --id <folderID> --role load-balancer.editor --subject serviceAccount:<userID>
```

## Generating an authorization key

Generate a JSON authorization file to use in your configuration:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```

Use the contents of the `deckhouse-sa-key.json` file in the `provider.serviceAccountJSON` field
when defining the cluster configuration.

## Checking and increasing quotas

Make sure your cloud account has the required quotas for cluster deployment and scaling:

- vCPUs: 64
- SSD storage: 2000 GB
- Number of VMs: 25
- RAM: 256 GB

Increase quotas via the Yandex Cloud Console if necessary.

## Reserving a public IP

If you are using the WithoutNAT or WithNATInstance deployment layout and need a fixed external IP address
(for example, to use in [`externalIPAddresses`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-nodegroups-instanceclass-externalipaddresses), [`natInstanceExternalAddress`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-natinstanceexternaladdress), or a bastion host), run the following command:

```shell
yc vpc address create --external-ipv4 zone=ru-central1-a
```

Example output:

```console
id: e9b4cfmmnc1mhgij75n7
folder_id: b1gog0h9k05lhqe5d88l
created_at: "2020-09-01T09:29:33Z"
external_ipv4_address:
  address: 178.154.226.159
  zone_id: ru-central1-a
  requirements: {}
reserved: true
```

After completing these steps, you will have all the necessary information to create a YandexClusterConfiguration resource
that describes your cluster in Yandex Cloud.
