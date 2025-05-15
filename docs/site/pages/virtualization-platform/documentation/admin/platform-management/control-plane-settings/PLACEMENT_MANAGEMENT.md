---
title: "Components placement"
permalink: en/virtualization-platform/documentation/admin/platform-management/control-plane-settings/placement-management.html
---

## Placement strategies

There are 3 placement strategies for virtualization management components: `master`, `system`, `any-node`.

### master

The components are placed on master nodes. These are components that implement APIService, or components that run the Validating webhook or Mutating webhook.

### system

By default, components with this strategy are placed on master nodes.

However, by creating a NodeGroup `system` or `virtualization`, you can remove the load from master nodes and move virtualization management components to dedicated nodes.

### any-node

This is a tolerations set that permits the component to run on any node in the cluster.

## Nodes for the system strategy

To allocate nodes for the `system` strategy, you need to create a NodeGroup `system` and add nodes to it.

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
name: system
spec:
nodeTemplate:
labels:
node-role.deckhouse.io/system: ""
taints:
- effect: NoExecute
key: dedicated.deckhouse.io
value: system
nodeType: Static
EOF
```

For a node to be added to the NodeGroup `system`, its StaticInstance must have the label `node-role.deckhouse.io/system` (for more details, see the section on adding a node [using CAPS and label selector](../node-management/adding-node.html#caps-with-label-selector)).

For example:

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: StaticInstance
metadata:
name: system-1
labels:
node-role.deckhouse.io/system: ""
spec:
address: "<SERVER-SYSTEM-IP1>"
credentialsRef:
kind: SSHCredentials
name: system-1-credentials
EOF
```

The `system` strategy is used by other platform components, such as Prometheus. Therefore, when system nodes are created, some platform components will be move to them.
To allocate nodes for virtualization components, you need to create a NodeGroup `virtualization`; platform components will not be placed on the nodes of this group.

```shell
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
name: virtualization
spec:
nodeTemplate:
labels:
node-role.deckhouse.io/virtualization: ""
taints:
- effect: NoExecute
key: dedicated.deckhouse.io
value: virtualization
nodeType: Static
EOF
```

<!-- ## Limiting virtual machine placement

TODO stub about limitations for virt-handler.-->
