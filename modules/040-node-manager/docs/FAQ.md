---
title: "Managing nodes: FAQ"
search: add a node to the cluster, set up a GPU-enabled node, ephemeral nodes
---
{% raw %}

## How do I add a static node to a cluster?

To add a new static node (e.g., VM or bare-metal server) to the cluster, you need to:

1. Create a `NodeGroup` with the necessary parameters (`nodeType` can be `Static` or `CloudStatic`) or use an existing one. Let's, for example, create a [`NodeGroup` called `worker`](examples.html#an-example-of-the-static-nodegroup-configuration).
2. Get the script for installing and configuring the node:

   ```shell
   kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r
   ```

3. Before configuring Kubernetes on the node, make sure you have performed all the necessary actions for the node to work correctly in the cluster:
   - Added all the necessary mount points (NFS, Ceph, etc.) to `/etc/fstab`;
   - Installed the suitable `ceph-common` version on the node as well as other packages;
   - Configured the network in the cluster;
4. Connect to the new node over SSH and run the following command using the data from the secret: `echo <base64> | base64 -d | bash`

## How do I add a batch of static nodes to a cluster?

If you don't have `NodeGroup` in your cluster, then you can find information how to do it [here](#how-do-i-add-a-static-node-to-a-cluster).
If you already have `NodeGroup`, you can automate the bootstrap process with any automation platform that you prefer. We will use Ansible as an example.

