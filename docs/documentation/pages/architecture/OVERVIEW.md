---
title: Architecture
permalink: en/architecture/
search: Deckhouse architecture, DKP architecture
description: Overview of the Deckhouse Kubernetes Platform architecture.
---

This section describes the architecture of Deckhouse Kubernetes Platform (DKP).

The section consists of the following subsections:

* [C4 model](c4-model/): Overview of the C4 model used to visualize the platform architecture,
  as well as a description of the DKP architecture at levels 1 and 2 of the C4 model.
* [Modules](module-development/): Description of the DKP module architecture.
* [Disaster resilience](disaster-resilience/): Description of the disaster resilience approaches implemented in DKP.
* [Updating](updating.html): Description of the DKP update mechanisms.
* Description of platform component architecture divided in the following subsystems:
  * [Deckhouse subsystem](deckhouse/)
  * [Kubernetes & Scheduling subsystem](kubernetes-and-scheduling/)
  * [Cluster & Infrastructure subsystem](cluster-and-infrastructure/)
  * [IAM subsystem](iam/)
  * [Security subsystem](security/)
  * [Network subsystem](network/)
  * [Observability subsystem](observability/)

{% alert level="info" %}
This section does not yet cover all DKP subsystems and modules.  
Documentation for the remaining components will be added as it becomes available.
{% endalert %}

## DKP architecture

DKP is a platform for managing Kubernetes clusters in various infrastructures — from isolated server environments to public clouds.
The platform includes:

* A Kubernetes cluster.
* The Deckhouse controller and the modules it manages.
* [Bashible](cluster-and-infrastructure/bashible/),
  an agent running as a service on cluster nodes that executes bash scripts to manage nodes.

Modules are grouped into subsystems according to their functional purpose.
The Deckhouse controller is also implemented as a module and is the only mandatory module required for the platform to function.

The DKP architecture at the subsystem and module level is described in the [C4 model](c4-model/) subsection.

## Modules

A module is a set of resources and applications designed to extend DKP functionality.

Key modules:

* [`deckhouse`](/modules/deckhouse/): The Deckhouse controller.
* [`control-plane-manager`](kubernetes-and-scheduling/control-plane-management/): Manages cluster control plane components.
* [`node-manager`](cluster-and-infrastructure/node-manager/): Manages cluster nodes.

{% alert level="info" %}
The [`control-plane-manager`](/modules/control-plane-manager/) and [`node-manager`](/modules/node-manager/) modules
are not present when the platform is installed into an existing Managed Kubernetes cluster.
{% endalert %}

A module includes:

* Helm charts
* [Addon-operator](https://github.com/flant/addon-operator/) hooks
* Build rules for module components (Deckhouse components)
* Other related files

DKP uses the [addon-operator](https://github.com/flant/addon-operator/) project to manage modules.
Refer to its documentation to learn how DKP works with [modules](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [module hooks](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md), and [module parameters](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md).

For more information about module architecture and developing custom modules, refer to [Modules](module-development/).
