---
title: "Cloud provider — AWS: configuration"
---

The module is configured automatically based on the chosen placement strategy (the [AWSClusterConfiguration](cluster_configuration.html) custom resource). In most cases, you do not have to configure the module manually.

You can configure the number and parameters of provisioning machines in the cloud via the [`NodeGroup`](../node-manager/cr.html#nodegroup) custom resource of the node-manager module. Also, in this custom resource, you can specify the instance class's name for the above group of nodes (the `cloudInstances.ClassReference` NodeGroup parameter). In the case of the AWS cloud provider, the instance class is the [`AWSInstanceClass`](cr.html#awsinstanceclass) custom resource that stores specific parameters of the machines.

The module automatically creates StorageClasses that are available in AWS: `gp3`, `gp2`, `sc1`, and `st1`. It lets you configure disks with the required IOPS. Also, it can filter out the unnecessary StorageClasses (you can do this via the `exclude` parameter).

{% include module-alerts.liquid %}

{% include module-conversion.liquid %}

{% include module-settings.liquid %}
