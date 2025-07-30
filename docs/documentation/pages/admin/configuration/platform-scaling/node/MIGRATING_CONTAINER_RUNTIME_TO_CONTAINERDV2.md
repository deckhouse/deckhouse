---
title: "Migrating container runtime to ContainerdV2"
permalink: en/admin/configuration/platform-scaling/node/migrating.html
lang: en
---

You can configure `ContainerdV2` as the primary container runtime either at the cluster level or for specific node groups. This runtime option enables the use of cgroups v2, provides improved security, and allows more flexible resource management.

## Requirements

Migration to `ContainerdV2` is possible under the following conditions:

- Nodes meet the requirements described in the [cluster-wide parameters](/installing/configuration.html#clusterconfiguration-defaultcri).
- There are no custom configurations on the server in `/etc/containerd/conf.d` ([example of a custom configuration](/modules/node-manager/faq.html#how-to-use-containerd-with-nvidia-gpu-support)).

## How to enable ContainerdV2

You can enable `ContainerdV2` in two ways:

1. **For the entire cluster**. Set the value `ContainerdV2` for the [`defaultCRI`](/installing/configuration.html#clusterconfiguration-defaultcri) parameter in the `ClusterConfiguration` resource. This value will apply to all [NodeGroup](/modules/node-manager/cr.html#nodegroup) objects where [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) is not explicitly defined.

   Example:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: ClusterConfiguration
   ...
   defaultCRI: ContainerdV2
   ```

1. **For a specific node group**. Set `ContainerdV2` in the [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter of the [NodeGroup](/modules/node-manager/cr.html#nodegroup) object.

   Example:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     cri:
       type: ContainerdV2
   ```

When migrating to `ContainerdV2`:

- The `/var/lib/containerd` directory, where Containerd stores its data, is cleared.
- `ContainerdV2` uses a separate configuration directory: `/etc/containerd/conf2.d` instead of `/etc/containerd/conf.d`.

This means that when `ContainerdV2` is enabled, all previous Containerd configurations are ignored, and the node starts using an isolated settings structure and data directory.
