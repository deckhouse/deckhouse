---
title: "Cloud provider — GCP: FAQ"
---

## How do I create a cluster?

1. Set up a cloud environment.
2. Enable the module or pass the `--extra-config-map-data base64_encoding_of_custom_config` flag with the [module parameters](configuration.html) to the `install.sh` script.
3. Create one or more [GCPInstanceClass](cr.html#gcpinstanceclass) custom resources.
4. Create one or more [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resources for managing the number and the process of provisioning machines in the cloud.

## Adding CloudStatic nodes to a cluster

For the VMs, you want to add to a cluster as nodes, add a `Network Tag` similar to the cluster prefix.

You can find out `prefix` using the command:

```shell
d8 k get mc global -o jsonpath='{.spec.settings.prefix}{"\n"}'
```

If the value is empty, get it from the deprecated [`cloud.prefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-cloud-prefix) parameter of ClusterConfiguration (the prefix has moved to the ModuleConfig `global`):

```shell
d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' \
  | base64 -d | grep prefix
```

## How to create preemptible nodes in GCP?

Preemptible virtual machines in Google Cloud are called preemptible instances. To make a node group order them, set the [`preemptible`](cr.html#gcpinstanceclass-v1-spec-preemptible) parameter of the GCPInstanceClass resource to `true`.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPInstanceClass
metadata:
  name: worker-preempt
spec:
  machineType: n1-standard-4
  diskSizeGb: 30
  preemptible: true
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-preempt
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: GCPInstanceClass
      name: worker-preempt
    minPerZone: 2
    maxPerZone: 4
    zones:
      - europe-west4-a
```

Along with this parameter, Deckhouse Platform (DP) disables the automatic restart of the virtual machine and makes it stop during host maintenance.

### Limitations of preemptible nodes in GCP

Consider the following limitations:

- The `preemptible` parameter applies only to CloudEphemeral node groups, because GCPInstanceClass applies only to them.
- DP does not track Google Cloud notices about an upcoming preemption and does not drain the node in advance. When Google Cloud stops the instance, the node switches to the `NotReady` state, and the group orders a new node in place of the lost one. Until then, the pods of the stopped node may appear in the Kubernetes API with the `Running` status even though they are no longer available.
- Place only the workload that tolerates a sudden node loss on preemptible nodes.

{% alert level="warning" %}
Changing the [`preemptible`](cr.html#gcpinstanceclass-v1-spec-preemptible) parameter in an existing GCPInstanceClass causes all nodes of the corresponding group to be recreated.
{% endalert %}
