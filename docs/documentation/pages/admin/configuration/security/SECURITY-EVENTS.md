---
title: Security events
permalink: en/admin/configuration/security/events/security-events.html
description: "Configure collection, processing, and delivery of security events in Deckhouse Kubernetes Platform. Unified security events pipeline from application and Kubernetes infrastructure logs."
---

Deckhouse Kubernetes Platform (DKP) provides tools for declarative collection, processing,
normalization, and delivery of security events extracted from logs of applications
and Kubernetes infrastructure components.

A security event is a structured record of an action or fact that is significant from an information security perspective.

DKP lets you:

- Collect security events from various sources (pod logs, node files, Kubernetes API audit).
- Bring events to a unified format with a mandatory minimum set of attributes.
- Enrich events with contextual data.
- Filter events by sources and severity level.
- Deliver events to storage and analytics systems (Loki, Elasticsearch, Kafka, Splunk, Vector, and others).

The [`security-events-manager`](/modules/security-events-manager/) module is responsible for collecting, processing, and delivering security events.
The auxiliary [`log-shipper`](/modules/log-shipper/) module is used for log collection.

## Dependencies and requirements

The `security-events-manager` module requires the following DKP modules:

- [`log-shipper`](/modules/log-shipper/): Collects logs from pod sources and node files, performs preliminary record selection.
- [`loki`](/modules/loki/): Provides in-cluster storage for security events (used by default as a destination).

For details about the security events architecture, refer to ["Security events architecture"](../../../../architecture/security/security-events.html).

## Data sources for security events

The `security-events-manager` module collects data from two types of sources:

- **Container sources**: Application logs in Kubernetes pods. Collection is performed via the `log-shipper` module, which selects records by pod labels and namespaces.
- **Cluster sources**: Node files and system service logs not bound to a specific namespace. For example:
  - `/var/log/kube-audit/audit.log`: Kubernetes API audit log.
  - `/var/log/auth.log`: Node system authentication event log.

Collection is made in two steps:

1. The `log-shipper` module performs preliminary selection of log records using simple comparison operators (`In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`) and forwards them to the `security-events-manager` gateway.
1. The `security-events-manager` gateway parses the records, transforms them to a unified model, and delivers to a configured destination.

Since log parsing is resource-intensive, it is performed only for records pre-selected as potentially containing security events.

## Enabling security events

To enable security events, follow these steps:

1. Enable the required modules if they are not yet enabled:

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

All available `security-events-manager` parameters are listed in the [module documentation](/modules/security-events-manager/configuration.html).

## Configuring event sources

Log collection and processing are configured through the following custom resources:

- [PodSecurityEventShipper](/modules/security-events-manager/cr.html#podsecurityeventshipper): For container sources (pod logs in a specific namespace).
- [ClusterSecurityEventShipper](/modules/security-events-manager/cr.html#clustersecurityeventshipper): For cluster sources (node files, system service logs).

These resources configure:

- Log source (`source` and `input` fields)
- Preliminary record selection rules (`produces` field)
- Parsing rules (`parser` or `parserRef` field with a reference to SecurityEventLoggingTransformationRules or ClusterSecurityEventLoggingTransformationRules)
- Transformation and enrichment rules (`transform` and `enrich` fields)

PodSecurityEventShipper configuration example defining a container source:

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

ClusterSecurityEventShipper configuration example defining a cluster source:

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

## Configuring event delivery

After events are formed, you should define the destinations they will be delivered to.
For this, you need to:

1. Configure security event destinations.
1. Configure event delivery rules to destinations.

### Configuring a destination

Destinations are configured via the [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination) resource.

The `security-events-manager` module supports all destination types available in the `log-shipper` ecosystem (Loki, Elasticsearch, Kafka, Splunk, Vector, File, and others).
For in-cluster security event storage, automatic Loki destination configuration is available when the corresponding module option is enabled.

ClusterSecurityEventDestination configuration example defining the Loki destination:

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

### Configuring event delivery rules

When configuring security event delivery rules, you need to define:

- Sources that will be sending the events
- Minimum severity level for sending
- Target destinations

For this, use the [ClusterSecurityEventConfig](/modules/security-events-manager/cr.html#clustersecurityeventconfig) resource.
It defines sources (in the format of exact names or masks), minimum severity level, and an array of destinations
(resources of type [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination))
that will receive the selected events.

ClusterSecurityEventConfig configuration example for sending events from the `kube-audit`, `runtime-audit-engine` and `user-authn` sources to the `cluster-loki` destination:

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
  # Alternatively, use masks.
  # enabledSourcesMasks:
  #   - clusterSecurityEventShipper/kube-audit/*
  #   - podSecurityEventShipper/*

  destinations:
    - cluster-loki
```

## Default module settings

If the module settings are not explicitly specified, the following objects are created in the cluster:

- [ClusterSecurityEventConfig](/modules/security-events-manager/cr.html#clustersecurityeventconfig): Configures security event delivery to destinations. By default, an object with the following configuration is created:

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

  This object configuration is controlled by the module parameter [`securityEventConfig`](/modules/security-events-manager/configuration.html#parameters-securityeventconfig).

- [ClusterSecurityEventDestination](/modules/security-events-manager/cr.html#clustersecurityeventdestination): Configures security event destination. By default, the following object is generated that allows sending events to the in-cluster Loki service:

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
        token: <token> # Filled automatically.
      endpoint: https://loki.d8-monitoring:3100
  ```

  You can disable generation of the default destination via the dedicated parameter [`clusterSecurityEventDestination.clusterLoki`](/modules/security-events-manager/configuration.html#parameters-clustersecurityeventdestination-clusterloki).

## Supported security events

The `security-events-manager` module ships with a built-in set of security event detection rules
covering authentication, configuration, RBAC, runtime, and other categories.
For the up-to-date list of implemented security events, their codes, severity levels, and descriptions,
refer to the [module documentation](/modules/security-events-manager/security_events.html).
