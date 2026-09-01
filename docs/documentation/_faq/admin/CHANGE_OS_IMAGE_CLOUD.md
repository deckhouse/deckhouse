---
title: How to replace an OS image in a cloud cluster?
subsystems:
  - cluster_infrastructure
lang: en
---

Replacing an OS image in a cloud cluster works the same way for all providers: change the image reference in the DKP configuration. After that, nodes are reordered by the same mechanism that created them — Terraform, MCM, or CAPI.

Where to change the configuration:

* for [CloudEphemeral](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#adding-cloudephemeral-nodes-in-a-cloud-cluster) — in the `<PROVIDER>InstanceClass` resource;
* for [CloudPermanent](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/cloud-node.html#adding-cloudpermanent-nodes-to-a-cloud-cluster) and master nodes — in `<PROVIDER>ClusterConfiguration`, in the `instanceClass` of the corresponding node group.

If the provider has already been migrated to [ModuleConfig](/products/kubernetes-platform/documentation/v1/faq.html#how-to-migrate-a-cloud-provider-to-moduleconfig-based-configurat)-based configuration (for example, DVP), VM parameters are set via `<PROVIDER>InstanceClass` for both ephemeral and permanent nodes.

The image field name and type depend on the provider and infrastructure and are not unified. For example:

* in VCD it is a `template` string;
* in DVP it is an `image` object with `kind` and `name`.

See the cloud provider documentation for the field you need.

## Replacement procedure

1. Create a **new** image or template in the cloud. If images are versioned, include the version or date in the name, for example `ubuntu-24-04-20260110` → `ubuntu-24-04-20260204`.
1. Set the new image in the DKP configuration.

   Example for VMware Cloud Director (`VCDInstanceClass`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: VCDInstanceClass
   metadata:
     name: worker
   spec:
     template: Templates/ubuntu-24-04-20260204
   ```

   Example for DVP (`DVPInstanceClass`):

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: DVPInstanceClass
   metadata:
     name: worker
   spec:
     rootDisk:
       image:
         kind: ClusterVirtualImage
         name: ubuntu-24-04-20260204
   ```

   For nodes managed via `<PROVIDER>ClusterConfiguration`, change the image field in the `instanceClass` of the required group. You can edit the configuration with:

   ```bash
   d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller edit provider-cluster-configuration
   ```

1. Wait for nodes to be reordered (Terraform / MCM / CAPI). If the new image exists and meets DKP requirements, no further action is needed.

For master nodes, if needed, use the separate instructions: [multi-master](/modules/control-plane-manager/faq.html#how-do-i-switch-to-a-different-os-image-in-a-multi-master-cluster) and [single-master](/modules/control-plane-manager/faq.html#how-do-i-switch-to-a-different-os-image-in-a-single-master-cluster).

## What to watch out for

{% alert level="warning" %}
If the image is referenced by name, the name in the configuration **must change**. Deleting the old image and creating a new one with the same name does not reorder nodes: DKP reacts to a configuration change, not to the content of the cloud resource.
{% endalert %}

* The new image must exist in the cloud by the time nodes are reordered.
* The new image must meet DKP requirements for VM images (including cloud-init support) — the same as when preparing an image initially.
