---
title: Integration with Basis Dynamix cloud services
permalink: en/admin/integrations/private/dynamix/services.html
---

Deckhouse Kubernetes Platform integrates with the Basis Dynamix cloud platform and uses [DynamixInstanceClass resources](/modules/cloud-provider-dynamix/cr.html#dynamixinstanceclass) to describe the characteristics of virtual machines deployed in the cluster.

## Key capabilities

- Ordering and removing virtual machines via the Basis Dynamix API;
- Configuring VM parameters, including the number of CPUs, memory size, and root disk size;
- Specifying the OS template (image name) and the storage policy used for disk placement;
- Connecting to external networks;
- Using multiple node groups with individual parameters.

Example DynamixInstanceClass:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixInstanceClass
metadata:
  name: frontend
spec:
  numCPUs: 4
  memory: 8192
  rootDiskSizeGb: 40
  imageName: alt-p10-cloud-x86_64.img
  storagePolicy: storage_policy01
  externalNetwork: extnet_vlan_1700
```

It is referenced by the NodeGroup's [`cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter.

## Recommendations

- Place OS images in the "Images" → "Template images" section of the Basis Dynamix portal.
- Use image names that exactly match the `imageName` values in DynamixInstanceClass.
- Make sure the selected storage policy is available (`status: ENABLED`) in the account for all nodes placed in the cluster.
- Verify that virtual machines have access to the internet and to DNS servers.

Cloud integration provides automatic scaling, configuration, and node management according to the parameters set in DynamixInstanceClass and the cluster configuration.
