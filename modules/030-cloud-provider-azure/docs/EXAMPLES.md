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

## Replacing the OS image

In this example, the nodes of a group are moved to a new OS image. The new image is set in the [`urn`](cr.html#azureinstanceclass-v1-spec-urn) parameter of the instance class that the node group refers to:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureInstanceClass
metadata:
  name: example
spec:
  machineSize: Standard_F4
  urn: Canonical:ubuntu-24_04-lts:server:24.04.202602040
```

After the instance class is changed, Deckhouse Platform recreates the `CloudEphemeral` nodes from the new image. For `CloudPermanent` nodes and master nodes, the image is set in the `instanceClass` section of the AzureClusterConfiguration resource. The replacement procedure is described in ["How do I replace the OS image in a cloud cluster?"](/products/kubernetes-platform/documentation/v1/faq.html#how-do-i-replace-the-os-image-in-a-cloud-cluster).
