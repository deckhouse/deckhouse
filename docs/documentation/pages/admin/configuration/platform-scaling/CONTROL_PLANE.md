---
title: "Control plane management"
permalink: en/admin/configuration/platform-scaling/control-plane.html
---

## Overview

Deckhouse Kubernetes Platform (DKP) manages control plane components using the `control-plane-manager` module, which runs on all master nodes (nodes with the label `node-role.kubernetes.io/control-plane: ""`).

The control plane management functionality includes:

- Managing certificates required for the control plane to function, including their renewal and issuance when the configuration changes. Secure configuration is maintained automatically, with the ability to quickly add additional SANs for secure access to the Kubernetes API.

- Component configuration. DKP generates all necessary configurations and manifests (kube-apiserver, etcd, etc.), reducing the risk of human error.

- Upgrade/downgrade of components. DKP supports consistent version upgrades or downgrades of the control plane, helping maintain version uniformity across the cluster.

- Managing the etcd cluster configuration and its members. DKP scales master nodes and performs migrations between single-master and multi-master modes.

- Configuring `kubeconfig`. DKP generates an up-to-date configuration file (with `cluster-admin` privileges), handles automatic renewal and updates, and creates a `symlink` for the `root` user.

> Some parameters affecting Control Plane behavior are taken from the ClusterConfiguration resource.

## Enabling, disabling, and configuring the module

### Enabling / disabling

You can enable or disable the `control-plane-manager` module in the following ways:

1. Create (or modify) the `ModuleConfig/control-plane-manager` resource by setting `spec.enabled` to `true` or `false`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     enabled: true
   ```

1. Using the command:

   ```bash
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller module enable control-plane-manager
   ```

   or to disable:

   ```bash
   kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller module disable control-plane-manager
   ```  
  
1. Via the [Deckhouse web interface](https://deckhouse.io/products/kubernetes-platform/modules/console/stable/):

   - Go to the “Deckhouse → Modules” section;
   - Find the `control-plane-manager` module and click on it;
   - Toggle the “Module enabled” switch.

### Configuration

To configure the module, use the ModuleConfig/control-plane-manager resource and specify the required parameters in `spec.settings`.

Example with the schema version, enabled module, and a few settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  enabled: true
  settings:
    apiserver:
      bindToWildcard: true
      certSANs:
      - bakery.infra
      - devs.infra
      loadBalancer: {}
```

How to verify that `control-plane-manager` is running correctly and is not in a pending state, and how to check active Deckhouse tasks (queues):

1. Make sure the module is enabled:

   ```console
   kubectl get modules control-plane-manager
   ```

1. Check the status of control-plane-manager pods (they run in the `kube-system` namespace and have the label `app=d8-control-plane-manager`):

   ```console
   kubectl -n kube-system get pods -l app=d8-control-plane-manager -o wide
   ```

   Ensure that all pods are in the Running or Completed state.

1. Verify that master nodes are in the Ready state:

   ```console
   kubectl get nodes -l node-role.kubernetes.io/control-plane
   ```

   To view detailed information:

   ```console
   kubectl describe node <имя-узла>
   ```

