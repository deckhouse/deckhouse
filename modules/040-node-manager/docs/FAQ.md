---
title: "Managing nodes: FAQ"
search: add a node to the cluster, set up a GPU-enabled node, ephemeral nodes
description: Managing nodes of a Kubernetes cluster. Adding or removing nodes in a cluster. Changing the CRI of the node.
---

## How do I add master nodes to a cloud cluster?

For the procedure of converting a cluster with a single master node to a multi-master cluster, refer to the [FAQ](/modules/control-plane-manager/faq.html#how-do-i-add-master-nodes-to-a-cloud-cluster) of the `control-plane-manager` module.

## How do I reduce the number of master nodes in a cloud cluster?

For the procedure of converting a multi-master cluster to a cluster with a single master node, refer to the [FAQ](/modules/control-plane-manager/faq.html#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster) of the `control-plane-manager` module.

## Static nodes

<span id="how-do-i-add-a-static-node-to-a-cluster"></span>

You can add a static node to the cluster manually ([an example](examples.html#manually)) or by using [Cluster API Provider Static](#how-do-i-add-a-static-node-to-a-cluster-cluster-api-provider-static).

### How do I add a static node to a cluster (Cluster API Provider Static)?

To add a static node to a cluster (bare metal server or virtual machine), follow these steps:

1. Prepare the required resources:

   - Allocate a server or virtual machine and ensure that the node has the necessary network connectivity with the cluster.

   - If necessary, install additional operating system packages and configure the mount points that will be used on the node.

1. Create a user with `sudo` privileges:

   - Add a new user (in this example, `caps`) with `sudo` privileges:

     ```shell
     useradd -m -s /bin/bash caps
     usermod -aG sudo caps
     ```

   - Allow the user to run `sudo` commands without having to enter a password. For this, add the following line to the `sudo` configuration on the server (you can either edit the `/etc/sudoers` file, or run the `sudo visudo` command, or use some other method):

     ```shell
     caps ALL=(ALL) NOPASSWD: ALL
     ```

1. Set `UsePAM` to `yes` in `/etc/ssh/sshd_config` on server and restart sshd service:

   ```shell
   sudo systemctl restart sshd
   ```

1. Generate a pair of SSH keys with an empty passphrase on the server:

   ```shell
   ssh-keygen -t ed25519 -f caps-id -C "" -N ""
   ```

   The public and private keys of the caps user will be stored in the `caps-id.pub` and `caps-id` files in the current directory on the server.

1. Add the generated public key to the `/home/caps/.ssh/authorized_keys` file of the `caps` user by executing the following commands in the keys directory on the server:

   ```shell
   mkdir -p /home/caps/.ssh
   cat caps-id.pub >> /home/caps/.ssh/authorized_keys
   chmod 700 /home/caps/.ssh
   chmod 600 /home/caps/.ssh/authorized_keys
   chown -R caps:caps /home/caps/
   ```

1. Create the [SSHCredentials](cr.html#sshcredentials) resource.
1. Create the [StaticInstance](cr.html#staticinstance) resource.
1. Create the [NodeGroup](cr.html#nodegroup) resource with the `Static` [nodeType](cr.html#nodegroup-v1-spec-nodetype), specify the [desired number of nodes](cr.html#nodegroup-v1-spec-staticinstances-count) in the group and, if necessary, the [filter](cr.html#nodegroup-v1-spec-staticinstances-labelselector) for `StaticInstance`.

[An example](examples.html#using-the-cluster-api-provider-static) of adding a static node.

### How do I add a batch of static nodes to a cluster manually?

Use an existing one or create a new [NodeGroup](cr.html#nodegroup) custom resource ([example](examples.html#an-example-of-the-static-nodegroup-configuration) of the `NodeGroup` called `worker`). The [nodeType](cr.html#nodegroup-v1-spec-nodetype) parameter for static nodes in the NodeGroup must be `Static` or `CloudStatic`.

You can automate the bootstrap process with any automation platform you prefer. The following is an example for Ansible.

1. Pick up one of Kubernetes API Server endpoints. Note that this IP must be accessible from nodes that are being added to the cluster:

   ```shell
   d8 k -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Check the K8s version. If the version >= 1.25, create `node-group` token:

   ```shell
   d8 k create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Save the token you got and add it to the `token:` field of the Ansible playbook in the next steps.

1. If the Kubernetes version is smaller than 1.25, get a Kubernetes API token for a special ServiceAccount that Deckhouse manages:

   ```shell
   d8 k -n d8-cloud-instance-manager get $(d8 k -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

1. Create Ansible playbook with `vars` replaced with values from previous steps:

{% raw %}

   ```yaml
   - hosts: all
     become: yes
     gather_facts: no
     vars:
       kube_apiserver: <KUBE_APISERVER>
       token: <TOKEN>
     tasks:
       - name: Check if node is already bootsrapped
         stat:
           path: /var/lib/bashible
         register: bootstrapped
       - name: Get bootstrap secret
         uri:
           url: "https://{{ kube_apiserver }}/api/v1/namespaces/d8-cloud-instance-manager/secrets/manual-bootstrap-for-{{ node_group }}"
           return_content: yes
           method: GET
           status_code: 200
           body_format: json
           headers:
             Authorization: "Bearer {{ token }}"
           validate_certs: no
         register: bootstrap_secret
         when: bootstrapped.stat.exists == False
       - name: Run bootstrap.sh
         shell: "{{ bootstrap_secret.json.data['bootstrap.sh'] | b64decode }}"
         args:
           executable: /bin/bash
         ignore_errors: yes
         when: bootstrapped.stat.exists == False
       - name: wait
         wait_for_connection:
           delay: 30
         when: bootstrapped.stat.exists == False
   ```

{% endraw %}

1. Specify one more `node_group` variable. This variable must be the same as the name of `NodeGroup` to which node will belong. Variable can be passed in different ways, for example, by using an inventory file.:

   ```text
   [system]
   system-0
   system-1

   [system:vars]
   node_group=system

   [worker]
   worker-0
   worker-1

   [worker:vars]
   node_group=worker
   ```

1. Run the playbook with the inventory file.

### How do I clean up a static node manually?

{% alert level="info" %}
This method is valid for both manually configured nodes (using the bootstrap script) and nodes configured using CAPS.
{% endalert %}

To decommission a node from the cluster and clean up the server (VM), run the following command on the node:

### For all nodes except the control-plane

1. Delete a node from the Kubernetes cluster:

   ```shell
   d8 k drain <node> 
   d8 k drain <node> --ignore-daemonsets --delete-emptydir-data 
   d8 k delete pods --all-namespaces --field-selector spec.nodeName=<node> --force 
   d8 k delete node <node>
   ```

1. Run the cleanup script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. After restarting the node [run](#how-do-i-add-a-static-node-to-a-cluster) the script `bootstrap.sh`.

### For control-plane nodes

1. Remove the labels `node-role.kubernetes.io/control-plane`, `node-role.kubernetes.io/master`, and `node.deckhouse.io/group` from the node:

   ```shell
   d8 k label node <node> node-role.kubernetes.io/control-plane- node-role.kubernetes.io/master- node.deckhouse.io/group-
   ```

1. Make sure the removed node with control-plane has disappeared from the list of etcd cluster members:

   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

1. Delete a node from the Kubernetes cluster:

   ```shell
   d8 k drain <node> 
   d8 k drain <node> --ignore-daemonsets --delete-emptydir-data 
   d8 k delete pods --all-namespaces --field-selector spec.nodeName=<node> --force 
   d8 k delete node <node>
   ```

1. Run the cleanup script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. After restarting the node [run](#how-do-i-add-a-static-node-to-a-cluster) the script `bootstrap.sh`.

1. Wait for the Deckhouse queues to be processed and ensure that the etcd cluster member has reappeared in the list:
  
   ```shell
   d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

### Can I delete a StaticInstance?

A StaticInstance that is in the `Pending` state can be deleted with no adverse effects.

To delete a StaticInstance in any state other than `Pending` (`Running`, `Cleaning`, `Bootstrapping`), you need to:

1. Add the label `"node.deckhouse.io/allow-bootstrap": "false"` to the StaticInstance.

   Example command for adding a label:

   ```shell
   d8 k label staticinstance d8cluster-worker node.deckhouse.io/allow-bootstrap=false
   ```

1. Wait until the StaticInstance status becomes `Pending`.

   To check the status of StaticInstance, use the command:

   ```shell
   d8 k get staticinstances
   ```

1. Delete the StaticInstance.

   Example command for deleting StaticInstance:

   ```shell
   d8 k delete staticinstance d8cluster-worker
   ```

1. Decrease the `NodeGroup.spec.staticInstances.count` field by 1.

### How do I change the IP address of a StaticInstance?

You cannot change the IP address in the `StaticInstance` resource. If an incorrect address is specified in `StaticInstance`, you have to [delete the StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

### How do I migrate a manually configured static node under CAPS control?

You need to [clean up the node](#how-do-i-clean-up-a-static-node-manually), then [hand over](#how-do-i-add-a-static-node-to-a-cluster-cluster-api-provider-static) the node under CAPS control.

### Why are the SSH key and the sudo password of SSHCredentials shown as `<omitted>`?

The [`privateSSHKey`](cr.html#sshcredentials-v1alpha1-spec-privatesshkey), [`sudoPassword`](cr.html#sshcredentials-v1alpha1-spec-sudopassword) (`v1alpha1`), and [`sudoPasswordEncoded`](cr.html#sshcredentials-v1alpha2-spec-sudopasswordencoded) (`v1alpha2`) fields of the [SSHCredentials](cr.html#sshcredentials) resource contain sensitive data and are protected from viewing.

If a user has no permissions to read sensitive SSHCredentials data, the API returns `<omitted>` instead of the actual value. The other fields — [`user`](cr.html#sshcredentials-v1alpha1-spec-user), [`sshPort`](cr.html#sshcredentials-v1alpha1-spec-sshport), [`sshExtraArgs`](cr.html#sshcredentials-v1alpha1-spec-sshextraargs) — and the resource metadata remain available to users who are allowed to read SSHCredentials.

Additionally, kube-apiserver protects this data as follows:

- removes the `kubectl.kubernetes.io/last-applied-configuration` annotation from API responses if it may contain a copy of sensitive values;
- replaces sensitive values with `"******"` in [audit events](/products/kubernetes-platform/documentation/v1/admin/configuration/security/events/kubernetes-api-audit.html), regardless of user permissions and audit level;
- encrypts the resource in etcd using the same mechanism as for Kubernetes Secrets, if the [`apiserver.encryptionEnabled`](/modules/control-plane-manager/configuration.html#parameters-apiserver-encryptionenabled) parameter of the [`control-plane-manager`](/modules/control-plane-manager/) module is enabled.

Unmasked values are available to the [CAPS](./#cluster-api-provider-static) controller, which needs them to connect to the node over SSH, as well as to users with the [`SuperAdmin`](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/rbac-current.html#high-level-roles-used-in-the-current-model) access level and members of the [`kubeadm:cluster-admins`](/products/kubernetes-platform/documentation/v1/admin/configuration/access/authorization/cluster-admin-access-model.html) group.

The [`d8:subsystem:cluster:viewer`](/modules/user-authz/#subsystems-of-the-role-based-model) and [`d8:subsystem:cluster:manager`](/modules/user-authz/#subsystems-of-the-role-based-model) roles allow reading the SSHCredentials resource, but do not grant access to the SSH key or the sudo password.

To check whether a user can read sensitive SSHCredentials data, run:

```shell
d8 k auth can-i get sshcredentials/sensitive --as=<user>
```

The `<omitted>` value means the actual value is hidden from the current user. This behavior applies to both `v1alpha1` and `v1alpha2`, so using a different API version does not reveal the hidden data.

## How do I change the NodeGroup of a static node?

Note that if a node is under [CAPS](./#cluster-api-provider-static) control, you **cannot** change the `NodeGroup` membership of such a node. The only alternative is to [delete StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

To switch an existing [manually created](./#working-with-static-nodes) static node to another `NodeGroup`, you need to change its group label:

```shell
d8 k label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
d8 k label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Applying the changes will take some time.

## How to clean up a node for adding to the cluster?

This is only needed if you have to move a static node from one cluster to another. Be aware these operations remove local storage data. If you just need to change a NodeGroup, follow [this instruction](#how-do-i-change-the-nodegroup-of-a-static-node).

{% alert level="warning" %}
Evict resources from the node and remove the node from LINSTOR/DRBD using the [instruction](/modules/sds-replicated-volume/stable/faq.html#how-do-i-evict-resources-from-a-node) if the node you are cleaning up has LINSTOR/DRBD storage pools.
{% endalert %}

1. Delete the node from the Kubernetes cluster:

   ```shell
   d8 k drain <node> --ignore-daemonsets --delete-emptydir-data
   d8 k delete node <node>
   ```

1. Run cleanup script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. [Run](#how-do-i-add-a-static-node-to-a-cluster) the `bootstrap.sh` script after reboot of the node.

## How do I know if something went wrong?

If a node in a nodeGroup is not updated (the value of `UPTODATE` when executing the `d8 k get nodegroup` command is less than the value of `NODES`) or you assume some other problems that may be related to the `node-manager` module, then you need to look at the logs of the `bashible` service. The `bashible` service runs on each node managed by the `node-manager` module.

To view the logs of the `bashible` service on a specific node, run the following command:

```shell
journalctl -fu bashible
```

Example of output when the `bashible` service has performed all necessary actions:

```console
May 25 04:39:16 kube-master-0 systemd[1]: Started Bashible service.
May 25 04:39:16 kube-master-0 bashible.sh[1976339]: Configuration is in sync, nothing to do.
May 25 04:39:16 kube-master-0 systemd[1]: bashible.service: Succeeded.
```

## How do I know what is running on a node while it is being created?

You can analyze `cloud-init` to find out what's happening on a node during the bootstrapping process:

1. Find the node that is currently bootstrapping:

   ```shell
   d8 k get instances | grep Pending
   ```

   An example:

   ```shell
   d8 k get instances | grep Pending
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Get information about connection parameters for viewing logs:

   ```shell
   d8 k get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   An example:

   ```shell
   d8 k get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   bootstrapStatus:
     description: Use 'curl -N http://192.168.199.158:8000' to get bootstrap logs.
     logsEndpoint: http://192.168.199.158:8000
   ```

1. Run the command you got (`curl -N http://192.168.199.158:8000` according to the example above) to see `cloud-init` logs and determine the cause of the problem on the node.

The logs of the initial node configuration are located at `/var/log/cloud-init-output.log`.

## How do I update kernel on nodes?

### Debian-based distros

Create a `Node Group Configuration` resource by specifying the desired kernel version in the `desired_version` variable of the shell script (the resource's spec.content parameter):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

### CentOS-based distros

Create a `Node Group Configuration` resource by specifying the desired kernel version in the `desired_version` variable of the shell script (the resource's spec.content parameter):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-dnf-install "kernel-${desired_version}"
```

## NodeGroup parameters and their result

| The NodeGroup parameter               | Disruption update          | Node provisioning | Kubelet restart |
|---------------------------------------|----------------------------|-------------------|-----------------|
| chaos                                 | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |
| cloudInstances.maxSurgePerZone        | -                          | -                 | -               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| disruptions                           | -                          | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| kubelet.topologyManager               | + (policy other than `None`, [details](#how-do-i-give-pods-dedicated-cpus-and-numa-alignment)) | - | + |
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

Refer to the description of the [NodeGroup](cr.html#nodegroup) custom resource for more information about the parameters.

When the `InstanceClass` or `instancePrefix` parameters are modified, the process is similar to updating a Deployment in Kubernetes (changing `InstanceClass` results in creating a new MachineSet, and changing `instancePrefix` results in creating a new MachineDeployment). New instances will be created according to the value of the [`cloudInstances.maxSurgePerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-maxsurgeperzone) parameter. As the new instances are created, the old ones will be gradually removed, with a drain operation performed beforehand.

During the disruption update, an evict of the pods from the node is performed. If any pod fails to evict, the evict is repeated every 20 seconds until a global timeout of 5 minutes is reached. After that, the pods that failed to evict are removed.

## How do I redeploy ephemeral machines in the cloud with a new configuration?

If the Deckhouse configuration is changed (both in the node-manager module and in any of the cloud providers), the VMs will not be redeployed. The redeployment is performed only in response to changing `InstanceClass` or `NodeGroup` objects.

To force the redeployment of all Machines, you need to add/modify the `manual-rollout-id` annotation to the `NodeGroup`: `d8 k annotate NodeGroup name_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## How do I allocate nodes to specific loads?

{% alert level="warning" %}
You cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the `NodeGroup`. It is reserved for Deckhouse components. Please, use the `dedicated` or `dedicated.client.com` keys.
{% endalert %}

There are two ways to solve this problem:

1. You can set labels to `NodeGroup`'s `spec.nodeTemplate.labels`, to use them in the `Pod`'s [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
2. You cat set taints to `NodeGroup`'s `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

{% alert level="info" %}
Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.

To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the array [`.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys) parameters. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.
{% endalert %}

## How do I give pods dedicated CPUs and NUMA alignment?

Use the [`spec.kubelet.topologyManager`](cr.html#nodegroup-v1-spec-kubelet-topologymanager) parameter of the NodeGroup. Deckhouse configures the CPU, memory and topology managers of the kubelet and calculates NUMA-dependent values on each node of the group.

The result depends on `policy`:

| `policy` | What pods get | kubelet settings |
|----------|---------------|------------------|
| `None` (default) | Nothing, all pods share all CPUs | Not set, the kubelet defaults are used |
| `BestEffort` | Exclusive CPUs for containers of Guaranteed pods that request a whole number of CPUs, NUMA-bound memory. NUMA alignment of CPUs, memory and devices when possible | `cpuManagerPolicy: static`, `memoryManagerPolicy: Static`, `topologyManagerPolicy: best-effort` |
| `Restricted` | Exclusive whole physical cores. The pod gets CPUs, memory and devices from the smallest possible set of NUMA nodes, otherwise it is rejected | `cpuManagerPolicy: static` with `full-pcpus-only`, `memoryManagerPolicy: Static`, `topologyManagerPolicy: restricted` |
| `SingleNumaNode` | Exclusive whole physical cores. CPUs, memory and devices come from one NUMA node, otherwise the pod is rejected | `cpuManagerPolicy: static` with `full-pcpus-only`, `memoryManagerPolicy: Static`, `topologyManagerPolicy: single-numa-node` |

The `scope` parameter sets what is aligned for every policy except `None`: `Pod` (default) aligns all containers of a pod together, `Container` aligns each container separately.

The `enabled` parameter is deprecated and has no effect: the managers are controlled by `policy` alone. To turn them off, set `policy: None`.

For every policy except `None`, Deckhouse also:

- splits `reservedMemory` between NUMA nodes in proportion to their memory without hugepages;
- sets `prefer-closest-numa-nodes` for `Restricted` if the node reports a full NUMA distance table and its NUMA node IDs are consecutive from 0;
- sets `max-allowable-numa-nodes` if the node has more than 8 NUMA nodes.

The policies `BestEffort`, `Restricted` and `SingleNumaNode` require [`resourceReservation.mode`](cr.html#nodegroup-v1-spec-kubelet-resourcereservation-mode) `Auto`, or `Static` with a positive `static.cpu`.

With every policy except `None`, the memory of a Guaranteed pod is bound to the NUMA nodes chosen at admission, `BestEffort` included. A pod whose memory cannot be placed is rejected with `UnexpectedAdmissionError`. A container can be killed by the OOM killer when its NUMA nodes run out of memory, even if other NUMA nodes have free memory.

Exclusive CPUs are taken from the shared pool. Burstable and BestEffort pods, including system components, run on the CPUs that remain. This matters for pods that use more CPU than they request: without exclusive CPUs they use idle time of all cores, with exclusive CPUs they share the remaining CPUs. With `Auto`, the shared pool can shrink to 1 CPU. If needed, use `Static` with a larger `static.cpu`.

The following NodeGroup gives Guaranteed pods on the `worker-dpdk` nodes exclusive whole cores on one NUMA node:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-dpdk
spec:
  nodeType: Static
  kubelet:
    topologyManager:
      policy: SingleNumaNode
    resourceReservation:
      mode: Auto
```

Only containers of Guaranteed pods with a whole number of CPUs get exclusive CPUs: requests are equal to limits for CPU and memory in every container, and the CPU value is an integer:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dpdk-app
spec:
  containers:
  - name: app
    image: registry.example.com/dpdk-app:latest
    resources:
      requests:
        cpu: "4"
        memory: 8Gi
      limits:
        cpu: "4"
        memory: 8Gi
```

### Rules of the Restricted and SingleNumaNode policies

- The number of CPUs of a container must be a multiple of the number of threads per core. With SMT (Hyper-Threading) enabled, an odd number of CPUs is rejected with `SMTAlignmentError`.
- With `SingleNumaNode`, the pod (with `scope: Pod`) or each container (with `scope: Container`) must fit into one NUMA node. Size pods by the smallest NUMA node in the group.
- A pod that cannot be aligned is rejected: with `TopologyAffinityError` if its CPUs do not fit, with `UnexpectedAdmissionError` if its memory does not fit.
- The scheduler does not know about NUMA nodes. It can place a pod on a node where the pod is then rejected. The controller of the pod (for example, a Deployment or a StatefulSet) creates a new pod, which can be rejected again.

### Changing the settings

Changing `policy`, or `scope` with a policy other than `None`, restarts the kubelet on the nodes of the group.

The node requests a disruption if all of the following is true:

- the new kubelet configuration uses a policy other than `None`;
- the kubelet CPU or memory settings change, for example `policy`, `scope`, the CPU or memory reservation in `resourceReservation`, or the amount of memory or hugepages on the node. Other reservations, such as `ephemeralStorage`, do not cause a disruption;
- the node runs Guaranteed pods. DaemonSet and static pods are not counted, because a drain does not evict them.

The disruption is handled according to the [`disruptions`](cr.html#nodegroup-v1-spec-disruptions) settings of the group:

- with `approvalMode: Automatic` the node is drained and then gets the new configuration;
- with `approvalMode: Automatic` and [`automatic.drainBeforeApproval: false`](cr.html#nodegroup-v1-spec-disruptions-automatic-drainbeforeapproval) the node is not drained. After the kubelet restart, running pods that do not fit the new policy are stopped;
- with `approvalMode: Manual` the node is not drained automatically. Drain it, then run `d8 k annotate node <NODE_NAME> update.node.deckhouse.io/disruption-approved=`;
- with `approvalMode: RollingUpdate` (cloud nodes only) the node is replaced with a new one.

Switching to `None` does not drain nodes: running containers keep their CPUs and the NUMA node of their memory until they are recreated. To spread them over all CPUs, restart the workloads, for example with `d8 k rollout restart`.

### Limitations

- Nodes with more than 8 NUMA nodes are supported, but pod admission takes longer.
- Nodes in one group may have different NUMA layouts. Each node gets its own values, but the pod size limit is set by the smallest NUMA node.
- NodeGroupConfiguration scripts must not change reservations or manager policies in `/var/lib/kubelet/config.yaml`.
- Allocate hugepages before the kubelet is configured. Steps of NodeGroupConfiguration run in the order of `weight` together with the built-in steps, and the kubelet is configured at step 64, so use a `weight` below 64. The NUMA split of the reservation counts hugepages allocated at that moment. Hugepages allocated later change the split and can cause one more drain if the node runs Guaranteed pods.

## How to allocate nodes to system components?

### Frontend

For Ingress controllers, use the `NodeGroup` with the following configuration:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/frontend: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

### System components

`NodeGroup` for components of Deckhouse subsystems will look as follows:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```

## How do I speed up node provisioning on the cloud when scaling applications horizontally?

The most efficient way is to have some extra nodes "ready". In this case, you can run new application replicas on them almost instantaneously. The obvious disadvantage of this approach is the additional maintenance costs related to these nodes.

Here is how you should configure the target `NodeGroup`:

1. Specify the number of "ready" nodes (or a percentage of the maximum number of nodes in the group) using the `cloudInstances.standby` parameter.
1. If there are additional service components on nodes that are not handled by Deckhouse (e.g., the `filebeat` DaemonSet), you can specify the percentage of node resources they can consume via the `standbyHolder.overprovisioningRate` parameter.
1. This feature requires that at least one group node is already running in the cluster. In other words, there must be either a single replica of the application, or the `cloudInstances.minPerZone` parameter must be set to `1`.

An example:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    overprovisioningRate: 30%
```

## How do I disable machine-controller-manager/CAPI in the case of potentially cluster-damaging changes?

{% alert level="danger" %}
Use this switch only if you know what you are doing and clearly understand the consequences.
{% endalert %}

Set the `mcmEmergencyBrake` parameter to true:

```yaml
mcmEmergencyBrake: true
```

For disabling CAPI, set the `capiEmergencyBrake` parameter to true:

```yaml
capiEmergencyBrake: true
```

## How do I restore the master node if kubelet cannot load the control plane components?

Such a situation may occur if images of the control plane components on the master were deleted in a cluster that has a single master node (e.g., the directory `/var/lib/containerd` was deleted). In this case, kubelet cannot pull images of the control plane components when restarted since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

Below is an instruction on how you can restore the master node.

### containerd

Execute the following command to restore the master node in any cluster running under Deckhouse:

```shell
d8 k -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Copy the command's output and use it for setting the `AUTH` variable on the corrupted master.

Next, you need to pull images of `control plane` components to the corrupted master:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

You need to restart `kubelet` after pulling the images.

## How to change CRI for NodeGroup?

{% alert level="warning" %}
CRI can only be switched from `Containerd` to `NotManaged` and back (the [cri.type](cr.html#nodegroup-v1-spec-cri-type) parameter).
{% endalert %}

Set NodeGroup `cri.type` to `Containerd` or `NotManaged`.

NodeGroup YAML example:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
```

Also, this operation can be done with patch:

* For `Containerd`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* For `NotManaged`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
While changing `cri.type` for NodeGroups, created using `dhctl`, you must change it in `dhctl config edit provider-cluster-configuration` and in `NodeGroup` object.
{% endalert %}

After setting up a new CRI for NodeGroup, the node-manager module drains nodes one by one and installs a new CRI on them. Node update
is accompanied by downtime (disruption). Depending on the `disruption` setting for NodeGroup, the node-manager module either automatically allows
node updates or requires manual confirmation.

## Why might the CRI change not apply?

When attempting to switch the CRI, the changes may not take effect. The most common reason is the presence of special node labels: `node.deckhouse.io/containerd-v2-unsupported` and `node.deckhouse.io/containerd-config=custom`.

The `node.deckhouse.io/containerd-v2-unsupported` label is set to a node if at least one of the following conditions is true:

- Kernel version lower than 5.8
- Linux kernel version is in the range 6.12.0–6.12.28 or 6.14.0–6.14.6 (these versions are affected by CVE-2025-37999 in EROFS)
- systemd version lower than 244
- cgroup v2 is disabled
- EROFS file system is unavailable.

The `node.deckhouse.io/containerd-config=custom` label is set if the node contains `.toml` files in the `/etc/containerd/conf.d/` (if CRI containerd v1 is used on the cluster nodes) or `/etc/containerd/conf2.d/` (if CRI containerd v2 is used on the cluster nodes) directories. In this case, you should remove such files (provided this will not have critical impact on running containers) and delete the corresponding NodeGroupConfiguration (NGC) resources through which they may have been added.

If the [Deckhouse Virtualization Platform](https://deckhouse.io/products/virtualization-platform/documentation/) is used, an additional reason why the CRI may fail to switch can be the `containerd-dvcr-config.sh` NGC. If the virtualization platform is already installed and running, this NGC can be removed.

If you cannot remove the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource that modifies the containerd configuration and is incompatible with containerd v2, use the universal template:

{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
spec:
  bundles:
  - '*'
  content: |
    {{- if eq .cri "ContainerdV2" }}
  # <Script to modify the configuration for ContainerdV2>.
    {{- else }}
  # <Script to modify the configuration for ContainerdV1>.
    {{- end }}
  nodeGroups:
  - '*'
  weight: 31
```

{% endraw %}

Additionally, to switch the CRI you may need to remove the custom label `node.deckhouse.io/containerd-config=custom`. You can do this with the following command:

```shell
for node in $(d8 k get nodes -l node-role.kubernetes.io/<Name of NodeGroup where CRI is changed>=); do kubectl label $node node.deckhouse.io/containerd-config-; done
```

## How to change CRI for the whole cluster?

{% alert level="warning" %}
CRI can only be switched from `Containerd` to `NotManaged` and back (the [cri.type](cr.html#nodegroup-v1-spec-cri-type) parameter).
{% endalert %}

It is necessary to use the `dhctl` utility to edit the `defaultCRI` parameter in the `cluster-configuration` config.

Also, this operation can be done with the following patch:

* For `Containerd`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* For `NotManaged`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

If it is necessary to leave some NodeGroup on another CRI, then before changing the `defaultCRI` it is necessary to set CRI for this NodeGroup,
as described [here](#how-to-change-cri-for-nodegroup).

{% alert level="danger" %}
Changing `defaultCRI` entails changing CRI on all nodes, including master nodes.
If there is only one master node, this operation is dangerous and can lead to a complete failure of the cluster!
The preferred option is to make a multi-master and change the CRI type.
{% endalert %}

When changing the CRI in the cluster, additional steps are required for the master nodes:

1. Deckhouse updates nodes in master NodeGroup one by one, so you need to discover which node is updating right now:

   ```shell
   d8 k get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Confirm the disruption of the master node that was discovered in the previous step:

   ```shell
   d8 k annotate node <master node name> update.node.deckhouse.io/disruption-approved=
   ```

1. Wait for the updated master node to switch to `Ready` state. Repeat steps for the next master node.

## How to add node configuration step?

Additional node configuration steps are set via the [NodeGroupConfiguration](cr.html#nodegroupconfiguration) custom resource.

## How to automatically put custom labels on the node?

1. On the node, create the directory `/var/lib/node_labels`.

1. Create a file or files containing the necessary labels in it. The number of files can be any, as well as the number of subdirectories containing them.

1. Add the necessary labels to the files in the `key=value` format. For example:

   ```console
   example-label=test
   ```

1. Save the files.

When adding a node to the cluster, the labels specified in the files will be automatically affixed to the node.

{% alert level="warning" %}
Please note that it is not possible to add labels used in DP in this way. This method will only work with custom labels that do not overlap with those reserved for Deckhouse.
{% endalert %}

## How to deploy custom containerd configuration?

{% alert level="info" %}
The example of `NodeGroupConfiguration` uses functions of the script [032_configure_containerd.sh](./#features-of-writing-scripts).
{% endalert %}

{% alert level="danger" %}
Adding custom settings causes a restart of the containerd service.
{% endalert %}

Bashible on nodes merges main DP containerd configuration with the following configuration files:

- `/etc/containerd/conf.d/*.toml`: If containerd v1 is used as the CRI on the cluster nodes.
- `/etc/containerd/conf2.d/*.toml`: If containerd v2 is used as the CRI on the cluster nodes.

{% alert level="warning" %}
You can override the values of the parameters that are specified in the file `/etc/containerd/deckhouse.toml`, but you will have to ensure their functionality on your own. Also, it is better not to change the configuration for the master nodes (nodeGroup `master`).
{% endalert %}

The following are configuration examples of the NodeGroupConfiguration resources adding a custom configuration file for a corresponding containerd version.

{% tabs containerd_version %}
{% tab "For containerd v1" %}
{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_option.toml - << EOF
    oom_score = 500
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

{% endraw %}
{% endtab %}
{% tab "For containerd v2" %}
{% raw %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2024 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf2.d
    bb-sync-file /etc/containerd/conf2.d/runtimeclass.toml - << "EOF"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.gpu-large-shm]
      runtime_type = "io.containerd.runc.v2"
      [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.gpu-large-shm.options]
        BinaryName = ""
        SystemdCgroup = true
        ShmSize = 17179869184
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

{% endraw %}
{% endtab %}
{% endtabs %}

### How to add configuration for an additional registry?

Containerd supports two methods for registry configuration: the **deprecated** method and the **actual** method.

To check for the presence of the **deprecated** configuration method, run the following commands on the cluster nodes:  

```bash
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.mirrors'
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.configs'

# Example output:
# [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
#   [plugins."io.containerd.grpc.v1.cri".registry.mirrors."<REGISTRY_URL>"]
# [plugins."io.containerd.grpc.v1.cri".registry.configs]
#   [plugins."io.containerd.grpc.v1.cri".registry.configs."<REGISTRY_URL>".auth]
```

To check for the presence of the **actual** configuration method, run the following command on the cluster nodes:

```bash
cat /etc/containerd/config.toml | grep '/etc/containerd/registry.d'

# Example output:
# config_path = "/etc/containerd/registry.d"
```

#### Old Method

{% alert level="warning" %}
This containerd configuration format is deprecated.
{% endalert %}

{% alert level="info" %}
Used in containerd v1 when Deckhouse is not managed by the [Registry module](/modules/registry/).
{% endalert %}

The configuration is described in the main containerd configuration file `/etc/containerd/config.toml`.

Adding custom configuration is carried out through the `toml merge` mechanism. Configuration files from the `/etc/containerd/conf.d` directory are merged with the main file `/etc/containerd/config.toml`. The merge takes place during the execution of the `032_configure_containerd.sh` script, so the corresponding files must be added in advance.

Example configuration file for the `/etc/containerd/conf.d/` directory:

```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
          endpoint = ["https://${REGISTRY_URL}"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
          auth = "${BASE_64_AUTH}"
          username = "${USERNAME}"
          password = "${PASSWORD}"
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
          ca_file = "${CERT_DIR}/${CERT_NAME}.crt"
          insecure_skip_verify = true
```

{% alert level="danger" %}
Adding custom settings through the `toml merge` mechanism causes the containerd service to restart.
{% endalert %}

##### How to add additional registry auth (deprecated method)?

Example of adding authorization to a additional registry when using the **deprecated** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # To add a file before the '032_configure_containerd.sh' step.
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
              endpoint = ["https://${REGISTRY_URL}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
              username = "username"
              password = "password"
              # OR
              auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
    EOF
```

##### How to configure a certificate for an additional registry (deprecated method)?

Example of configuring a certificate for an additional registry when using the **deprecated** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # To add a file before the '032_configure_containerd.sh' step.
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"


    mkdir -p ${CERTS_FOLDER}
    bb-sync-file "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
```

{% alert level="info" %}
In addition to containerd, the certificate can be [added into the OS](examples.html#adding-a-certificate-to-the-os-and-containerd).
{% endalert %}

##### How to add TLS skip verify (deprecated method)?

Example of adding TLS skip verify when using the **deprecated** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # To add a file before the '032_configure_containerd.sh' step.
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              insecure_skip_verify = true
    EOF
```

After applying the configuration file, verify access to the registry from the nodes using the command:

```bash
# Via the CRI interface
crictl pull private.registry.example/image/repo:tag
```

##### How to set up a mirror for public image registries (deprecated method)?

Example of configuring a mirror for public image registries when using the **deprecated** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: mirror-to-harbor.sh
spec:
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    sed -i '/endpoint = \["https:\/\/registry-1.docker.io"\]/d' /var/lib/bashible/bundle_steps/032_configure_containerd.sh
    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/mirror-to-harbor.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry.private.network/v2/dockerhub-proxy/"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."gcr.io"]
              endpoint = ["https://registry.private.network/v2/YOUR_GCR_PROXY_REPO/"]
    EOF
```

#### New Method

{% alert level="info" %}
Used in containerd v2.

Used in containerd v1 when managed through the [`registry` module](/modules/registry/) (for example, in [`Direct`](/modules/deckhouse/configuration.html#parameters-registry) mode).
{% endalert %}

The configuration is defined in the `/etc/containerd/registry.d` directory.  
Configuration is specified by creating subdirectories named after the registry address:

```bash
/etc/containerd/registry.d
├── private.registry.example:5001
│   ├── ca.crt
│   └── hosts.toml
└── registry.deckhouse.io
    ├── ca.crt
    └── hosts.toml
```

Example contents of the `hosts.toml` file:

```toml
[host]
  # Mirror 1.
  [host."https://${REGISTRY_URL_1}"]
    capabilities = ["pull", "resolve"]
    ca = ["${CERT_DIR}/${CERT_NAME}.crt"]

    [host."https://${REGISTRY_URL_1}".auth]
      username = "${USERNAME}"
      password = "${PASSWORD}"

  # Mirror 2.
  [host."http://${REGISTRY_URL_2}"]
    capabilities = ["pull", "resolve"]
    skip_verify = true
```

{% alert level="info" %}
Configuration changes do not cause the containerd service to restart.
{% endalert %}

##### How to add additional registry auth (actual method)?

Example of adding authorization to a additional registry when using the **actual** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # The step can be arbitrary, as restarting the containerd service is not required.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        [host."https://${REGISTRY_URL}".auth]
          username = "username"
          password = "password"
    EOF
```

##### How to configure a certificate for an additional registry (actual method)?

Example of configuring a certificate for an additional registry when using the **actual** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # The step can be arbitrary, as restarting the containerd service is not required.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        ca = ["/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt"]
    EOF
```

{% alert level="info" %}
In addition to containerd, the certificate can be [added into the OS](examples.html#adding-a-certificate-to-the-os-and-containerd).
{% endalert %}

##### How to add TLS skip verify (actual method)?

Example of adding TLS skip verify when using the **actual** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # The step can be arbitrary, as restarting the containerd service is not required.
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        skip_verify = true
    EOF
```

After applying the configuration file, check access to the registry from the nodes using the following commands:

```bash
# Via the CRI interface.
crictl pull private.registry.example/image/repo:tag

# Via ctr with the configuration directory specified.
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/image/repo:tag

# Via ctr for an HTTP registry.
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/image/repo:tag
```

##### How to set up a mirror for public image registries (actual method)?

Example of configuring a mirror for public image registries when using the **actual** configuration method:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: mirror-to-harbor.sh
spec:
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    REGISTRY1_URL=docker.io
    mkdir -p "/etc/containerd/registry.d/${REGISTRY1_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY1_URL}/hosts.toml" - << EOF
    [host."https://registry.private.network/v2/dockerhub-proxy/"]
      capabilities = ["pull", "resolve"]
      override_path = true
    EOF
    REGISTRY2_URL=gcr.io
    mkdir -p "/etc/containerd/registry.d/${REGISTRY2_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY2_URL}/hosts.toml" - << EOF
    [host."https://registry.private.network/v2/dockerhub-proxy/"]
      capabilities = ["pull", "resolve"]
      override_path = true
    EOF
```

## How to use NodeGroup's priority feature

The [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) field of the `NodeGroup` CustomResource allows you to define the order in which nodes will be provisioned in the cluster. For example, `cluster-autoscaler` can first provision *spot-nodes* and switch to regular ones when they run out. Or it can provision larger nodes when there are plenty of resources in the cluster and then switch to smaller nodes once cluster resources run out.

Here is an example of creating two `NodeGroups` using spot-node nodes:

```yaml
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-spot
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-spot
    maxPerZone: 5
    minPerZone: 0
    priority: 50
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 0
    priority: 30
  nodeType: CloudEphemeral
```

In the above example, `cluster-autoscaler` will first try to provision a spot-node. If it fails to add such a node to the cluster within 15 minutes, the `worker-spot` NodeGroup will be paused (for 20 minutes), and `cluster-autoscaler` will start provisioning nodes from the `worker` NodeGroup.
If, after 30 minutes, another node needs to be deployed in the cluster, `cluster-autoscaler` will first attempt to provision a node from the `worker-spot` NodeGroup before provisioning one from the `worker` NodeGroup.

Once the `worker-spot` NodeGroup reaches its maximum (5 nodes in the example above), the nodes will be provisioned from the `worker` NodeGroup.

Note that node templates (labels/taints) for `worker` and `worker-spot` NodeGroups must be the same (or at least suitable for the load that triggers the cluster scaling process).

## How to interpret Node Group states?

**Ready** — the node group contains the minimum required number of scheduled nodes with the status ```Ready``` for all zones.

Example 1. A group of nodes in the ``Ready`` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Example 2. A group of nodes in the ``Not Ready`` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating** — a node group contains at least one node in which there is an annotation with the prefix ```update.node.deckhouse.io``` (for example, ```update.node.deckhouse.io/waiting-for-approval```).

**WaitingForDisruptiveApproval** - a node group contains at least one node that has an annotation ```update.node.deckhouse.io/disruption-required``` and
there is no annotation ```update.node.deckhouse.io/disruption-approved```.

**Scaling** — calculated only for node groups with the type ```CloudEphemeral```. The state ```True``` can be in two cases:

1. When the number of nodes is less than the *desired number of nodes* in the group, i.e. when it is necessary to increase the number of nodes in the group.
1. When a node is marked for deletion or the number of nodes is greater than the *desired number of nodes*, i.e. when it is necessary to reduce the number of nodes in the group.

The *desired number of nodes* is the sum of all replicas in the node group.

Example. The desired number of nodes is 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error** — contains the last error that occurred when creating a node in a node group.

## How do I make werf ignore the Ready conditions in a node group?

[werf](https://werf.io) checks the ```Ready``` status of resources and, if available, waits for the value to become ```True```.

Creating (updating) a [nodeGroup](cr.html#nodegroup) resource in a cluster can take a significant amount of time to create the required number of nodes. When deploying such a resource in a cluster using werf (e.g., as part of a CI/CD process), deployment may terminate when resource readiness timeout is exceeded. To make werf ignore the nodeGroup status, the following `nodeGroup` annotations must be added:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

## What is an Instance resource?

An Instance resource contains a description of an implementation-independent ephemeral machine resource. For example, machines created by MachineControllerManager or Cluster API Provider Static will have a corresponding Instance resource.

The object does not contain a specification. The status contains:

1. A link to the InstanceClass if it exists for this implementation;
1. A link to the Kubernetes Node object;
1. Current machine status;
1. Information on how to view [machine creation logs](#how-do-i-know-what-is-running-on-a-node-while-it-is-being-created) (at the machine creation stage).

When a machine is created/deleted, the Instance object is created/deleted accordingly.
You cannot create an Instance resource yourself, but you can delete it. In this case, the machine will be removed from the cluster (the removal process depends on implementation details).

## When is a node reboot required?

Node reboots may be required after configuration changes. For example, after changing certain sysctl settings, specifically when modifying the `kernel.yama.ptrace_scope` parameter (e.g., using `astra-ptrace-lock enable/disable` in the Astra Linux distribution).

## How to enable a delay before a node shutdown or restart while critical pods are running on it?

{% alert level="info" %}
Available in the EE and Ultimate editions.
{% endalert %}

{% alert level="warning" %}
To decide whether to block a node shutdown, DP additionally queries the NodeGroup. If the current node belongs to the `master` group and it is the only master node in the cluster, the shutdown block will not be applied to it.
{% endalert %}

To enable the mechanism that delays a pod's restart or shutdown, add the label `pod.deckhouse.io/inhibit-node-shutdown` to the Pod (for a Deployment, specify the label in the pod template).

Example of a Pod manifest with the `pod.deckhouse.io/inhibit-node-shutdown` label:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-stateful-app
  labels:
    app: my-stateful-app
    pod.deckhouse.io/inhibit-node-shutdown: "true"
spec:
  containers:
    - name: app
      image: my-registry/my-app:1.0.0
```

{% alert level="warning" %}
The block only applies to pods labeled with `pod.deckhouse.io/inhibit-node-shutdown` that are in the `Running` phase on the node. Kubelet shuts down the pods without this label according to the standard graceful shutdown process.
{% endalert %}

### Searching pods that prevent shutdown and performing the node troubleshooting

To search the pods that are preventing a node shutdown, use the following command:

```shell
d8 k get po -A -l pod.deckhouse.io/inhibit-node-shutdown -o wide
```

To view the `GracefulShutdownPostpone` condition on a node, use the following command:

```shell
d8 k get node <NODE_NAME> -o jsonpath='{range .status.conditions[?(@.type=="GracefulShutdownPostpone")]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

Possible values of the `reason` field:

- `WaitingForShutdownSignal`: The shutdown blocking mechanism is running on the node, but the blocking has not yet begun.
- `PodsWithLabelAreRunningOnNode`: The shutdown has been blocked because there are pods with the label `pod.deckhouse.io/inhibit-node-shutdown` still running on the node.
- `NoRunningPodsWithLabel`: The blocking has been released and the shutdown can continue.

## How do I set up fencing for a node group?

Fencing protects the cluster from nodes that lost the connection to it. A node that cannot reach the cluster stops its own work with a kernel panic. After that, Kubernetes can safely move the workloads to other nodes.

Set the fencing mode in the [`spec.fencing.mode`](cr.html#nodegroup-v1-spec-fencing-mode) parameter of the NodeGroup resource:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  fencing:
    mode: Normal
```

Each mode selects a built-in FencingSLAProfile. The profile sets the timings of the failure detection and the watchdog timeout:

| Mode       | Profile    | Time to consider the control plane lost (`controlPlaneLostAfter`) | Watchdog timeout | Reaction to a failure |
| ---------- | ---------- | ----------------------------------------------------------------- | ---------------- | --------------------- |
| `Disabled` | none       | none                                                              | none             | Fencing is off. This is the default mode. |
| `Critical` | `critical` | 4.15 s                                                            | 5 s              | About 12 s            |
| `Normal`   | `normal`   | 9.1 s                                                             | 15 s             | About 30 s            |
| `Slow`     | `slow`     | 20.2 s                                                            | 30 s             | About 60 s            |

The reaction time is a target. The tolerance is ±20 %. A failed node stops not later than `controlPlaneLostAfter` + 0.2 s + the watchdog timeout after the failure: 9.35 s with `Critical`, 24.3 s with `Normal` and 50.4 s with `Slow`.

A short timeout gives a fast reaction. But a short timeout also gives a higher risk of a false panic when the cluster is slow. Use `Slow` for groups with an unstable network or a heavy load. Use `Critical` only when a fast reaction is more important than the risk of a false panic.

{% alert level="info" %}
The `Critical` mode is available only in the commercial editions of Deckhouse. In Community Edition (CE), Deckhouse does not let you set the `Critical` mode for a node group. A group that already has this mode, for example after switching the edition to CE, works as `Normal`.
{% endalert %}

Note the following:

- The `Watchdog` value of the `spec.fencing.mode` parameter is deprecated. It works as `Slow`. Use `Normal`, `Slow`, or `Critical` (`Critical` is available only in the commercial editions).
- The `spec.fencing.watchdog.timeout` parameter is deprecated. Deckhouse ignores it. The watchdog timeout comes from the profile of the mode.
- In a cluster with one master node, Deckhouse does not let you turn on fencing. Refer to [Fencing in a cluster with one master node](#fencing-in-a-cluster-with-one-master-node).
- Deckhouse does not let you change the mode of a node group while a node of the group has a FencingFailedNodeState object. Refer to [Known limitations of the evacuation](#known-limitations-of-the-evacuation).
- The fencing agent runs on all nodes of the cluster when at least one node group has a mode other than `Disabled`. On the nodes of a `Disabled` group, the agent only takes part in the failure detection. It does not arm the watchdog.
- Fencing sets the `kernel.panic` kernel parameter to `0` on the nodes of the groups with a mode other than `Disabled`. A node does not restart by itself after a fencing panic.
- When you set a mode other than `Disabled` for a group, the new agent pod on a node starts only after bashible loads `softdog` and `/dev/watchdog` appears. Until then, the pod stays in the `ContainerCreating` state with the `FailedMount` event, and the agent does not work on the node. If the group already has agent pods, the DaemonSet replaces them by 10 percent of the nodes at a time. On Deckhouse Engine nodes, `softdog` is built into the kernel, and the pod does not wait. If a node waits for more than 5 minutes, the `D8FencingAgentDown` alert rises. When bashible loads `softdog`, the pod starts and the alert stops. No action is necessary. If the pod does not start, check the bashible logs on the node.

## How do I check the fencing state of the nodes?

Run the following command:

```shell
d8 k get nodes -o custom-columns='NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,FENCING:.status.conditions[?(@.type=="Fencing")].reason,PROFILE:.metadata.annotations.node-manager\.deckhouse\.io/fencing-profile'
```

The `READY` column shows the status of the `Ready` condition of the node. The `FENCING` column shows the reason of the `Fencing` condition, for example, `Armed` or `GroupDisabled`. The `PROFILE` column shows the `node-manager.deckhouse.io/fencing-profile` annotation of the node. The annotation has the FencingSLAProfile of the node group mode (`slow` for the deprecated `Watchdog` mode). This is also true when the watchdog of the node is not armed. If the group has the `Disabled` mode, the annotation is `disabled`. If the fencing agent does not run on the node, the `FENCING` and `PROFILE` columns show `<none>`. For example, this occurs when all node groups have the `Disabled` mode.

When fencing stops a node after the loss of the connection to the API server, the node cannot update its `Fencing` condition. Until the node restarts and the agent starts again, the `FENCING` column can still show `Armed`. To find a stopped node, look at the `READY` column: its value is not `True`.

## What happens when the fencing agent restarts?

- A planned stop of the agent, for example, an update of the DaemonSet, a deletion of the pod or a shutdown of the node, disarms the watchdog. The node does not stop. If the agent has already stopped the reset of the timer after the loss of the control plane, the watchdog stays armed.
- If the agent crashes or is killed, the watchdog stays armed. The watchdog timeout starts when the agent process stops. At the start, the new agent process checks whether the control plane is available:
  - If the control plane is available, the agent disarms the watchdog. Then the agent arms the watchdog again, as at a usual start. Until then, the reason of the `Fencing` condition is not `Armed`.
  - If the control plane is not available, the agent does not disarm the watchdog. If the agent does not arm the watchdog again before the timeout expires, the node stops.
- If the agent crashes at each start and the control plane is available, the node usually continues to work, but its watchdog is not armed. The `D8FencingAgentRestarted` alert rises, and the reason of the `Fencing` condition is not `Armed`. Read the logs of the agent and remove the cause of the crashes.

## What are the limits of softdog?

Fencing uses the `softdog` kernel module as the watchdog. Deckhouse loads the module with the `soft_panic=1` parameter. If the fencing agent does not reset the timer, the timer expires and the kernel panics.

`softdog` is a timer of the same kernel that it must protect. If the kernel has a hard lockup, the timer does not work and the node does not panic. If the node has a hardware watchdog, use it instead of `softdog`. A hardware watchdog works also when the kernel hangs.

### Deckhouse Engine nodes

On nodes of groups with `systemType: Immutable` (Deckhouse Engine), `softdog` is built into the kernel. The NodeGroupConfiguration script does not run on these nodes.

If you installed a node before the Deckhouse Engine release with the built-in `softdog`, the node does not have the built-in `softdog`. The node gets `softdog` only after you reinstall it or update its boot partition.

## How does fencing work on master nodes?

The fencing agent treats master nodes in the same way as worker nodes. The same modes and conditions apply. There is no special rule for the `master` group.

Note the following:

- After a fencing panic, a master node does not start again by itself. Return the node to the cluster manually. If etcd runs on the node, return the node to the etcd cluster manually.
- When a master node fails, the taint comes later than on a worker node, usually by 10–20 seconds. kube-apiserver on the other master nodes also sends requests to etcd on the failed master node and waits for an answer for approximately 10 seconds. The requests of the fencing-controller wait too. No action is necessary.
- The fencing-controller runs on the master nodes. If the master node of the controller fails or is not ready, Kubernetes evicts the controller pod from it at once, and the controller starts on another master node. In this case, the taint of a failed node comes later. The delay is the time in which Kubernetes marks the master node as unreachable ([`nodeMonitorGracePeriodSeconds`](/modules/control-plane-manager/configuration.html#parameters-nodemonitorgraceperiodseconds) of the `control-plane-manager` module, 40 seconds by default), plus the time to start the pod. In a cluster without the high availability mode, the controller has one replica.

### Fencing in a cluster with one master node

A node with fencing stops when it loses the control plane. In a cluster with one master node, a failure or a restart of this master node stops all nodes with fencing. Also, when the master node stops, no component remains to set the taint and to move the workloads. Thus, Deckhouse does not let you turn on fencing in a cluster with one master node:

- Deckhouse refuses a change of a NodeGroup that turns fencing on, that is, a change of `spec.fencing.mode` from `Disabled` to another mode.
- When you create a NodeGroup with a mode other than `Disabled`, Deckhouse only shows a warning. This does not stop the installation of a cluster with fencing in its resources.
- A NodeGroup that already has fencing on stays editable. Deckhouse does not turn off fencing that is already on.

Deckhouse counts the master nodes as follows. It counts the nodes with the `node-role.kubernetes.io/control-plane` label. If there are no such nodes, it counts the system nodes with the `node-role.deckhouse.io/master` label.

Until the installation of the cluster is complete, Deckhouse only shows a warning also when you turn fencing on. For this check, the installation is complete when the first node that is not a master node becomes `Ready`. Thus, in a cluster of one master node without other nodes, Deckhouse only shows a warning.

To turn on fencing in the `master` node group, first add master nodes to the cluster. Then set the mode of the group.

## Which network ports does fencing need?

The fencing agents on all nodes connect to each other on port 8500. Open the following access rules before you enable fencing:

- Port 8500 TCP and UDP between the InternalIP addresses of all cluster nodes.
- Port 8500 UDP between the addresses of the SystemNetwork on all nodes. This rule is necessary only if you use the second network (the `settings.fencing.systemNetworkFallback.systemNetworkName` parameter of the `node-manager` module). TCP is not necessary for this network. Currently the agents do not exchange messages through the second network. Open the rule in advance for the fallback through the SystemNetwork.

## What does the D8FencingSystemNetworkNotFound alert mean?

The alert means that the `settings.fencing.systemNetworkFallback.systemNetworkName` parameter of the `node-manager` module names a SystemNetwork that does not exist in the cluster. The agents cannot use the second network. Deckhouse does not change the parameter.

To fix the problem, do one of the following:

- Create the SystemNetwork.
- Set the name of an existing network.
- Delete the name from the parameter.

Note the following:

- Deckhouse checks that the SystemNetwork exists when you set `systemNetworkName` to a new non-empty value. Deckhouse can reject only this change. Deckhouse always accepts the deletion of the value and the changes of the other ModuleConfig fields, also when the check is not available.
- During `dhctl bootstrap`, the check is not active. Deckhouse accepts the parameter without the check. If the network does not exist, the alert rises after the module starts.
- The alert stops about 10 minutes after you restore the network.
- While all node groups have the `Disabled` mode, the alert does not rise. The agents do not use the second network in this case. The alert rises when you enable fencing.

## How does fencing protect the network key?

The fencing agents use a shared key to encrypt the messages between them. The key is in the `fencing-memberlist-key` Secret in the `d8-cloud-instance-manager` namespace. If you delete the Secret, Deckhouse generates a new key and the `D8FencingKeyRegenerated` alert rises.

When an agent gets a new key, it keeps the old key and starts to use the new key after 2 minutes. This delay lets the kubelet on all nodes update the Secret. The agent removes the old key after the same delay.

## How do I rotate the fencing network key?

The `fencing-memberlist-key` Secret in the `d8-cloud-instance-manager` namespace has two keys in the `data` field:

- `primary`: the agents use this key to encrypt the messages. It is necessary.
- `secondary`: the agents use this key only to decrypt the messages. It is optional.

Each key is exactly 32 random bytes. In the `data` field, the value is these 32 bytes encoded in base64. Do not write a base64 string into the `stringData` field: the agents then get a key of a wrong length.

To rotate the key, do the following steps:

1. Make a new key, write it into `primary` and move the current key to `secondary`. The key stays in a shell variable only, so it does not appear in the output and in the shell history:

   ```shell
   NEW_KEY=$(head -c 32 /dev/urandom | base64)
   d8 k -n d8-cloud-instance-manager patch secret fencing-memberlist-key --type=json -p "[{\"op\":\"copy\",\"from\":\"/data/primary\",\"path\":\"/data/secondary\"},{\"op\":\"replace\",\"path\":\"/data/primary\",\"value\":\"${NEW_KEY}\"}]"
   unset NEW_KEY
   ```

   The kubelet updates the Secret on the nodes, usually in approximately 1 minute. Each agent keeps the current key, adds the new key and starts to use the new key after 2 minutes. During the rotation, the agents with the old key and the agents with the new key decrypt the messages of each other.

1. Wait at least 10 minutes. Then all agents use the new key. Remove the old key from the Secret:

   ```shell
   d8 k -n d8-cloud-instance-manager patch secret fencing-memberlist-key --type=json -p '[
     {"op": "remove", "path": "/data/secondary"}
   ]'
   ```

Do not delete the Secret to rotate the key. Deckhouse then makes a new key without the old key, and the `D8FencingKeyRegenerated` alert rises.

If the key has a wrong format, the agents that run continue to use their current keys and ignore the wrong key. The `d8_fencing_agent_config_invalid{parameter="key"}` metric of these agents is `1`. An agent that starts with a wrong `primary` key cannot connect to the other agents and does not arm the watchdog. The `Fencing` condition of its node is `False` with the `InvalidKey` reason, and after 10 minutes the `D8FencingAgentInvalidKey` alert rises. To find these nodes, use the command from [How do I check the fencing state of the nodes?](#how-do-i-check-the-fencing-state-of-the-nodes). To check the length of the `primary` key, run the following command. It must show `32`:

```shell
d8 k -n d8-cloud-instance-manager get secret fencing-memberlist-key -o jsonpath='{.data.primary}' | base64 -d | wc -c
```

## How the fencing mechanism handles different node types?

The fencing mechanism stops a node that lost the connection with the cluster. It protects the cluster from "zombie" nodes in an undefined state.

A node with an armed watchdog works only while it sees the control plane. When the node does not see the control plane for longer than `controlPlaneLostAfter` of the profile, the fencing-agent stops the reset of the watchdog timer (the `softdog` kernel module) at once. When the timeout expires, a kernel panic occurs. This guarantees that no workloads remain running on the node.

While the node has a FencingFailedNodeState object, the node sees the control plane only when it can write its heartbeat through the API server. A node that cannot write its heartbeat stops in the same way.

After the agent stops the reset of the timer, it continues to check the control plane:

- If the control plane is available again before the watchdog timeout expires, the agent resets the timer again, and the node continues to work. Thus, a short pause of the control plane does not stop the nodes.
- The agent also asks the agents of the other nodes. It looks for a witness, that is, a node that sees the control plane. The agent sends a stop message to the witness, and the witness writes an attestation into the FencingFailedNodeState object of the node. After this message, the stop is final: the agent does not reset the timer again. The stop is also final when the FencingFailedNodeState object of the node is in the `Proven` phase.

A pause of the control plane in the whole cluster affects all nodes with an armed watchdog at the same time. Measurements on a cluster show that all nodes survive a pause of up to the following time:

| Mode       | Pause that the nodes survive |
| ---------- | ---------------------------- |
| `Critical` | 7.35 s                       |
| `Normal`   | 21.6 s                       |
| `Slow`     | 45.7 s                       |

If the pause is longer, some or all nodes with an armed watchdog can stop.

For more details on how the fencing-agent works, refer to the description of the [`spec.fencing.mode`](cr.html#nodegroup-v1-spec-fencing-mode) parameter of the NodeGroup resource.

After the node stops, the behavior depends on the type of the node group (the `node.deckhouse.io/type` label) and its engine (`status.engine`):

1. The node stays down. Deckhouse disables the automatic reboot after a kernel panic at the OS level. This applies to all nodes of the groups with a fencing mode other than `Disabled`. This prevents a node from rejoining the cluster in an undefined state.
2. When the stop of the node is certain, the fencing-controller sets the `node.kubernetes.io/out-of-service` taint on the Node. The stop is certain when a witness confirmed it and the watchdog timeout of the node passed, or when the node is silent for the time that it needs to stop. For the times and for the cases when the controller does not set the taint, refer to [When the controller sets the taint](#when-the-controller-sets-the-taint).
   After the taint, the controller sets the `Failed` phase and the `DisruptionTarget` condition on the pods of the node and deletes the pods immediately. This includes the pods that tolerate the taint. The controller does not delete the pods of DaemonSets and the static pods. The controller does not remove the finalizers of the pods: a pod with a finalizer stays in the API until the owner of the finalizer removes it. Pods of a StatefulSet and pods with ReadWriteOnce volumes start on other nodes.
   The evacuation is complete when the node has no `VolumeAttachment` objects. If the evacuation is not complete after 60 seconds, the incident stays in the `Evicting` phase and gets the `Progressing` condition with the status `False`. The reason of the condition tells what stops the evacuation now: `VolumesAttached`, `PodsRemaining` or `TooManyUnhealthyMachines`. The controller continues to examine the node and completes the evacuation when the cause is gone.
3. After the evacuation, Deckhouse acts on the node as follows:
   - For a node of a `CloudPermanent`, `CloudStatic`, or `Static` group (this includes a `Static` group with `staticInstances`, that is, CAPS), for a master node, and for an etcd arbiter node, Deckhouse only keeps the taint. It never deletes the Node object.
   - For a node of a `CloudEphemeral` group with `status.engine: CAPI`, Deckhouse deletes the Node object only when the MachineHealthCheck of the Cluster API cluster reports its machines and 50 percent or fewer of them are unhealthy. Deckhouse counts the stopped machine as unhealthy. MachineHealthCheck then sets the `HealthCheckSucceeded` condition of the Machine to `False` with the `NodeDeleted` reason and replaces the machine. If more than 50 percent of the machines are unhealthy, or the MachineHealthCheck does not report its machines yet, Deckhouse does not delete the Node object yet. The controller continues to examine the machines. When 50 percent or fewer of them are unhealthy, the controller deletes the Node object and completes the evacuation.
   - For a node of a `CloudEphemeral` group with `status.engine: MCM`, Deckhouse deletes the `Machine` (`machine.sapcloud.io`). Machine Controller Manager creates a new machine.
4. A node that Deckhouse does not replace stays down until you start it. When the node returns (the Node is `Ready` and it has a new boot ID), the fencing-controller removes the taint automatically. The controller works only while at least one node group has a mode other than `Disabled`. If you turn fencing off in all node groups, the taint stays on the node. Refer to [What to do with the out-of-service taint before you turn fencing off in all node groups?](#what-to-do-with-the-out-of-service-taint-before-you-turn-fencing-off-in-all-node-groups)

If the fencing-controller does not set the taint, Kubernetes sets the `node.kubernetes.io/unreachable` taint on the Node and evicts the pods of the node after the time in the [`failedNodePodEvictionTimeoutSeconds`](/modules/control-plane-manager/configuration.html#parameters-failednodepodevictiontimeoutseconds) parameter of the `control-plane-manager` module (300 seconds by default).

A node of a group with fencing can return to the cluster in a way that is different from the behavior above. If this occurs, contact the [support](https://deckhouse.io/tech-support/). Your report helps to clarify the sequence of checks and protection mechanisms.

### When the controller sets the taint

The controller sets the `node.kubernetes.io/out-of-service` taint only on a node that is stopped. The controller proves the stop in one of two ways:

- Attestation of a witness. The witness is a node whose agent received the stop message of the failed node. The agent of the witness writes the attestation into the FencingFailedNodeState object of the failed node. After the controller sees the attestation, it waits for `watchdog.timeout` + `watchdog.margin` of the profile. During this time, the watchdog of the failed node expires.
- Silence of the node. For example, the node is off or lost all networks, and no witness can confirm its stop. The controller waits for `controlPlaneLostAfter` + 0.2 s + `watchdog.timeout` + `watchdog.margin` + 0.9 s of the profile after the last sign of life of the node. The last sign of life is the later of two moments: when the controller saw the FencingFailedNodeState object first, and when it saw the last heartbeat of the node plus `heartbeat.interval`. During this time, the node detects the loss of the control plane and its watchdog expires.

The controller uses the way that gives the proof first:

| Mode       | Profile    | Time after the attestation | Time after the last sign of life |
| ---------- | ---------- | -------------------------- | -------------------------------- |
| `Critical` | `critical` | 5.3 s                      | 10.55 s                          |
| `Normal`   | `normal`   | 15.75 s                    | 25.95 s                          |
| `Slow`     | `slow`     | 31.5 s                     | 52.8 s                           |

When the incident gets the `Proven` phase, the controller writes an event. The event tells the way of the proof and the time.

The controller takes the profile from the `spec.profileRef.name` field of the FencingFailedNodeState object. The agent that creates the object writes the slower of two profiles: the profile that the failed node applied, and the profile of the node group at the time of the creation of the object. Deckhouse does not let you change the mode of the group while a node of the group has a FencingFailedNodeState object. Refer to [Known limitations of the evacuation](#known-limitations-of-the-evacuation).

The controller does not set the taint in these cases:

- The watchdog of the node was not armed (`spec.armed` is `false`). For example, the node is in a group with the `Disabled` mode.
- The `Fencing` condition of the Node is not `True`, that is, the agent of the node did not report that the watchdog is armed. The controller does not prove the failure, and the incident stays in the `Suspected` phase. The controller examines the node again every 5 seconds.
- The node sends heartbeats. The controller counts the time after the last sign of life again from each new heartbeat. A heartbeat after the attestation also cancels the proof by this attestation, and the `D8FencingNodeAliveAfterAttestation` alert rises.
- The node has one of the maintenance annotations `node-manager.deckhouse.io/fencing-disable`, `update.node.deckhouse.io/approved` or `update.node.deckhouse.io/disruption-approved`, or the agent of the node stops by plan. The controller examines the node again every 5 seconds.
- The owner reference of the FencingFailedNodeState object does not have the UID of the node with this name, for example after the node is created again with the same name. The controller does nothing for such an object.
- The controller cannot use the profile. The `D8FencingProfileIsUnavailable` alert rises.

### Phases of a fencing incident

The controller writes the phase of the incident into the `status.phase` field of the FencingFailedNodeState object. The name of the object is the name of the node. To see the incidents, run the following command:

```shell
d8 k get fencingfailednodestates
```

The phases:

- `Suspected`: an agent found the failure of the node. The controller waits for the proof of the failure.
- `FallbackAlive`: the node sends its heartbeat through the API server. The node works, but the agents of other nodes cannot connect to it.
- `Proven`: the controller proved the failure by the attestation of a witness or by the silence of the node. Refer to [When the controller sets the taint](#when-the-controller-sets-the-taint).
- `Evicting`: the node has the taint. The controller deletes the pods and waits until the node has no `VolumeAttachment` objects.
- `Evicted`: the evacuation is complete. The controller writes this phase first. Then it deletes the Node or the `Machine` of a node that the cloud replaces.

For the description of all fields, refer to [FencingFailedNodeState](cr.html#fencingfailednodestate).

### Fencing alerts

The fencing-controller gives these alerts:

- `D8FencingNodeFailed`: a node with an armed watchdog has an incident.
- `D8FencingNodeIsolatedFromPeers`: the incident is in the `FallbackAlive` phase for 1 minute. The node works, but the network between this node and the other nodes is broken. Usually a firewall rule or an ACL stops the traffic of the agents.
- `D8FencingNodeAliveAfterAttestation`: the node sent a heartbeat after the attestation. The controller does not set the taint on this node while the node sends heartbeats. Examine the node.
- `D8FencingEvictionNotProgressing`: the evacuation is not complete after 60 seconds. The `reason` label tells the reason of the `Progressing` condition.
- `D8NodeOutOfServiceTainted`: a node has the `node.kubernetes.io/out-of-service` taint for 30 minutes. Start the node again or remove it from the cluster.
- `D8FencingProfileIsUnavailable`: the controller cannot use the profile of the incident and does not set the taint.

The alerts of the controller exist only while the controller works, that is, while at least one node group has a mode other than `Disabled`.

The `D8FencingWatchdogDeviceMissing` alert of the fencing agent rises when the agent has no watchdog device for 30 minutes. The agent does not arm the watchdog, so the controller never sets the taint on this node. Usually the DaemonSet of the node group did not replace the agent pod of the previous revision yet.

### Known limitations of the evacuation

The fencing-controller runs on the master nodes. In a cluster with several master nodes, the controller has several replicas, and only the leader of the replicas works. A new leader counts the times of the incidents again from what it sees. Thus, if the leader changes during an incident, the taint comes later by up to approximately 20 seconds plus the time after the last sign of life from the table in [When the controller sets the taint](#when-the-controller-sets-the-taint). No action is necessary. The leader can change in these cases:

- The master node of the leader fails.
- kubernetes-api-proxy restarts with a new manifest on the master node of the leader, for example, during an update of Deckhouse. The controller connects to the API server through this proxy.

On the master nodes of Deckhouse Engine (`systemType: Immutable`), and when the `control-plane-manager` module is disabled, the controller connects to the API server through the ClusterIP of the `kubernetes` Service. When a master node fails, the requests of the controller can go to the failed master node for some seconds. Then the taint of the failed node comes later, and the leader can change.

When the master node of the controller fails, the controller starts on another master node only after Kubernetes marks the failed master node as unreachable. Until then, the controller does not set the taint. Refer to [How does fencing work on master nodes?](#how-does-fencing-work-on-master-nodes)

Deckhouse does not let you change the mode of a node group while a node of the group has a FencingFailedNodeState object. The controller calculates the times of an incident from the profile in the object, but the watchdog of the node can already use the times of the new mode. The error message names the nodes that have the objects. Wait until the incidents end, then change the mode. An incident ends when the node comes back after a reboot, or when its Node object is deleted. To find the objects:

```shell
d8 k get fencingfailednodestates
```

Change the mode of a node group only when all nodes of the group work correctly.

After switching the edition to CE, a FencingFailedNodeState object with the `critical` profile stays in the cluster, because CE has no such profile. The object gets the `ConfigurationError` condition, the `D8FencingProfileIsUnavailable` alert rises, and the controller does not evacuate the node. When the node comes back, that is, the Node is `Ready` and has a new boot ID, the controller deletes the object. In other cases, make sure that the node works or is off. Then delete the object:

```shell
d8 k delete fencingfailednodestate <NODE_NAME>
```

If the node fails again, the agents create a new object with the profile of the current mode. If the node has the `node.kubernetes.io/out-of-service` taint, the taint stays after the deletion. Remove the taint when the node works again:

```shell
d8 k taint node <NODE_NAME> node.kubernetes.io/out-of-service-
```

A node with an armed watchdog stops when it does not see the control plane. If the network separates a part of the cluster from the control plane, all nodes with an armed watchdog in this part stop, also when they work correctly.

The controller knows that the watchdog of a node is armed only from the `Fencing` condition of the Node. The agent writes this condition after it arms or disarms the watchdog. Between these two moments, the condition can be incorrect. This gives two known cases:

- The agent armed the watchdog, but the `True` status of the condition did not reach the API server, and at this moment the node failed. For example, this can occur when the node fails during the restart of the agent. The controller does not set the taint on this node until the node comes back.
- The agent crashed, and the new agent process disarmed the watchdog. Then the node lost the control plane before the agent wrote the `False` status of the condition. The node works and its watchdog is not armed, but the condition stays `True`. The controller does not set the taint on this node: if the node gets a FencingFailedNodeState object, the object has `spec.armed: false`.

A rollback of Deckhouse to a version with fencing 1.0 is not supported. If you do a rollback, unload the `softdog` kernel module on the nodes. If you do not, the module keeps the timeout of fencing 2.0 (5 seconds for the `Critical` profile), the agent of fencing 1.0 resets the timer less frequently, and the node goes into a kernel panic.

## What to do with the out-of-service taint before you turn fencing off in all node groups?

Only the fencing-controller removes the `node.kubernetes.io/out-of-service` taint from a node that came back. When the last node group gets the `Disabled` mode, Deckhouse deletes the controller together with the fencing agents. If a node has the taint at this time, the taint stays on the node. Deckhouse does not remove it, and no fencing alert rises after that. Kubernetes does not run pods on this node.

The same occurs when you disable the `node-manager` module, because Deckhouse deletes the controller together with the module. Deckhouse disables the module only after you confirm it with the `modules.deckhouse.io/allow-disabling=true` annotation on the ModuleConfig. Before you confirm, set the `Disabled` mode in all node groups and wait until no `fencing-agent` pods are left in the `d8-cloud-instance-manager` namespace. If you do not, an agent can fail to report its stop, and the watchdog stops its node. Also do the steps below before you set the `Disabled` mode in the last node group.

Before you set the `Disabled` mode in the last node group, do these steps:

1. Find the nodes with the taint and the fencing incidents:

   ```shell
   d8 k get nodes -o custom-columns='NAME:.metadata.name,TAINTS:.spec.taints[*].key' | grep -E '^NAME|node.kubernetes.io/out-of-service'
   d8 k get fencingfailednodestates
   ```

1. For each node with the taint, make sure that the node works again or that you replaced it. Then remove the taint:

   ```shell
   d8 k taint node <NODE_NAME> node.kubernetes.io/out-of-service-
   ```

   If the node is still stopped, you can turn fencing off. But then you must remove the taint manually after you start the node.

If you turn fencing on again, the controller removes the taint by the usual rule. It removes the taint when the Node is `Ready`, has a new boot ID and still has its FencingFailedNodeState object.

## How do I work with GPU nodes?

{% alert level="info" %}
GPU-node management is available in DP Enterprise Edition and DP Ultimate.
{% endalert %}

### Step-by-step procedure for adding a GPU node to the cluster

Starting with Deckhouse 1.71, if a NodeGroup resource contains the [`spec.gpu`](cr.html#nodegroup-v1-spec-gpu) section, the `node-manager` module **automatically**:

- Configures containerd with `default_runtime = "nvidia"`.
- Applies the required system settings (including fixes for the NVIDIA Container Toolkit).
- Deploys the following system components:
  - NFD
  - GFD
  - NVIDIA Device Plugin
  - DCGM Exporter
  - MIG Manager (if needed)

For the list of platforms supported by NVIDIA Container Toolkit, see the [official documentation](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/supported-platforms.html).

{% alert level="info" %}
To ensure correct operation, clearly specify the GPU sharing strategy in the [`spec.gpu.sharing`](cr.html#nodegroup-v1-spec-gpu-sharing) parameter (`Exclusive`, `TimeSlicing`, or `MIG`).

Containerd on GPU nodes is configured automatically. Do not change its configuration manually (for example, via the NodeGroupConfiguration resource or a configuration on the node).
{% endalert %}

{% alert level="info" %}
If a GPU is used in a single-node cluster and the `MIG` strategy is selected, an additional manual procedure is required after each node reboot to reapply the MIG configuration. For details, see [MIG in a single-node cluster](/products/kubernetes-platform/documentation/admin/configuration/platform-scaling/node/node-customization.html#mig-in-a-single-node-cluster) section.
{% endalert %}

To add a GPU node to the cluster, perform the following steps:

1. Create a [NodeGroup](cr.html#nodegroup) resource for GPU nodes.

   An example with **TimeSlicing** enabled (`partitionCount: 4`) and typical `taints` and `labels`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: gpu
   spec:
     nodeType: CloudStatic   # Can also be Static or CloudEphemeral — depending on your infrastructure.
     gpu:
       sharing: TimeSlicing
       timeSlicing:
         partitionCount: 4
     nodeTemplate:
       labels:
         node-role/gpu: ""
       taints:
       - key: node-role
         value: gpu
         effect: NoSchedule
   ```

   > If you use custom taint keys, ensure they are allowed in the ModuleConfig `global` in the array [`.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys), so that workloads can add the corresponding `tolerations`.

   A full field schema is available in the [NodeGroup custom resource reference](cr.html#nodegroup-v1-spec-gpu).

1. Install the NVIDIA driver and NVIDIA Container Toolkit.

   Install the NVIDIA driver and NVIDIA Container Toolkit directly on the nodes, either manually or via the [NodeGroupConfiguration](cr.html#nodegroupconfiguration) resource.
   Current NodeGroupConfiguration examples for the `gpu` NodeGroup (Ubuntu, Debian, CentOS) are available in the [`gpu` module FAQ](/modules/gpu/faq.html#how-do-i-work-with-gpu-nodes).

1. After the configuration is applied, perform a bootstrap and reboot the nodes so that settings are applied and the drivers get installed.

1. Verify installation on the node using the command:

   ```bash
   nvidia-smi
   ```

   {% alert level="warning" %}
   If the GPU node was prepared in advance, shipped with preinstalled NVIDIA drivers, or runs an OS with a graphical environment, make sure that the GPU is not being used by third-party processes.

   In the `nvidia-smi` output, the `Processes` section must not contain third-party processes using the GPU. On nodes with a graphical environment, these may include, for example, graphical session or display manager processes: `Xorg`, `gnome-shell`, `gdm`, `sddm`, `lightdm`, and others. Such processes can consume GPU memory and interfere with correct workload operation as well as MIG configuration.
   {% endalert %}

   Expected proper output (example):

   ```console
   root@k8s-dvp-w1-gpu:~# nvidia-smi
   Tue Aug  5 07:08:48 2025
   +---------------------------------------------------------------------------------------+
   | NVIDIA-SMI 535.247.01             Driver Version: 535.247.01   CUDA Version: 12.2     |
   |-----------------------------------------+----------------------+----------------------+
   | GPU  Name                 Persistence-M | Bus-Id        Disp.A | Volatile Uncorr. ECC |
   | Fan  Temp   Perf          Pwr:Usage/Cap |         Memory-Usage | GPU-Util  Compute M. |
   |                                         |                      |               MIG M. |
   |=========================================+======================+======================|
   |   0  Tesla V100-PCIE-32GB           Off | 00000000:65:00.0 Off |                    0 |
   | N/A   32C    P0              35W / 250W |      0MiB / 32768MiB |      0%      Default |
   |                                         |                      |                  N/A |
   +-----------------------------------------+----------------------+----------------------+
   
   +---------------------------------------------------------------------------------------+
   | Processes:                                                                            |
   |  No running processes found                                                           |
   +---------------------------------------------------------------------------------------+
   ```

1. Verify infrastructure components in the cluster.

   NVIDIA Pods in the `d8-nvidia-gpu` namespace:

   ```bash
   d8 k -n d8-nvidia-gpu get pod
   ```

   Expected proper output (example):

   ```console
   NAME                                  READY   STATUS    RESTARTS   AGE
   gpu-feature-discovery-80ceb7d-r842q   2/2     Running   0          2m53s
   nvidia-dcgm-exporter-w9v9h            1/1     Running   0          2m53s
   nvidia-dcgm-njqqb                     1/1     Running   0          2m53s
   nvidia-device-plugin-80ceb7d-8xt8g    2/2     Running   0          2m53s
   ```

   NFD Pods in the `d8-cloud-instance-manager` namespace:

   ```bash
   d8 k -n d8-cloud-instance-manager get pods | egrep '^(NAME|node-feature-discovery)'
   ```

   Expected proper output (example):

   ```console
   NAME                                             READY   STATUS      RESTARTS       AGE
   node-feature-discovery-gc-6d845765df-45vpj       1/1     Running     0              3m6s
   node-feature-discovery-master-74696fd9d5-wkjk4   1/1     Running     0              3m6s
   node-feature-discovery-worker-5f4kv              1/1     Running     0              3m8s
   ```

   Resource exposure on the node:

   ```bash
   d8 k describe node <node-name>
   ```

   Output snippet (example):

   ```console
   Capacity:
     cpu:                40
     memory:             263566308Ki
     nvidia.com/gpu:     4
   Allocatable:
     cpu:                39930m
     memory:             262648294441
     nvidia.com/gpu:     4
   ```

1. Run functional tests.

   **Option A. Invoke `nvidia-smi` from inside a container.**

   1. Create a Job:

      ```yaml
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: nvidia-cuda-test
        namespace: default
      spec:
        completions: 1
        template:
          spec:
            restartPolicy: Never
            nodeSelector:
              node.deckhouse.io/group: gpu
              node-role/gpu: ""
            tolerations:
              - key: "node-role"
                operator: "Equal"
                value: "gpu"
                effect: "NoSchedule"
            containers:
              - name: nvidia-cuda-test
                image: nvidia/cuda:11.6.2-base-ubuntu20.04
                imagePullPolicy: "IfNotPresent"
                command:
                  - nvidia-smi
      ```

   1. Check the logs using the command:

      ```bash
      d8 k logs job/nvidia-cuda-test
      ```

      Output example:

      ```console
      Tue Aug  5 07:48:02 2025
      +---------------------------------------------------------------------------------------+
      | NVIDIA-SMI 535.247.01             Driver Version: 535.247.01   CUDA Version: 12.2     |
      |-----------------------------------------+----------------------+----------------------+
      | GPU  Name                 Persistence-M | Bus-Id        Disp.A | Volatile Uncorr. ECC |
      | Fan  Temp   Perf          Pwr:Usage/Cap |         Memory-Usage | GPU-Util  Compute M. |
      |                                         |                      |               MIG M. |
      |=========================================+======================+======================|
      |   0  Tesla V100-PCIE-32GB           Off | 00000000:65:00.0 Off |                    0 |
      | N/A   31C    P0              23W / 250W |      0MiB / 32768MiB |      0%      Default |
      |                                         |                      |                  N/A |
      +-----------------------------------------+----------------------+----------------------+
      
      +---------------------------------------------------------------------------------------+
      | Processes:                                                                            |
      |  GPU   GI   CI        PID   Type   Process name                            GPU Memory |
      |        ID   ID                                                             Usage      |
      |=======================================================================================|
      |  No running processes found                                                           |
      +---------------------------------------------------------------------------------------+
      ```

   **Option B. CUDA sample (vectoradd).**

   1. Create a Job:

      ```yaml
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: gpu-operator-test
        namespace: default
      spec:
        completions: 1
        template:
          spec:
            restartPolicy: Never
            nodeSelector:
              node.deckhouse.io/group: gpu
            tolerations:
              - key: "node-role"
                operator: "Equal"
                value: "gpu"
                effect: "NoSchedule"
            containers:
              - name: gpu-operator-test
                image: nvidia/samples:vectoradd-cuda10.2
                imagePullPolicy: "IfNotPresent"
      ```

   1. Check the logs using the command:

      ```bash
      d8 k logs job/gpu-operator-test
      ```

      Output example:

      ```console
      [Vector addition of 50000 elements]
      Copy input data from the host memory to the CUDA device
      CUDA kernel launch with 196 blocks of 256 threads
      Copy output data from the CUDA device to the host memory
      Test PASSED
      Done
      ```

## How to monitor GPUs?

Deckhouse Platform automatically deploys **DCGM Exporter**; GPU metrics are scraped by Prometheus and available in Grafana.

## Which GPU modes are supported?

- **Exclusive** — the node exposes the `nvidia.com/gpu` resource; each Pod receives an entire GPU.
- **TimeSlicing** — time-sharing a single GPU among multiple Pods (default `partitionCount: 4`); Pods still request `nvidia.com/gpu`.
- **MIG (Multi-Instance GPU)** — hardware partitioning of supported GPUs into independent instances; with the `all-1g.5gb` profile the cluster exposes resources like `nvidia.com/mig-1g.5gb`.

See examples in [Managing nodes: examples](examples.html#example-gpu-nodegroup) section.

## How to view available MIG profiles in the cluster?

<span id="how-to-view-available-mig-profiles"></span>

Pre-defined profiles are stored in the **`mig-parted-config`** ConfigMap inside the **`d8-nvidia-gpu`** namespace and can be viewed with the command:

```bash
d8 k -n d8-nvidia-gpu get cm mig-parted-config -o json | jq -r '.data["config.yaml"]'
```

The `mig-configs:` section lists the **GPU models (by PCI ID) and the MIG profiles each card supports**—for example `all-1g.5gb`, `all-2g.10gb`, `all-balanced`.
Select the profile that matches your accelerator and set its name in `spec.gpu.mig.partedConfig` of the NodeGroup.

## How to set a custom per-GPU MIG layout on a node?

Set the [`partedConfig`](/modules/node-manager/cr.html#nodegroup-v1-spec-gpu-mig-partedconfig) parameter in NodeGroup to `custom` and define MIG partitioning per GPU index:

```yaml
spec:
  gpu:
    sharing: MIG
    mig:
      partedConfig: custom
      customConfigs:
        - index: 0
          slices:
            - profile: 7g.80gb
              count: 1    # Can be in the range from 1 to 7.
        - index: 1
          slices:
            - profile: 3g.40gb
              count: 2
            - profile: 1g.10gb
              count: 1
        # Add more indexes as needed.
```

As a result, the `node-manager` module automatically:

- Adds a configuration named `custom-<node-group-name>-<hash>` into the `mig-parted-config` ConfigMap, where `<hash>` is calculated based on the partitioning scheme (long NodeGroup names are truncated with a hash suffix to fit into a label).
- Sets the label `nvidia.com/mig.config=custom-<node-group-name>-<hash>` to the nodes in the corresponding group.

A separate `custom-<ng>-<hash>` configuration is created for each group of nodes, and configuration names do not overlap.

## MIG profile does not activate — what to check?

1. Check the GPU model. MIG is supported in the H100/A100/A30 models and **not** supported in V100/T4. To verify the support in a model, refer to [profile tables](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/latest/supported-mig-profiles.html) in the NVIDIA MIG guide.

1. Ensure the GPU is not being used by OS processes or user applications. If a graphical environment, display manager, or other GPU-consuming processes are running on the node, applying the MIG configuration may fail or may not take effect until the GPU is released. Check this with the following command:

   ```shell
   nvidia-smi
   ```

1. Check the [NodeGroup](cr.html#nodegroup-v1-spec-gpu) configuration:

   ```yaml
   gpu:
     sharing: MIG
     mig:
       partedConfig: all-1g.5gb
   ```

1. Wait until `nvidia-mig-manager` completes the **drain** of the node and reconfigures the GPU.

   **This process can take several minutes.**

   While it is running, the node is tainted with `mig-reconfigure`. When the operation succeeds, that taint is removed.

1. Track the progress via the `nvidia.com/mig.config.state` label on the node:

   `pending`, `rebooting`, `success` (or `error` if something goes wrong).

1. If `nvidia.com/mig-*` resources are still missing, check:

   ```bash
   d8 k -n d8-nvidia-gpu logs daemonset/nvidia-mig-manager
   nvidia-smi -L
   ```

## Are AMD or Intel GPUs supported?

At this time, Deckhouse Platform automatically configures **NVIDIA GPUs only**. Support for **AMD (ROCm)** and **Intel GPUs** is being worked on and is planned for future releases.
