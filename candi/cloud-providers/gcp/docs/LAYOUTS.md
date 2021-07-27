---
title: "Cloud provider — GCP: Layouts"
---

## Layouts
### Standard
* A separate VPC with [Cloud NAT](https://cloud.google.com/nat/docs/overview) is created for the cluster.
* Nodes in the cluster do not have public IP addresses.
* Public IP addresses can be allocated to master and static nodes.
    * In this case, one-to-one NAT is used to translate the public IP address to the node's IP address (note that CloudNAT is not used in such a case).
* If the master does not have a public IP, then an additional instance with a public IP (aka bastion host) is required for installation tasks and accessing the cluster.
* Peering can also be configured between the cluster VPC and other VPCs.
![resources](https://docs.google.com/drawings/d/e/2PACX-1vR1oHqbXPJPYxUXwpkRGM6VPpZaNc8WoGH-N0Zqb9GexSc-NQDvsGiXe_Hc-Z1fMQWBRawuoy8FGENt/pub?w=989&amp;h=721)
<!--- Source: https://docs.google.com/drawings/d/1VTAoz6-65q7m99KA933e1phWImirxvb9-OLH9DRtWPE/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: Standard
standard:
  cloudNATAddresses:                                         # optional, compute address names from this list are used as addresses for Cloud NAT
  - example-address-1
  - example-address-2
subnetworkCIDR: 10.0.0.0/24                                  # required
peeredVPCs:                                                  # optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered
- default
sshKey: "ssh-rsa ..."                                        # required
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    disableExternalIP: false                                 # optional, by default master has externalIP
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    disableExternalIP: true                                  # optional, by default nodes do not have externalIP
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: static
provider:
  region: europe-west4                                       # required
  serviceAccountJSON: |                                      # required
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

### WithoutNAT
* A dedicated VPC is created for the cluster; all cluster nodes have public IP addresses;
* Peering can be configured between the cluster VPC and other VPCs;

![resources](https://docs.google.com/drawings/d/e/2PACX-1vTq2Jlx4k8OXt4acHeW6NvqABsZIPSDoOldDiGERYHWHmmKykSjXZ_ADvKecCC1L8Jjq4143uv5GWDR/pub?w=989&amp;h=721)
<!--- Source: https://docs.google.com/drawings/d/1uhWbQFiycsFkG9D1vNbJNrb33Ih4YMdCxvOX5maW5XQ/edit --->

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
subnetworkCIDR: 10.0.0.0/24                                 # required
peeredVPCs:                                                 # optional, list of GCP VPC Networks with which Kubernetes VPC Network will be peered
- default
labels:
  kube: example
masterNodeGroup:
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: master
nodeGroups:
- name: static
  replicas: 1
  zones:                                                     # optional
  - europe-west4-b
  instanceClass:
    machineType: n1-standard-4                               # required
    image: projects/ubuntu-os-cloud/global/images/ubuntu-1804-bionic-v20190911    # required
    diskSizeGb: 20                                           # optional, local disk is used if not specified
    additionalNetworkTags:                                   # optional
    - tag1
    additionalLabels:                                        # optional
      kube-node: static
provider:
  region: europe-west4                                       # required
  serviceAccountJSON: |                                      # required
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

## GCPClusterConfiguration
A particular placement strategy is defined via the `GCPClusterConfiguration` struct:
* `layout` — the way resources are located in the cloud;
    * Possible values: `Standard` or `WithoutNAT` (see the description below);
* `standard` — settings for the `Standard` layout;
    * `cloudNATAddresses` — a list of public static IP addresses for `Cloud NAT`. [Learn more about CloudNAT](https://cloud.google.com/nat/docs/overview#benefits);
        * If this parameter is omitted, Deckhouse will use the [automatic NAT IP address allocation](https://cloud.google.com/nat/docs/ports-and-addresses#addresses) depending on the number of instances and the number of reserved ports per instance;
        * **CAUTION!** By default, 1024 ports are reserved for each node for outbound connections to a single ip:port pair. There are 64512 TCP / UDP ports available for one external IP. If the automatic NAT IP address allocation is used, Cloud NAT automatically adds another external IP address if there are more nodes than there are ports available. Detailed information is available in the [official documentation](https://cloud.google.com/nat/docs/ports-and-addresses).
* `sshKey` — a public key to access nodes as `user`;
* `subnetworkCIDR` — a subnet to use for cluster nodes;
* `peeredVPCs` — a list of GCP VPC networks to peer with the cluster network. The service account must have access to all the VPCs listed. You have to configure the peering connection [manually](https://cloud.google.com/vpc/docs/using-vpc-peering#gcloud) if no access is available;
* `labels` — a list of labels to attach to cluster resources. Npte that you have to re-create all the machines to add new tags if tags were modified in the running cluster. You can learn more about the labels in the [official documentation](https://cloud.google.com/resource-manager/docs/creating-managing-labels);
    * Format — `key: value`;
* `masterNodeGroup` — parameters of the master's NodeGroup;
    * `replicas` — the number of master nodes to create;
    * `zones` — a list of zones where master nodes can be created;
    * `instanceClass` — partial contents of the [GCPInstanceClass](cr.html#gcpinstanceclass) fields.  The parameters in **bold** are unique for `GCPClusterConfiguration`. Possible values:
        * `machineType`
        * `image`
        * `diskSizeGb`
        * `additionalNetworkTags`
        * `additionalLabels`
        * **`disableExternalIP`** — this parameter is only available for the `Standard` layout;
            * It is set to `true` by default. The nodes do not have public addresses and connect to the Internet over `CloudNAT`;
            * `false` — static public addresses are created for nodes; the same addresses are also used for one-to-one NAT;
* `nodeGroups` — an array of additional NodeGroups for creating static nodes (e.g., for dedicated front nodes or gateways). Each NodeGroup has the following parameters:
    * `name` — the name of the NodeGroup to use for generating node names;
    * `replicas` — the number of nodes;
    * `zones` — a list of zones where static nodes can be created;
    * `instanceClass` — partial contents of the [GCPInstanceClass](cr.html#gcpinstanceclass) fields.  The parameters in **bold** are unique for  `GCPClusterConfiguration`. Possible values:
        * `machineType`
        * `image`
        * `diskSizeGb`
        * `additionalNetworkTags`
        * `additionalLabels`
        * **`disableExternalIP`** — this parameter is only available for the `Standard` layout;
            * It is set to `true` by default. The nodes do not have public addresses and connect to the Internet over `CloudNAT`;
            * `false` — static public addresses are created for nodes; the same addresses are also used for one-to-one NAT;
    * `nodeTemplate` — parameters of Node objects in Kubernetes to add after registering the node;
        * `labels` — the same as the `metadata.labels` standard [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
          * An example:
            ```yaml
            labels:
              environment: production
              app: warp-drive-ai
            ```
        * `annotations` — the same as the `metadata.annotations` [field](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta);
          * An example:
            ```yaml
            annotations:
              ai.fleet.com/discombobulate: "true"
            ```
        * `taints` — the same as the `.spec.taints` field of the [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core) object. **CAUTION!** Only the `effect`, `key`, `values` fields are available;
          * An example:

            ```yaml
            taints:
            - effect: NoExecute
              key: ship-class
              value: frigate
            ```
* `provider` — parameters for connecting to the GCP API;
    * `region` — the name of the region where instances will be provisioned;
    * `serviceAccountJSON` — `service account key` in the JSON format. [Creating a service account](environment.html)
* `zones` — a limited set of zones in which nodes can be created;
  * An optional parameter;
  * Format — an array of strings;
