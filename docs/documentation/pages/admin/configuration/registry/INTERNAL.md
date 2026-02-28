---
title: Working with container registry and revisions in a cluster fully managed by DKP
permalink: en/admin/configuration/registry/internal.html
description: "Configure container image registry in Deckhouse Kubernetes Platform. Image caching, storage optimization, and high availability registry management."
---

The ability to use registry is implemented by the [`registry`](/modules/registry/) module.

## Modes of operation with the registry

DKP implements the following modes of operation with registry:

- `Direct`: Provides direct access to an external registry via the fixed address `registry.d8-system.svc:5001/system/deckhouse`. This fixed address prevents DKP images from being re-downloaded and components from being restarted when registry parameters are changed. Switching between modes and registries is done through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The switching process is automatic (for more details, see the switching examples below) for more information. The architecture of the mode is described in the section ["Direct mode Architecture"](../../../architecture/registry-modes.html#direct-mode-architecture).

- `Proxy`: Using an internal caching proxy registry that accesses an external registry, with the caching proxy registry running on control-plane (master) nodes. This mode reduces the number of requests to the external registry by caching images. Cached data is stored on the control-plane (master) nodes. Access to the internal registry is via the fixed address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` mode. Switching between modes and registries is done through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The switching process is automatic (for more details, see the switching examples below) for more information. The architecture of the mode is described in the section [Proxy mode Architecture](../../../architecture/registry-modes.html#proxy-mode-architecture).

- `Local`: Using a local internal registry, with the registry running on control-plane (master) nodes. This mode allows the cluster to operate in an isolated environment. All data is stored on the control-plane (master) nodes. Access to the internal registry is via the fixed address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` and `Proxy` modes. Switching between modes and registries is done through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The switching process is automatic (for more details, see the switching examples below) for more information. The architecture of the mode is described in the section [Local mode Architecture](../../../architecture/registry-modes.html#local-mode-architecture).

- `Unmanaged`: Operation without using the internal registry. Access within the cluster is performed directly to the external registry.
  There are two types of the `Unmanaged` mode:
  - Configurable: A mode managed via the `registry` module. Switching between modes and registries is handled through the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry). The switch is performed automatically (for more details, see the switching examples below).
  - Non-configurable (deprecated): The default mode. Configuration parameters are set during [cluster installation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo) or [changed in a running cluster](/products/kubernetes-platform/documentation/v1/admin/configuration/registry/third-party.html) using the (deprecated) `helper change registry` command.

{% alert level="info" %}

- The `Direct` mode requires using the `Containerd` or `Containerd V2` CRI on all cluster nodes. For CRI setup, refer to the [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).
{% endalert %}

## Restrictions on working with the registry

Working with registry has a number of limitations and peculiarities related to installation, operating conditions, and mode switching.

Registry configuration via the `deckhouse` moduleConfig during DKP cluster bootstrap is not supported.

### Cluster installation limitations

The following restrictions apply when installing a cluster:

- DKP cluster bootstrap is only supported in the `Direct` and `Unmanaged` modes (`Local` and `Proxy` modes are not supported). Registry settings during cluster installation are configured via the [`deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry).
- To launch a cluster in the non-configurable `Unmanaged` mode (Legacy), registry parameters must be specified in [`initConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo).

### Operating conditions restrictions

To use the registry in DKP, the following conditions must be met:

- If CRI containerd or containerd v2 is used on the cluster nodes. To configure CRI, refer to the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) configuration.
- The cluster is fully managed by DKP. The module will not work in Managed Kubernetes clusters.
- The `Local` and `Proxy` modes are only supported on static clusters.

### Mode switching restrictions

Mode switching restrictions are as follows:

- Changing registry parameters and switching modes is only available after the bootstrap phase is fully complete.
- For the first switch, migration of user registry configurations must be performed. For more details, see the [Registry Module: FAQ](/modules/registry/faq.html) section.
- Switching to the non-configurable `Unmanaged` mode is only available from the `Unmanaged` mode. For more details, see the [Registry Module: FAQ](/modules/registry/faq.html) section.
- Switching between `Local` and `Proxy` modes is only possible via the intermediate `Direct` or `Unmanaged` modes. Example switching sequence: `Local`/`Proxy` → `Direct` → `Proxy`/`Local`.

