---
title: "The operator-trivy module"
description: Periodic scanning for vulnerabilities in a Deckhouse Kubernetes Platform cluster.
---

The module allows you to run a regular vulnerability scans of user images in runtime on known CVEs. The module uses the [Trivy](https://github.com/aquasecurity/trivy) project. [Public databases](https://github.com/aquasecurity/travy-db/tree/main/pkg/vulnsrc) are used for scanning vulnerabilities.

Scanning is performed in namespaces that contain the label `security-scanning.deckhouse.io/enabled=""`.
If there are no namespaces with this label in the cluster, the `default` namespace is scanned.

Once a namespace with the label `security-scanning.deckhouse.io/enabled=""` is detected in the cluster, scanning of the `default` namespace stops.
To re-enable scanning for the `default` namespace, use the following command to set the label to the namespace:

```shell
d8 k label namespace default security-scanning.deckhouse.io/enabled=""
```

## Conditions for starting scanning

Scanning starts:

- automatically every 24 hours,
- when components using new images are deployed in the namespaces for which scanning is enabled.

## Where to view scan results

In Grafana:

- `Security/Trivy Image Vulnerability Overview` — a summary of vulnerabilities found in container images and cluster resources.
- `Security/CIS Kubernetes Benchmark` — results of cluster compliance with the CIS Kubernetes Benchmark.

In cluster resources:

- Cluster-wide security reports:
  - [`ClusterComplianceReport`](cr.html#clustercompliancereport)
  - [`RbacAssessmentReport`](cr.html#rbacassessmentreport)

- Resource-level security reports:
  - [`VulnerabilityReport`](cr.html#vulnerabilityreport) — vulnerabilities found in container images;
  - [`SbomReport`](cr.html#sbomreport) — software composition in container images (SBOM);
  - [`ConfigAuditReport`](cr.html#configauditreport) — misconfiguration issues in Kubernetes objects;
  - [`ExposedSecretReport`](cr.html#exposedsecretreport) — secrets exposed in containers.
