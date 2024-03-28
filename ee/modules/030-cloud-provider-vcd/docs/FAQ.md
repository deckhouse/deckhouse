---
title: "Cloud provider — VMware Cloud Director: FAQ"
---

## How do I create a hybrid cluster?

A hybrid cluster combines bare metal and VMware Cloud Director nodes. To create such a cluster, you will need an L2 network between all nodes of the cluster.

To create a hybrid cluster, you need to:

1. Enable DHCP-server in internal network.

1. Prepare a file with the provider configuration, replacing the designations with those valid for your cloud

```yaml
apiVersion: deckhouse.io/v1
internalNetworkCIDR: <NETWORK_CIRD>
kind: VCDClusterConfiguration
layout: Standard
mainNetwork: <NETWORK_NAME>
masterNodeGroup:
  instanceClass:
    etcdDiskSizeGb: 10
    mainNetworkIPAddresses:
    - 192.168.199.2
    rootDiskSizeGb: 20
    sizingPolicy: not_exists
    storageProfile: not_exists
    template: not_exists
  replicas: 1
organization: <ORGANIZATION>
provider:
  insecure: true
  password: <PASSWORD>
  server: <API_URL>
  username: <USER_NAME>
sshPublicKey: <SSH_PUBLIC_KEY>
virtualApplicationName: <VAPP_NAME>
virtualDataCenter: <VDC_NAME>
```

Please note that `masterNodeGroup` is required, but can be left as is.

1. Encode the resulting file in base64. 1
1. Create a secret with the following content:

```yaml

apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: <BASE64_WAS_GOT_IN_BEFORE_STEP> 
  cloud-provider-discovery-data.json: eyJhcGlWZXJzaW9uIjoiZGVja2hvdXNlLmlvL3YxIiwia2luZCI6IlZDRENsb3VkUHJvdmlkZXJEaXNjb3ZlcnlEYXRhIiwiem9uZXMiOlsiZGVmYXVsdCJdfQo=
kind: Secret
metadata:
  labels:
    heritage: deckhouse
    name: d8-provider-cluster-configuration
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
```

1. Enable the module `cloud-provider-vcd`:

```shell

kubectl -n d8-system exec -it deployments/deckhouse -- deckhouse-controller module enable cloud-provider-vcd
```