1. Get the list of queues and active tasks:

   ```console
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- \
    deckhouse-controller queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

   > Before performing heavy operations (e.g., transitioning from single-master to multi-master or upgrading the Kubernetes version), it is recommended to wait until all tasks in the queues are completed.

## Certificate management

In DKP, the `control-plane-manager` module is responsible for issuing and renewing all SSL certificates for control plane components. It manages:

1. Server certificates for kube-apiserver and etcd, stored in the `d8-pki` secret (namespace: `kube-system`):
   - Kubernetes root CA (`ca.crt`, `ca.key`);
   - etcd root CA (`etcd/ca.crt`, `etcd/ca.key`);
   - RSA certificate and key for signing Service Accounts (`sa.pub`, `sa.key`);
   - Root CA for extension API servers (`front-proxy-ca.key`, `front-proxy-ca.crt`).

1. Client certificates required for mutual communication between control plane components (e.g., `apiserver.crt`, `apiserver-etcd-client.crt`, etc.). These files are stored only on the nodes. If any changes occur (e.g., new SANs are added), certificates are automatically reissued, and the kubeconfig is synchronized.

### PKI management

DKP also manages the Public Key Infrastructure (PKI) used for encryption and authentication throughout the Kubernetes cluster:

- PKI for control plane components (kube-apiserver, kube-controller-manager, kube-scheduler, etc.).
- PKI for the etcd cluster (etcd certificates and inter-node communication).

DKP assumes control of this PKI after the initial cluster installation and once its pods are running. As a result, all key issuance, renewal, and rotation operations are performed automatically and centrally, without requiring manual intervention.

### Additional SANs and auto-update

Deckhouse simplifies the addition of new Subject Alternative Names (SANs) for the Kubernetes API endpoint: you only need to specify them in the configuration. After any SAN change, the module automatically regenerates the certificates and updates the kubeconfig.

To add additional SANs (extra DNS names or IP addresses) for the Kubernetes API:

1. Add the new SANs to `spec.settings.apiserver.certSANs` in your `ModuleConfig/control-plane-manager`.
1. DKP will automatically generate new certificates and update all required configuration files (including the kubeconfig).

### Kubelet certificate rotation

In Deckhouse Kubernetes Platform, kubelet does not use the `--tls-cert-file` or `--tls-private-key-file` flags directly. Instead, it relies on dynamic certificates:

- By default, kubelet generates its keys in `/var/lib/kubelet/pki/` and requests renewal from the kube-apiserver when needed;
- Issued certificates are valid for 1 year, but kubelet starts renewing them early (around 5–10% of the remaining validity period);
- If the certificate fails to renew in time, the node is marked as `NotReady` and is eventually replaced.

### Manual renewal of control plane certificates

If the master nodes were offline for an extended period (e.g., the servers were shut down), some control plane certificates may become outdated. In such cases, automatic renewal will not occur upon reboot — manual intervention is required.

To manually renew the control plane certificates, use the `kubeadm` utility on each master node:

1. Locate the `kubeadm` binary on the master node and create a symbolic link:

   ```shell
   ln -s  $(find /var/lib/containerd  -name kubeadm -type f -executable -print) /usr/bin/kubeadm
   ```

1. Execute the following command:

   ```shell
   kubeadm certs renew all
   ```

   This command will regenerate the necessary certificates (for kube-apiserver, kube-controller-manager, kube-scheduler, etcd, and others).

## Scaling and single/multi-master transition

### Control plane operation modes

Deckhouse Kubernetes Platform (DKP) supports two operation modes for the control plane:

1. **Single-master**:
   - `kube-apiserver` uses only the local `etcd` instance.
   - A proxy server runs on the node to handle requests on `localhost`.
   - The `kube-apiserver` listens only on the master node's IP address.

2. **Multi-master**:
   - `kube-apiserver` interacts with all `etcd` instances in the cluster.
   - A proxy is configured on all nodes:
     - If the local `kube-apiserver` is unavailable, requests are redirected to other nodes.
   - This ensures high availability and supports scaling.

### Automatic scaling of master nodes

DKP allows automatic addition and removal of master nodes using the label `node-role.kubernetes.io/control-plane=""`.

Automatic control of master nodes includes:

- **Adding the label** `node-role.kubernetes.io/control-plane=""` to a node:
  - All control plane components are deployed.
  - The node is added to the `etcd` cluster.
  - Certificates and configuration files are regenerated automatically.

- **Removing the label**:
  - Control plane components are removed.
  - The node is properly removed from the `etcd` cluster.
  - Related configuration files are updated.

> **Warning**. Transitioning from 2 to 1 master node requires manual `etcd` adjustment. All other changes in master node count are handled automatically.

### Common scaling scenarios

DKP supports both automatic and manual scaling of master nodes in cloud and bare-metal clusters:

1. **Single-master → Multi-master**:

   - Add one or more master nodes.
   - Apply the label `node-role.kubernetes.io/control-plane=""` to them.
   - DKP will automatically:
     - Deploy all control plane components.
     - Configure the nodes to work with the `etcd` cluster.
     - Synchronize certificates and configuration files.

1. **Multi-master → Single-master**:

   - Remove the labels `node-role.kubernetes.io/control-plane=""` and `node-role.kubernetes.io/master=""` from the extra master nodes.
   - For **bare-metal clusters**:
     - To correctly remove the nodes from `etcd`:
       - Run `kubectl delete node <node-name>`;
       - Power off the corresponding VMs or servers.
       > **Warning**. In cloud clusters, all necessary actions are automatically handled by the `dhctl converge` command.

1. **Changing the number of master nodes in a cloud cluster**:

   - Similar to node addition/removal, typically done using the `dhctl converge` command or cloud tools.
     > **Warning**. An odd number of master nodes is required to maintain `etcd` quorum stability.

### How to remove the master role from a node while retaining the machine

If you need to remove a node from the set of master nodes but keep it in the cluster for other purposes, follow these steps:

1. Remove the labels so the node is no longer treated as a master:

   ```bash
   kubectl label node <node-name> node-role.kubernetes.io/control-plane-
   kubectl label node <node-name> node-role.kubernetes.io/master-
   kubectl label node <node-name> node.deckhouse.io/group-
   ```

1. Delete the static manifests of the control plane components so they no longer start on the node, and remove unnecessary PKI files:

   ```bash
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Check the node's status in the etcd cluster using `etcdctl member list`.

   Example:

   ```bash
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

After completing these steps, the node will no longer be considered a master node, but it will remain part of the cluster and can be used for other tasks.

### Changing the OS image of master nodes in a multi-master cluster

1. Back up `etcd` and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Make sure there are no alerts in the cluster that could interfere with updating master nodes.
1. Ensure the Deckhouse queue is empty.
1. **On your local machine**, run the Deckhouse installer container for the corresponding edition and version (adjust the container registry address if necessary):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command to check the state before starting the operation:

   ```bash
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The output should indicate that Terraform has found no discrepancies and no changes are required.

