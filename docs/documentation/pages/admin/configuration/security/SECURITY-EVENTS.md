---
title: Security events
permalink: en/admin/configuration/security/events/security-events.html
description: "Configure collection, processing, and delivery of security events in Deckhouse Kubernetes Platform. Unified security events pipeline from application and Kubernetes infrastructure logs."
---

Deckhouse Kubernetes Platform (DKP) provides tools for declarative collection, processing,
normalization, and delivery of security events extracted from logs of applications
and Kubernetes infrastructure components.

A security event is a structured record of an action or fact that is significant from an information security perspective.
DKP allows you to:

- Collect security events from various sources (pod logs, node files, Kubernetes API audit);
- Normalize events to a unified format with a mandatory minimum set of attributes;
- Enrich events with contextual data;
- Filter events by sources and severity level;
- Deliver events to storage and analytics systems (Loki, Elasticsearch, Kafka, Splunk, Vector, etc.).

## Module responsible for security events

The [`security-events-manager`](/modules/security-events-manager/) module is responsible for collecting, processing, and delivering security events.
This module uses the auxiliary [`log-shipper`](/modules/log-shipper/) module for log collection.

## Dependencies and requirements

The `security-events-manager` module requires the following DKP modules:

- [`log-shipper`](/modules/log-shipper/) — collects logs from pod sources and node files, performs preliminary record selection;
- [`loki`](/modules/loki/) — provides in-cluster storage for security events (used by default as a destination).

{% alert level="warning" %}
The `security-events-manager` module is in the `Experimental` stage and may change in future releases.
{% endalert %}

For details about the security events architecture, refer to [Architecture](../../../../architecture/security/security-events.html).

## Data sources for security events

The `security-events-manager` module collects data from two types of sources:

- **Container sources** — application logs in Kubernetes pods. Collection is performed via the `log-shipper` module, which selects records by pod labels and namespaces.
- **Cluster sources** — node files and system service logs not bound to a specific namespace. For example:
  - `/var/log/kube-audit/audit.log` — Kubernetes API audit logs;
  - `/var/log/auth.log` — node system authentication events.

A two-tier collection scheme is used:

1. The `log-shipper` module performs preliminary selection of log records using simple comparison operators (`In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`) and forwards them to the `security-events-manager` gateway.
1. The `security-events-manager` gateway performs field recognition (parsing), processing, transformation to a unified model, and delivery.

Because log parsing is resource-intensive, it is performed only for records pre-selected as potentially containing security events.

## Enabling security events

1. Enable the required dependency modules if they are not already enabled:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: log-shipper
   spec:
     enabled: true
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: loki
   spec:
     enabled: true
   ```

1. Enable the `security-events-manager` module:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: security-events-manager
   spec:
     enabled: true
   ```

All available module parameters are listed in the [`security-events-manager`](/modules/security-events-manager/configuration.html) module documentation.

## Configuring collection sources

Log collection and processing are configured through the following custom resources:

