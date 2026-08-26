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

### Default Network Security Group

The module creates a Network Security Group (NSG) named after the cluster prefix and associates it with the node subnet. By default the NSG contains:

* `AllowIcmp` — inbound ICMP from any source;
* `AllowSsh` — inbound TCP/22 from CIDRs in [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) (from any source if the list is not set).

The module does not add extra ports (for example, HTTP/HTTPS) to this NSG. Configure them manually in Azure (NSG rules or a separate NSG). You cannot attach a pre-created custom NSG to nodes via module parameters — only [`sshAllowList`](cluster_configuration.html#azureclusterconfiguration-sshallowlist) is available to restrict SSH.
