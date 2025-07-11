---
title: "Module neuvector"
search: neuvector
description: The neuvector module in Deckhouse offers runtime protection, vulnerability management, and compliance monitoring for Kubernetes clusters.
---

## Description

The `neuvector` module provides comprehensive container security throughout the DevOps lifecycle. It offers runtime protection, vulnerability management, and compliance monitoring for Kubernetes clusters.
`neuvector` supports all major container runtimes, automatically detecting them: Docker, containerd, CRI-O.
Read more about the security platform on the official website [NeuVector](https://open-docs.neuvector.com/).

## Features

The `neuvector` module solves critical security challenges for containerized applications:

- **Runtime Protection:**
  - Real-time threat detection and prevention during container execution.
  - Behavioral learning to establish baseline security profiles.
  - Zero-trust network segmentation between services.
  - Process and file integrity monitoring.
  - Detection of suspicious activities and anomalies.

- **Vulnerability Management:**
  - Continuous scanning of container images for known vulnerabilities.
  - Registry scanning integration with popular container registries.
  - CVE database updates and vulnerability prioritization.
  - Compliance reporting and risk assessment.

- **Network Security:**
  - Automatic discovery and visualization of application connectivity.
  - Microsegmentation with automated network policy generation.
  - East-west traffic inspection and monitoring.
  - DLP (Data Loss Prevention) for sensitive data protection.

- **Compliance and Governance:**
  - CIS benchmark compliance checking.
  - PCI DSS, HIPAA, and other regulatory compliance reporting.
  - Security event audit trails and forensics.
  - Risk scoring and security posture assessment.

- **DevSecOps Integration:**
  - CI/CD pipeline integration for security scanning.
  - Admission control to prevent vulnerable containers from running.
  - Security policy as code with version control.
  - Automated response and remediation capabilities.

## Architecture

The `neuvector` module consists of several key components that work together to provide comprehensive container security:

### Control Plane Components

- **Controller** — the central management component that:
  - Coordinates security policies across the cluster.
  - Manages the REST API for configuration and monitoring.
  - Handles vulnerability database updates and scanning orchestration.
  - Provides centralized logging and event management.
  - Manages user authentication and role-based access control.

- **Manager** — the web-based management console that:
  - Provides a comprehensive security dashboard and visualization.
  - Offers policy management and configuration interfaces.
  - Displays security events, alerts, and compliance reports.
  - Enables security analytics and threat investigation tools.

### Data Plane Components

- **Enforcer** — deployed as a DaemonSet on each node to:
  - Monitor container runtime behavior and network traffic.
  - Enforce security policies in real-time.
  - Perform deep packet inspection and protocol analysis.
  - Collect security telemetry and behavioral data.
  - Integrate with container runtimes (Docker, containerd, CRI-O).

- **Scanner** — provides vulnerability assessment services:
  - Scans container images for known vulnerabilities.
  - Performs continuous monitoring of running containers.
  - Integrates with container registries for automated scanning.
  - Maintains up-to-date CVE databases and security feeds.

### Additional Components

- **Registry Adapter** — optional component for:
  - Harbor registry integration and webhook scanning.
  - Custom registry integration capabilities.
  - Automated vulnerability scanning workflows.

- **CVE Updater** — maintains security intelligence:
  - Regular updates of vulnerability databases.
  - Security feed synchronization and management.
  - Threat intelligence integration and correlation.

## Deployment Modes

NeuVector supports flexible deployment configurations:

### High Availability Mode
- Multiple controller replicas for redundancy.
- Load balancing across scanner instances.
- Persistent storage for security data and configurations.

### Resource-Optimized Mode
- Single controller deployment for smaller clusters.
- Reduced resource allocation for development environments.
- Optional component disable for specific use cases.

## Prerequisites

- Deckhouse Kubernetes Platform v1.30+.
- Kubernetes cluster with at least 3 nodes.
- Sufficient resources: 4 CPU cores and 8GB RAM minimum.

## Enabling NeuVector

1. To enable the module, use the web interface or the following command:

    ```bash
    d8 platform module enable neuvector
    ```

## Getting the password

If you need to get the admin password stored in the Kubernetes secret in the d8-neuvector namespace, use the following command:

```txt
d8 k -n d8-neuvector get secret admin -o jsonpath='{.data.password}' | base64 -d
```
