---
title: System requirements
permalink: en/architecture/system-requirements/
description: Installation variants and prerequisites for Deckhouse Kubernetes Platform — cloud, bare-metal, and existing Kubernetes clusters.
---

Deckhouse Kubernetes Platform (DKP) can be installed in the following variants:

* **In a supported cloud**, including [public](/products/kubernetes-platform/documentation/v1/admin/integrations/public/overview.html) and [private clouds](/products/kubernetes-platform/documentation/v1/admin/integrations/private/overview.html), as well as [virtualization systems](/products/kubernetes-platform/documentation/v1/admin/integrations/virtualization/overview.html). The installer automatically creates and configures all required resources (including virtual machines, network objects, etc.), deploys a Kubernetes cluster, and installs DKP. Each cloud provider and virtualization system has its own set of requirements. The full list of requirements for each IaaS integration variant is available in the [IaaS integrations](/products/kubernetes-platform/documentation/v1/admin/integrations/integrations-overview.html) section of the documentation.

* **On bare-metal servers (including hybrid clusters) or in unsupported clouds**. The installer configures the servers or virtual machines specified in the configuration, deploys a Kubernetes cluster, and installs DKP. The system requirements for servers used to deploy DKP depend on the [deployment scenarios](/products/kubernetes-platform/guides/hardware-requirements.html#deployment-scenarios). For detailed resource estimation for DKP installation, refer to the following guides:

  * [Picking resources for a bare metal cluster](/products/kubernetes-platform/guides/hardware-requirements.html)
  * [Disk layout and size](/products/kubernetes-platform/guides/fs-requirements.html)
  * [Going to Production](/products/kubernetes-platform/guides/production.html)

* **In an existing Kubernetes cluster**. The installer deploys DKP and integrates it with the existing infrastructure. To estimate the resources required in the existing cluster, use the bare-metal guides listed above. The Kubernetes cluster in which DKP is installed must be one of the [supported versions](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes).

Before installation, ensure the following:

* For bare-metal clusters (including hybrid clusters) and installations in unsupported clouds: the server runs an operating system from the [supported OS list](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html), or a compatible one, and is accessible via SSH using a key.

* When configuring integration with supported clouds: the required quotas for resource creation are available, and cloud infrastructure access credentials are prepared (these depend on the specific provider).

* Access to the Deckhouse container image registry is available (public: `registry.deckhouse.io` or a mirror).
