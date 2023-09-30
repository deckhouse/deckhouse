---
title: "Cloud provider — GCP: Layouts"
description: "Schemes of placement and interaction of resources in GCP when working with the Deckhouse cloud provider."
---

Two layouts are supported. Below is more information about each of them.

## Standard

* A separate VPC with [Cloud NAT](https://cloud.google.com/nat/docs/overview) is created for the cluster.
* Nodes in the cluster do not have public IP addresses.
* Public IP addresses can be allocated to master and static nodes:
  * In this case, one-to-one NAT is used to translate the public IP address to the node's IP address (note that CloudNAT is not used in such a case).
* If the master does not have a public IP, then an additional instance with a public IP (aka bastion host) is required for installation tasks and accessing the cluster.
* Peering can also be configured between the cluster VPC and other VPCs.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vR1oHqbXPJPYxUXwpkRGM6VPpZaNc8WoGH-N0Zqb9GexSc-NQDvsGiXe_Hc-Z1fMQWBRawuoy8FGENt/pub?w=989&amp;h=721)
<!--- Source: https://docs.google.com/drawings/d/1VTAoz6-65q7m99KA933e1phWImirxvb9-OLH9DRtWPE/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: Standard
standard:
  # Optional, compute address names from this list are used as addresses for Cloud NAT.
  cloudNATAddresses:
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24         # Required.
peeredVPCs:
# Optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911  # Required.
    diskSizeGb: 20                  # Optional, local disk is used if not specified.
    disableExternalIP: false        # Optional, by default master has externalIP.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911  # Required.
    diskSizeGb: 20                  # Optional, local disk is used if not specified.
    disableExternalIP: true         # Optional, by default nodes do not have externalIP.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: |             # Required.
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```

## WithoutNAT

* A dedicated VPC is created for the cluster; all cluster nodes have public IP addresses.
* Peering can be configured between the cluster VPC and other VPCs.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTq2Jlx4k8OXt4acHeW6NvqABsZIPSDoOldDiGERYHWHmmKykSjXZ_ADvKecCC1L8Jjq4143uv5GWDR/pub?w=989&amp;h=721)
<!--- Source: https://docs.google.com/drawings/d/1uhWbQFiycsFkG9D1vNbJNrb33Ih4YMdCxvOX5maW5XQ/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24         # Required.
# Optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911  # Required.
    diskSizeGb: 20                  # Optional, local disk is used if not specified.
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
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911  # Required.
    diskSizeGb: 20                  # Optional, local disk is used if not specified.
    additionalNetworkTags:          # Optional.
    - tag1
    additionalLabels:               # Optional.
      kube-node: static
provider:
  region: europe-west4              # Required.
  serviceAccountJSON: |             # Required.
    {
      "type": "service_account",
      "project_id": "sandbox",
      "private_key_id": "98sdcj5e8c7asd98j4j3n9csakn",
      "private_key": "-----BEGIN PRIVATE KEY-----",
      "client_id": "342975658279248",
      "auth_uri": "https://accounts.google.com/o/oauth2/auth",
      "token_uri": "https://oauth2.googleapis.com/token",
      "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
      "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/k8s-test%40sandbox.iam.gserviceaccount.com"
    }
```
