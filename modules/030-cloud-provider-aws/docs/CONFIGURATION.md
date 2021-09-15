---
title: "Cloud provider — AWS: configuration"
---

## Parameters

The module is configured automatically based on the chosen placement strategy (the `AWSClusterConfiguration` custom resource). In most cases, you do not have to configure the module manually.

You can configure the number and parameters of provisioning machines in the cloud via the [`NodeGroup`]({{"/modules/040-node-manager/cr.html#nodegroup" | true_relative_url }} ) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the AWS cloud provider, the instance class is the [`AWSInstanceClass`](cr.html#awsinstanceclass) custom resource that stores specific parameters of the machines.

## Storage

The module automatically creates StorageClasses that are available in AWS: `gp3`, `gp2`, `sc1`, and `st1`. It lets you configure disks with the required IOPS. Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

* `provision` — defines additional StorageClasses or override the parameters of created by default;
  * Format — an array of objects;
    * `name` — the name of the class to create;
    * `type` — the volume type, `gp3`, `gp2`, `sc1`, `st1` or `io1`, `io2`;
    * `iopsPerGB` — the number of I/O operations per second per GB (this parameter is `3` for `gp2` volumes) for `io1` and `io2` volume types;
      * **Caution!** If the iopsPerGB value multiplied by the target volume's size is less than 100 or more than 64000, the creation of such a volume will fail;
      * You can find a detailed description of the volume types and their IOPS in the [official documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html);
    * `iops` — I/O operations per second for `gp3` volume type;
      * Format — number;
      * an optional parameter;
      * Possible values from 3000 to 16000;
      * If empty, it will set to 3000 as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html);
    * `throughput` — throughput in MiB/s for `gp3` volume type;
      * Format — number;
      * an optional parameter;
      * Possible values from 125 to 1000;
      * If empty, it will set to 125 as documented [here](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ebs-volume-types.html);
    > ℹ️ Changing `iops` and `throughput` parameters in configuration doesn't apply to existing volumes. You can modify them manually through AWS Console.
  * an optional parameter;
* `exclude` — a list of StorageClass names (or regex expressions for names) to exclude from the creation in the cluster;
  * Format — an array of strings;
  * an optional parameter;
* `default` — the name of StorageClass that will be used by default in the cluster;
  * Format — a string.
  * an optional parameter;
  * If the parameter is omitted, the default StorageClass is either: 
    * an arbitrary StorageClass present in the cluster that has the default annotation;
    * the first (in lexicographic order) StorageClass of those created by the module.

```yaml
cloudProviderAws: |
  storageClass:
    provision:
    - iopsPerGB: 5
      name: iops-foo
      type: io1
    exclude: 
    - sc.*
    - st1
    default: gp2
```
