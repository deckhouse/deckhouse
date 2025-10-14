---
title: Container image vulnerability scanning
permalink: en/admin/configuration/security/scanning.html
description: "Configure container image vulnerability scanning in Deckhouse Kubernetes Platform using Trivy. Automated security scanning, vulnerability detection, and security policy enforcement."
---

Deckhouse Kubernetes Platform (DKP) provides a built-in tool for automatically detecting vulnerabilities
in container images based on the [Trivy](https://github.com/aquasecurity/trivy) project.

## Vulnerability scanning

DKP scans all container images used by cluster pods.
The checks include:

- Known vulnerabilities (CVEs) in the images in use.
- Compliance with CIS benchmarks (compliance checks).

Scanning uses [public vulnerability databases](https://github.com/aquasecurity/trivy-db/tree/main/pkg/vulnsrc).

## Scanning in namespaces

By default, only the `default` namespace is scanned.

To enable scanning for a specific namespace, add the label `security-scanning.deckhouse.io/enabled=""` to it.

As soon as at least one namespace with this label is detected in the cluster, scanning of the `default` namespace stops.
To re-enable scanning for the `default` namespace, add the label with the following command:

```shell
d8 k label namespace default security-scanning.deckhouse.io/enabled=""
```

In the current version, there is no option to limit the list of resources to be scanned within a namespace.
DKP scans **all resources** in a namespace labeled with `security-scanning.deckhouse.io/enabled=""`.

## Start conditions and scanning process

Scanning starts:

- automatically every 24 hours,
- when components with new images are deployed in namespaces with scanning enabled.

Scanning occurs according to the following process:

1. In the namespace of each scanned resource, a VulnerabilityReport object is created.
1. This object contains the annotation `trivy-operator.aquasecurity.github.io/report-ttl`,
   which specifies the report's time-to-live (default: `24h`).
1. When the TTL expires, the VulnerabilityReport object is deleted, and the scan is run again.

### Forcing a rescan

To manually trigger a rescan of a resource, you can use either of the following methods:

- Update the `trivy-operator.aquasecurity.github.io/report-ttl` annotation with a short time-to-live value.

  Example:

  ```shell
  d8 k annotate VulnerabilityReport -n <NAMESPACE> <REPORT_NAME> trivy-operator.aquasecurity.github.io/report-ttl=1s --overwrite
  ```

- Delete the VulnerabilityReport object from the namespace containing the scanned resource.

  Example:

  ```shell
  d8 k delete VulnerabilityReport -n <NAMESPACE> <REPORT_NAME>
  ```

For details on how to view scan results, see [Scanning container images for vulnerabilities](../../../user/security/scanning.html).