## Examples of switching

{% alert level="warning" %}
If, during the switching process, the image of a module did not reload and the module did not reinstall, use the [instructions](../../../faq.html#what-should-i-do-if-the-module-image-did-not-download-and-the-mo) to resolve the issue.
{% endalert %}

### Switching to Direct Mode

To switch an already running cluster to `Direct` mode, follow these steps:

{% alert level="danger" %}
The first switch from `Unmanaged` to `Direct` mode will result in a full restart of all DKP components.
{% endalert %}

1. Before switching, perform the [migration to registry management format using the `registry` module](#migration-to-registry-management-format-using-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Direct` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-direct). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Direct
   target_mode: Direct
   ```

### Switching to the `Proxy` Mode

To switch an already running cluster to `Proxy` mode, follow these steps:

{% alert level="danger" %}

- The first switch from `Unmanaged` to `Proxy` mode will result in a full restart of all DKP components.
- Switching from `Local` mode to `Proxy` mode is not available. To switch from `Local` mode, you must switch the registry to another available mode (for example: `Direct`).
{% endalert %}

1. Before switching, perform the [migration to registry management format using the `registry` module](#migration-to-registry-management-format-using-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Proxy` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-proxy). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Proxy
         proxy:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Proxy
   target_mode: Proxy
   ```

### Switching to the `Local` mode

To switch an already running cluster to `Local` mode, follow these steps:

{% alert level="danger" %}

- The first switch from `Unmanaged` to `Local` mode will result in a full restart of all DKP components.
- Switching from `Proxy` mode to `Local` mode is not available. To switch from `Proxy` mode, you must switch the registry to another available mode (for example: `Direct`).
{% endalert %}

1. Before switching, perform the [migration to registry management format using the `registry` module](#migration-to-registry-management-format-using-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Make sure all master nodes are in the `Ready` state and do not have the `SchedulingDisabled` status, using the following command:

   ```bash
   d8 k get nodes
   ```

   Example output:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Example of output when the master node (`master-2` in the example) is in the `SchedulingDisabled` status:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Prepare archives with DKP images of the current version. To do this, use the `d8 mirror` command.

   Example:

   ```bash
   TAG=$(
    d8 k -n d8-system get deployment/deckhouse -o yaml \
    | yq -r '.spec.template.spec.containers[] | select(.name == "deckhouse").image | split(":")[-1]'
   ) && echo "TAG: $TAG"

   EDITION=$(
    d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller global values -o yaml \
    | yq .deckhouseEdition
   ) && echo "EDITION: $EDITION"
   ```

   ```bash
   d8 mirror pull \
   --license="<LICENSE_KEY>" \
   --source="registry.deckhouse.io/deckhouse/$EDITION" \
   --deckhouse-tag="$TAG" \
   /home/user/d8-bundle
   ```

1. Set the `Local` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-mode).

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Local
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status). In the status, you need to wait for the `RegistryContainsRequiredImages` check to start. The condition will show the absence or presence of images in the running local registry.

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: 0 of 166 items processed, 166 items with errors:
       - source: module/control-plane-manager/control-plane-manager133
         image: 10.128.0.5:5001/system/deckhouse@sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/cloud-provider-yandex/cloud-metrics-exporter
         image: 10.128.0.5:5001/system/deckhouse@sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0e28263da59edf11f0fad
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0ed86d6a1f465f4556fb8: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/control-plane-manager/kube-controller-manager132
         image: 10.128.0.5:5001/system/deckhouse@sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)

         ...and more
     reason: Processing
     status: "False"
     type: RegistryContainsRequiredImages
   ```

1. Upload the images to the local registry using the `d8 mirror` command. Image upload to the local registry is performed via Ingress at `registry.${PUBLIC_DOMAIN}`.

   Get the password for the read-write user of the local registry:

   ```bash
   $ d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   name: rw
   password: KFVxXZGuqKkkumPz
   passwordHash: $2a$10$Phjbr6iinLf00ZZDD2Y7O.p9H3nDOgYzFmpYKW5eydGvIsdaHQY0a
   ```

   Upload images to the local registry:

   ```bash
   d8 mirror push \
   --registry-login="rw" \
   --registry-password="KFVxXZGuqKkkumPz" \
   /home/user/d8-bundle \
   registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status). After uploading the images, the `RegistryContainsRequiredImages` status should be in the `Ready` state.

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: all 166 items are checked
     reason: Ready
     status: "True"
     type: RegistryContainsRequiredImages
   hash: ..
   mode: Direct
   target_mode: Local
   ```

1. Wait for the switch to complete. To check the switch status, use [this guide](#check-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Local
   target_mode: Local
   ```

### Switching to Unmanaged Mode

To switch an already running cluster to `Unmanaged` mode, follow these steps:

{% alert level="danger" %}
Changing the registry in `Unmanaged` mode will result in a full restart of all DKP components.
{% endalert %}

1. Before switching, perform the [migration to registry management format using the `registry` module](#migration-to-registry-management-format-using-the-registry-module).

1. Make sure the `registry` module is enabled and running. To do this, execute the following command:

   ```bash
   d8 k get module registry -o wide
   ```

   Example output:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Ensure the Deckhouse job queue is empty and contains no errors:

   ```shell
   d8 system queue list
   ```

   Example output:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Set the `Unmanaged` mode configuration in the [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). If you're using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/) module documentation for correct configuration.

   Configuration example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Check the registry switch status in the `registry-state` secret using [this guide](#check-registry-mode-switch-status).

   Example output:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. If you need to switch back to the old registry management method, refer to the [instruction](#migration-to-an-deprecated-registry-management-format-without-the-registry-module).

{% alert level="warning" %}
This is a deprecated format for registry management.
{% endalert %}

## Migration to registry management format using the registry module

During the migration, Containerd v1 will switch to the new registry configuration format.
Containerd v2 uses the new format by default. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry).

### For containerd v2

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` `moduleConfig`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify this configuration when setting up the `Unmanaged` mode:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. Wait for the switch to complete. Example [status output](#check-registry-mode-switch-status):

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

### For Containerd v1

{% alert level="danger" %}
- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the new registry configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Make sure that nodes with containerd v1 do not have any [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) located in the `/etc/containerd/conf.d` directory.

1. If configurations are present, you need to migrate to the new registry configuration format in containerd. To do this, add new configuration files to the `/etc/containerd/registry.d` directory. These configurations will take effect after switching to the `registry` module. To add configurations, prepare a `NodeGroupConfiguration`. For more details, see the section [with a description of configuration methods](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry). Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # The step can be arbitrary, as restarting the containerd service is not required
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

1. Apply the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Wait until the configuration files appear in the `/etc/containerd/registry.d` directory on all nodes.

1. Verify that the configurations are working correctly. To do this, use the following command:

   ```bash
   # For HTTPS:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # For HTTP:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Switch to using the `registry` module. To do this, specify the `Unmanaged` mode parameters in the `deckhouse` `moduleConfig`. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   You can view the current registry settings using the following command:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Specify this configuration when setting up the `Unmanaged` mode:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Replace with your license key
   ```

1. After applying, wait for the following message to appear in the [switch status](#check-registry-mode-switch-status):

   Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T15:22:34Z"
     message: |
       Check current nodes configuration
       2/2 node(s) Unready:
       - master-0: has custom toml merge containerd configuration
       - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
     reason: Processing
     status: "False"
     type: ContainerdConfigPreflightReady
   ```

   This message means that there are old registry configurations on the nodes located in the `/etc/containerd/conf.d` directory. The switch to the new containerd configuration is currently blocked. To allow the switch, you need to remove the old configuration files.

1. Remove the old configuration files to allow switching to the `registry` module. To do this, create a [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Example of a NodeGroupConfiguration manifest:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     # To add a file before the '032_configure_containerd.sh' step
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
  
       file="/etc/containerd/conf.d/old-config.toml"

       [ -f "$file" ] && rm -f "$file"
   ```

1. After removing the old configurations, make sure that the switch has resumed. Example of the [switch status](#check-registry-mode-switch-status):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Wait for the switch to complete. Example of the [switch status](#check-registry-mode-switch-status):

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Delete the [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) created in the step for deleting old configuration files:

   ```shell
   d8 k delete nodegroupconfiguration containerd-additional-config-auth-delete.sh
   ```

   To verify that NodeGroupConfiguration has been deleted, use the command:

   ```shell
   d8 k get nodegroupconfiguration
   ```

   The list should not contain the NodeGroupConfiguration to be deleted (for this example, `containerd-additional-config-auth-delete.sh`).

## Migration to an deprecated registry management format (without the registry module)

{% alert level="danger" %}

- This is a deprecated registry management format.
- During the switch, containerd v1 will be restarted.
- During the switch, containerd v1 will be migrated to the legacy registry configuration scheme.
- During the switch, [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) for containerd v1 will be temporarily unavailable.
{% endalert %}

1. Switch the registry to `Unmanaged` mode. If you are using a registry other than `registry.deckhouse.io`, refer to the [`deckhouse`](/modules/deckhouse/latest/configuration.html) module documentation for proper configuration.

   Example configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.io/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY>
   ```

1. Check the switch status using the [instruction](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Switch the registry to the non-configurable `Unmanaged` mode. Example configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
   ```

1. Check the switch status using the [instruction](#check-registry-mode-switch-status). Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. If containerd v1 is used and [custom registry configurations](/modules/node-manager/latest/faq.html#how-to-add-configuration-for-an-additional-registry) are applied in the cluster, they must be replaced with the old format. To do this, prepare the registry configurations in the old format. These configurations do not need to be applied at this stage. Example configuration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # To add a file before the '032_configure_containerd.sh' step
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

1. Delete the `registry-bashible-config` secret. This will trigger containerd v1 to switch back to the legacy registry format:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. After deletion, wait for the switch to complete. Use the [instruction](#check-registry-mode-switch-status) to track the progress. Example output:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. If containerd v1 is used, apply the previously prepared `NodeGroupConfiguration` with custom registry configurations.

1. Disable the `registry` module. Example:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: false
     settings: {}
     version: 1
   ```

## Check registry mode switch status

The status of the registry mode switch can be retrieved using the following command:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Example output:

```yaml
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.io: all 157 items are checked'
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2025-07-11T11:59:03Z"
    message: ""
    reason: ""
    status: "True"
    type: ContainerdConfigPreflightReady
  - lastTransitionTime: "2025-07-15T12:47:47Z"
    message: ""
    reason: ""
    status: "True"
    type: TransitionContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:52:48Z"
    message: ""
    reason: ""
    status: "True"
    type: InClusterProxyReady
  - lastTransitionTime: "2025-07-15T12:54:53Z"
    message: ""
    reason: ""
    status: "True"
    type: DeckhouseRegistrySwitchReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: FinalContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

The output displays the status of the switch process. Each condition can have a status of `True` or `False`, and may contain a `message` field with additional details.

Description of conditions:

| Condition                         | Description                                                                                                                                                                                                                |
| --------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerdConfigPreflightReady`  | State of the containerd configuration preflight check. Verifies there are no custom containerd auth configurations on the nodes.                                                                                           |
| `TransitionContainerdConfigReady` | State of preparing the containerd configuration for the new mode. Verifies that the configuration contains both the old and new mode settings.                                                                             |
| `FinalContainerdConfigReady`      | State of finalizing the switch to the new containerd mode. Verifies that the containerd configuration has been successfully applied and contains only the new mode settings.                                               |
| `DeckhouseRegistrySwitchReady`    | State of switching Deckhouse and its components to use the new registry. `True` means Deckhouse successfully switched and is ready to operate.                                                                             |
| `InClusterProxyReady`             | State of In-Cluster Proxy readiness. Checks that the In-Cluster Proxy has started successfully and is running.                                                                                                             |
| `CleanupInClusterProxy`           | State of cleaning up the In-Cluster Proxy if it is not needed in the selected mode. Verifies that all related resources have been removed.                                                                                 |
| `NodeServicesReady`               | State of Node Services Manager and Static-Pod registry readiness. Verifies that the Node Services Manager is successfully launched and operational, and that the Static-Pod registry has been successfully deployed by it. |
| `CleanupNodeServices`             | State of cleaning up the Node Services Manager and Static-Pod registry if they are not needed in the selected mode. Verifies that all related resources have been removed.                                                 |
| `RegistryContainsRequiredImages`  | State of checking the registry for the presence of required images.                                                                                                                                                        |
| `Ready`                           | Overall state of registry readiness in the selected mode. Indicates that all other conditions are met and the `modul`e is ready to operate.                                                                                |
