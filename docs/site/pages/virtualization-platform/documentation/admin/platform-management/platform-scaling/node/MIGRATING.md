---
title: "Migrating container runtime to containerd v2"
permalink: en/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/migrating.html
lang: en
---

You can configure containerd v2 as the primary container runtime either at the cluster level or for specific node groups. This runtime option enables the use of cgroups v2, provides improved security, and allows more flexible resource management.

## Requirements

Migration to containerd v2 is possible under the following conditions:

- Nodes meet the requirements described in the [cluster-wide parameters](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
- There are no custom configurations on the server in `/etc/containerd/conf.d` ([example of a custom configuration](/modules/node-manager/faq.html#how-to-use-containerd-with-nvidia-gpu-support)).

If any of the requirements described in the [general cluster parameters](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) are not met, Deckhouse Virtualization Platform adds the label `node.deckhouse.io/containerd-v2-unsupported` to the node. If the node has custom configurations in `/etc/containerd/conf.d`, the label `node.deckhouse.io/containerd-config=custom` is added to it.

If one of these labels is present, changing the [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) parameter for the node group will be unavailable. Nodes that do not meet the migration conditions can be viewed using the following commands:

```shell
d8 k get node -l node.deckhouse.io/containerd-v2-unsupported
d8 k get node -l node.deckhouse.io/containerd-config=custom
```

Additionally, a administrator can verify if a specific node meets the requirements using the following commands:

```shell
uname -r | cut -d- -f1
stat -f -c %T /sys/fs/cgroup
systemctl --version | awk 'NR==1{print $2}'
modprobe -qn erofs && echo "TRUE" || echo "FALSE"
ls -l /etc/containerd/conf.d
```

## How to enable containerd v2

You can enable containerd v2 in two ways:

1. **For the entire cluster**. Set the value `ContainerdV2` for the [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) parameter in the `ClusterConfiguration` resource. This value will apply to all [NodeGroup](/modules/node-manager/cr.html#nodegroup) objects where [`spec.cri.type`](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) is not explicitly defined.

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

When migrating to containerd v2 Deckhouse Kubernetes Platform will begin sequentially updating the nodes. Updating a node results in the disruption of the workload hosted on it (disruptive update). The node update process is managed by the parameters for applying disruptive updates to the node group ([spec.disruptions.approvalMode](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions-approvalmode)).

{% alert level="info" %}
At migration process the folder `/var/lib/containerd` will be cleared, causing all pod images to be re-downloaded, and the node will reboot.
{% endalert %}
