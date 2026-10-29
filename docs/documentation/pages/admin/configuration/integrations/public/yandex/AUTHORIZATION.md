---
title: Connection and authorization in Yandex Cloud
permalink: en/admin/integrations/public/yandex/authorization.html
---

To allow Deckhouse Platform (DP) to manage resources in Yandex Cloud, you need to:

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

{% alert level="warning" %}
Starting with version 1.77, new clusters in Yandex Cloud use the Cilium CNI by default with pod traffic tunneling over VXLAN. In existing clusters, the CNI stays unchanged.

Make sure all nodes run Linux kernel 5.8 or newer and that security groups allow UDP traffic between nodes. For the list of ports, see ["Network interaction of the platform components"](../../../../reference/network_interaction.html); for CNI settings, see ["Internal network configuration"](../../../configuration/network/internal/configuration.html).
{% endalert %}

## Creating a service account

To enable DP to manage Yandex Cloud resources, create a service account and assign it editing permissions.
You can find detailed instructions on service account creation in the [Yandex Cloud documentation](https://yandex.cloud/en/docs/resource-manager/operations/cloud/set-access-bindings).

To create the service account, run the following command:

```shell
yc iam service-account create --name deckhouse
```

The command will return information about the newly created service account:

```console
id: <USER_ID>
folder_id: <FOLDER_ID>
created_at: "YYYY-MM-DDTHH:MM:SSZ"
name: deckhouse
```

{% alert level="warning" %}
Save the `userID` and `folderID` as they will be needed in the following steps.
{% endalert %}

## Assigning IAM roles

To allow DP to work with cloud resources, assign the following roles to the service account:

```shell
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role compute.editor --subject serviceAccount:<USER_ID>
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role vpc.admin --subject serviceAccount:<USER_ID>
yc resource-manager folder add-access-binding --id <FOLDER_ID> --role load-balancer.editor --subject serviceAccount:<USER_ID>
```

## Generating an authorization key

Generate a JSON authorization file to use in your configuration:

```shell
yc iam key create --service-account-name deckhouse --output deckhouse-sa-key.json
```

The contents of the `deckhouse-sa-key.json` file are needed for the credentials Secret.

## Credentials Secret

The service account key is stored in a separate Secret rather than in [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig). The provider reads it when starting the components that access the Yandex Cloud API.

The Secret is created during cluster installation together with the other resources of the initial configuration. If the provider is connected to a running cluster, the Secret is applied after the module creates the namespace. For the order of steps, see ["Hybrid cluster with Yandex Cloud"](../../hybrid/yandex-hybrid.html). The Secret must meet the following requirements:

- The name is `d8-credentials` and the namespace is `d8-cloud-provider-yandex`.
- The type is `cloud-provider.deckhouse.io/credentials`.
- The `authScheme` field is set to `serviceAccount`.
- The `secret` field contains the service account key in JSON.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: d8-credentials
  namespace: d8-cloud-provider-yandex
type: cloud-provider.deckhouse.io/credentials
stringData:
  authScheme: serviceAccount
  secret: |
    <SERVICE_ACCOUNT_JSON>
```

Replace `<SERVICE_ACCOUNT_JSON>` with the contents of the `deckhouse-sa-key.json` file.

To change the credentials, update the `secret` field:

```shell
d8 k -n d8-cloud-provider-yandex edit secret d8-credentials
```

In the ["WithNATInstance layout"](layout.html#withnatinstance), the API key for monitoring the NAT instance is stored in a second Secret, `d8-credentials-exporter`, in the same namespace. Its `authScheme` field is set to `apiToken` and the key is placed in the `secret` field.

Instead of a ready key, the `secret` field can hold the `Auto` value. In that case, while provisioning the infrastructure, DP creates a service account named `<CLUSTER_PREFIX>-monitoring` in the folder, grants it the `monitoring.viewer` role, and generates an API key for it, which the metrics exporter then uses.

In clusters migrated to ModuleConfig, the service account key moves to the `d8-credentials` Secret from the [`provider.serviceAccountJSON`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-provider-serviceaccountjson) parameter of the [YandexClusterConfiguration](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration) resource. The monitoring API key moves to the `d8-credentials-exporter` Secret from the [`withNATInstance.exporterAPIKey`](/modules/cloud-provider-yandex/cluster_configuration.html#yandexclusterconfiguration-withnatinstance-exporterapikey) parameter.

## Checking and increasing quotas

Make sure your cloud account has the required quotas for cluster deployment and scaling:

- vCPUs: 64
- SSD storage: 2000 GB
- Number of VMs: 25
- RAM: 256 GB

Increase quotas via the Yandex Cloud Console if necessary.

## Reserving a public IP

If you are using the WithoutNAT or WithNATInstance deployment [layout](../../../../glossary.html#layout) and need a fixed external IP address
(for example, to use in [`externalIPAddresses`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-externalipaddresses), [`natInstanceExternalAddress`](/modules/cloud-provider-yandex/configuration.html#parameters-nodes-parameters-withnatinstance-natinstanceexternaladdress), or a [bastion host](../../../../glossary.html#bastion-host)), run the following command:

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

After completing these steps, you will have all the information needed to configure the provider. For the order of steps, see ["Configuration"](layout.html#configuration).
