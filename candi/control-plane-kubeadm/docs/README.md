---
title: Control Plane Kubeadm
---

Go templates to render a Kubernetes bootstrap configuration.
These templates are used during first master node creation and in the [control-plane-manager]({{"/modules/040-control-plane-manager/" | true_relative_url }} ).

* `config.yaml.tpl` - main config
* `patches/` - control plane components patches

### How to render control-plane-kubeadm?

Kubeadm config file compilation is possible with using `dhctl` tool.

```bash
dhctl render kubeadm-config --config=/config.yaml
```

Example `config.yaml`:

```yaml
apiVersion: deckhouse.io/v1
kind: KubeadmConfigTemplateData
clusterConfiguration:
  cloud:
    prefix: pivot
    provider: OpenStack
  clusterDomain: cluster.local
  clusterType: Cloud
  kubernetesVersion: "1.27"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
nodeIP: 192.168.199.161
extraArgs: {}
```
