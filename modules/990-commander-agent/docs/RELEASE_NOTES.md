---
title: "Release Notes"
---

## v1.4.4

### Chore

* Fixed vulnerabilities found in the module components.

## v1.4.3

### Bug Fixes

* Fixed issues with the operation of the TCP tunnel on the agent side when using Citrix NetScaler for filtering and load balancing traffic between master and slave clusters.
* Fixed an issue with starting Dex in slave clusters when using self-signed certificates in the master cluster.
The problem occurred due to the absence of the root certificate authority of the master cluster in the `DexProvider` resource, which is created by the agent when support for access rights in Commander is enabled.


## v1.4.2

### Bug Fixes

* Fixed the creation of an incorrect `DexProvider`, which caused authentication to fail in subordinate clusters through the managing cluster Deckhouse Commander.

### Chore

* Updated module component images to fix vulnerabilities.
* Added a restricted `securityContext` for the `commander-agent` container to enhance security.

## v1.4.1

### Bug Fixes

* Now the agent sends correct telemetry regarding the statuses of the Deckhouse subsystems.

## v1.4.0

### New Features

* Experimental support for access rights in Commander. Added a mechanism for exchanging user's Dex tokens between managing and subordinate clusters.
* Support for working through an HTTP proxy in all outgoing requests.
* Added support for authentication when accessing the service methods of the Commander API.

### Bug Fixes

* Added `d8:` prefix to the ClusterRole and ClusterRoleBinding resources.
* Added service account token rotation to fix the `D8KubernetesStaleTokensDetected` alert.

### Chore

* Disabling the module now requires confirmation.
* A page with release notes has been added to the module documentation.
* Component images of the module have been updated to fix vulnerabilities.

## v1.3.3

### New Features

* Added support for cluster manager operation mode in the control cluster via proxy (for correct TCP tunnel operation).

### Chore

* Updated module component dependencies to fix identified vulnerabilities.

## v1.3.2

### Chore

* The module build has been switched to distroless to fix and minimize vulnerabilities.
* commander-agent is now scheduled on master nodes by default.

## v1.3.1

### New Features

* Cluster telemetry now includes the names of active alerts.

### Bug Fixes

* Fixed an issue with sending resource application statuses to Commander, where not all resources were displayed on the Kubernetes tab.

## v1.3.0

### New Features

* Added support for managing Kubernetes resource group synchronization mode for Commander 1.10
* When disabling the control of a Kubernetes resource group in Commander, they are not removed from the cluster but are labeled with `agent.commander.deckhouse.io/is-orphan`

