---
title: "Cloud provider — Yandex.Cloud: Layouts"
---

## Layouts
### Standard

In this placement strategy, nodes do not have public IP addresses allocated to them; they use Yandex.Cloud NAT to connect to the Internet.

> ⚠️ Caution! The Yandex.Cloud NAT feature is at the [Preview stage](https://cloud.yandex.com/en/docs/vpc/operations/enable-nat) as of July 2021.  To enable the Cloud NAT feature for your cloud, you need to contact Yandex.Cloud support in advance (in a week or so) and request access to it.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTSpvzjcEBpD1qad9u_UgvsOrYT_Xtnxwg6Pzb64HQHLqQWcZi6hhCNRPKVUdYKX32nXEVJeCzACVRG/pub?w=812&h=655)
<!--- Source: https://docs.google.com/drawings/d/1WI8tu-QZYcz3DvYBNlZG4s5OKQ9JKyna7ESHjnjuCVQ/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: dsafsafewf
  folderID: enh1233214367
  serviceAccountJSON: |
    {"test": "test"}
masterNodeGroup:
  replicas: 1
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    additionalLabels:
      takes: priority
nodeGroups:
- name: khm
  replicas: 1
  zones:
  - ru-central1-a
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    additionalLabels:
      toy: example
labels:
  billing: prod
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: tewt243tewsdf
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 213.177.96.1
  - 231.177.97.1
```

#### Enabling Cloud NAT

**Caution!** Note that you must manually (using the web interface) enable Cloud NAT within 3 minutes after creating the primary network resources. The bootstrap process won't complete if you fail to do this.

![Enabling NAT](../../images/030-cloud-provider-yandex/enable_cloud_nat.png)

### WithoutNAT

In this layout, NAT (of any kind) is not used, and each node is assigned a public IP.

**Caution!** Currently, the cloud-provider-yandex module does not support Security Groups; thus, is why all cluster nodes connect directly to the Internet.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTgwXWsNX6CKCRaMf5t6rl3kpKQQFHK6T8Dsg1jAwAwYaN1MRbxKFsSFQHeo1N3Qec4etPpeA0guB6-/pub?w=812&h=655)
<!--- Source: https://docs.google.com/drawings/d/1I7M9DquzLNu-aTjqLx1_6ZexPckL__-501Mt393W1fw/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
provider:
  cloudID: dsafsafewf
  folderID: enh1233214367
  serviceAccountJSON: |
    {"test": "test"}
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    zones:
    - ru-central1-a
    - ru-central1-b
nodeGroups:
- name: khm
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    zones:
    - ru-central1-a
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: tewt243tewsdf
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 213.177.96.1
  - 231.177.97.1
```

### WithNATInstance

In this placement strategy, Deckhouse creates a NAT instance and adds a rule to a route table containing a route to 0.0.0.0/0 with a NAT instance as the next hop.

If the `withNATInstance.externalSubnetID` parameter is set, the NAT instance will be created in this subnet.
IF the `withNATInstance.externalSubnetID` parameter is not set and `withNATInstance.internalSubnetID` is set, the NAT instance will be created in this last subnet.
If neither `withNATInstance.externalSubnetID` nor `withNATInstance.internalSubnetID` is set, the NAT instance will be created in the  `ru-central1-c` zone.

**Caution!** Note that you must manually enter the route to the created NAT instance within 3 minutes after creating the primary network resources. The bootstrap process won't complete if you fail to do this.

```text
$ yc compute instance list | grep nat
| ef378c62hvqi075cp57j | kube-yc-nat | ru-central1-c | RUNNING | 130.193.44.28   | 192.168.178.22 |

$ yc vpc route-table update --name kube-yc --route destination=0.0.0.0/0,next-hop=192.168.178.22
```

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSnNqebgRdwGP8lhKMJfrn5c0QXDpe9YdmIlK4eDberysLLgYiKNuwaPLHcyQhJigvQ21SANH89uipE/pub?w=812&h=655)
<!--- Source: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
withNATInstance:
  natInstanceExternalAddress: 30.11.34.45
  internalSubnetID: sjfwefasjdfadsfj
  externalSubnetID: etasjflsjdfiorej
provider:
  cloudID: dsafsafewf
  folderID: enh1233214367
  serviceAccountJSON: |
    {"test": "test"}
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    zones:
    - ru-central1-a
    - ru-central1-b
nodeGroups:
- name: khm
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: tewt243tewsdf
    zones:
    - ru-central1-a
sshPublicKey: "ssh-rsa ewasfef3wqefwefqf43qgqwfsd"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: tewt243tewsdf
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 213.177.96.1
  - 231.177.97.1
