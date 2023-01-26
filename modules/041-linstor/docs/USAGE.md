---
title: "The linstor module: configuration examples"
---

Linstor module supports two methods of configuration:

- Automatic configuration based on LVM tags. Please refer to [Configuration](configuration.html) page.
- Manual configuration using LINSTOR CLI. Please refer to [Advanced Usage](advanced_usage.html) page.

## Additional features for your applications using LINSTOR storage

### Placing the application "closer" to the data (data locality)

In a hyperconverged infrastructure you may want your Pods to run on the same nodes as their data volumes, as it can help get the best performance from the storage.

The linstor module provides a custom kube-scheduler `linstor` for such tasks, that takes into account the placement of data in storage and tries to place Pod first on those nodes where data is available locally.

The linstor scheduler considers the placement of data in storage and tries to place Pods on nodes where data is available locally first.  
Any Pod using linstor volumes will be automatically configured to use the `linstor` scheduler.

### Application reschedule in case of node problem (storage-based fencing)

In case your application does not support high availability and runs in a single instance, you may want to force a migration from a node where problems occurred may arise. For example, if there are network issues, disk subsystem issues, etc.

The linstor module automatically removes the Pods from the node where the problem occurred (network or storage issues, etc.) and adds specfic taint on it that guarantees restarting pods on other healthy nodes in a cluster.
