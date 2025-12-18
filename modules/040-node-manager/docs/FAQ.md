---
title: "Managing nodes: FAQ"
search: add a node to the cluster, set up a GPU-enabled node, ephemeral nodes
description: Managing nodes of a Kubernetes cluster. Adding or removing nodes in a cluster. Changing the CRI of the node.
---

## How do I add a master nodes to a cloud cluster (single-master to a multi-master)?

See [the control-plane-manager module FAQ.](/modules/control-plane-manager/faq.html#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master)

## How do I reduce the number of master nodes in a cloud cluster (multi-master to single-master)?

See [the control-plane-manager module FAQ.](/modules/control-plane-manager/faq.html#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster-multi-master-to-single-master)

## Static nodes

<span id="how-do-i-add-a-static-node-to-a-cluster"></span>

You can add a static node to the cluster manually ([an example](examples.html#manually)) or by using [Cluster API Provider Static](#how-do-i-add-a-static-node-to-a-cluster-cluster-api-provider-static).

### How do I add a static node to a cluster (Cluster API Provider Static)?

To add a static node to a cluster (bare metal server or virtual machine), follow these steps:

1. Prepare the required resources:

   - Allocate a server or virtual machine and ensure that the node has the necessary network connectivity with the cluster.

   - If necessary, install additional operating system packages and configure the mount points that will be used on the node.

1. Create a user with `s`udo` privileges:

   - Add a new user (in this example, `caps`) with s`udo` privileges:

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
   ssh-keygen -t rsa -f caps-id -C "" -N ""
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
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. Run the command you got (`nc 192.168.199.115 8000` according to the example above) to see `cloud-init` logs and determine the cause of the problem on the node.

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
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

Refer to the description of the [NodeGroup](cr.html#nodegroup) custom resource for more information about the parameters.

When the `InstanceClass` or `instancePrefix` parameters are modified, the process is similar to updating a Deployment in Kubernetes (changing `InstanceClass` results in creating a new MachineSet, and changing `instancePrefix` results in creating a new MachineDeployment). New instances will be created according to the value of the [`cloudInstances.maxSurgePerZone`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-maxsurgeperzone) parameter. As the new instances are created, the old ones will be gradually removed, with a drain operation performed beforehand.

During the disruption update, an evict of the pods from the node is performed. If any pod failes to evict, the evict is repeated every 20 seconds until a global timeout of 5 minutes is reached. After that, the pods that failed to evict are removed.

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
Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️

To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the array [`.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys) parameters. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.
{% endalert %}

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

1. Specify the number of "ready" nodes (or a percentage of the maximum number of nodes in the group) using the `cloudInstances.standby` paramter.
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

The `node.deckhouse.io/containerd-v2-unsupported` label is set if the node does not meet at least one of the following requirements:

- Kernel version is at least 5.8;
- systemd version is at least 244;
- cgroup v2 is enabled;
- The EROFS filesystem is available.

The `node.deckhouse.io/containerd-config=custom` label is set if the node contains `.toml` files in the `conf.d` or `conf2.d` directories. In this case, you should remove such files (provided this will not have critical impact on running containers) and delete the corresponding NGCs through which they may have been added.

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
  # <Script to modify the configuration for ContainerdV2>
    {{- else }}
  # <Script to modify the configuration for ContainerdV1>
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
Please note that it is not possible to add labels used in DKP in this way. This method will only work with custom labels that do not overlap with those reserved for Deckhouse.
{% endalert %}

## How to deploy custom containerd configuration?

{% alert level="info" %}
The example of `NodeGroupConfiguration` uses functions of the script [032_configure_containerd.sh](./#features-of-writing-scripts).
{% endalert %}

{% alert level="danger" %}
Adding custom settings causes a restart of the containerd service.
{% endalert %}

Bashible on nodes merges main Deckhouse containerd config with configs from `/etc/containerd/conf.d/*.toml`.

{% alert level="warning" %}
You can override the values of the parameters that are specified in the file `/etc/containerd/deckhouse.toml`, but you will have to ensure their functionality on your own. Also, it is better not to change the configuration for the master nodes (nodeGroup `master`).
{% endalert %}

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
  # The step can be arbitrary, as restarting the containerd service is not required7
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

## How do I work with GPU nodes?

{% alert level="info" %}
GPU-node management is available in Enterprise Edition only.
{% endalert %}

### Step-by-step procedure for adding a GPU node to the cluster

Starting with Deckhouse 1.71, if a `NodeGroup` contains the `spec.gpu` section, the `node-manager` module **automatically**:

- configures containerd with `default_runtime = "nvidia"`;
- applies the required system settings (including fixes for the NVIDIA Container Toolkit);
- deploys system components: **NFD**, **GFD**, **NVIDIA Device Plugin**, **DCGM Exporter**, and, if needed, **MIG Manager**.

For the list of platforms supported by NVIDIA Container Toolkit, see [the official documentation](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/supported-platforms.html).

{% alert level="info" %}
Always specify the desired mode in `spec.gpu.sharing` (`Exclusive`, `TimeSlicing`, or `MIG`).

containerd on GPU nodes is configured automatically. Do not change its configuration manually (e.g. via `NodeGroupConfiguration` or TOML config).
{% endalert %}

To add a GPU node to the cluster, perform the following steps:

1. Create a NodeGroup for GPU nodes.

   An example with **TimeSlicing** enabled (`partitionCount: 4`) and typical taint/label:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: gpu
   spec:
     nodeType: CloudStatic   # or Static/CloudEphemeral — depending on your infrastructure.
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

   > If you use custom taint keys, ensure they are allowed in ModuleConfig `global` in the array [`.spec.settings.modules.placement.customTolerationKeys`](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-modules-placement-customtolerationkeys) so workloads can add the corresponding `tolerations`.

   Full field schema: see [NodeGroup CR documentation](cr.html#nodegroup-v1-spec-gpu).

1. Install the NVIDIA driver and nvidia-container-toolkit.

   Install the NVIDIA driver and NVIDIA Container Toolkit on the nodes—either manually or via a NodeGroupConfiguration.
   Below are NodeGroupConfiguration examples for the `gpu` NodeGroup.

   **Ubuntu**

   > Tested for Ubuntu 22.04.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: install-cuda.sh
   spec:
     bundles:
     - ubuntu-lts
     content: |
       #!/bin/bash
       set -e
 
       # Checking if curl is installed
       if ! command -v curl &> /dev/null || ! command -v wget &> /dev/null
       then
         echo "curl or wget is not installed. Installing..."
         sudo apt update
         sudo apt install -y curl wget
       fi
 
       # Define file paths
       CUDA_KEYRING_DEB="cuda-keyring_1.1-1_all.deb"
       NVIDIA_GPG_KEY="/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg"
 
       # Update repos
       sudo apt update
 
       # Install CUDA keyring
       if [ ! -f "$CUDA_KEYRING_DEB" ]; then
         wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2404/x86_64/$CUDA_KEYRING_DEB
         sudo dpkg -i $CUDA_KEYRING_DEB
       fi
 
       # Add NVIDIA container toolkit repos
       if [ ! -f "$NVIDIA_GPG_KEY" ]; then
         curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
           sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
         curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
           sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
           sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
       fi
 
       # Check and install Linux headers
       if ! dpkg-query -W -f='${Status}' "linux-headers-$(uname -r)" 2>/dev/null | grep -q "ok installed"; then
         echo "Installing linux headers..."
         sudo apt install -y "linux-headers-$(uname -r)"
       fi
 
       # Installation of NVIDIA drivers
       if ! dpkg-query -W -f='${Status}' cuda-drivers-575 2>/dev/null | grep -q "ok installed"; then
         echo "Installing CUDA drivers..."
         sudo apt install -y cuda-drivers-575
       fi
 
       # Installation of NVIDIA Container Toolkit
       if ! dpkg-query -W -f='${Status}' nvidia-container-toolkit 2>/dev/null | grep -q "ok installed"; then
         echo "Installing NVIDIA container toolkit..."
         sudo apt install -y nvidia-container-toolkit
       fi
 
     nodeGroups:
     - gpu
     weight: 5   
   ```

   **Debian**

   > Tested for Debian 12.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: install-cuda.sh
   spec:
     bundles:
     - debian
     content: |
       #!/bin/bash
       set -e
 
       # Checking if curl is installed
       if ! command -v curl &> /dev/null || ! command -v wget &> /dev/null
       then
         echo "curl or wget is not installed. Installing..."
         sudo apt update
         sudo apt install -y curl wget
       fi
 
       # Define file paths
       CUDA_KEYRING_DEB="cuda-keyring_1.1-1_all.deb"
       NVIDIA_GPG_KEY="/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg"
 
       # Update repos
       sudo apt update
 
       # Install CUDA keyring
       if [ ! -f "$CUDA_KEYRING_DEB" ]; then
         wget https://developer.download.nvidia.com/compute/cuda/repos/debian12/x86_64/$CUDA_KEYRING_DEB
         sudo dpkg -i $CUDA_KEYRING_DEB
       fi
 
       # Add NVIDIA container toolkit repos
       if [ ! -f "$NVIDIA_GPG_KEY" ]; then
         curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | \
           sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg
         curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
           sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
           sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
       fi
 
       # Check and install Linux headers
       if ! dpkg-query -W -f='${Status}' "linux-headers-$(uname -r)" 2>/dev/null | grep -q "ok installed"; then
         echo "Installing linux headers..."
         sudo apt install -y "linux-headers-$(uname -r)"
       fi
 
       # Installation of NVIDIA drivers
       if ! dpkg-query -W -f='${Status}' cuda-drivers-575 2>/dev/null | grep -q "ok installed"; then
         echo "Installing CUDA drivers..."
         sudo apt install -y cuda-drivers-575
       fi
 
       # Installation of NVIDIA Container Toolkit
       if ! dpkg-query -W -f='${Status}' nvidia-container-toolkit 2>/dev/null | grep -q "ok installed"; then
         echo "Installing NVIDIA container toolkit..."
         sudo apt install -y nvidia-container-toolkit
       fi
 
     nodeGroups:
     - gpu
     weight: 5  
   ```

   **CentOS**

   > Tested for CentOS 9.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: install-cuda.sh
   spec:
     bundles:
     - centos
     content: |
       #!/bin/bash
       set -e
       INSTALL_NEEDED=false
 
       # Checking if curl is installed
       if ! command -v curl &> /dev/null; then
         echo "curl is not installed. Installing..."
         sudo dnf install -y curl
         INSTALL_NEEDED=true
       fi
 
       # Checking another necessary packages and dependencies are installed
       if ! rpm -q epel-release &> /dev/null; then
         echo "EPEL release is not installed. Installing..."
         sudo dnf install -y epel-release
         INSTALL_NEEDED=true
       fi
       
       # Checking if dev tools are installed
       if ! rpm -q gcc kernel-devel-$(uname -r) &> /dev/null; then
         echo "Development tools are not completely installed. Installing..."
         sudo dnf update -y
         sudo dnf install -y gcc make dracut kernel-devel-$(uname -r) elfutils-libelf-devel
         INSTALL_NEEDED=true
       fi
       
       # Installation of NVIDIA drivers
       if ! rpm -q nvidia-driver-cuda nvidia-driver-cuda-libs &> /dev/null; then
         echo "NVIDIA CUDA drivers and libs are not installed. Installing..."
         sudo dnf config-manager --add-repo https://developer.download.nvidia.com/compute/cuda/repos/rhel9/x86_64/cuda-rhel9.repo
         sudo rpm --import https://developer.download.nvidia.com/compute/cuda/repos/GPGKEY
         sudo dnf clean all
         sudo dnf install -y nvidia-driver-cuda nvidia-driver-cuda-libs nvidia-settings nvidia-persistenced
         INSTALL_NEEDED=true
       fi
 
       # Installation of NVIDIA Container Toolkit
       if ! rpm -q nvidia-container-toolkit &> /dev/null; then
         echo "NVIDIA container toolkit is not installed. Installing..."
         curl -s -L https://nvidia.github.io/libnvidia-container/stable/rpm/nvidia-container-toolkit.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
         sudo dnf install -y nvidia-container-toolkit
         INSTALL_NEEDED=true
       fi
 
       # Bashible service creating if drivers were installed
       if [ "$INSTALL_NEEDED" = true ]; then
         base64_timer="W1VuaXRdCkRlc2NyaXB0aW9uPWJhc2hpYmxlIHRpbWVyCgpbVGltZXJdCk9uQm9vdFNlYz0xbWluCk9uVW5pdEFjdGl2ZVNlYz0xbWluCgpbSW5zdGFsbF0KV2FudGVkQnk9bXVsdGktdXNlci50YXJnZXQK"
         echo "$base64_timer" | base64 -d | sudo tee /etc/systemd/system/bashible.timer
         sudo systemctl enable bashible.timer
         base64_bashible="W1VuaXRdCkRlc2NyaXB0aW9uPUJhc2hpYmxlIHNlcnZpY2UKCltTZXJ2aWNlXQpFbnZpcm9ubWVudEZpbGU9L2V0Yy9lbnZpcm9ubWVudApFeGVjU3RhcnQ9L2Jpbi9iYXNoIC0tbm9wcm9maWxlIC0tbm9yYyAtYyAiL3Zhci9saWIvYmFzaGlibGUvYmFzaGlibGUuc2ggLS1tYXgtcmV0cmllcyAxMCIKUnVudGltZU1heFNlYz0zaAo="
         echo "$base64_bashible" | base64 -d | sudo tee /etc/systemd/system/bashible.service
         sudo systemctl enable bashible.service
         sudo systemctl reboot
       fi
 
     nodeGroups:
     - gpu
     weight: 5
   ```

   After these configurations are applied, perform bootstrap and **reboot** the nodes so that settings are applied and the drivers get installed.

1. Verify installation on the node using the command:

   ```bash
   nvidia-smi
   ```

   **Expected healthy output (example):**

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

1. Verify infrastructure components in the cluster

   NVIDIA Pods in `d8-nvidia-gpu`:

   ```bash
   d8 k -n d8-nvidia-gpu get pod
   ```

   **Expected healthy output (example):**

   ```console
   NAME                                  READY   STATUS    RESTARTS   AGE
   gpu-feature-discovery-80ceb7d-r842q   2/2     Running   0          2m53s
   nvidia-dcgm-exporter-w9v9h            1/1     Running   0          2m53s
   nvidia-dcgm-njqqb                     1/1     Running   0          2m53s
   nvidia-device-plugin-80ceb7d-8xt8g    2/2     Running   0          2m53s
   ```

   NFD Pods in `d8-cloud-instance-manager`:

   ```bash
   d8 k -n d8-cloud-instance-manager get pods | egrep '^(NAME|node-feature-discovery)'
   ```

   **Expected healthy output (example):**

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

   **Output snippet (example):**

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

1. Run functional tests

   **Option A. Invoke `nvidia-smi` from inside a container.**

   Create a Job:

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

   Check the logs using the command:

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

   Create a Job:

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

   Check the logs using the command:

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

Deckhouse Kubernetes Platform automatically deploys **DCGM Exporter**; GPU metrics are scraped by Prometheus and available in Grafana.

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

## MIG profile does not activate — what to check?

1. **GPU model:** MIG is supported on H100/A100/A30; it is **not** supported on V100/T4. See the profile tables in the [NVIDIA MIG guide](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/contents.html).
1. **NodeGroup configuration:**

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

At this time, Deckhouse Kubernetes Platform automatically configures **NVIDIA GPUs only**. Support for **AMD (ROCm)** and **Intel GPUs** is being worked on and is planned for future releases.
