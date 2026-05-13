---
title: Architecture of security event audit
permalink: en/architecture/security/runtime-audit.html
search: security audit, audit rules, falco
description: Architecture of security event audit in Deckhouse Kubernetes Platform.
---

The Deckhouse Kubernetes Platform (DKP) security event audit is based on the [Falco](https://falco.org/) threat detection system.
DKP deploys Falco agents on each node as part of a DaemonSet.
Once started, the agents begin collecting OS system calls and Kubernetes audit data.

{% alert level="info" %}
Falco developers recommend running it as a systemd service,
which can be challenging in Kubernetes clusters that support autoscaling.
DKP includes additional security mechanisms such as multitenancy and resource control policies.
Combined with the DaemonSet deployment, these mechanisms ensure a high level of protection.
{% endalert %}

![Falco agents on DKP cluster nodes](../../images/runtime-audit-engine/falco_daemonset.svg)

Each cluster node runs a Falco Pod with the following components:

- `falco`: Collects events, enriches them with metadata, and outputs them to stdout.
- `rules-loader`: Retrieves rule data from [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resources
  and stores them in a shared directory.
- [`falcosidekick`](https://github.com/falcosecurity/falcosidekick): Receives events from `falco`
  and exports them as metrics to external systems.
- `kube-rbac-proxy`: Protects the `falcosidekick` metrics endpoint from unauthorized access.

![Falco Pod components](../../images/runtime-audit-engine/falco_pod.svg)
<!--- Source: https://docs.google.com/drawings/d/1rxSuJFs0tumfZ56WbAJ36crtPoy_NiPBHE6Hq5lejuI --->

## Audit rules

Security event analysis is performed using rules that define suspicious behavior patterns.
Each rule consists of a condition expression written in accordance with [Falco's condition syntax](https://falco.org/docs/concepts/rules/conditions/).

### Built-in rules

DKP provides the following types of built-in rules:

- **Kubernetes audit rules**: Help detect security issues in DKP and in the audit mechanism itself.
  These rules are located in the `falco` container at `/etc/falco/k8s_audit_rules.yaml`.

### Custom rules

Custom rules can be defined using the [FalcoAuditRules](/modules/runtime-audit-engine/cr.html#falcoauditrules) custom resource.

Each Falco agent includes a sidecar container with a [`shell-operator`](https://github.com/flant/shell-operator) instance.
This instance reads rules from Kubernetes resources, converts them into Falco rule format,
and stores them in the `/etc/falco/rules.d/` directory inside the Pod.
When a new rule is added, Falco automatically reloads the configuration.

![Shell-operator handling Falco rules](../../images/runtime-audit-engine/falco_shop.svg)

## New architecture

{% alert level="warning" %}
The new architecture is under active development and may change in future releases.
{% endalert %}

The proposed solution is intended to build a unified pipeline for working with security events extracted from the logs of applications and Kubernetes infrastructure components.

Key idea: a security event is information from logs of various services, normalized into a single contract.

### What is considered a security event

A security event is a structured record of an action or fact that is significant from an information security perspective. Typical categories of such events include:

- Authentication and authorization
- Access to APIs and configuration
- Changes to cluster objects
- Runtime and network activity anomalies

Regardless of the original log format, the output is a uniform event model with a mandatory minimum set of attributes:

- Event identifier (`id`) and time (`timestamp`)
- Source (`source.component`)
- Classification (`event.code`, `event.category`, `event.severity`, `event.outcome`)
- Service metadata (`eventMetadata`), including the cluster identifier

Additionally, attributes describing the subject (`actor`) and the object (`object`) may be present if they can be extracted from the original log.

### Final event schema

Below is the structure of the event **that is sent to storage**.

Required fields:

- `id`, `timestamp`
- `source.component`
- `event.code`, `event.category`, `event.severity`, `event.outcome`
- `eventMetadata.cluster`

Optional fields:

- `eventMetadata.sourceIPs`
- `actor.*`
- `object.*`

```json
{
  "id": "2f0de5c2-2e58-4d3f-b4fe-5ec6f1935b9f",
  "timestamp": "2026-05-10T14:21:03Z",
  "source": {
    "component": "kube-apiserver"
  },
  "event": {
    "code": "UNAUTHORIZED_ACCESS",
    "category": "Rbac",
    "severity": "High",
    "outcome": "Failure"
  },
  "eventMetadata": {
    "cluster": "prod-cluster",
    "sourceIPs": [
      "206.123.145.70"
    ]
  },
  "actor": {
    "id": "system:serviceaccount:default:demo",
    "type": "ServiceAccount"
  },
  "object": {
    "id": "/api/v1/namespaces/default/secrets",
    "type": "KubernetesResource"
  }
}
```

### Solution architecture

The architecture includes the following stages of security event processing:

- Collecting logs from pod sources and node files.
- Initial selection of records that may contain security events.
- Processing and extracting useful fields.
- Transforming into a unified model and enriching with context.
- Policy-based filtering and delivery to the configured storages.

The architecture separates three stages of the pipeline: **collection**, **processing and enrichment**, and **delivery**. At the delivery stage, sending to several types of storage and analytics systems is supported (e.g., Loki, Elasticsearch, Kafka, Splunk, Vector, File).

#### Processing pipeline

![Processing pipeline diagram](../../images/architecture/security/runtime-audit-security-events.png)

#### Log collection

Collection is performed via the auxiliary [`log-shipper`](/modules/log-shipper/) module:

- Container sources use application logs within the namespace.
- Cluster sources use node files and system service logs (for example, `/var/log/kube-audit/audit.log`, `/var/log/auth.log`).

At the collection stage, only lightweight selective filtering is applied (comparison operators and patterns: `In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`), without deep content processing. This reduces processing load and decreases the amount of irrelevant traffic.

Example of an input log for the selection stage:

```json
{
  "time": "2026-05-10T14:21:03Z",
  "kind": "Event",
  "source": "kube-apiserver",
  "level": "Metadata",
  "message": "Unauthorized",
  "reason": "Unauthorized",
  "code": 401,
  "requestURI": "/api/v1/namespaces/default/secrets",
  "user": "system:serviceaccount:default:demo",
  "sourceIPs": [
    "206.123.145.70"
  ]
}
```

#### Processing and event extraction

After being forwarded to the `gateway`, the log structure is recognized. Standard processing strategies are supported:

- `JSON`: For structured logs.
- `Regex`: For string formats with predictable patterns.
- `Grok`: For complex, non-unified formats.

The processing result is used to derive event signals and build a unified set of fields.

Example of an input log for processing (the same fragment as in the selection stage):

```json
{
  "time": "2026-05-10T14:21:03Z",
  "kind": "Event",
  "source": "kube-apiserver",
  "level": "Metadata",
  "message": "Unauthorized",
  "reason": "Unauthorized",
  "code": 401,
  "requestURI": "/api/v1/namespaces/default/secrets",
  "user": "system:serviceaccount:default:demo",
  "sourceIPs": [
    "206.123.145.70"
  ]
}
```

After processing, the record becomes a source of fields for event classification (code/category/severity/outcome), as well as for building context (`actor`, network attributes, and source metadata).

Example output of the processing stage:

```json
{
  "parsed": {
    "timestamp": "2026-05-10T14:21:03Z",
    "source_component": "kube-apiserver",
    "http_status": 401,
    "request_uri": "/api/v1/namespaces/default/secrets",
    "actor_id": "system:serviceaccount:default:demo",
    "source_ip": "206.123.145.70"
  }
}
```

#### Event transformation and enrichment

Transformation is implemented in two steps:

1. **Transform**: Map fields from the original log to the fields of the target event model.
2. **Enrich**: Add or refine fields from additional context sources (for example, static environment attributes, subject roles, and service indicators).

The order is fixed: `Transform` is applied first, then `Enrich`. In case of a conflict for a target field, the final value is determined by the enrichment stage.

Example data after `Transform`/`Enrich` (based on the same input log):

```json
{
  "id": "2f0de5c2-2e58-4d3f-b4fe-5ec6f1935b9f",
  "timestamp": "2026-05-10T14:21:03Z",
  "source": {
    "component": "kube-apiserver"
  },
  "event": {
    "code": "UNAUTHORIZED_ACCESS",
    "category": "Rbac",
    "severity": "High",
    "outcome": "Failure"
  },
  "actor": {
    "id": "system:serviceaccount:default:demo",
    "type": "ServiceAccount"
  },
  "eventMetadata": {
    "cluster": "prod-cluster",
    "sourceIPs": [
      "206.123.145.70"
    ],
    "requestURI": "/api/v1/namespaces/default/secrets"
  }
}
```

#### Filtering and delivery

After the event is formed, a delivery policy is applied:

- Source filtering
- Filtering by minimum severity
- Routing to one or more destinations

Filtering rules can use both exact source identifiers and source masks, which makes it possible to manage flows at the level of individual services or entire groups.

Destinations can be storage and processing systems such as cluster Loki, external SIEM/log platforms, and streaming buses. The delivery scheme supports parallel delivery to multiple target systems.

### Minimal sufficient lifecycle

In a practical scenario, the architecture works along the following chain:

1. Log sources are connected to the collection pipeline.
2. An initial selection of potentially relevant records is performed.
3. Records are processed and transformed into unified events.
4. Events are enriched with contextual attributes.
5. Filtering and routing rules are applied.
6. Events are delivered to target storages and analytics systems.

The result is a unified and manageable flow of security events suitable for monitoring, investigations, and long-term auditing.

### Implemented security events

Security events in the new format are delivered with the following set of rules:

| Rule                                           | Description                                                                                                                                                                                                   | Source (module)      |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------- |
| Launch Package Management Process in container | Detects execution of a package manager process (`apt`/`yum`/`dnf`/`apk`, etc.) inside a container. Often indicates container drift, installing tools at runtime, or post-compromise activity.                         | `runtime-audit-engine` |
| Drop and execute new binary in container       | Detects execution of a binary inside a container that is not present in the base image (an executable from the upper overlayfs layer). Typical “drop & execute” behavior after gaining access to a container. | `runtime-audit-engine` |
| Container drift detected (chmod)               | Detects permission changes (`chmod`) inside a container that result in an executable file appearing/being enabled. May indicate container drift or an attempt to prepare a malicious tool for execution.        | `runtime-audit-engine` |
| Container drift detected (open+create)         | Detects creation of an executable file inside a container via `open` or `create` followed by execution. Often seen during container drift or when downloading and running malicious binaries.                        | `runtime-audit-engine` |
| Modify binary dirs                             | Detects renaming/deleting files in standard binary directories (`/bin`, `/sbin`, `/usr/bin`, `/usr/sbin`) inside a container. May indicate an attempt to replace system utilities or cover tracks.                    | `runtime-audit-engine` |
| K8s Pod created                                | Detects successful Pod creation in Kubernetes based on audit logs. Useful for tracking new workloads and investigating unexpected launches.                                                                   | `runtime-audit-engine` |
| K8s Pod deleted                                | Detects successful Pod deletion in Kubernetes based on audit logs. Useful for detecting sabotage, attempts to hide activity, and incident analysis.                                                           | `runtime-audit-engine` |
| ServiceAccount created in a system namespace   | Detects creation of a ServiceAccount in system namespaces (`kube-system`/`kube-public`/`default` or `d8-*`). May indicate an attempt to persist in the cluster and obtain additional privileges.                      | `runtime-audit-engine` |
| Attach to cluster-admin Role                   | Detects creation of a ClusterRoleBinding that binds a subject to the cluster-admin role. This is a critical action that grants full administrative access to the cluster.                                     | `runtime-audit-engine` |
| ClusterRole with wildcard created              | Detects creation of a Role/ClusterRole with wildcard resources or verb (`*`) in RBAC rules. Such roles greatly expand permissions and are often a sign of misconfiguration or privilege escalation.           | `runtime-audit-engine` |
| Attach/Exec Pod                                | Detects attempts to exec/attach to a Pod (`exec`/`attach` subresources) via audit logs. May indicate interactive access to a container and possible manual runtime activity.                                      | `runtime-audit-engine` |
| EphemeralContainers created                    | Detects adding ephemeral containers to a Pod. Ephemeral containers are often used for debugging but can also be abused by attackers for covert access.                                                        | `runtime-audit-engine` |
| ClusterRole with write privileges created      | Detects creation of a Role/ClusterRole with write permissions (`create`/`update`/`patch`/`delete`). Such roles allow modifying cluster objects and may be used for escalation or injecting changes.                   | `runtime-audit-engine` |
| ClusterRole with Pod Exec created              | Detects creation of a Role/ClusterRole with access to `pods/exec`. Exec access allows running commands in containers and is often equivalent to a high level of access to workloads.                            | `runtime-audit-engine` |
| System ClusterRole modified/deleted            | Detects modification or deletion of system Role/ClusterRole objects (`system:*`), except for some allowed ones. May indicate attempts to disrupt the cluster or weaken security.                                | `runtime-audit-engine` |
| K8s ServiceAccount created                     | Detects ServiceAccount creation outside system namespaces. May be normal for applications but can also be used to prepare access and later grant RBAC permissions.                                            | `runtime-audit-engine` |
| K8s ModuleConfig modified                      | Detects modification of ModuleConfig objects via audit logs. Changes can affect component behavior and configuration (including security) and should be controlled.                                           | `runtime-audit-engine` |
| K8s ServiceAccount deleted                     | Detects ServiceAccount deletion via audit logs. May indicate attempts to remove access artifacts or changes in application configuration.                                                                     | `runtime-audit-engine` |
| K8s Role/ClusterRole created                   | Detects creation of Role/ClusterRole via audit logs. Used to track RBAC changes and identify unexpected permission expansion.                                                                                 | `runtime-audit-engine` |
| K8s Role/ClusterRole deleted                   | Detects deletion of Role/ClusterRole via audit logs. May indicate attempts to hide traces, roll back settings, or destroy access configuration.                                                               | `runtime-audit-engine` |
| K8s ClusterRoleBinding created                 | Detects creation of ClusterRoleBinding via audit logs. ClusterRoleBinding changes cluster-level role assignments and can be a vector for privilege escalation.                                                | `runtime-audit-engine` |
| K8s ClusterRoleBinding deleted                 | Detects deletion of ClusterRoleBinding via audit logs. It can lead to loss of access or be used to conceal unauthorized assignments.                                                                          | `runtime-audit-engine` |
| Read below containerd images dir               | Detects reading files under containerd directories related to container/CRI data. May indicate attempts to extract information about containers/images or unusual access to runtime data.                     | `runtime-audit-engine` |
| Write below containerd images dir              | Detects writing/modifying files under containerd directories. May indicate attempts to tamper with runtime data, container drift, or supply chain attacks against images.                                     | `runtime-audit-engine` |
| Container tag is not @sha256                   | Detects Pod creation in a system namespace using an image not pinned by digest (`@sha256:`). Using tags without digests complicates integrity control and increases the risk of image substitution.             | `runtime-audit-engine` |
| Inbound SSH Connection                         | Detects an inbound SSH connection to port `22` on the host. It can be legitimate administration, but it is also often used for initial access and should be investigated.                                       | `runtime-audit-engine` |
| Unauthorized request to Kubernetes API         | Detects Kubernetes API requests that ended with the `401 Unauthorized` error in audit logs (excluding health/version endpoints). May indicate token guessing, misconfigured clients, or attacker activity.              | `runtime-audit-engine` |
| Security Reports Created                       | Detects creation of security report objects (`configauditreports`/`vulnerabilityreports`). May indicate a scan run, new vulnerability results, or security-related activity around container security.            | `operator-trivy`       |
| Security Reports Created                       | Detects creation of security report objects (`configauditreports`/`vulnerabilityreports`). May indicate a scan run, new vulnerability results, or security-related activity around container security.            | `user-authn`           |
| Successful login to platform web interface     | Records a successful user login to the platform web interface using their own credentials.                                                                                                                    | `user-authn`           |
| Failed login attempt to platform web interface | Detects failed authentication attempts, which may indicate brute-force password guessing or user error.                                                                                                       | `user-authn`           |
| Account password reset                         | Records user password change or reset events performed by an administrator or via the recovery system.                                                                                                        | `user-authn`           |
| User account lockout                           | Detects user account lockout (manually by an administrator or automatically after exceeding login attempts).                                                                                                  | `user-authn`           |
| User logout                                    | Records proper session termination by a user in the platform web interface.                                                                                                                                   | `user-authn`           |
| Security events data export                    | Detects exporting (downloading) logs or security events from the data storage system.                                                                                                                         | `prometheus`           |
