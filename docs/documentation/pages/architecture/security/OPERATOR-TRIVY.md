---
title: Module operator-trivy
permalink: en/architecture/security/operator-trivy.html
search: operator-trivy, image scanning, vulnerability scanning
description: Architecture of the operator-trivy module in Deckhouse Kubernetes Platform.
---

The [`operator-trivy`](/modules/operator-trivy/) module scans user images at runtime for known CVEs (Common Vulnerabilities and Exposures), including vulnerabilities specific to Astra Linux, ALT Linux, and RED OS. It is based on the [Trivy](https://github.com/aquasecurity/trivy) project. Scanning uses [public vulnerability databases](https://github.com/aquasecurity/trivy-db/tree/main/pkg/vulnsrc), enriched with Astra Linux, ALT Linux, RED OS databases, and [BDU FSTEC (Data Bank of Information Security Threats by the Federal Service for Technical and Export Control of Russia)](https://bdu.fstec.ru/vul).

The module also analyzes Kubernetes cluster compliance with the [CIS (Center for Internet Security) Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes) requirements.

Additionally, the `operator-trivy` module can collect workload behavior models when the optional node-agent component is enabled.

The `operator-trivy` module works with custom resources from the `trivy.deckhouse.io` and `spdx.softwarecomposition.kubescape.io` API groups.

The `trivy.deckhouse.io` API group includes the following resources:

- [ClusterComplianceReport](/modules/operator-trivy/cr.html#clustercompliancereport): Stores an aggregated cluster compliance report for security requirements (for example, CIS Kubernetes Benchmark).
- ClusterConfigAuditReport: Stores cluster-level audit results for Kubernetes object configuration.
- ClusterInfraAssessmentReport: Stores cluster-level Kubernetes infrastructure security assessment results.
- ClusterRbacAssessmentReport: Stores cluster-level role-based access control (RBAC) settings assessment results.
- ClusterSbomReport: Stores an aggregated cluster-level SBOM (Software Bill of Materials) for software components.
- ClusterVulnerabilityReport: Stores aggregated cluster-level vulnerability scanning results.
- [ConfigAuditReport](/modules/operator-trivy/cr.html#configauditreport): Stores audit results for Kubernetes object configuration.
- [ExposedSecretReport](/modules/operator-trivy/cr.html#exposedsecretreport): Stores results of searching for potential secrets in container images.
- InfraAssessmentReport: Stores security assessment results for Kubernetes infrastructure objects.
- [NodeVulnerabilityReport](/modules/operator-trivy/cr.html#nodevulnerabilityreport): Stores vulnerability scanning results for node `rootfs` (the root filesystem).
- [RbacAssessmentReport](/modules/operator-trivy/cr.html#rbacassessmentreport): Stores RBAC settings assessment results for excessive privileges and other risks.
- [RegistryImageVulnerabilityReport](/modules/operator-trivy/cr.html#registryimagevulnerabilityreport): Stores CVE scanning results for a specific image tag in a registry.
- [RegistryScanTarget](/modules/operator-trivy/cr.html#registryscantarget): Defines a target registry, repositories, and periodic scanning parameters.
- [SbomReport](/modules/operator-trivy/cr.html#sbomreport): Stores an SBOM, that is, the software and dependency composition of a container image.
- [VulnerabilityReport](/modules/operator-trivy/cr.html#vulnerabilityreport): Stores vulnerability scanning results for container images.

The `spdx.softwarecomposition.kubescape.io` API group includes the following resources:

- ApplicationProfiles: Stores an application behavior profile at runtime (system calls, executed processes, file access, HTTP endpoints).
- CollapseConfigurations: Defines dynamic path, endpoint, and network address aggregation parameters to reduce runtime profile size.
- ConfigurationScanSummaries: Stores a summary of configuration scan results for a workload group within a specified scope (for example, a namespace).
- ContainerProfiles: Stores an individual container profile, including runtime behavior and network interactions.
- GeneratedNetworkPolicies: Stores NetworkPolicy objects generated from observed traffic.
- KnownServers: Stores a catalog of known servers and IP address ranges to enrich generated network policies.
- NetworkNeighborhoods: Stores an observed map of incoming and outgoing workload network interactions.
- OpenVulnerabilityExchangeContainers: Stores OpenVEX documents with vulnerability statuses for components.
- SbomSyftFiltereds: Stores a filtered Syft SBOM with only relevant vulnerable components.
- SbomSyfts: Stores SBOM in Syft format.
- SeccompProfiles: Stores container seccomp profiles (allowed system calls and filtering rules).
- VulnerabilityManifestSummaries: Stores VulnerabilityManifest summaries by severity level with links to related objects.
- VulnerabilityManifests: Stores a detailed manifest of discovered vulnerabilities.
- VulnerabilitySummaries: Stores an aggregated vulnerability summary for a specified scope.
- WorkloadConfigurationScans: Stores detailed configuration scan results for a specific workload.
- WorkloadConfigurationScanSummaries: Stores a summary of WorkloadConfigurationScan results.

For more details, see the [module documentation section](/modules/operator-trivy/).

## Module architecture

{% alert level="info" %}
The following assumptions are made to simplify the diagram:

- The diagram shows direct interaction between containers from different pods. In practice, they communicate through corresponding Kubernetes Services (internal load balancers). Service names are omitted when they are clear from context. In other cases, the service name is shown above the arrow.
- Pods can run with multiple replicas, but all pods are shown as a single replica in the diagram.
{% endalert %}

The level 2 C4 architecture of the [`operator-trivy`](/modules/operator-trivy/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown in the following diagram:

![Operator-trivy module architecture](../../images/architecture/security/c4-l2-operator-trivy.svg)

## Module components

The module consists of the following components:

1. **Operator** (Deployment): A component based on [Trivy Operator](https://github.com/aquasecurity/trivy-operator) that watches Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob, and Job resources, and starts a Job task for scanning when their specifications change.

   Operator analyzes Job execution results and generates the following reports as custom resources:

   - ClusterComplianceReport.
   - ClusterConfigAuditReport.
   - ClusterInfraAssessmentReport.
   - ClusterRbacAssessmentReport.
   - ClusterVulnerabilityReport.
   - ConfigAuditReport.
   - ExposedSecretReport.
   - InfraAssessmentReport.
   - NodeVulnerabilityReport.
   - RbacAssessmentReport.
   - SbomReport.
   - VulnerabilityReport.

   Includes the following containers:

   - **operator**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to controller metrics.

1. **Trivy-server** (StatefulSet): This component implements a security scanning service and is based on the Open Source [Trivy](https://github.com/aquasecurity/trivy) project.

   Trivy-server updates its vulnerability database from an Open Container Initiative (OCI) image at startup and then periodically.

   Trivy-server processes requests from other module components (operator, registry-scanner, trivy-provider, and scanning tasks), performs the requested scan, and returns the result.

   Includes the following containers:

   - **server**: Main container.
   - **trivy-db-info**: Sidecar container that synchronizes Trivy cache metadata with the `trivy-db-info` ConfigMap resource in the `d8-operator-trivy` namespace.

1. **Trivy-provider** (StatefulSet): This component includes a single **trivy-provider** container and provides an image verification interface for the Gatekeeper component of the [`admission-policy-engine`](/modules/admission-policy-engine/) module.

   When the module is installed, a Provider resource is created to register trivy-provider as a provider in Gatekeeper. For details about the integration, see the [relevant Gatekeeper documentation section](https://open-policy-agent.github.io/gatekeeper/website/docs/externaldata/).

   The Deckhouse controller deploys this component if the [`.settings.denyVulnerableImages.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-denyvulnerableimages-enabled) parameter of the ModuleConfig custom resource is set to `true` (default is `false`) and the Gatekeeper component of the [`admission-policy-engine`](/modules/admission-policy-engine/) module is enabled.

1. **Report-updater** (Deployment): An optional controller that includes a single **report-updater** container, implements a MutatingWebhook, and enriches the VulnerabilityReport custom resource with BDU identifiers. The BDU dictionary is regularly updated from an OCI image every six hours.

   The Deckhouse controller deploys this component if the [`.settings.linkCVEtoBDU`](/modules/operator-trivy/stable/configuration.html#parameters-linkcvetobdu) parameter of the ModuleConfig custom resource is set to `true` (default is `false`).

1. **Registry-scanner** (StatefulSet): An optional component that includes a single **registry-scanner** container and regularly scans images located in arbitrary container registries specified by the user. Scanning of workloads in the cluster itself is performed by the operator component.

   The Deckhouse controller deploys this component if the [`.settings.registryScanning.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-registryscanning-enabled) parameter of the ModuleConfig custom resource is set to `true` (default is `false`).

   Registry-scanner reads [RegistryScanTarget](/modules/operator-trivy/cr.html#registryscantarget) custom resources, runs image scans through the trivy-server component, and stores processed results in the [RegistryImageVulnerabilityReport](/modules/operator-trivy/cr.html#registryimagevulnerabilityreport) custom resource.

1. **Security-storage** (Deployment): A controller that includes a single **apiserver** container, extends Kubernetes API, and processes create, read, update, and delete (CRUD) operations, watch requests, and list requests for custom resources from the `trivy.deckhouse.io` and `spdx.softwarecomposition.kubescape.io` API groups.

   Security-storage also implements the backend for storing these resources:
   - Metadata is stored in an SQLite database.
   - Object body is stored in gob format in the `/data` directory.

1. **Node-agent** (DaemonSet): An optional component with a single **node-agent** container that runs on all cluster nodes in privileged mode, observes container behavior by using eBPF programs, and builds runtime profiles.

   Node-agent stores runtime profiles in ApplicationProfile and NetworkNeighborhood custom resources (in the security-storage component). For details about node-agent behavior, see the [module documentation section](/modules/operator-trivy/stable/runtime_map.html).

   {% alert level="warning" %}
   The node-agent component has privileged access to the operating system of each node. In Linux, this requires the following capabilities:
   - SYS_ADMIN
   - SYS_PTRACE
   - NET_ADMIN
   - SYSLOG
   - SYS_RESOURCE
   - IPC_LOCK
   - NET_RAW

   This is required to observe container behavior by using eBPF programs.

   Node-agent runs in profiling mode and does not perform active rule-based attack detection.
   {% endalert %}

   The Deckhouse controller deploys this component if the [`.settings.nodeAgent.enabled`](/modules/operator-trivy/stable/configuration.html#parameters-nodeagent-enabled) parameter of the ModuleConfig custom resource is set to `true` (default is `false`).

1. **Scan-noderootfs-&lt;HASH&gt;** (Job): A component that includes a single **node-rootfs-scanner** container and implements a task to scan the node root filesystem (scan-noderootfs). The task is created and managed by the operator component.

1. **Scan-vulnerabilityreport-&lt;HASH&gt;** (Job): A component that launches tasks for security scanning of Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob, and Job resources by using the trivy-server component. The task is created and managed by the `operator` component.

   Consists of a set of **&lt;CONTAINER_NAME&gt;** containers, each responsible for scanning the corresponding workload container. Their base image is Trivy with an added trivy-wrapper. The target container image from the workload specification is passed in arguments. Trivy-wrapper authenticates to the registry storage by using the `trivy registry login` command and then hands control over to Trivy.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   - Authorizes requests to operator metrics.
   - Works with custom resources from the `trivy.deckhouse.io` API group.
   - Works with RegistryScanTarget and RegistryImageVulnerabilityReport custom resources.
   - Watches Pod, ReplicaSet, ReplicationController, StatefulSet, DaemonSet, CronJob, and Job resources.
   - Creates, updates, and deletes Secret, ConfigMap, and Job resources.

1. [**Registry**](/modules/registry/): Pulls OCI images with the vulnerability database and the BDU database.

1. **Container registry**: Scanning images.

The following external components interact with the module:

1. **Kube-apiserver**:

   - Forwards API requests for custom resources from the `trivy.deckhouse.io` and `spdx.softwarecomposition.kubescape.io` API groups.
   - Mutates VulnerabilityReport custom resources.

1. **Prometheus-main**: Collects module metrics.
