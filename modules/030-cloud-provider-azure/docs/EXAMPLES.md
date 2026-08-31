---
title: "Cloud provider — Azure: examples"
---

## An example of the `AzureInstanceClass` custom resource

Below is a simple example of the `AzureInstanceClass` custom resource:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
```

## Configuring security policies on nodes

Default Network Security Group rules are described in the [layouts](layouts.html#network-security-group) section. You cannot attach a pre-created custom NSG to nodes via module parameters — only [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) is available to restrict SSH.
