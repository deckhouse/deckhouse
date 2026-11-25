---
title: Node management in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/node-management.html
lang: en
---

Machine provisioning and parameters are configured in the [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resource,
where the [instance class](../../../../glossary.html#instance-class) for the [node group](../../../../glossary.html#node-group) is specified (the [`cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter).
For GCP, the instance class is a [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) custom resource that defines the machine parameters.

## Enabling nested virtualization

To run virtual machine workloads (e.g., KVM-based VMs) inside GCP instances, enable nested virtualization.

{% alert %}
Only specific machine types support nested virtualization. See [the GCP documentation](https://cloud.google.com/compute/docs/instances/nested-virtualization/overview#supported_machine_types) for the list of compatible types.
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: vm-nodes
spec:
  machineType: n2-standard-8
  nestedVirtualization: true
```

## Adding CloudStatic nodes to the cluster

For virtual machines that will be nodes in the cluster, set a `Network Tag` matching the cluster prefix.

You can find the cluster prefix by running the following command:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of [ClusterConfiguration](../../../../glossary.html#cluster-configuration) (the prefix has moved to the ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```

## Preemptible nodes

Preemptible virtual machines in Google Cloud are called preemptible instances. To make a [node group](../../../../glossary.html#node-group) order them, set the [`preemptible`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-preemptible) parameter of the GCPInstanceClass resource to `true`.

How to create such nodes and how they behave is described in ["How to create preemptible nodes in GCP?"](/modules/cloud-provider-gcp/faq.html#how-to-create-preemptible-nodes-in-gcp).

## Replacing the OS image

The OS image of the nodes is set by the [`image`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-image) parameter of the GCPInstanceClass resource for `CloudEphemeral` nodes and in the `instanceClass` section of the GCPClusterConfiguration resource for `CloudPermanent` nodes and master nodes. To move the cluster nodes to a new image, follow the steps from ["How do I replace the OS image in a cloud cluster?"](../../../../faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
