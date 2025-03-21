---
title: "Cloud provider — GCP: configuration"
---

The module is configured automatically based on the chosen placement strategy defined in the [GCPClusterConfiguration](cluster_configuration.html) struct. In most cases, you do not have to configure the module manually.

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

You can configure the number and parameters of provisioning machines in the cloud via the [`NodeGroup`](../../modules/node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` parameter of NodeGroup). In the case of the GCP cloud provider, the instance class is the [`GCPInstanceClass`](cr.html#gcpinstanceclass) custom resource that stores specific parameters of the machines.

The module automatically creates StorageClasses that cover all the available disk types in GCP:

| Type | Replication | StorageClass Name |
|---|---|---|
| standard | none | pd-standard-not-replicated |
| standard | regional | pd-standard-replicated |
| balanced | none | pd-balanced-not-replicated |
| balanced | regional | pd-balanced-replicated |
| ssd | none | pd-ssd-not-replicated |
| ssd | regional | pd-ssd-replicated |

Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

{% include module-settings.liquid %}
