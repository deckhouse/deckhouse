---
title: How to migrate a cloud provider to ModuleConfig-based configuration?
subsystems:
  - cluster_infrastructure
lang: en
---

If a cloud cluster was installed using the `<PROVIDER>ClusterConfiguration` configuration (for example, DVPClusterConfiguration, AWSClusterConfiguration, and so on), this configuration must be migrated to the new ModuleConfig-based model.

Deckhouse Platform is transitioning from a single `<PROVIDER>ClusterConfiguration` resource to a model where the cloud provider configuration is split across four separate resources:

1. [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) for the `cloud-provider-<PROVIDER>` module — provider and layout settings;
1. A Secret with credentials of the `cloud-provider.deckhouse.io/credentials` type — access to the cloud API;
1. Provider InstanceClass (for example, [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass)) — virtual machine parameters;
1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — node group configuration.

{% alert level="warning" %}
Migration is mandatory. This is not an optional step: support for `<PROVIDER>ClusterConfiguration` will be removed. Until the migration is completed, DP upgrades may be blocked.
{% endalert %}

The migration is safe. It implies the applying of automatically prepared DP resources **does not cause nodes to be recreated**. However, explicit administrator action is required, as the resources have to be reviewed and applied manually.

## How to migrate

1. Wait for the migration alert. Each provider has its own, for example, [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending) for DVP and [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) for Yandex Cloud. The alert means the module has detected the legacy configuration and prepared manifests for the transition.

1. Review the prepared resources in the `d8-migration-resources` Secret in the `d8-cloud-provider-<PROVIDER>` namespace:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

   Example for built-in virtualization:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

1. Verify that the manifests match the expected cluster configuration (ModuleConfig, credentials Secret, InstanceClass, and NodeGroup).

1. Apply the resources:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

   Example for built-in virtualization:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

1. Wait until the alert disappears. The module clears it automatically once it detects that all required resources are in place.

## How to delete the legacy secret

After the migration of a cluster deployed in built-in virtualization, Yandex Cloud, or zVirt, the provider settings are defined in ModuleConfig, and you can manually delete the legacy `d8-provider-cluster-configuration` Secret in the `kube-system` namespace.

To delete the Secret, follow these steps:

1. Make sure the migration is complete and its alert no longer fires. For built-in virtualization, it is the [`D8CloudProviderDVPMigrationPending`](reference/alerts.html#cloud-provider-dvp-d8cloudproviderdvpmigrationpending) alert, for Yandex Cloud, the [`D8CloudProviderYandexCloudMigrationPending`](reference/alerts.html#cloud-provider-yandex-d8cloudprovideryandexcloudmigrationpending) alert, and for zVirt, the [`D8CloudProviderZvirtMigrationPending`](reference/alerts.html#cloud-provider-zvirt-d8cloudproviderzvirtmigrationpending) alert. If you delete the Secret before the migration is complete, the cloud provider module stops working correctly.

1. Check the number of nodes in node groups of the `CloudPermanent` type:

   ```shell
   d8 k get nodegroups \
     -o custom-columns='NAME:.metadata.name,TYPE:.spec.nodeType,MIN_PER_ZONE:.spec.cloudInstances.minPerZone'
   ```

   After the Secret is deleted, `dhctl converge` takes the number of nodes in these groups from the [`spec.cloudInstances.minPerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-minperzone) parameter. If the parameter is not set for a node group of the `CloudPermanent` type, `dhctl converge` fails.

1. Make sure the cloud infrastructure data, such as network IDs and zones, is stored in a separate Secret. The command must print a non-empty value:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-candi-cloud-provider-discovery-data \
     -o jsonpath='{.data.cloud-provider-discovery-data\.json}'
   ```

   Replace `<PROVIDER>` with `dvp` for built-in virtualization, `yandex` for Yandex Cloud, or `zvirt` for zVirt.

   If the command prints nothing or the Secret is not found, the data is stored only in the legacy Secret. If you delete it without a copy of the data, the `terraform-manager` module is disabled. The `cloud-provider-yandex` module also loses the IDs of the network, subnets, and route table that the platform created in Yandex Cloud. Copy the data to a separate Secret and delete the temporary file:

   ```shell
   d8 k -n kube-system get secret d8-provider-cluster-configuration \
     -o jsonpath='{.data.cloud-provider-discovery-data\.json}' \
     | base64 -d > /tmp/discovery-data.json

   d8 k -n d8-cloud-provider-<PROVIDER> create secret generic d8-candi-cloud-provider-discovery-data \
     --from-file=cloud-provider-discovery-data.json=/tmp/discovery-data.json \
     --dry-run=client -o yaml \
     | d8 k apply -f -

   rm /tmp/discovery-data.json
   ```

   Then repeat the check and make sure the command prints a non-empty value.

1. Back up the Secret. The Secret contains cloud credentials, so store the file securely:

   ```shell
   d8 k -n kube-system get secret d8-provider-cluster-configuration \
     -o yaml > d8-provider-cluster-configuration-backup.yaml
   ```

1. Allow deletion of the Secret. The Secret is protected by a webhook that rejects deletion without the `deckhouse.io/allow-delete` annotation:

   ```shell
   d8 k -n kube-system annotate secret d8-provider-cluster-configuration \
     deckhouse.io/allow-delete=true
   ```

1. Delete the Secret:

   ```shell
   d8 k -n kube-system delete secret d8-provider-cluster-configuration
   ```