1. Pick up one of Kubernetes API Server endpoints. Note that this IP have to be accessible from nodes that are prepared to be bootstrapped:

   ```shell
   kubectl get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

2. Get Kubernetes API token for special `ServiceAccount` that is managed by Deckhouse:

   ```shell
   kubectl -n d8-cloud-instance-manager get $(kubectl -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

3. Create Ansible playbook with `vars` replaced with values from previous steps:

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
         ignore_errors: yes
         when: bootstrapped.stat.exists == False
       - name: wait
         wait_for_connection:
           delay: 30
         when: bootstrapped.stat.exists == False
   ```

4. You have to specify one more variable `node_group`. This variable must be the same as the name of `NodeGroup` to which node will belong. Variable can be passed in different ways, here is an example using inventory file:

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

5. Now you can simply run this playbook with your inventory file.

## How to put an existing cluster node under the node-manager's control?

To make an existing Node controllable by the `node-manager`, perform the following steps:

1. Create a `NodeGroup` with the necessary parameters (`nodeType` can be `Static` or `CloudStatic`) or use an existing one. Let's, for example, create a [`NodeGroup` called `worker`](examples.html#an-example-of-the-static-nodegroup-configuration).
2. Get the script for installing and configuring the node: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."adopt.sh"' -r`
3. Connect to the new node over SSH and run the following command using the data from the secret: `echo <base64> | base64 -d | bash`

## How do I change the NodeGroup of a static node?

To switch an existing static node to another NodeGroup, you need to change its group label:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

The changes will not be applied instantly. One of the deckhouse hooks is responsible for updating the state of NodeGroup objects. It subscribes to node changes.

## How do I take a node out of the node-manager's control?

To take a node out of `node-manager` control, you need to:

1. Stop the bashible service and timer: `systemctl stop bashible.timer bashible.service`.
2. Delete bashible scripts: `rm -rf /var/lib/bashible`;
3. Remove annotations and labels from the node:

   ```shell
   kubectl annotate node <node_name> node.deckhouse.io/configuration-checksum- update.node.deckhouse.io/waiting-for-approval- update.node.deckhouse.io/disruption-approved- update.node.deckhouse.io/disruption-required- update.node.deckhouse.io/approved- update.node.deckhouse.io/draining- update.node.deckhouse.io/drained-
   kubectl label node <node_name> node.deckhouse.io/group-
   ```

## How to clean up a node for adding to the cluster?

This is only needed if you have to move a static node from one cluster to another. Be aware these operations remove local storage data. If you just need to change a NodeGroup, follow [this instruction](#how-do-i-change-the-nodegroup-of-a-static-node).

1. Delete the node from the Kubernetes cluster:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Stop all the services and running containers:

   ```shell
   systemctl stop kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl stop bashible.service bashible.timer
   systemctl stop kubelet.service
   systemctl stop containerd
   systemctl list-units --full --all | grep -q docker.service && systemctl stop docker
   kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
   ```

1. Unmount all mounted partitions:

   ```shell
   for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done
   ```

1. Delete all directories and files:

   ```shell
   rm -rf /var/lib/bashible
   rm -rf /var/cache/registrypackages
   rm -rf /etc/kubernetes
   rm -rf /var/lib/kubelet
   rm -rf /var/lib/docker
   rm -rf /var/lib/containerd
   rm -rf /etc/cni
   rm -rf /var/lib/cni
   rm -rf /var/lib/etcd
   rm -rf /etc/systemd/system/kubernetes-api-proxy*
   rm -rf /etc/systemd/system/bashible*
   rm -rf /etc/systemd/system/sysctl-tuner*
   rm -rf /etc/systemd/system/kubelet*
   ```

1. Delete all interfaces:

   ```shell
   ifconfig cni0 down
   ifconfig flannel.1 down
   ifconfig docker0 down
   ip link delete cni0
   ip link delete flannel.1
   ```

1. Cleanup systemd:

   ```shell
   systemctl daemon-reload
   systemctl reset-failed
   ```

1. Reboot the node.

1. Start CRI:

   ```shell
   systemctl start containerd
   systemctl list-units --full --all | grep -q docker.service && systemctl start docker
   ```

1. [Run](#how-do-i-add-a-static-node-to-a-cluster) the `bootstrap.sh` script.
1. Turn on all the services:

   ```shell
   systemctl start kubelet.service
   systemctl start kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl start bashible.service bashible.timer
   ```

## How do I know if something went wrong?

The `node-manager` module creates the `bashible` service on each node. You can browse its logs using the following command:

```shell
journalctl -fu bashible
```

## How do I know what is running on a node while it is being created?

You can analyze `cloud-init` to find out what's happening on a node during the bootstrapping process:

- Find the node that is currently bootstrapping: `kubectl -n d8-cloud-instance-manager get machine | grep Pending`
- To show details about a specific `machine`, enter: `kubectl -n d8-cloud-instance-manager describe machine kube-2-worker-01f438cf-757f758c4b-r2nx2`
  You will see the following information:

  ```shell
  Status:
    Bootstrap Status:
      Description:   Use 'nc 192.168.199.115 8000' to get bootstrap logs.
      Tcp Endpoint:  192.168.199.115
  ```

- Run the `nc 192.168.199.115 8000`command to see `cloud-init` logs and determine the cause of the problem on the node.

The logs of the initial node configuration are located at `/var/log/cloud-init-output.log`.

## How do I configure a GPU-enabled node?

If you have a GPU-enabled node and want to configure Docker to work with the `node-manager`, you must configure this node according to the [documentation](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

Create a `NodeGroup` with the following parameters:

```yaml
cri:
  type: NotManaged
operatingSystem:
  manageKernel: false
```

Then put the node under the control of `node-manager`.

## NodeGroup parameters and their result

| The NodeGroup parameter               | Disruption update          | Node provisioning | Kubelet restart |
|---------------------------------------|----------------------------|-------------------|-----------------|
| operatingSystem.manageKernel          | + (true) / - (false)       | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.docker.maxConcurrentDownloads     | +                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| nodeTemplate                          | -                          | -                 | -               |
| chaos                                 | -                          | -                 | -               |
| kubernetesVersion                     | -                          | -                 | +               |
| static                                | -                          | -                 | +               |
| disruptions                           | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |

Refer to the description of the [NodeGroup](cr.html#nodegroup) custom resource for more information about the parameters.

Changing the `instancePrefix` parameter in the Deckhouse configuration won't result in a `RollingUpdate`. Deckhouse will create new `MachineDeployment`s and delete the old ones.

During the disruption update, an evict of the pods from the node is performed. If any pod failes to evict, the evict is repeated every 20 seconds until a global timeout of 5 minutes is reached. After that, the pods that failed to evict are removed.

## How do I redeploy ephemeral machines in the cloud with a new configuration?

If the Deckhouse configuration is changed (both in the node-manager module and in any of the cloud providers), the VMs will not be redeployed. The redeployment is performed only in response to changing `InstanceClass` or `NodeGroup` objects.

To force the redeployment of all Machines, you need to add/modify the `manual-rollout-id` annotation to the `NodeGroup`: `kubectl annotate NodeGroup name_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## How do I allocate nodes to specific loads?

> **Note** that you cannot use the `deckhouse.io` domain in `labels` and `taints` keys of the `NodeGroup`. It is reserved for **Deckhouse** components. Please, use the `dedicated` or `dedicated.client.com` keys.

There are two ways to solve this problem:

1. You can set labels to `NodeGroup`'s `spec.nodeTemplate.labels`, to use them in the `Pod`'s [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters. In this case, you select nodes that the scheduler will use for running the target application.
2. You cat set taints to `NodeGroup`'s `spec.nodeTemplate.taints` and then remove them via the `Pod`'s [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter. In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

> Deckhouse tolerates the `dedicated` by default, so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️
> To use custom keys for `taints` (e.g., `dedicated.client.com`), you must add the key's value to the `global.modules.placement.customTolerationKeys` field of the `d8-system/deckhouse` ConfigMap. This way, deckhouse can deploy system components (e.g., `cni-flannel`) to these dedicated nodes.

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

Such a situation may occur if images of the control plane components on the master were deleted in a cluster that has a single master node (e.g., the directory `/var/lib/docker` (`/var/lib/containerd`) was deleted if Docker (container) is used). In this case, kubelet cannot pull images of the control plane components when restarted since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

Below is an instruction on how you can restore the master node.

### Docker

Execute the following command to restore the master node in any cluster running under Deckhouse:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r 'del(.auths."registry.deckhouse.io".username, .auths."registry.deckhouse.io".password)'
```

Copy the output of the command and add it to the `/root/.docker/config.json` file on the corrupted master.
Next, you need to pull images of control plane components to the corrupted master:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  docker pull $image
done
```

You need to restart kubelet after pulling the images.
Please, pay attention that you must **delete the changes made to the `/root/.docker/config.json` file after restoring the master node!**

### Containerd

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

Set NodeGroup `cri.type` to `Docker` or `Containerd`.

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

* For Containerd:

  ```shell
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* For Docker:

  ```shell
  kubectl patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Docker"}}}'
  ```

> **Note!** You cannot set `cri.type` for NodeGroups, created using `dhctl` (e.g. the `master` NodeGroup).

After setting up a new CRI for NodeGroup, the node-manager module drains nodes one by one and installs a new CRI on them. Node update
is accompanied by downtime (disruption). Depending on the `disruption` setting for NodeGroup, the node-manager module either automatically allows
node updates or requires manual confirmation.

## How to change CRI for the whole cluster?

> **Note!** Docker is deprecated, CRI can only be switched from Docker to Containerd. It's prohibited to switch from Containerd to Docker.

It is necessary to use the `dhctl` utility to edit the `defaultCRI` parameter in the `cluster-configuration` config.

Also, this operation can be done with patch:
* For Containerd

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Docker/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* For Docker

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/Docker/" | base64 -w0)"
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

Additional node configuration steps are set by custom resource `NodeGroupConfiguration`.

## How to use containerd with Nvidia GPU support?

Since using the Nvidia GPU requires a custom containerd configuration, it is necessary to create a NodeGroup with the `Unmanaged` CRI type.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  cri:
    type: NotManaged
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

### Debian

Debian-based distributions contain packages with Nvidia drivers in the base repository, so we do not need to prepare special images to support Nvidia GPU.

Deploy `NodeGroupConfiguration` scripts:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-containerd.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 31
  content: |
    # Copyright 2021 Flant JSC
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

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      systemctl daemon-reload
      systemctl enable containerd.service
      systemctl restart containerd.service
    }

    # set default
    desired_version={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | quote }}
    allowed_versions_pattern={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "allowedPattern" | quote }}

    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "debian" }}
      {{- $debianVersion := toString $key }}
      {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
    if bb-is-debian-version? {{ $debianVersion }} ; then
      desired_version={{ $value.containerd.desiredVersion | quote }}
      allowed_versions_pattern={{ $value.containerd.allowedPattern | quote }}
    fi
      {{- end }}
    {{- end }}

    if [[ -z $desired_version ]]; then
      bb-log-error "Desired version must be set"
      exit 1
    fi

    should_install_containerd=true
    version_in_use="$(dpkg -l containerd.io 2>/dev/null | grep -E "(hi|ii)\s+(containerd.io)" | awk '{print $2"="$3}' || true)"
    if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
      should_install_containerd=false
    fi

    if [[ "$version_in_use" == "$desired_version" ]]; then
      should_install_containerd=false
    fi

    if [[ "$should_install_containerd" == true ]]; then
      # set default
      containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%sStretch" (index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | replace "containerd.io=" "" | replace "." "" | replace "-" "")) }}"

    {{- $debianName := dict "9" "Stretch" "10" "Buster" "11" "Bullseye" }}
    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "debian" }}
      {{- $debianVersion := toString $key }}
      if bb-is-debian-version? {{ $debianVersion }} ; then
        containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%s%s" ($value.containerd.desiredVersion | replace "containerd.io=" "" | replace "." "" | replace "-" "") (index $debianName $debianVersion)) }}"
      fi
    {{- end }}

      crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"

      bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}"
    fi

    # Upgrade containerd-flant-edition if needed
    containerd_fe_tag="{{ index .images.registrypackages "containerdFe1511" | toString }}"
    if ! bb-rp-is-installed? "containerd-flant-edition" "${containerd_fe_tag}" ; then
      systemctl stop containerd.service
      bb-rp-install "containerd-flant-edition:${containerd_fe_tag}"

      mkdir -p /etc/systemd/system/containerd.service.d
      bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
    [Service]
    ExecStart=
    ExecStart=-/usr/local/bin/containerd
    EOF
    fi
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-and-start-containerd.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 50
  content: |
    # Copyright 2021 Flant JSC
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

    bb-event-on 'bb-sync-file-changed' '_on_containerd_config_changed'
    _on_containerd_config_changed() {
      systemctl restart containerd.service
    }

      {{- $max_concurrent_downloads := 3 }}
      {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
      {{- if .images }}
        {{- if .images.common.pause }}
          {{- $sandbox_image = printf "%s%s:%s" .registry.address .registry.path .images.common.pause }}
        {{- end }}
      {{- end }}

    systemd_cgroup=true
    # Overriding cgroup type from external config file
    if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
      systemd_cgroup=false
    fi

    # generated using `containerd config default` by containerd version `containerd containerd.io 1.4.3 269548fa27e0089a8b8278fc4fc781d7f65a939b`
    bb-sync-file /etc/containerd/config.toml - << EOF
    version = 2
    root = "/var/lib/containerd"
    state = "/run/containerd"
    plugin_dir = ""
    disabled_plugins = []
    required_plugins = []
    oom_score = 0
    [grpc]
      address = "/run/containerd/containerd.sock"
      tcp_address = ""
      tcp_tls_cert = ""
      tcp_tls_key = ""
      uid = 0
      gid = 0
      max_recv_message_size = 16777216
      max_send_message_size = 16777216
    [ttrpc]
      address = ""
      uid = 0
      gid = 0
    [debug]
      address = ""
      uid = 0
      gid = 0
      level = ""
    [metrics]
      address = ""
      grpc_histogram = false
    [cgroup]
      path = ""
    [timeouts]
      "io.containerd.timeout.shim.cleanup" = "5s"
      "io.containerd.timeout.shim.load" = "5s"
      "io.containerd.timeout.shim.shutdown" = "3s"
      "io.containerd.timeout.task.state" = "2s"
    [plugins]
      [plugins."io.containerd.gc.v1.scheduler"]
        pause_threshold = 0.02
        deletion_threshold = 0
        mutation_threshold = 100
        schedule_delay = "0s"
        startup_delay = "100ms"
      [plugins."io.containerd.grpc.v1.cri"]
        disable_tcp_service = true
        stream_server_address = "127.0.0.1"
        stream_server_port = "0"
        stream_idle_timeout = "4h0m0s"
        enable_selinux = false
        selinux_category_range = 1024
        sandbox_image = {{ $sandbox_image | quote }}
        stats_collect_period = 10
        systemd_cgroup = false
        enable_tls_streaming = false
        max_container_log_line_size = 16384
        disable_cgroup = false
        disable_apparmor = false
        restrict_oom_score_adj = false
        max_concurrent_downloads = {{ $max_concurrent_downloads }}
        disable_proc_mount = false
        unset_seccomp_profile = ""
        tolerate_missing_hugetlb_controller = true
        disable_hugetlb_controller = true
        ignore_image_defined_volumes = false
        [plugins."io.containerd.grpc.v1.cri".containerd]
          snapshotter = "overlayfs"
          default_runtime_name = "nvidia"
          no_pivot = false
          disable_snapshot_annotations = true
          discard_unpacked_layers = false
          [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              runtime_type = "io.containerd.runc.v2"
              runtime_engine = ""
              runtime_root = ""
              privileged_without_host_devices = false
              base_runtime_spec = ""
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
                SystemdCgroup = ${systemd_cgroup}
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = ${systemd_cgroup}
        [plugins."io.containerd.grpc.v1.cri".cni]
          bin_dir = "/opt/cni/bin"
          conf_dir = "/etc/cni/net.d"
          max_conf_num = 1
          conf_template = ""
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .registry.address }}"]
              endpoint = ["{{ .registry.scheme }}://{{ .registry.address }}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".auth]
              auth = "{{ .registry.auth | default "" }}"
      {{- if eq .registry.scheme "http" }}
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".tls]
              insecure_skip_verify = true
      {{- end }}
        [plugins."io.containerd.grpc.v1.cri".image_decryption]
          key_model = ""
        [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
          tls_cert_file = ""
          tls_key_file = ""
      [plugins."io.containerd.internal.v1.opt"]
        path = "/opt/containerd"
      [plugins."io.containerd.internal.v1.restart"]
        interval = "10s"
      [plugins."io.containerd.metadata.v1.bolt"]
        content_sharing_policy = "shared"
      [plugins."io.containerd.monitor.v1.cgroups"]
        no_prometheus = false
      [plugins."io.containerd.runtime.v1.linux"]
        shim = "containerd-shim"
        runtime = "runc"
        runtime_root = ""
        no_shim = false
        shim_debug = false
      [plugins."io.containerd.runtime.v2.task"]
        platforms = ["linux/amd64"]
      [plugins."io.containerd.service.v1.diff-service"]
        default = ["walking"]
      [plugins."io.containerd.snapshotter.v1.devmapper"]
        root_path = ""
        pool_name = ""
        base_image_size = ""
        async_remove = false
    EOF

    bb-sync-file /etc/crictl.yaml - << "EOF"
    runtime-endpoint: unix:/var/run/containerd/containerd.sock
    image-endpoint: unix:/var/run/containerd/containerd.sock
    timeout: 2
    debug: false
    pull-image-on-create: false
    EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 30
  content: |
    # Copyright 2021 Flant JSC
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

    distribution="debian9"
    curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey -o - | apt-key add -
    curl -s -L https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.list -o /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get update
    apt-get install -y nvidia-container-toolkit nvidia-driver-470
```

For other Debian versions you will need to correct the `distribution` variable and Nvidia driver package name (the `nvidia-driver-470` in the example above).

### CentOS

CentOS-based distributions do not contain Nvidia drivers in the base repositories.

The installation of Nvidia drivers in CentOS-based distributions is difficult to automate, so it is advisable to have a prepared image with the drivers installed.
How to install Nvidia drivers is written in [instruction](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/index.html#redhat-installation).

Deploy `NodeGroupConfiguration` scripts:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-containerd.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 31
  content: |
    # Copyright 2021 Flant JSC
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

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      systemctl daemon-reload
      systemctl enable containerd.service
      systemctl restart containerd.service
    }

    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
      {{- $centosVersion := toString $key }}
      {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
    if bb-is-centos-version? {{ $centosVersion }} ; then
      desired_version={{ $value.containerd.desiredVersion | quote }}
      allowed_versions_pattern={{ $value.containerd.allowedPattern | quote }}
    fi
      {{- end }}
    {{- end }}

    if [[ -z $desired_version ]]; then
      bb-log-error "Desired version must be set"
      exit 1
    fi

    should_install_containerd=true
    version_in_use="$(rpm -q containerd.io | head -1 || true)"
    if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
      should_install_containerd=false
    fi

    if [[ "$version_in_use" == "$desired_version" ]]; then
      should_install_containerd=false
    fi

    if [[ "$should_install_containerd" == true ]]; then

    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
      {{- $centosVersion := toString $key }}
      if bb-is-centos-version? {{ $centosVersion }} ; then
        containerd_tag="{{- index $.images.registrypackages (printf "containerdCentos%s" ($value.containerd.desiredVersion | replace "containerd.io-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
      fi
    {{- end }}

      crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"

      bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}"
    fi

    # Upgrade containerd-flant-edition if needed
    containerd_fe_tag="{{ index .images.registrypackages "containerdFe1511" | toString }}"
    if ! bb-rp-is-installed? "containerd-flant-edition" "${containerd_fe_tag}" ; then
      systemctl stop containerd.service
      bb-rp-install "containerd-flant-edition:${containerd_fe_tag}"

      mkdir -p /etc/systemd/system/containerd.service.d
      bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
    [Service]
    ExecStart=
    ExecStart=-/usr/local/bin/containerd
    EOF
    fi
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-and-start-containerd.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 50
  content: |
    # Copyright 2021 Flant JSC
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

    bb-event-on 'bb-sync-file-changed' '_on_containerd_config_changed'
    _on_containerd_config_changed() {
      systemctl restart containerd.service
    }

      {{- $max_concurrent_downloads := 3 }}
      {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
      {{- if .images }}
        {{- if .images.common.pause }}
          {{- $sandbox_image = printf "%s%s:%s" .registry.address .registry.path .images.common.pause }}
        {{- end }}
      {{- end }}

    systemd_cgroup=true
    # Overriding cgroup type from external config file
    if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
      systemd_cgroup=false
    fi

    # generated using `containerd config default` by containerd version `containerd containerd.io 1.4.3 269548fa27e0089a8b8278fc4fc781d7f65a939b`
    bb-sync-file /etc/containerd/config.toml - << EOF
    version = 2
    root = "/var/lib/containerd"
    state = "/run/containerd"
    plugin_dir = ""
    disabled_plugins = []
    required_plugins = []
    oom_score = 0
    [grpc]
      address = "/run/containerd/containerd.sock"
      tcp_address = ""
      tcp_tls_cert = ""
      tcp_tls_key = ""
      uid = 0
      gid = 0
      max_recv_message_size = 16777216
      max_send_message_size = 16777216
    [ttrpc]
      address = ""
      uid = 0
      gid = 0
    [debug]
      address = ""
      uid = 0
      gid = 0
      level = ""
    [metrics]
      address = ""
      grpc_histogram = false
    [cgroup]
      path = ""
    [timeouts]
      "io.containerd.timeout.shim.cleanup" = "5s"
      "io.containerd.timeout.shim.load" = "5s"
      "io.containerd.timeout.shim.shutdown" = "3s"
      "io.containerd.timeout.task.state" = "2s"
    [plugins]
      [plugins."io.containerd.gc.v1.scheduler"]
        pause_threshold = 0.02
        deletion_threshold = 0
        mutation_threshold = 100
        schedule_delay = "0s"
        startup_delay = "100ms"
      [plugins."io.containerd.grpc.v1.cri"]
        disable_tcp_service = true
        stream_server_address = "127.0.0.1"
        stream_server_port = "0"
        stream_idle_timeout = "4h0m0s"
        enable_selinux = false
        selinux_category_range = 1024
        sandbox_image = {{ $sandbox_image | quote }}
        stats_collect_period = 10
        systemd_cgroup = false
        enable_tls_streaming = false
        max_container_log_line_size = 16384
        disable_cgroup = false
        disable_apparmor = false
        restrict_oom_score_adj = false
        max_concurrent_downloads = {{ $max_concurrent_downloads }}
        disable_proc_mount = false
        unset_seccomp_profile = ""
        tolerate_missing_hugetlb_controller = true
        disable_hugetlb_controller = true
        ignore_image_defined_volumes = false
        [plugins."io.containerd.grpc.v1.cri".containerd]
          snapshotter = "overlayfs"
          default_runtime_name = "nvidia"
          no_pivot = false
          disable_snapshot_annotations = true
          discard_unpacked_layers = false
          [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              runtime_type = "io.containerd.runc.v2"
              runtime_engine = ""
              runtime_root = ""
              privileged_without_host_devices = false
              base_runtime_spec = ""
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
                SystemdCgroup = ${systemd_cgroup}
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = ${systemd_cgroup}
        [plugins."io.containerd.grpc.v1.cri".cni]
          bin_dir = "/opt/cni/bin"
          conf_dir = "/etc/cni/net.d"
          max_conf_num = 1
          conf_template = ""
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .registry.address }}"]
              endpoint = ["{{ .registry.scheme }}://{{ .registry.address }}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".auth]
              auth = "{{ .registry.auth | default "" }}"
      {{- if eq .registry.scheme "http" }}
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".tls]
              insecure_skip_verify = true
      {{- end }}
        [plugins."io.containerd.grpc.v1.cri".image_decryption]
          key_model = ""
        [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
          tls_cert_file = ""
          tls_key_file = ""
      [plugins."io.containerd.internal.v1.opt"]
        path = "/opt/containerd"
      [plugins."io.containerd.internal.v1.restart"]
        interval = "10s"
      [plugins."io.containerd.metadata.v1.bolt"]
        content_sharing_policy = "shared"
      [plugins."io.containerd.monitor.v1.cgroups"]
        no_prometheus = false
      [plugins."io.containerd.runtime.v1.linux"]
        shim = "containerd-shim"
        runtime = "runc"
        runtime_root = ""
        no_shim = false
        shim_debug = false
      [plugins."io.containerd.runtime.v2.task"]
        platforms = ["linux/amd64"]
      [plugins."io.containerd.service.v1.diff-service"]
        default = ["walking"]
      [plugins."io.containerd.snapshotter.v1.devmapper"]
        root_path = ""
        pool_name = ""
        base_image_size = ""
        async_remove = false
    EOF

    bb-sync-file /etc/crictl.yaml - << "EOF"
    runtime-endpoint: unix:/var/run/containerd/containerd.sock
    image-endpoint: unix:/var/run/containerd/containerd.sock
    timeout: 2
    debug: false
    pull-image-on-create: false
    EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 30
  content: |
    # Copyright 2021 Flant JSC
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

    distribution="centos7"
    curl -s -L https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.repo -o /etc/yum.repos.d/nvidia-container-toolkit.repo
    yum install -y nvidia-container-toolkit
```

### How to check if it was successful?

Deploy Job:

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
          image: docker.io/nvidia/cuda:11.0-base
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

And check the logs:

```shell
$ kubectl logs job/nvidia-cuda-test
Fri May  6 07:45:37 2022
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 470.57.02    Driver Version: 470.57.02    CUDA Version: 11.4     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla V100-SXM2...  Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   32C    P0    22W / 300W |      0MiB / 32510MiB |      0%      Default |
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

Deploy Job:

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

{% endraw %}
