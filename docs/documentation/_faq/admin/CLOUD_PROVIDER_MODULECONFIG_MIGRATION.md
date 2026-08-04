---
title: How to migrate a cloud provider to ModuleConfig-based configuration?
subsystems:
  - cluster_infrastructure
lang: en
---

If a cloud cluster was installed using the `<PROVIDER>ClusterConfiguration` configuration (for example, DVPClusterConfiguration, AWSClusterConfiguration, and so on), this configuration must be migrated to the new ModuleConfig-based model.

Deckhouse Kubernetes Platform is transitioning from a single `<PROVIDER>ClusterConfiguration` resource to a model where the cloud provider configuration is split across four separate resources:

1. [ModuleConfig](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#moduleconfig) for the `cloud-provider-<PROVIDER>` module — provider and layout settings;
1. A Secret with credentials of the `cloud-provider.deckhouse.io/credentials` type — access to the cloud API;
1. Provider InstanceClass (for example, [DVPInstanceClass](/modules/cloud-provider-dvp/cr.html#dvpinstanceclass)) — virtual machine parameters;
1. [NodeGroup](/modules/node-manager/cr.html#nodegroup) — node group configuration.

{% alert level="warning" %}
Migration is mandatory. This is not an optional step: support for `<PROVIDER>ClusterConfiguration` will be removed. Until the migration is completed, DKP upgrades may be blocked.
{% endalert %}

The migration is safe. It implies the applying of automatically prepared DKP resources **does not cause nodes to be recreated**. However, explicit administrator action is required — the resources must be reviewed and applied manually.

## How to migrate

1. Wait for the migration alert (for DVP — `D8CloudProviderDVPMigrationPending`). It means the module has detected the legacy configuration and prepared manifests for the transition.

1. Review the prepared resources in the `d8-migration-resources` Secret in the `d8-cloud-provider-<PROVIDER>` namespace:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

   Example for DVP:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d
   ```

1. Verify that the manifests match the expected cluster configuration (ModuleConfig, credentials Secret, InstanceClass, and NodeGroup).

1. Apply the resources:

   ```shell
   d8 k -n d8-cloud-provider-<PROVIDER> get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

   Example for DVP:

   ```shell
   d8 k -n d8-cloud-provider-dvp get secret d8-migration-resources -o jsonpath='{.data.resources\.yaml}' | base64 -d | d8 k apply -f -
   ```

1. Wait until the alert disappears. The module clears it automatically once it detects that all required resources are in place.
