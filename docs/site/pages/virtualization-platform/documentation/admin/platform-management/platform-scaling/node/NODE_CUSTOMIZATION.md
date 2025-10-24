---
title: "Custom node configuration"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/node-customization.html
---

To automate actions on group nodes, use the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource. It allows you to run bash scripts on the nodes using the [Bash Booster](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bashbooster) command set, as well as apply the [Go Template](https://pkg.go.dev/text/template) templating engine. This is useful for automating operations such as:

- Installing and configuring additional OS packages.  

  Examples:  
  - [Installing the kubectl plugin](node-management.html#installing-the-cert-manager-plugin-for-kubectl-on-master-nodes)

- Updating the OS kernel to a specific version.

- Modifying OS parameters.

- Collecting information on the node and performing similar tasks.

The NodeGroupConfiguration resource allows you to define execution priority for scripts, limit execution to specific node groups or OS types.

The script code is specified in the `content` field of the resource. When a script is created on a node, the content passes through the [Go Template](https://pkg.go.dev/text/template) templating engine, which adds a layer of logic to script generation. A dynamic context with a set of variables is available in the template.

Available template variables include:
<ul>
<li><code>.cloudProvider</code> (for node groups with <code>CloudEphemeral</code> or <code>CloudPermanent</code> nodeType) — an array of cloud provider data.
{% offtopic title="Example data..." %}
```yaml
cloudProvider:
  instanceClassKind: OpenStackInstanceClass
  machineClassKind: OpenStackMachineClass
  openstack:
    connection:
      authURL: https://cloud.provider.com/v3/
      domainName: Default
      password: p@ssw0rd
      region: region2
      tenantName: mytenantname
      username: mytenantusername
    externalNetworkNames:
    - public
    instances:
      imageName: ubuntu-22-04-cloud-amd64
      mainNetwork: kube
      securityGroups:
      - kube
      sshKeyPairName: kube
    internalNetworkNames:
    - kube
    podNetworkMode: DirectRoutingWithPortSecurityEnabled
  region: region2
  type: openstack
  zones:
  - nova
```
{% endofftopic %}
</li>
<li><code>.cri</code> — the container runtime interface in use (since Deckhouse version 1.49, only <code>Containerd</code> is used).</li>
<li><code>.kubernetesVersion</code> — the version of Kubernetes in use.</li>
<li><code>.nodeUsers</code> — an array of user data added to the node using the <a href="/modules/node-manager/cr.html#nodeuser">NodeUser</a> resource.
{% offtopic title="Example data..." %}
```yaml
nodeUsers:
- name: user1
  spec:
    isSudoer: true
    nodeGroups:
    - '*'
    passwordHash: PASSWORD_HASH
    sshPublicKey: SSH_PUBLIC_KEY
    uid: 1050
```
{% endofftopic %}
</li>
<li><code>.nodeGroup</code> — an array of NodeGroup data.
{% offtopic title="Example data..." %}
```yaml
nodeGroup:
  cri:
    type: Containerd
  disruptions:
    approvalMode: Automatic
  kubelet:
    containerLogMaxFiles: 4
    containerLogMaxSize: 50Mi
    resourceReservation:
      mode: "Off"
  kubernetesVersion: "1.29"
  manualRolloutID: ""
  name: master
  nodeTemplate:
    labels:
      node-role.kubernetes.io/control-plane: ""
      node-role.kubernetes.io/master: ""
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  nodeType: CloudPermanent
  updateEpoch: "1699879470"
```
{% endofftopic %}</li>
</ul>

{% raw %}
Example of using variables in the template engine:

```shell
{{- range .nodeUsers }}
echo 'Tuning environment for user {{ .name }}'
# Some code for tuning user environment
{{- end }}
```

Example of using Bash Booster commands:

```shell
bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  bb-log-info "Setting reboot flag due to kernel was updated"
  bb-flag-set reboot
}
```

{% endraw %}

## Monitoring script execution

You can view the script execution log on a node in the `bashible` service log using the following command:

```bash
journalctl -u bashible.service
```

The scripts are located in the `/var/lib/bashible/bundle_steps/` directory on the node.

## Script re-execution mechanism

The service decides whether to re-run the scripts by comparing a unified checksum of all files, located at `/var/lib/bashible/configuration_checksum`, with the checksum stored in the `configuration-checksums` secret in the `d8-cloud-instance-manager` namespace in the Kubernetes cluster.

You can check the checksum with the following command:

```bash
d8 k -n d8-cloud-instance-manager get secret configuration-checksums -o yaml
```

The checksum comparison is performed by the service every minute.

The checksum in the cluster is updated every 4 hours, thereby re-triggering the execution of scripts on all nodes.

To manually trigger `bashible` execution on a node, you can delete the checksum file using the following command:

```bash
rm /var/lib/bashible/configuration_checksum
```

## Script writing specifics

When writing scripts, it's important to consider the following features of their usage in Deckhouse:

1. Scripts in DVP are executed every 4 hours or based on external triggers. Therefore, it's important to write scripts in a way that they first check whether changes are necessary, to avoid repeated or unnecessary actions on each execution.
1. There are [predefined scripts](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all) that perform various actions, including service installation and configuration. It's important to consider this when assigning priority to custom scripts. For example, if a custom script restarts a service, it must run after the script that installs that service. Otherwise, the custom script won't be able to run during the initial provisioning of the node (since the service won’t be installed yet).

Useful specifics of certain scripts:

* [`032_configure_containerd.sh`](https://github.com/deckhouse/deckhouse/blob/main/candi/bashible/common-steps/all/032_configure_containerd.sh.tpl): Merges all `containerd` service configuration files located in `/etc/containerd/conf.d/*.toml`, and also **restarts** the service. Note that the `/etc/containerd/conf.d/` directory is not created automatically, and any configuration files in it should be created by scripts with a priority lower than `32`.

## How to use containerd with Nvidia GPU support

You need to create a separate [NodeGroup](/modules/node-manager/cr.html#nodegroup) for GPU nodes:

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

Next, create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) resource for the `gpu` NodeGroup to configure `containerd`:

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

Add a NodeGroupConfiguration resource to install Nvidia drivers for the `gpu` NodeGroup.

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

After the configurations are applied, bootstrap and reboot the nodes to apply the settings and install the drivers.

### Verifying successful installation

Create the following Job in your cluster:

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

Check the logs with the following command:

```shell
d8 k logs job/nvidia-cuda-test
```

Example output:

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

Create the following Job in your cluster:

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

Check the logs with the following command:

```shell
d8 k logs job/gpu-operator-test
```

Example output:

```console
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## How to deploy a custom containerd configuration file

{% alert level="danger" %}
Adding custom settings will trigger a restart of the `containerd` service.
{% endalert %}

`bashible` on the nodes merges the DVP `containerd` configuration with configurations from `/etc/containerd/conf.d/*.toml`.

{% alert level="warning" %}
You can override the parameter values defined in the `/etc/containerd/deckhouse.toml` file. However, you are responsible for ensuring the correct operation of such changes. It is recommended **not to modify the configuration** on control plane (master) nodes (NodeGroup `master`).
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

## Adding a configuration for an additional registry

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

### Old Method

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

#### Adding authorization to an additional registry (deprecated method)

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

#### Configuring a certificate for an additional registry (deprecated method)

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
In addition to containerd, the certificate can be added to the OS.
{% endalert %}

#### Adding TLS skip verify (deprecated method)

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

#### Configuring a mirror for access to public registries (deprecated method)

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

### New Method

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
└── registry.deckhouse.ru
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

#### Adding authorization to an additional registry (actual method)

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

#### Configuring a certificate for an additional registry (actual method)

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
In addition to containerd, the certificate can be added into the OS.
{% endalert %}

#### Adding TLS skip verify (actual method)

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

#### Configuring a mirror for access to public registries (actual method)

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

## How to automatically put custom labels on the node

1. On the node, create the directory `/var/lib/node_labels`.

1. In this directory, create a single of multiple files containing the necessary labels. There can be any number of files, as well as the number of subdirectories containing them.

1. Add the necessary labels to the files in the `key=value` format. For example:

   ```console
   example-label=test
   ```

1. Save the files.

When adding a node to the cluster, the labels specified in the files will be automatically affixed to the node.

{% alert level="warning" %}
Note that it is not possible to add labels used in DVP in this way. This method will only work with custom labels that do not overlap with those reserved for Deckhouse.
{% endalert %}
