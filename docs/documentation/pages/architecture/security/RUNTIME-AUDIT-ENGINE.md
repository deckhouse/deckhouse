---
title: Runtime-audit-engine module
permalink: en/architecture/security/runtime-audit-engine.html
search: security audit, audit rules, falco, runtime-audit-engine
description: Architecture of the runtime-audit-engine module in Deckhouse Kubernetes Platform.
---

The [`runtime-audit-engine`](/modules/runtime-audit-engine/) module implements [security event auditing](./runtime-audit.html) in Deckhouse Kubernetes Platform (DKP) based on the [Falco](https://falco.org/) threat detection system. The module collects Linux kernel events and Kubernetes API audit events (using the `k8saudit` plugin), enriches them with Kubernetes Pod metadata, and generates security events according to configured rules. Audit rules are defined using the [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

When the module is enabled, the `control-plane-configurator` ConfigMap is created in the `d8-runtime-audit-engine` namespace with the audit webhook URL and CA. The [`control-plane-manager`](/modules/control-plane-manager/) module detects this ConfigMap and configures the control plane to send Kubernetes API audit events to the `runtime-audit-engine` module.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

- The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
- Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`runtime-audit-engine`](/modules/runtime-audit-engine/) module and its interactions with other DKP components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Runtime-audit-engine module architecture](../../images/architecture/security/c4-l2-runtime-audit-engine.svg)

## Module components

The `runtime-audit-engine` module consists of the following components:

1. **Runtime-audit-engine** (DaemonSet): A component deployed on each cluster node. It collects audit events, evaluates rules, outputs triggered rules to stdout, and exports them as Prometheus metrics. Data is received from the Linux kernel via interception of system calls (syscalls) and from `containerd` via a Unix socket.

   The component includes the following containers:

   - **falco**: Main container that collects security events from cluster nodes and containerized applications in DKP based on the [Falco](https://falco.org/) threat detection system. 
   - **falcosidekick**: Sidecar container that receives events from the `falco` component and exports audit events as Prometheus metrics.
   - **rules-loader**: Sidecar container that performs the following operations:
      - watches [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resources and stores them in the shared `/etc/falco/rules.d/` Pod directory for processing by the `falco` component;
      - validates the FalcoAuditRules custom resource.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC (Role-Based Access Control) that provides secure access to component metrics.

   {% alert level="warning" %}
   The `falco` container has privileged access to each node operating system. The container security context includes the `BPF`, `SYS_RESOURCE`, `PERFMON`, `SYS_PTRACE`, and `SYS_ADMIN` capabilities.
   {% endalert %}

1. **K8s-metacollector** (Deployment): A component that proxies requests to `kube-apiserver` to reduce control plane load. It also reduces the amount of metadata passed to `falco` by keeping only node-related data. K8s-metacollector collects metadata from `kube-apiserver` about Pod, Namespace, Deployment, ReplicaSet, ReplicationController, and Service resources.

   The component includes the following containers:

   - **k8s-metacollector**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to k8s-metacollector metrics.

## Module interactions

The `runtime-audit-engine` module interacts with the following components:

1. **Kube-apiserver**:

   - Manages [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resources.
   - Monitors Pod, Namespace, Deployment, ReplicaSet, ReplicationController, and Service resources.
   - Authorizes module component requests.

1. **Containerd**:

   - Provides container metadata.
   - Provides `containerd` events.

1. **Linux kernel**: Intercepts Linux kernel system calls (syscalls) in real time.

The following external components interact with the module:

1. **Kube-apiserver**:

   - Sends webhook requests to validate the FalcoAuditRules custom resource.
   - Sends audit events.

1. **Prometheus-main**: Collects module metrics.
