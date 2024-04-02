---
title: "Managing nodes: FAQ"
search: add a node to the cluster, set up a GPU-enabled node, ephemeral nodes
description: Managing nodes of a Kubernetes cluster. Adding or removing nodes in a cluster. Changing the CRI of the node.
---
{% raw %}

## How do I add a master nodes to a cloud cluster (single-master to a multi-master)?

See [the control-plane-manager module FAQ...](../040-control-plane-manager/faq.html#how-do-i-add-a-master-nodes-to-a-cloud-cluster-single-master-to-a-multi-master)

## How do I reduce the number of master nodes in a cloud cluster (multi-master to single-master)?

See [the control-plane-manager module FAQ...](../040-control-plane-manager/faq.html#how-do-i-reduce-the-number-of-master-nodes-in-a-cloud-cluster-multi-master-to-single-master)

## Static nodes

<span id="how-do-i-add-a-static-node-to-a-cluster"></span>

You can add a static node to the cluster manually ([an example](examples.html#manually)) or by using [Cluster API Provider Static](#how-do-i-add-a-static-node-to-a-cluster-cluster-api-provider-static).

### How do I add a static node to a cluster (Cluster API Provider Static)?

To add a static node to a cluster (bare metal server or virtual machine), follow these steps:

1. Prepare the required resources — servers/virtual machines, install specific OS packages, add mount points, configure the network, etc.
1. Create the [SSHCredentials](cr.html#sshcredentials) resource.
1. Create the [StaticInstance](cr.html#staticinstance) resource.
1. Create the [NodeGroup](cr.html#nodegroup) resource with the `Static` [nodeType](cr.html#nodegroup-v1-spec-nodetype), specify the [desired number of nodes](cr.html#nodegroup-v1-spec-staticinstances-count) in the group and, if necessary, the [filter](cr.html#nodegroup-v1-spec-staticinstances-labelselector) for `StaticInstance`.

[An example](examples.html#using-the-cluster-api-provider-static) of adding a static node.

### How do I add a batch of static nodes to a cluster manually?

Use an existing one or create a new [NodeGroup](cr.html#nodegroup) custom resource ([example](examples.html#an-example-of-the-static-nodegroup-configuration) of the `NodeGroup` called `worker`). The [nodeType](cr.html#nodegroup-v1-spec-nodetype) parameter for static nodes in the NodeGroup must be `Static` or `CloudStatic`.

You can automate the bootstrap process with any automation platform you prefer. The following is an example for Ansible.

1. Pick up one of Kubernetes API Server endpoints. Note that this IP must be accessible from nodes that are being added to the cluster:

   ```shell
   kubectl -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Check the K8s version. If the version >= 1.25, create `node-group` token:

   ```shell
   kubectl create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Save the token you got and add it to the `token:` field of the Ansible playbook in the next steps.

1. If the Kubernetes version is smaller than 1.25, get a Kubernetes API token for a special ServiceAccount that Deckhouse manages:

   ```shell
   kubectl -n d8-cloud-instance-manager get $(kubectl -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

1. Create Ansible playbook with `vars` replaced with values from previous steps:

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

> This method is valid for both manually configured nodes (using the bootstrap script) and nodes configured using CAPS.

To decommission a node from the cluster and clean up the server (VM), run the following command on the node:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

### Can I delete a StaticInstance?

A `StaticInstance` that is in the `Pending` state can be deleted with no adverse effects.

To delete a `StaticInstance` in any state other than `Pending` (`Runnig`, `Cleaning`, `Bootstraping`), you need to:
1. Add the label `"node.deckhouse.io/allow-bootstrap": "false"` to the `StaticInstance`.
1. Wait until the `StaticInstance` status becomes `Pending`.
1. Delete the `StaticInstance`.
1. Decrease the `NodeGroup.spec.staticInstances.count` field by 1.

### How do I change the IP address of a StaticInstance?

You cannot change the IP address in the `StaticInstance` resource. If an incorrect address is specified in `StaticInstance`, you have to [delete the StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

### How do I migrate a manually configured static node under CAPS control?

You need to [clean up the node](#how-do-i-clean-up-a-static-node-manually), then [hand over](#how-do-i-add-a-static-node-to-a-cluster-cluster-api-provider-static) the node under CAPS control.

## How do I change the NodeGroup of a static node?

Note that if a node is under [CAPS](./#cluster-api-provider-static) control, you **cannot** change the `NodeGroup` membership of such a node. The only alternative is to [delete StaticInstance](#can-i-delete-a-staticinstance) and create a new one.

To switch an existing [manually created](./#working-with-static-nodes) static node to another `NodeGroup`, you need to change its group label:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Applying the changes will take some time.

## How to clean up a node for adding to the cluster?

This is only needed if you have to move a static node from one cluster to another. Be aware these operations remove local storage data. If you just need to change a NodeGroup, follow [this instruction](#how-do-i-change-the-nodegroup-of-a-static-node).

> **Note!** Evict resources from the node and remove the node from LINSTOR/DRBD using the [instruction](/modules/sds-replicated-volume/stable/faq.html#how-do-i-evict-resources-from-a-node) if the node you are cleaning up has LINSTOR/DRBD storage pools.

1. Delete the node from the Kubernetes cluster:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Run cleanup script on the node:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. [Run](#how-do-i-add-a-static-node-to-a-cluster) the `bootstrap.sh` script after reboot of the node.

## How do I know if something went wrong?

If a node in a nodeGroup is not updated (the value of `UPTODATE` when executing the `kubectl get nodegroup` command is less than the value of `NODES`) or you assume some other problems that may be related to the `node-manager` module, then you need to look at the logs of the `bashible` service. The `bashible` service runs on each node managed by the `node-manager` module.

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
   kubectl get instances | grep Pending
   ```

   An example:

   ```shell
   $ kubectl get instances | grep Pending
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Get information about connection parameters for viewing logs:

   ```shell
   kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   An example:

   ```shell
   $ kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
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
    bb-yum-install "kernel-${desired_version}"
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

Changing the `InstanceClass` or `instancePrefix` parameter in the Deckhouse configuration won't result in a `RollingUpdate`. Deckhouse will create new `MachineDeployment`s and delete the old ones. The number of `machinedeployments` ordered at the same time is determined by the `cloud Instances.maxSurgePerZone` parameter.

During the disruption update, an evict of the pods from the node is performed. If any pod failes to evict, the evict is repeated every 20 seconds until a global timeout of 5 minutes is reached. After that, the pods that failed to evict are removed.

## How do I redeploy ephemeral machines in the cloud with a new configuration?

If the Deckhouse configuration is changed (both in the node-manager module and in any of the cloud providers), the VMs will not be redeployed. The redeployment is performed only in response to changing `InstanceClass` or `NodeGroup` objects.

To force the redeployment of all Machines, you need to add/modify the `manual-rollout-id` annotation to the `NodeGroup`: `kubectl annotate NodeGroup name_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## How do I allocate nodes to specific loads?

> **Note!** You cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the `NodeGroup`. It is reserved for **Deckhouse** components. Please, use the `dedicated` or `dedicated.client.com` keys.

There are two ways to solve this problem:

1. You can set labels to `NodeGroup`'s `spec.nodeTemplate.labels`, to use them in the `Pod`'s [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
2. You cat set taints to `NodeGroup`'s `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

> Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️
> To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the [modules.placement.customTolerationKeys](../../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys) parameters. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.

## How to allocate nodes to system components?

### Frontend

For **Ingress** controllers, use the `NodeGroup` with the following configuration:

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
1. If there are additional service components (not maintained by Deckhouse, such as `filebeat` DaemonSet) for these nodes, you need to specify their combined resource consumption via the `standbyHolder.notHeldResources` parameter.
1. This feature requires that at least one group node is already running in the cluster. In other words, there must be either a single replica of the application, or the `cloudInstances.minPerZone` parameter must be set to `1`.

An example:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    notHeldResources:
      cpu: 300m
      memory: 2Gi
```

## How do I disable machine-controller-manager in the case of potentially cluster-damaging changes?

> **Note!** Use this switch only if you know what you are doing and clearly understand the consequences.

Set the `mcmEmergencyBrake` parameter to true:

```yaml
mcmEmergencyBrake: true
```

## How do I restore the master node if kubelet cannot load the control plane components?

Such a situation may occur if images of the control plane components on the master were deleted in a cluster that has a single master node (e.g., the directory `/var/lib/containerd` was deleted). In this case, kubelet cannot pull images of the control plane components when restarted since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

Below is an instruction on how you can restore the master node.

### containerd

Execute the following command to restore the master node in any cluster running under Deckhouse:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Copy the command's output and use it for setting the AUTH variable on the corrupted master.
Next, you need to pull images of `control plane` components to the corrupted master:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

You need to restart `kubelet` after pulling the images.

## How to change CRI for NodeGroup?

> **Note!** CRI can only be switched from `Containerd` to `NotManaged` and back (the [cri.type](cr.html#nodegroup-v1-spec-cri-type) parameter).

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
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* For `NotManaged`:

  ```shell
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

> **Note!** While changing `cri.type` for NodeGroups, created using `dhctl`, you must change it in `dhctl config edit provider-cluster-configuration` and in `NodeGroup` object.

After setting up a new CRI for NodeGroup, the node-manager module drains nodes one by one and installs a new CRI on them. Node update
is accompanied by downtime (disruption). Depending on the `disruption` setting for NodeGroup, the node-manager module either automatically allows
node updates or requires manual confirmation.

## How to change CRI for the whole cluster?

> **Note!** CRI can only be switched from `Containerd` to `NotManaged` and back (the [cri.type](cr.html#nodegroup-v1-spec-cri-type) parameter).

It is necessary to use the `dhctl` utility to edit the `defaultCRI` parameter in the `cluster-configuration` config.

Also, this operation can be done with the following patch:
* For `Containerd`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* For `NotManaged`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

If it is necessary to leave some NodeGroup on another CRI, then before changing the `defaultCRI` it is necessary to set CRI for this NodeGroup,
as described [here](#how-to-change-cri-for-nodegroup).

> **Note!** Changing `defaultCRI` entails changing CRI on all nodes, including master nodes.
> If there is only one master node, this operation is dangerous and can lead to a complete failure of the cluster!
> The preferred option is to make a multi-master and change the CRI type!

When changing the CRI in the cluster, additional steps are required for the master nodes:

1. Deckhouse updates nodes in master NodeGroup one by one, so you need to discover which node is updating right now:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Confirm the disruption of the master node that was discovered in the previous step:

   ```shell
   kubectl annotate node <master node name> update.node.deckhouse.io/disruption-approved=
   ```

1. Wait for the updated master node to switch to `Ready` state. Repeat steps for the next master node.

## How to add node configuration step?

Additional node configuration steps are set via the [NodeGroupConfiguration](cr.html#nodegroupconfiguration) custom resource.

## How to use containerd with Nvidia GPU support?

Create NodeGroup for GPU-nodes.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

Create NodeGroupConfiguration for containerd configuration of NodeGroup `gpu`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
  - '*'
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

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/nvidia_gpu.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".containerd]
          default_runtime_name = "nvidia"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = false
    EOF
  nodeGroups:
  - gpu
  weight: 49
```

Create NodeGroupConfiguration for Nvidia drivers setup on NodeGroup `gpu`.

### Ubuntu

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - ubuntu-lts
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

    distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
    curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get update
    apt-get install -y nvidia-container-toolkit nvidia-driver-525-server
  nodeGroups:
  - gpu
  weight: 30
```

### Centos

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - centos
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

    distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    yum install -y nvidia-container-toolkit nvidia-driver
  nodeGroups:
  - gpu
  weight: 30
```

Bootstrap and reboot node.

### How to check if it was successful?

Deploy the Job:

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
      containers:
        - name: nvidia-cuda-test
          image: nvidia/cuda:11.6.2-base-ubuntu20.04
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

And check the logs:

```shell
$ kubectl logs job/nvidia-cuda-test
Tue Jan 24 11:36:18 2023
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.60.13    Driver Version: 525.60.13    CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla T4            Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   45C    P0    25W /  70W |      0MiB / 15360MiB |      0%      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+

+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

Deploy the Job:

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
      containers:
        - name: gpu-operator-test
          image: nvidia/samples:vectoradd-cuda10.2
          imagePullPolicy: "IfNotPresent"
```

And check the logs:

```shell
$ kubectl logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## How to deploy custom containerd configuration?

Bashible on nodes merges main deckhouse containerd config with configs from `/etc/containerd/conf.d/*.toml`.

### How to add additional registry auth?

Deploy `NodeGroupConfiguration` script:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
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

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."artifactory.proxy"]
              endpoint = ["https://artifactory.proxy"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."artifactory.proxy".auth]
              auth = "AAAABBBCCCDDD=="
    EOF
  nodeGroups:
    - "*"
  weight: 49
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
You cannot create an Instance resource yourself, but you can delete it. In this case, the machine will be removed from the cluster (the removal process depends on implementation details.

{% endraw %}