```

## YandexClusterConfiguration
A particular placement strategy is defined via the `YandexClusterConfiguration` struct. It has the following fields:

* `layout` — the way resources are located in the cloud;
  * Possible values: `Standard`, `WithoutNAT`, `WithNATInstance` (the description is provided below);
* `withNATInstance` — settings for the `WithNATInstance` layout;
  * `natInstanceExternalAddress` — a [reserved external IP address](#reserving-a-public-ip-address) (or `externalSubnetID` address if specified);
  * `internalSubnetID` — ID of a subnet for the internal interface;
  * `externalSubnetID` — if specified, an additional network interface will be added to the node (the node will use it as a default route);
* `provider` — parameters for connecting to the Yandex.Cloud API;
  * `cloudID` — the cloud ID;
  * `folderID` — ID of the directory;
  * `serviceAccountJSON` — a JSON key generated by [yc iam key create](#permissions)
* `masterNodeGroup` — parameters of the master's NodeGroup;
  * `replicas` — the number of master nodes to create;
  * `zones` — nodes can only be created in these zones;
  * `instanceClass` — partial contents of the fields of the [YandexInstanceClass]({{"/modules/030-cloud-provider-yandex/#yandexinstanceclass-custom-resource" | true_relative_url }} ) CR. The `cores`, `memory`, `imageID` parameters are mandatory. The parameters in **bold** are unique for `YandexClusterConfiguration`. Possible values:
    * `cores`
    * `memory`
    * `imageID`
    * `additionalLabels` — additional labels to add to static nodes;
    * **`externalIPAddresses`** — a list of external addresses. The number of array elements must correspond to the number of `replicas`.
      * If `externalSubnetID` is not set, you have to use either [reserved public IP addresses](#reserving-a-public-ip-address) or the `Auto` constant;
      * If `externalSubnetID` is set, you must select specific unallocated IP addresses from the specified subnet;
    * **`externalSubnetID`** [DEPRECATED] — if specified, an additional network interface will be added to the node (the latter will use it as a default route);
    * **`externalSubnetIDs`** — if specified, an additional network interface will be added to the node (the latter will use it as a default route);
      Also, a route for the node's internal interface will be added (it will cover the entire `nodeNetworkCIDR` subnet);
      The number of array elements must correspond to the number of `replicas`.
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). Each NodeGroup has the following parameters:
  * `name` — the name of the NodeGroup for generating node names;
  * `replicas` — the number of nodes to create;
  * `zones` — nodes can only be created in these zones;
  * `instanceClass` — partial contents of the fields of the [YandexInstanceClass]({{"/modules/030-cloud-provider-yandex/#yandexinstanceclass-custom-resource" | true_relative_url }} ) CR. The `cores`, `memory`, `imageID` parameters are mandatory.  The parameters in **bold** are unique for  `YandexClusterConfiguration`. Possible values:
    * `cores`
    * `memory`
    * `imageID`
    * `coreFraction`
    * `additionalLabels` — additional labels to add to static nodes;
    * **`externalIPAddresses`** — a list of external addresses. The number of array elements must correspond to the number of `replicas`.
      * If `externalSubnetID` is not set, you have to use either [reserved public IP addresses](#reserving-a-public-ip) or the `Auto` constant;
      * If `externalSubnetID` is set, you must select specific unallocated IP addresses from the specified subnet;
    * **`externalSubnetID`** [DEPRECATED] — if specified, an additional network interface will be added to the node (the latter will use it as a default route);
    * **`externalSubnetIDs`** — if specified, an additional network interface will be added to the node (the latter will use it as a default route);
      Also, a route for the node's internal interface will be added (It will cover the entire `nodeNetworkCIDR` subnet);
      The number of array elements must correspond to the number of `replicas`.
  * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
    * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — the same as the `metadata.annotations` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
      * An example:
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — the same as the `.spec.taints` field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **Caution!** Only the `effect`, `key`, `values` fields are available.
      * An example:
        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `nodeNetworkCIDR` — this subnet will be split into **three** equal parts; they will serve as a basis for subnets in three Yandex.Cloud zones;
* `existingNetworkID` — the ID of the existing VPC Network;
* `dhcpOptions` — a list of DHCP parameters to use for all subnets. Note that setting dhcpOptions may lead to [problems](#dhcpoptions-related-problems-and-ways-to-address-them);
  * `domainName` — the name of the search domain;
  * `domainNameServers` —  a list of recursive DNS addresses;
* `sshPublicKey` — a public key for accessing nodes;
* `labels` — labels to attach to resources created in the Yandex.Cloud. Note that you have to re-create all the machines to add new labels if labels were modified in the running cluster;
* `zones` — a limited set of zones in which nodes can be created;
  * An optional parameter;
  * Format — an array of strings;
