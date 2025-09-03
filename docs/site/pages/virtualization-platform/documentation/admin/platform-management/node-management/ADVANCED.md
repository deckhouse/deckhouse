---
title: "Advanced configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/advanced.html
lang: en
---

## Restoring a master node when kubelet can't load control plane components

This situation may occur if the control plane component images were deleted
in a cluster with a single master node (for example, the `/var/lib/containerd` directory was deleted).
In this case, kubelet can't pull the control plane component images during a restart,
since the master node lacks authorization parameters required for accessing `registry.deckhouse.io`.

### containerd

To restore the master node, follow these steps:

1. Run the following command in any cluster managed by Deckhouse:

   ```shell
   d8 k -n d8-system get secrets deckhouse-registry -o json |
   jq -r '.data.".dockerconfigjson"' | base64 -d |
   jq -r '.auths."registry.deckhouse.io".auth'
   ```

1. Copy the command output and set it for the `AUTH` variable on the corrupted master node.
1. Pull the control plane component images to the corrupted master node:

   ```shell
   for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
     crictl pull --auth $AUTH $image
   done
   ```

1. Once the images are pulled, restart kubelet.

## Changing CRI for a NodeGroup

{% alert level="info" %}
Container Runtime Interface (CRI) can only be switched from `Containerd` to `NotManaged` and back.
{% endalert %}

To change the CRI for a NodeGroup, set the [`cri.type`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-cri-type) parameter to either `Containerd` or `NotManaged`.

Example of a NodeGroup YAML manifest:

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

Alternatively, you can change the CRI using the following patch:

- For `Containerd`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

- For `NotManaged`:

  ```shell
  d8 k patch nodegroup <NodeGroup name> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

{% alert level="warning" %}
When changing `cri.type` for the NodeGroup objects created using `dhctl`, change it in `dhctl config edit provider-cluster-configuration` and in the NodeGroup object configuration.
{% endalert %}

After setting up a new CRI for NodeGroup, the node-manager module drains nodes one by one and installs a new CRI on them.
The node update is followed by a disruption.
Depending on the `disruption` setting for a NodeGroup,
the node-manager module either automatically allows node updates or requires manual confirmation.

## Changing CRI for the whole cluster

{% alert level="info" %}
CRI can only be switched from `Containerd` to `NotManaged` and back.
{% endalert %}

To change the CRI for for the whole cluster, use the `dhctl` utility and edit the `defaultCRI` parameter in `cluster-configuration`.

Alternatively, you can change the CRI for the cluster using the following patch:

- For `Containerd`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p '{"data":{"cluster-configuration.yaml":"'${data}'"}}'
  ```

- For `NotManaged`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p '{"data":{"cluster-configuration.yaml":"'${data}'"}}'
  ```

If you need to keep a NodeGroup on a different CRI,
before changing `defaultCRI`, set the CRI for this NodeGroup following the [corresponding procedure](#changing-cri-for-a-nodegroup).

{% alert level="danger" %}
Changing `defaultCRI` will cause the CRI changing on all nodes, including master nodes.
If there's only one master node, this operation is dangerous and can lead to a complete failure of the cluster.
The preferred option is to make a multi-master and change the CRI type.
{% endalert %}

When you change the CRI in the cluster, complete several additional steps for the master nodes:

1. Deckhouse updates nodes in the master NodeGroup one by one, so to find out which node is currently being updated, run the following command:

   ```shell
   d8 k get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Confirm the disruption of the master node you found in the previous step:

   ```shell
   d8 k annotate node <master node name> update.node.deckhouse.io/disruption-approved=
   ```

1. Wait for the updated master node to switch to the `Ready` state.
   Repeat the steps for the next master node.

## How to use containerd with Nvidia GPU support?

1. Create a separate NodeGroup for GPU nodes:

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

1. Create NodeGroupConfiguration for the `gpu` NodeGroup for containerd configuration:

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
                   runtime_type = "io.containerd.runc.v2"
                   [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                     BinaryName = "/usr/bin/nvidia-container-runtime"
                     SystemdCgroup = false
       EOF
     nodeGroups:
     - gpu
     weight: 31
   ```

1. Add NodeGroupConfiguration to install Nvidia drivers on the `gpu` NodeGroup:
   - [Configuration example for Ubuntu](#ubuntu)
   - [Configuration example for CentOS](#centos)
1. Bootstrap and reboot the node.

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

    if [ ! -f "/etc/apt/sources.list.d/nvidia-container-toolkit.list" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
      curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    fi
    bb-apt-install nvidia-container-toolkit nvidia-driver-535-server
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

### CentOS

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

    if [ ! -f "/etc/yum.repos.d/nvidia-container-toolkit.repo" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    fi
    bb-dnf-install nvidia-container-toolkit nvidia-driver
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

### How to check if it was successful?

Create the `Job` named `nvidia-cuda-test` in the cluster:

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

Check the logs by running the following command:

```shell
d8 k logs job/nvidia-cuda-test
```

Example of the output:

```console
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

Create the `Job` named `gpu-operator-test` in the cluster:

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

Check the logs by running the following command:

```shell
d8 k logs job/gpu-operator-test
```

Example of the output:

```console
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## How to manually add several static nodes to the cluster?

Use the existing [NodeGroup](../../../../reference/cr/nodegroup.html) custom resource or create a new one.

Example of a NodeGroup resource named `worker`:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

You can automate the process of adding the nodes by using any configuration automation tool.
The following is an example using Ansible.

1. Find out a Kubernetes API server IP address.
   Note that this IP address must be accessible from the nodes you will be adding to the cluster.
   Run the following command:

   ```shell
   d8 k -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Check the Kubernetes version. If the version is higher than 1.25, create the `node-group` token:

   ```shell
   d8 k create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Create the token you receive and add it to the `token` field in the Ansible playbook in the following steps.

1. If the Kubernetes version is older than 1.25, get the Kubernetes API token for the ServiceAccount managed by Deckhouse:

   ```shell
   d8 k -n d8-cloud-instance-manager get $(d8 k -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

1. Create an Ansible playbook and replace the `vars` values with the data you received on previous steps.

   ```yaml
   - hosts: all
     become: yes
     gather_facts: no
     vars:
       kube_apiserver: <KUBE_APISERVER>
       token: <TOKEN>
     tasks:
       - name: # Check if the node is already bootstrapped
         stat:
           path: /var/lib/bashible
         register: bootstrapped
       - name: # Get the bootstrap secret
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

1. Define the additional `node_group` variable.
   The variable value must match the name of a NodeGroup that the node will be assigned to.
   There are several ways to pass the variable, for example, using the inventory file:

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

1. Run the playbook using the inventory file.

## How do I make werf ignore the Ready state in a node group?

[werf](https://werf.io) checks that the resources are in the `Ready` state.
If they are, it waits until the value becomes `True`.

Creating (updating) a NodeGroup resource in a cluster can take a significant amount of time to deploy the required number of nodes.
When deploying such a resource in a cluster using werf (for example, as part of a CI/CD process),
the deployment may terminate when resource readiness timeout is exceeded.

To make werf ignore the NodeGroup status, add the following NodeGroup annotations:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```
