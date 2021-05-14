---
title: "Сloud provider — GCP: configuraton"
---

The module is configured automatically based on the chosen placement strategy (the `GCPClusterConfiguration` custom resource). In most cases, you do not have to configure the module manually.

You can configure the number and parameters of provisioning machines in the cloud via the [`NodeGroup`](../../modules/040-node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` parameter of NodeGroup). In the case of the GCP cloud provider, the instance class is the [`GCPInstanceClass`](cr.html#awsinstanceclass) custom resource that stores specific parameters of the machines.

## Parameters

> **Note** that if the parameters provided below are changed (i.e., the parameters specified in the deckhouse ConfigMap), the **existing Machines are NOT re-deployed** (new machines will be created with the updated parameters). Re-deployment is only performed when `NodeGroup` and `GCPInstanceClass` are changed. You can learn more in the [node-manager module's documentation](../../modules/040-node-manager/faq.html#how-do-i-redeploy-ephemeral-machines-in-the-cloud-with-a-new-configuration).

* `networkName` — the name of the VPC network in GCP where instances will be provisioned;
* `subnetworkName` — the name of the subnet in the `networkName` VPC network networkName where instances will be provisioned;
* `region` — the name of the GCP region where instances will be provisioned;
* `zones` — a list of the `region` zones where instances will be provisioned. It is the default value of the zones field in the [NodeGroup](../../modules/040-node-manager/cr.html#nodegroup) object;
    * Format — an array of strings;
* `extraInstanceTags` — a list of additional GCP tags to assign to the provisioned instances. These tags allow attaching various firewall rules to the instances provisioned by GCP;
    * Format — an array of strings;
    * An optional parameter;
* `sshKey` — a public SSH key;
    * Format — a string similar to the one in `~/.ssh/id_rsa.pub`.
* `serviceAccountJSON` — a key to the Service Account with Project Admin privileges;
    * Format — a JSON string;
    * [How to create it](https://cloud.google.com/iam/docs/creating-managing-service-account-keys#creating_service_account_keys).
* `disableExternalIP` — defines if an external IPv4 address should be assigned to new instances. If this parameter is set to `true`, you need to create a [Cloud NAT](https://cloud.google.com/nat/docs/overview) service in GCP;
    * Format — bool. An optional parameter;
    * Set to true `true` by default.

## Storage

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

* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
  * Format — an array of strings;
  * An optional parameter;
* `default` — the name of StorageClass that will be used in the cluster by default;
  * Format — a string;
  * An optional parameter;
  * If the parameter is omitted, the default StorageClass is either: 
    * an arbitrary StorageClass present in the cluster that has the default annotation;
    * the first StorageClass created by the module (in accordance with the order listed in the table above).

```yaml
cloudProviderGcp: |
  storageClass:
    exclude: 
    - "pd-standard.*"
    - pd-ssd-replicated
    default: pd-ssd-not-replicated
```