1. **In the installer container**, run the following command and specify the desired OS image in the `masterNodeGroup.instanceClass` parameter  
(provide all master node addresses using the `--ssh-host` parameter):

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

1. **In the installer container**, run the following command to update the nodes:

   Carefully review the actions that `converge` plans to perform when it prompts for confirmation.

   During execution, nodes will be replaced with new ones, one by one, starting from the highest numbered node (2) down to the lowest (0):

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST> --ssh-host <MASTER-NODE-1-HOST> --ssh-host <MASTER-NODE-2-HOST>
   ```

   The following steps (9–12) should be performed sequentially on each master node, starting with the highest numbered node (with suffix 2) and ending with the lowest (with suffix 0).

1. **On the newly created node**, open the systemd journal for the `bashible.service`.  
   Wait until the setup process is complete — the log should contain the message `nothing to do`:

   ```bash
   journalctl -fu bashible.service
   ```

1. Verify that the etcd node appears in the cluster node list:

   ```bash
   kubectl -n kube-system exec -ti \
   $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o json | jq -r '.items[] | select( .status.conditions[] | select(.type == "ContainersReady" and .status == "True")) | .metadata.name' | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Make sure that `control-plane-manager` is running on the node:

   ```bash
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady \
     -l app=d8-control-plane-manager --field-selector spec.nodeName=<MASTER-NODE-N-NAME>
   ```

1. Proceed to updating the next master node.

### Changing the OS image in a single-master cluster

