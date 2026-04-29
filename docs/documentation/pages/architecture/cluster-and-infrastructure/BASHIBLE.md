---
title: Bashible
permalink: en/architecture/cluster-and-infrastructure/node-management/bashible.html
search: bashible architecture, bashible-api-server
description: Bashible architecture in Deckhouse Kubernetes Platform — executing bash scripts for node configuration, bashible-api-server operation.
---

## Bashible scripts and the bashible service

Node management functions are implemented using specially prepared bash scripts called **bashible**. The service that runs on cluster nodes and executes these scripts is also called bashible. A collection of scripts is referred to as a bundle.

4 bundles are used:

* [Bashible installation scripts](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/bootstrap)
* [Bootstrap scripts for the first node](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/cluster-bootstrap)
* Node configuration scripts for a specific cloud provider (for example, [AWS](https://github.com/deckhouse/deckhouse/tree/main/modules/030-cloud-provider-aws/candi/bashible))
* [Common scripts](https://github.com/deckhouse/deckhouse/tree/main/candi/bashible/common-steps/all)

The scripts are implemented as *gotemplate* templates, which allows flexible node configuration depending on the node group. The scripts must be written in such a manner so that they can be safely re-executed in case of failure or repeated runs. An individual script is referred to as a step.

Main node configuration stages:

* Configuring the NodeUser to ensure access to the node.
* Installing CA certificates.
* Creating the `/opt/deckhouse/bin` directory, adding it to `PATH`, and placing required binaries there.
* Downloading required packages from `registrypackages`.
* Installing and configuring the containerd CRI.
* Downloading and configuring **kubernetes-api-proxy**. This component provides access to the Kubernetes API and is implemented as an NGINX instance with upstream servers pointing to master nodes. This ensures high availability of the API in case one master node is unavailable, as well as load balancing.
* Installing, configuring, and starting [kubelet](../../kubernetes-and-scheduling/kubelet.html).
* Starting the bashible service, which runs `bashible.sh` every minute.
* Rebooting the node if required.

## Bashible-api-server

Due to the large number of bashible script variations for different supported operating systems, storing all versions in etcd is not feasible because of key size limitations and the additional load on etcd. For this reason, the **bashible-api-server** component was developed to generate bashible scripts from templates stored in Custom Resources.

Bashible-api-server is a [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) deployed on master nodes.

When kube-apiserver receives a request for resources containing bashible bundles, it forwards the request to bashible-api-server and returns the generated result. The interactions between bashible and bashible-api-server are shown in the architecture diagrams of the `node-manager` module (for example, in the [CloudEphemeral nodes diagram](cloud-ephemeral-nodes.html)).

Bashible-api-server returns the following resources:

* The **second-phase bootstrap script** downloaded during the first phase.
* **Bashibles**: The `bashible.sh` script.
* **Nodegroupbundles**: Rendered bundle containing the set of scripts for bootstrapping and configuring a node.

These resources can be retrieved either via the Kubernetes API or using `kubectl` by specifying the node group name:

* `kubectl get bootstrap.bashible.deckhouse.io master -o yaml`
* `kubectl get bashibles.bashible.deckhouse.io master -o yaml`
* `kubectl get nodegroupbundles.bashible.deckhouse.io master -o yaml`

Bashible-api-server calculates a checksum of all scripts associated with a node group. This is required to implement the update mechanism and ensure correct node group status updates. The checksums are stored in the `d8-cloud-instance-manager/configuration-checksums` secret. A checksum change triggers the re-execution of bashible scripts on nodes when the configuration is modified. In addition, the checksum of the bashible service is reset every 4 hours to force periodic re-execution of bashible.
