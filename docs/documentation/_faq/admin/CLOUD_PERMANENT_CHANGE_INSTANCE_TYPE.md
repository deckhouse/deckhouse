---
title: How to change the instance type for nodes with the CloudPermanent type?
subsystems:
  - сluster_infrastructure
lang: en
---

To change the instance type for nodes with the CloudPermanent type, follow these steps:

1. Make a [backup of etcd](/products/kubernetes-platform/documentation/latest/admin/configuration/backup/backup-and-restore.html#etcd-backup-and-restore) and the `/etc/kubernetes` directory.
1. Transfer the archive to a server outside the cluster (e.g., on a local machine).
1. Ensure there are no [alerts](/modules/prometheus/faq.html#how-to-get-information-about-alerts-in-a-cluster) in the cluster that can prevent the update of the master nodes.
1. Make sure that Deckhouse queue is empty. To view the status of all Deckhouse job queues, run the following command:

   ```shell
   d8 s queue list
   ```

   Example output (queues are empty):

   ```console
   Summary:
   - 'main' queue: empty.
   - 88 other queues (0 active, 88 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Run the appropriate edition and version of the Deckhouse installer container **on the local machine** (change the container registry address if necessary):

   ```bash
   DH_VERSION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') 
   DH_EDITION=$(d8 k -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) 
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.ru/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before working:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The command output should indicate that Terraform found no inconsistencies and no changes are required.

1. **In the installer container**, run the command to edit the cluster configuration (specify the addresses of all master nodes in the `--ssh-host` parameter):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. Edit the `instanceClass` parameter of the desired node group by changing the instance type and save the changes. Example settings for the `masterNodeGroup` of the Yandex Cloud provider:

   ```yaml
   masterNodeGroup:
    replicas: 3  # required number of master nodes
    instanceClass:
      cores: 4      # change the number of CPUs
      memory: 8192  # change the memory size (in MB)
      # other instance parameters...
      externalIPAddresses:
      - "Auto"      # for each master node
      - "Auto"
      - "Auto"
   ```

1. **In the installer container**, run the following command to perform nodes upgrade:

   You should read carefully what converge is going to do when it asks for approval.

   When the command is executed, the nodes will be replaced by new nodes with confirmation on each node. The replacement will be performed one by one in reverse order (2,1,0).

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   Repeat the steps below (Sec. 9-12) for each master node one by one, starting with the node with the highest number (suffix 2) and ending with the node with the lowest number (suffix 0).

1. **On the newly created node**, check the systemd-unit log for the `bashible.service`. Wait until the node configuration is complete (you will see a message `nothing to do` in the log):

   ```bash
   journalctl -fu bashible.service
   ```

1. Make sure the node is listed as an etcd cluster member:

   ```bash
   for pod in $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name); do
     d8 k -n kube-system exec "$pod" -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
     if [ $? -eq 0 ]; then
       break
     fi
   done
   ```

1. Make sure `control-plane-manager` is running on the node:

   ```bash
   d8 k -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Proceed to update the next node (repeat the steps above).