- [`PodSecurityEventShipper`](/modules/security-events-manager/cr.html#podsecurityeventshipper) — for container sources (pod logs in a specific namespace);
- [`ClusterSecurityEventShipper`](/modules/security-events-manager/cr.html#clustersecurityeventshipper) — for cluster sources (node files, system services).

These resources configure:

1. Log source (`source` and `input` fields);
1. Preliminary record selection rules (`produces` field);
1. Parsing rules (`parser` field or `parserRef` with a reference to `SecurityEventLoggingTransformationRules` / `ClusterSecurityEventLoggingTransformationRules`);
1. Transformation and enrichment rules (`transform` and `enrich` fields).

Example of a container source configuration:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: PodSecurityEventShipper
metadata:
  name: runtime-audit-engine
  namespace: d8-runtime-audit-engine
spec:
  - source: runtime-audit-engine
    input:
      type: KubernetesPods
      kubernetesPods:
        labelSelector:
          matchLabels:
            app: runtime-audit-engine
    produces:
      - eventCode: K8S_POD_CREATED
        extract:
          field: message
          operator: Regex
          values:
            - ".*K8s Pod Created.*"
```

Example of a cluster source configuration:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventShipper
metadata:
  name: kube-audit
spec:
  - source: kube-audit
    input:
      type: File
      files:
        - /var/log/kube-audit/audit.log
    produces:
      - eventCode: UNAUTHORIZED_ACCESS
        extract:
          field: message
          operator: Regex
          values:
            - ".*\"code\":401.*"
```

## Configuring security event delivery

After events are formed, you must define where they should be delivered.
For this, you need to:

1. Configure security event destinations.
1. Configure event delivery rules to destinations.

### Destination configuration

Destinations are configured via the [`ClusterSecurityEventDestination`](/modules/security-events-manager/cr.html#clustersecurityeventdestination) resource.

Destination types are aligned with the `log-shipper` ecosystem (Loki, Elasticsearch, Kafka, Splunk, Vector, File, etc.).
For in-cluster security event storage, automatic `Loki` destination configuration is available when the corresponding module option is enabled.

Example:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventDestination
metadata:
  name: cluster-loki
spec:
  type: Loki
  loki:
    auth:
      strategy: Bearer
      token: <EXAMPLE>
    endpoint: https://loki.d8-monitoring:3100
    tls:
      verifyCertificate: true
      verifyHostname: true
      ca: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0t...
```

### Delivery rule configuration

When configuring delivery rules, you need to define:

- Which source events are sent;
- Minimum severity level for sending;
- Target destinations.

For this, use the [`ClusterSecurityEventConfig`](/modules/security-events-manager/cr.html#clustersecurityeventconfig) resource.
The resource defines sources (exact names or masks), minimum `severity`, and an array of destinations
(resources of type [`ClusterSecurityEventDestination`](/modules/security-events-manager/cr.html#clustersecurityeventdestination))
that receive selected events.

Example:

```yaml
apiVersion: security.deckhouse.io/v1alpha1
kind: ClusterSecurityEventConfig
metadata:
  name: default
spec:
  defaultSeverityThreshold: Low
  enabledSources:
    - clusterSecurityEventShipper/kube-audit/kube-audit
    - podSecurityEventShipper/d8-runtime-audit-engine/runtime-audit-engine/falco
    - podSecurityEventShipper/d8-user-authn/user-authn/dex
  # OR
  # enabledSourcesMasks:
  #   - clusterSecurityEventShipper/kube-audit/*
  #   - podSecurityEventShipper/*

  destinations:
    - cluster-loki
```

## Default module settings

If module settings are not explicitly specified, the following objects are created in the cluster:

1. `ClusterSecurityEventConfig` — configures security event delivery to destinations. The following settings are used by default:

    ```yaml
    apiVersion: security.deckhouse.io/v1alpha1
    kind: ClusterSecurityEventConfig
    metadata:
      name: default
    spec:
      defaultSeverityThreshold: Medium
      destinations:
        - cluster-loki
      enabledSourcesMasks:
        - podSecurityEventShipper/*
        - clusterSecurityEventShipper/*
    ```

    Configuration of these parameters is controlled by module setting [`securityEventConfig`](/modules/security-events-manager/configuration.html#securityeventconfig).

1. `ClusterSecurityEventDestination` — configures security event storage. By default, an object is generated that allows sending security events to the in-cluster `loki` service:

    ```yaml
    apiVersion: security.deckhouse.io/v1alpha1
    kind: ClusterSecurityEventDestination
    metadata:
      name: cluster-loki
    spec:
      type: Loki
      loki:
        auth:
          strategy: Bearer
          token: <token> # Filled automatically
        endpoint: https://loki.d8-monitoring:3100
    ```

    You can disable generation of the default destination via the dedicated parameter [`clusterSecurityEventDestination.clusterLoki`](/modules/security-events-manager/configuration.html#clustersecurityeventdestination).

## Implemented security events

The module ships with a built-in set of security event detection rules
covering authentication, configuration, RBAC, runtime, and other categories.
For the up-to-date list of implemented security events, their codes, severity levels, and descriptions,
refer to the [`security-events-manager`](/modules/security-events-manager/security_events.html) module documentation.