1. Convert the single-master cluster into a multi-master one according to the [instructions](#how-to-add-master-nodes-in-a-cloud-cluster).
1. Update the master nodes as described in the [instructions](#changing-the-os-image-of-master-nodes-in-a-multi-master-cluster).
1. Convert the multi-master cluster back to a single-master one following the [instructions](#how-to-add-master-nodes-in-a-cloud-cluster).

## How to add master nodes in a cloud cluster

This section describes how to convert a single-master cluster into a multi-master cluster.

> Before adding nodes, make sure the required quotas are available.
>
> It's important to have an odd number of master nodes to maintain etcd quorum.

1. Create a [backup of `etcd`](/admin/backup/backup-and-restore.html) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no active alerts in the cluster that may interfere with adding new master nodes.
1. Make sure the Deckhouse queue is empty:

   ```console
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. On the local machine, run the Deckhouse installer container for the appropriate edition and version (adjust the container registry address if necessary):

   ```console
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. In the installer container, run the following command to verify the state before proceeding:

   ```console
   dhctl terraform check --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   The output should confirm that Terraform found no differences and no changes are needed.

1. In the installer container, run the following command and set the desired number of master nodes in the `masterNodeGroup.replicas` parameter:

   ```console
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> \
     --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For Yandex Cloud, if public IPs are assigned to master nodes, the number of elements in the `masterNodeGroup.instanceClass.externalIPAddresses` array must match the number of master nodes. Even when using the Auto value (automatic public IP assignment), the number of items in the array must still match.
   >
   >For example, with three master nodes (`masterNodeGroup.replicas: 3`) and automatic IP assignment, the `externalIPAddresses` section would look like:
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. In the installer container, run the following command to trigger scaling:

   ```console
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

1. Wait until the required number of master nodes reaches the `Ready` status and all control-plane-manager pods become ready:

   ```console
   kubectl -n kube-system wait pod --timeout=10m --for=condition=ContainersReady -l app=d8-control-plane-manager
   ```

## How to reduce the number of master nodes in a cloud cluster

This section describes the process of converting a multi-master cluster into a single-master cluster.

{% alert level="warning" %}
The following steps must be performed starting from the first master node (master-0) in the cluster. This is because the cluster scales in order — for example, it is not possible to remove master-0 and master-1 while leaving master-2.
{% endalert %}

1. Create a [backup of `etcd`](/admin/backup/backup-and-restore.html) and the `/etc/kubernetes` directory.
1. Copy the resulting archive outside the cluster (e.g., to a local machine).
1. Ensure there are no alerts in the cluster that may interfere with the master node update process.
1. Make sure the Deckhouse queue is empty:

   ```console
   kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
   ```

1. On the **local machine**, run the Deckhouse installer container for the corresponding edition and version (change the container registry address if needed):

   ```bash
   DH_VERSION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/version}') \
   DH_EDITION=$(kubectl -n d8-system get deployment deckhouse -o jsonpath='{.metadata.annotations.core\.deckhouse\.io\/edition}' | tr '[:upper:]' '[:lower:]' ) \
   docker run --pull=always -it -v "$HOME/.ssh/:/tmp/.ssh/" \
     registry.deckhouse.io/deckhouse/${DH_EDITION}/install:${DH_VERSION} bash
   ```

1. **In the installer container**, run the following command and set `masterNodeGroup.replicas` to `1`:

   ```bash
   dhctl config edit provider-cluster-configuration --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> \
     --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

   > For Yandex Cloud, if external IPs are used for master nodes, the number of items in the `masterNodeGroup.instanceClass.externalIPAddresses` array must match the number of master nodes. Even when using `Auto` (automatic public IP allocation), the number of entries must still match.
   >
   > For example, for a single master node (`masterNodeGroup.replicas: 1`) with automatic public IPs::
   >
   > ```yaml
   > externalIPAddresses:
   > - "Auto"
   > ```

1. Remove the following labels from the master nodes you plan to delete:
   * `node-role.kubernetes.io/control-plane`
   * `node-role.kubernetes.io/master`
   * `node.deckhouse.io/group`

   Command to remove the labels:

   ```bash
   kubectl label node <MASTER-NODE-N-NAME> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Make sure the nodes to be removed are no longer part of the etcd cluster:

   ```bash
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Drain the nodes to be removed:

   ```bash
   kubectl drain <MASTER-NODE-N-NAME> --ignore-daemonsets --delete-emptydir-data
   ```

1. Power off the corresponding VMs, delete their instances from the cloud, and detach any associated disks (e.g., `kubernetes-data-master-<N>`).

1. Delete any remaining pods on the removed nodes:

   ```bash
   kubectl delete pods --all-namespaces --field-selector spec.nodeName=<MASTER-NODE-N-NAME> --force
   ```

1. Delete the `Node` objects for the removed nodes:

   ```bash
   kubectl delete node <MASTER-NODE-N-NAME>
   ```

1. **In the installer container**, run the following command to trigger the scaling operation:

   ```bash
   dhctl converge --ssh-agent-private-keys=/tmp/.ssh/<SSH_KEY_FILENAME> --ssh-user=<USERNAME> --ssh-host <MASTER-NODE-0-HOST>
   ```

## Recovery from Failures

During its operation, the `control-plane-manager` automatically creates backups of configuration and data that may be useful in case of problems. These backups are saved in the `/etc/kubernetes/deckhouse/backup` directory. If any issues or unexpected situations occur during operation, you can use these backups to restore the system to a previously healthy state.

## What to do if the etcd cluster is not functioning

If the etcd cluster is not functioning and cannot be restored from a backup, you can attempt to recover it from scratch by following the steps below.

1. On all nodes that are part of your etcd cluster, **except one**, delete the `etcd.yaml` manifest located in `/etc/kubernetes/manifests/`. This will leave only one active node, from which the multi-master cluster state will be restored.
1. On the remaining node, open the `etcd.yaml` manifest and add the `--force-new-cluster` flag under `spec.containers.command`.
1. After the cluster is successfully restored, remove the `--force-new-cluster` flag.

{% alert level="warning" %}
This operation is destructive: it completely wipes the existing data and initializes a new cluster based on the state preserved on the remaining node. All pending records will be lost.
{% endalert %}


## High Availability

If any component of the control plane becomes unavailable, the cluster temporarily maintains its current state but cannot process new events. For example:

- If `kube-controller-manager` fails, Deployment scaling will stop working.
- If `kube-apiserver` is unavailable, no requests can be made to the Kubernetes API, although existing applications will continue to function.

However, prolonged unavailability of components disrupts the processing of new objects, response to node failures, and other operations. Eventually, this may impact end users.

To mitigate these risks, the control plane should be scaled to a high-availability configuration — a minimum of three nodes. This is especially critical for `etcd`, which requires a quorum to elect a leader. The quorum works on a majority basis (N/2 + 1) of the total number of nodes.

Example:

| Cluster Size | Quorum (Majority) | Max Fault Tolerance |
|--------------|-------------------|----------------------|
| 1            | 1                 | 0                    |
| 3            | 2                 | 1                    |
| 5            | 3                 | 2                    |
| 7            | 4                 | 3                    |
| 9            | 5                 | 4                    |

> **Note:** An even number of nodes does not improve fault tolerance but increases replication overhead.

In most cases, three etcd nodes are sufficient. Use five if high availability is critical. More than seven is rarely necessary and not recommended due to high resource consumption.

After new control plane nodes are added:

- The label `node-role.kubernetes.io/control-plane=""` is applied.
- A DaemonSet launches control plane pods on the new nodes.
- The Control Plane Manager (CPM) creates or updates files in `/etc/kubernetes`: manifests, configuration files, certificates, etc.
- All DKP modules that support high availability will enable it automatically, unless the global setting `highAvailability` is manually overridden.

Control plane node removal happens in reverse:

- Labels `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, and `node.deckhouse.io/group` are removed.
- CPM removes its pods from these nodes.
- etcd members on the nodes are automatically deleted.
- If the number of nodes drops from two to one, etcd may enter `readonly` mode. In this case, you must start etcd with the `--force-new-cluster` flag, which should be removed after a successful startup.

## Updating and version management

The control plane update process in DKP is fully automated.

- DKP supports the latest five Kubernetes versions.
- You can roll back the control plane one minor version and upgrade forward several minor versions — one at a time.
- Patch versions (e.g., 1.27.3 → 1.27.5) are updated automatically with Deckhouse and cannot be managed manually.
- Minor versions are set manually using the `kubernetesVersion` parameter in the ClusterConfiguration resource.

### How to change the Kubernetes version

1. Open the ClusterConfiguration editor:

   ```console
   kubectl -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller edit cluster-configuration
   ```

1. Set the desired Kubernetes version using the `kubernetesVersion` field:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   cloud:
     prefix: demo-stand
     provider: Yandex
   clusterDomain: cloud.education
   clusterType: Cloud
   defaultCRI: Containerd
   kubernetesVersion: "1.30"
   podSubnetCIDR: 10.111.0.0/16
   podSubnetNodeCIDRPrefix: "24"
   serviceSubnetCIDR: 10.222.0.0/16
   ```

1. Save the changes.

## etcd restore

### Viewing etcd cluster members

Below are the steps to view the list of nodes that are part of the etcd cluster:

1. Find the etcd pod:

   ```console
   kubectl -n kube-system get pods -l component=etcd,tier=control-plane
   ```

   Typically, pod name has the `etcd-` prefix.

1. Run the following command on any available etcd Pod (assuming it is running in the `kube-system` namespace):

   ```console
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
     etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ member list -w table
   ```

   This command uses substitution: `$(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1)`.
   It automatically inserts the name of the first Pod matching the specified labels.  

### If etcd is not functioning

1. Stop all etcd nodes except one by deleting the `etcd.yaml` manifest on the others.
1. On the remaining node, add the `--force-new-cluster` option to the etcd startup command.
1. After the cluster is restored, remove this option.
   > **Caution**: this action completely wipes previous data and creates a new etcd cluster.

### If etcd keeps restarting with the error panic: unexpected removal of unknown remote peer

In some cases, manual restoration via `etcdutl snapshot restore` can help:

1. Save a local snapshot from `/var/lib/etcd/member/snap/db`.
1. Use `etcdutl` with the `--force-new-cluster` option to restore.
1. Completely wipe the `/var/lib/etcd` directory and place the restored snapshot there.
1. Remove any "stuck" etcd/kube-apiserver containers and restart the node.

### What to do if the database volume of etcd reaches the limit set in quota-backend-bytes

When the database volume of etcd reaches the limit set by the `quota-backend-bytes` parameter, it switches to "read-only" mode. This means that the etcd database stops accepting new entries but remains available for reading data. You can tell that you are facing a similar situation by executing the command:

```shell
kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ endpoint status -w table --cluster
```

If you see a message like `alarm:NOSPACE` in the `ERRORS` field, you need to take the following steps:

1. Make change to `/etc/kubernetes/manifests/etcd.yaml` — find the line with `--quota-backend-bytes` and edit it. If there is no such line — add, for example: `- --quota-backend-bytes=8589934592` - this sets the limit to 8 GB.
1. Disarm the active alarm that occurred due to reaching the limit. To do this, execute the command:

   ```shell
   kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
   etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
   --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Change the [maxDbSize](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/configuration.html#parameters-etcd-maxdbsize) parameter in the `control-plane-manager` settings  to match the value specified in the manifest.

### How to speed up pod rescheduling when a node becomes unreachable

By default, if a node does not report its status within 40 seconds, it is marked as `Unreachable`. Then, after another 5 minutes, the pods on that node begin to be restarted on other nodes. As a result, the total application downtime can reach approximately 6 minutes.

In specific cases where an application cannot run in multiple instances, there is a way to reduce the downtime period:

1. Reduce the time before a node is marked as `Unreachable` by configuring the `nodeMonitorGracePeriodSeconds` parameter.
1. Set a shorter timeout for evicting pods from the unreachable node using the `failedNodePodEvictionTimeoutSeconds` parameter.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    nodeMonitorGracePeriodSeconds: 10
    failedNodePodEvictionTimeoutSeconds: 50
```

## Configuring custom audit policies

1. Enable the `auditPolicyEnabled` parameter:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
     settings:
       apiserver:
         auditPolicyEnabled: true
   ```

1. Create a secret `kube-system/audit-policy` containing the policy YAML file encoded in Base64:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: audit-policy
     namespace: kube-system
   data:
     audit-policy.yaml: <base64>
   ```

   A minimal working example of `audit-policy.yaml`:

   ```yaml
   apiVersion: audit.k8s.io/v1
   kind: Policy
   rules:
   - level: Metadata
     omitStages:
     - RequestReceived
   ```

   For more details on configuring the content of `audit-policy.yaml`, see:
   * [Official Kubernetes documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy);
   * [GCE helper script source code](https://github.com/kubernetes/kubernetes/blob/0ef45b4fcf7697ea94b96d1a2fe1d9bffb692f3a/cluster/gce/gci/configure-helper.sh#L722-L862).

### Disabling built-in audit policies

Set the `apiserver.basicAuditPolicyEnabled` parameter to `false`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      basicAuditPolicyEnabled: false
```

### Output audit log to stdout instead of files

Set the `apiserver.auditLog.output` parameter to `Stdout`.

Example:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  version: 1
  settings:
    apiserver:
      auditPolicyEnabled: true
      auditLog:
        output: Stdout
```

### Working with the audit log

It is assumed that a log scraper is installed on master nodes (`log-shipper`, `promtail`, or `filebeat`) to monitor the audit log file:

```bash
/var/log/kube-audit/audit.log
```

The log rotation parameters for this file are predefined and cannot be changed:

* Maximum disk space: `1000 МБ`.
* Maximum retention period: `7 дней`.

Depending on the policy settings and the number of requests to the `apiserver`, logs may accumulate rapidly. In such cases, the actual retention period may be less than 30 minutes.

{% alert level="warning" %}
This feature does not guarantee safety. If the secret contains unsupported options or typos, the `apiserver` may fail to start.
{% endalert %}

If problems arise with launching the `apiserver`, you need to manually remove the `--audit-log-*` parameters from `/etc/kubernetes/manifests/kube-apiserver.yaml` and restart the apiserver:

```bash
docker stop $(docker ps | grep kube-apiserver- | awk '{print $1}')
# Or, depending on the CRI.
crictl stop $(crictl pods --name=kube-apiserver -q)
```

After restarting, you will have time to fix or delete the secret:

```bash
kubectl -n kube-system delete secret audit-policy
```
