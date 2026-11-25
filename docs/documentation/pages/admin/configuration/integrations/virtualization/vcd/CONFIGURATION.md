---
title: Integration configuration in VMware Cloud Director
permalink: en/admin/integrations/virtualization/vcd/configuration.html
lang: en
---

## Configuration

Integration is carried out using the [VCDClusterConfiguration](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration) resource, which describes the configuration of the [cloud cluster](../../../../glossary.html#cloud-cluster) in VCD and is used by the virtualization system if the cluster’s control plane is hosted there. The Deckhouse Platform module responsible for integration is automatically configured based on the selected deployment [layout](../../../../glossary.html#layout).

To modify the configuration in a running cluster, execute the following command:

```shell
d8 -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller edit provider-cluster-configuration
```

{% alert level="info" %}
After changing the node parameters, you must run the `dhctl converge` command for the changes to take effect.
{% endalert %}

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: VCDClusterConfiguration
sshPublicKey: "<SSH_PUBLIC_KEY>"
organization: My_Org
virtualDataCenter: My_Org
virtualApplicationName: Cloud
mainNetwork: internal
layout: Standard
internalNetworkCIDR: 172.16.2.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    template: Templates/ubuntu-focal-20.04
    sizingPolicy: 4cpu8ram
    rootDiskSizeGb: 20
    etcdDiskSizeGb: 20
    storageProfile: nvme
nodeGroups:
  - name: worker
    replicas: 1
    instanceClass:
      template: Org/Templates/ubuntu-focal-20.04
      sizingPolicy: 16cpu32ram
      storageProfile: ssd
provider:
  server: "<SERVER>"
  username: "<USERNAME>"
  password: "<PASSWORD>"
  insecure: true
```
