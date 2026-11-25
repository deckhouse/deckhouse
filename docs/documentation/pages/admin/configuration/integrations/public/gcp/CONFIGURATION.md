---
title: Integration configuration in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/configuration.html
lang: en
---

## Configuration

Integration with GCP is handled via the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource,
which describes the [cloud cluster](../../../../glossary.html#cloud-cluster) configuration in GCP
and is used by the [cloud provider](../../../../glossary.html#cloud-provider) when the control plane is hosted in the cloud.
The responsible Deckhouse Platform module configures itself automatically based on the selected [layout](../../../../glossary.html#layout).

To update the configuration of a running cluster, run the following command:

```shell
d8 system edit provider-cluster-configuration
```

{% alert level="info" %}
After modifying node-related parameters, you must run the `dhctl converge` command for the changes to take effect.
{% endalert %}

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
sshKey: "<SSH_PUBLIC_KEY>"
subnetworkCIDR: 10.36.0.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
nodeGroups:
- name: static
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
    additionalNetworkTags:
    - tag1
    additionalLabels:
      kube-node: static
provider:
  region: europe-west3
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"
```
