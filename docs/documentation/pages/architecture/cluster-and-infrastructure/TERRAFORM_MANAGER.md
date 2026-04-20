---
title: Terraform-manager module
permalink: en/architecture/cluster-and-infrastructure/infrastructure/terraform-manager.html
search: terraform manager, terraform
description: Architecture of the terraform-manager module in Deckhouse Kubernetes Platform for managing Terraform state and cluster infrastructure resources.
---

The `terraform-manager` module provides tools for managing the Terraform state in a DKP cluster.

For more details about the module configuration, refer to the [corresponding documentation section](/modules/terraform-manager/configuration.html).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`terraform-manager`](/modules/terraform-manager/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Terraform-manager architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-terraform-manager.png)

## Module components

The module consists of the following components:

1. **Terraform-auto-converger**: Periodically (once per hour by default) checks the Terraform state and applies non-destructive changes to infrastructure resources.

   The component operates only on the base infrastructure of the cluster. Cluster nodes are not automatically reconciled to the desired state. The check interval is configured using the [`autoConvergerPeriod`](/modules/terraform-manager/configuration.html#parameters-autoconvergerperiod) parameter.

   It consists of the following containers:

   * **to-tofu-migrator**: Init container used to migrate the Terraform state to OpenTofu. It runs the [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) utility with the `converge-migration` command.
   * **converger**: Main container running the [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) utility with the `converge-periodical` command.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the metrics of the converger container. This component is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

2. **Terraform-state-exporter**: Checks the Terraform state and exports related metrics.

   It consists of the following containers:

   * **exporter**: Main container running the [`dhctl`](https://github.com/deckhouse/deckhouse/tree/main/dhctl) utility with the `terraform converge-exporter` command.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the metrics of the exporter container.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Reading and writing the Secret containing the Terraform state.
   * Authorization of metric requests.

2. **Cloud infrastructure** (or virtualization platform): Manages base infrastructure resources and reconciles them to the desired state.

The following external components interact with the module:

* **prometheus-main**: Collects metrics from terraform-auto-converger and terraform-state-exporter.
