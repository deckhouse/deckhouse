---
title: "The kube-proxy module"
description: "Managing kube-proxy components on a Deckhouse Kubernetes Platform cluster node."
---

The module deletes the entire `kubeadm` kube-proxy set  (`DaemonSet`, `ConfigMap`, `RBAC`) and installs its own.

For security reasons, for NodePort services, connections are only allowed to the nodes' InternalIP by default. You can lift this restriction using the `node.deckhouse.io/nodeport-bind-internal-ip: "false"` annotation.

Here is an example of a NodeGroup annotation:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: myng
spec:
  nodeTemplate:
    annotations:
      node.deckhouse.io/nodeport-bind-internal-ip: "false"
...
```

> **Note!** Following the addition, deletion, or changing the annotation, you have to restart kube-proxy Pods manually.
>
> **Note!** The kube-proxy module is automatically disabled when the [cni-cilium](../cni-cilium/) module is enabled.
