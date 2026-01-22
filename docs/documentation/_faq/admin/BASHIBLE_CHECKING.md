---
title: What to do if you encounter problems updating Kubernetes components on cluster nodes, synchronizing nodes, or applying NodeGroup Configuration?
subsystems:
  - сluster_infrastructure
lang: en  
---

If Kubernetes components are not updated on the cluster node, the [NodeGroup](/modules/node-manager/cr.html#nodegroup) configuration is not applied, and not all [NodeGroup](/modules/node-manager/cr.html#nodegroup) nodes are synchronized (have the `UPTODATE` status), perform the following steps:

1. Check the bashible logs on the node where the problems are occurring. The bashible mechanism is used to keep cluster nodes up to date. It is started by the `bashible.timer` timer at regular intervals as a service on the cluster nodes. This involves restarting, synchronizing scripts, and executing them (if necessary).

   To check bashible logs, use the command:

   ```shell
   journalctl -u bashible
   ```

   If the response contains the message `Configuration is in sync, nothing to do`, the node is synchronized and there are no problems. The absence of this message or the presence of errors indicates a problem.

1. Check the synchronization status of cluster nodes using the command:

   ```shell
   d8 k get ng
   ```

   The number of nodes in the `UPTODATE` state must match the total number of nodes in each group.

   Example output:

   ```console
   NAME       TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   frontend   Static   1       1       1                                                               118d   True
   master     Static   3       3       3                                                               118d   True
   system     Static   2       2       2                                                               118d   True
   worker     Static   2       2       2                                                               118d   True
   ```
