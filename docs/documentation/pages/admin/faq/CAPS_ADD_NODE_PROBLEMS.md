---
title: What to do if problems adding a node to the cluster via Cluster API Provider Static are present?
permalink: en/faq-common/caps-adding-node-problems.html
---

If, when adding a node to the cluster via Cluster API Provider Static (CAPS), it remains in `Pending` or `Bootstrapping` status, perform the following steps:

1. Verify that the access keys specified in the [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) resource are correct. Ensure that the username and SSH key specified in [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) are correct.

1. On the node where the problem occurred, check that the public key corresponding to the private key from [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) is present in `authorized_keys`. Example command for checking:

   ```shell
   cat ~/.ssh/authorized_keys
   ```

1. Check the number of nodes specified in [NodeGroup](/modules/node-manager/cr.html#nodegroup), which should include the node being added. Make sure that the maximum number of nodes is not exceeded.

1. Check the status of the `bashible.service` on the node that caused the problem:

   ```shell
   systemctl status bashible.service
   ```

   It must have the status `active (running)`. If the service has the status `inactive` or `failed`, the service has not started. This indicates a problem with the configuration process.

1. If the steps above did not resolve the issue, remove the problematic node and its [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource from the cluster so that the system will attempt to recreate them. To do this:

   - Get a list of nodes and locate the problematic one:

     ```shell
     d8 k get nodes
     ```

   - Find the corresponding [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource:

     ```shell
     kubectl get staticinstances -n <namespace-name>
     ```

   - Remove the problematic node:

     ```shell
     kubectl delete node <node-name>
     ```

   - Remove the corresponding [StaticInstance](/modules/node-manager/cr.html#staticinstance) resource:

     ```shell
     kubectl delete staticinstances -n <namespace-name> <static-instance-name>
     ```
