---
title: Layouts and configuration in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/layout.html
---

Deckhouse Kubernetes Platform supports two layouts in the Basis Dynamix cloud:

- Standard — a layout that uses only an external network;
- StandardWithInternalNetwork — a layout with an internal (private) network and DNS servers.

Both layouts let you manage cluster node placement, network settings, the OS image, and data storage.

## Standard

![resources](../../../../images/cloud-provider-dynamix/dynamix-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11150&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
location: dynamix
account: acc_user
storagePolicy: storage_policy01
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    externalNetwork: "<external network>"
```

## StandardWithInternalNetwork

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "<SSH_PUBLIC_KEY>"
location: dynamix
account: acc_user
storagePolicy: storage_policy01
nodeNetworkCIDR: "10.241.32.0/24"
nameservers:
  - "10.0.0.10"
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    externalNetwork: "<external network>"
```

## Required parameters

- `sshPublicKey` — a public key for accessing the nodes;
- `location` — the name of the grid used to resolve the resource group the cluster is created in (has no effect on disk placement);
- `account` — the account name in the cloud;
- `storagePolicy` — the name of the storage policy used by default for VM disks;
- `provider.controllerUrl`, `oAuth2Url`, `appId`, `appSecret` — API access parameters;
- `imageName` — the OS image name;
- `externalNetwork` — the external network name;
- `nodeNetworkCIDR` and `nameservers` — internal network parameters (only for the StandardWithInternalNetwork layout).

After changing the parameters, run `dhctl converge` for the changes to take effect.
