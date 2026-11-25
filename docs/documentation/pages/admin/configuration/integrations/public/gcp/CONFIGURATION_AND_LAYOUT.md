---
title: Layouts in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/layout.html
lang: en
---

## Layouts

This section describes the possible node placement [layouts](../../../../glossary.html#layout) in Google Cloud Platform (GCP) infrastructure
and the related configuration options.
The selected layout affects networking behavior, availability of public IP addresses, outgoing traffic routing,
and how nodes are accessed.

Deckhouse Platform supports two layouts for deploying resources in GCP.

### Standard

This layout uses a dedicated VPC and Cloud NAT for the cluster.

- A dedicated VPC is created for the cluster along with [Cloud NAT](https://cloud.google.com/nat/docs/overview).
- Cluster nodes do not have public IP addresses by default.
- Public IP addresses can be assigned to static and [master nodes](../../../../glossary.html#master-node):
  - In this case, One-to-One NAT is used to map the public IP address to the node's internal IP address
    (Cloud NAT will not be used in this scenario).
- If the master node does not have a public IP address, an additional instance with a public IP address
  (for example, a [bastion host](../../../../glossary.html#bastion-host)) is required for cluster installation and access.
- Peering can be configured between the cluster's VPC and other VPCs.

![Standard layout in GCP](../../../../images/cloud-provider-gcp/gcp-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10164&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: Standard
standard:
  # Optional. Listed addresses will be used for
  # addressing Cloud NAT.
  cloudNATAddresses:
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24         # Required.
# Optional. A list of GCP VPC Networks used by Kubernetes VPC
# Network to connect over peering.
peeredVPCs:
- default
sshKey: "<SSH_PUBLIC_KEY>"  # Required.
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    disableExternalIP: false        # Optional. The master node has externalIP by default.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    disableExternalIP: true         # Optional. The nodes don't have externalIP by default.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"  # Required.
```

Replace `<SERVICE_ACCOUNT_JSON>` with the GCP service account JSON key.

### WithoutNAT

This layout uses a dedicated VPC without Cloud NAT.

- A dedicated VPC is created for the cluster.
  All cluster nodes are assigned public IP addresses.
- Peering can be configured between the cluster's VPC and other VPCs.

![WithoutNAT layout in GCP](../../../../images/cloud-provider-gcp/gcp-withoutnat.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10296&t=Qb5yyWumzPiTBtfL-0 --->

Example layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24         # Required.
# Optional. A list of GCP VPC Networks used by Kubernetes VPC
# Network to connect over peering.
peeredVPCs:
- default
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                            # Optional.
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4      # Required.
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20260128  # Required.
    diskSizeGb: 20                  # Optional. A local disk is used if not set.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"  # Required.
```

Replace `<SERVICE_ACCOUNT_JSON>` with the GCP service account JSON key.

[Integration configuration](configuration.html).

[Network configuration](network.html).
