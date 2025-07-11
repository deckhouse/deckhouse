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

### Core Security Capabilities
- **Runtime Protection**: Behavioral learning and anomaly detection for containers
- **Network Firewall**: Application-layer container firewall with micro-segmentation
- **Vulnerability Scanning**: Comprehensive CVE scanning for images, registries, and running containers
- **Threat Detection**: Real-time monitoring for suspicious activities and attacks
- **Compliance Management**: Automated compliance reporting and policy enforcement

### Kubernetes Integration
- **Native Kubernetes**: Deployed as Kubernetes resources with CRD-based configuration
- **Policy as Code**: Security policies defined and versioned in Git repositories
- **RBAC Integration**: Native Kubernetes RBAC with additional security roles
- **Multi-cluster Support**: Centralized management across multiple Kubernetes clusters
- **CI/CD Integration**: Automated security scanning in build pipelines

### Deckhouse Platform Benefits
- **Simplified Deployment**: One-click installation through Deckhouse module system
- **Automated Updates**: Managed updates and lifecycle through Deckhouse
- **Monitoring Integration**: Native integration with Deckhouse monitoring stack
- **Unified Management**: Consistent experience with other Deckhouse modules

## Architecture

The `neuvector` module deploys the following components:

- **Controller**: Central management and policy engine
- **Enforcer**: DaemonSet for runtime protection on each node
- **Manager**: Web-based management console
- **Scanner**: Vulnerability scanning engine
- **Updater**: Security feed and CVE database updates

## Deployment modes

NeuVector supports flexible deployment configurations:

### High availability mode

- Multiple controller replicas for redundancy.
- Load balancing between scanner instances.
- Persistent storage for security and configuration data.

### Resource optimization mode

- Single controller deployment for small clusters.
- Reduced resource allocation for development environments.
- Disabling additional components for specific use cases.

## Prerequisites

- Deckhouse Kubernetes Platform v1.30+.
- Kubernetes cluster with at least 3 nodes.
- Sufficient resources: 4 CPU cores and 8GB RAM minimum.

## Working with the module

### Installation

1. Enable the NeuVector module in your Deckhouse configuration:
  
    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: neuvector
    spec:
      enabled: true
      settings:
        controller:
          ingress:
            enabled: true
            host: neuvector.example.com
        manager:
          ingress:
            enabled: true
            host: neuvector-ui.example.com
    ```

1. Apply the configuration:

    ```bash
    kubectl apply -f neuvector-config.yaml
    ```

   Example of basic configuration (use your own data in the `name`, `bootstrapPassword`, `host` fields):

    ```yaml
    apiVersion: deckhouse.io/v1alpha1
    kind: ModuleConfig
    metadata:
      name: neuvector
    spec:
      enabled: true
      settings:
        controller:
          ingress:
            enabled: true
            host: neuvector.example.com
        manager:
          ingress:
            enabled: true
            host: neuvector-ui.example.com
    ```

1. Access the NeuVector console at your configured ingress host.
- Navigate to the configured hostname ingress.
- Log in with the username `admin` and your configured password.
- Start configuring security and monitoring policies.

### Configure Vulnerability Scanning

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: neuvector
spec:
  settings:
    scanner:
      enabled: true
      replicas: 2
      resources:
        requests:
          cpu: 500m
          memory: 1Gi
```
