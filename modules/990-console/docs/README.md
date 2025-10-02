---
title: "Web Interface"
description: Web Interface for Deckhouse Kubernetes Platform
menuTitle: "Web Interface"
---

Web interface aims the simplicity of control and the transparency of a state of Deckhouse Kubernetes Platform.

All users have access to the interface according to their rights in the platform.

Assuming public domain template is `%s.example.com`, the web app will be available at
`https://console.example.com`.

![Overview](/images/console/screenshot-sys-overview.png)

## Features

- Cluster overview, versions of Deckhouse and Kubernetes, the overall condition and updates
- Deckhouse modules and their settings
- Node management: configuration, scaling, and update settings
- Multitenancy: projects and project templates
- Access control: external authentication providers, group and user permissions
- Ingress controllers to rul incoming traffic
- Journaling: collecting logs from node file and pods, and sending them to various storage types
- Monitoring: processing and sending of metrics, recording rules and alerts, Grafana dashboards and data sources, Prometheus settings, and a list of firing alerts
- GitOps support: special marks on Kubernetes resources, created by automation like werf, Argo CD, Helm.
- Metrics and monitoring dashboards in Nodegroups, Nodes, and Ingress Controllers
- Pods of Prometheus, Ingress Controllers, and Nodes
- And much more!

## Turning on

The module must be turned on explicitly in ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: console
spec:
  enabled: true
```


## Resources requirements

Resources consumed by server-side pods are estimated as follows

| Users | CPU, cores | RAM, MiB |
| ----: | ---------: | -------: |
|     0 |     0.0005 |       18 |
|     1 |     0.0500 |       25 |
|    10 |     0.4000 |       53 |
|   100 |     0.6500 |      130 |

Vertical Pod Autoscaler is configured with a minimum CPU/memory limit of 100m/100MiB and a maximum of 1/512MiB.
The server side pods are deployed in two replicas automatically for Deckhouse platform installation in HA mode.
