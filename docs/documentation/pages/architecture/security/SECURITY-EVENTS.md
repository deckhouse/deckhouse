---
title: Security events architecture
permalink: en/architecture/security/security-events.html
search: security events, security-events-manager, event collection, event delivery
description: Architecture for collecting, processing, and delivering security events in Deckhouse Kubernetes Platform.
---

{% alert level="warning" %}
The `security-events-manager` module is in the `Experimental` stage and may change in future releases.
{% endalert %}

## Purpose

The [`security-events-manager`](/modules/security-events-manager/) module performs declarative collection, processing,
normalization, and delivery of security events extracted from logs of applications
and Kubernetes infrastructure components.

A security event is a structured record of an action or fact that is significant from an information security perspective.
Typical categories of such events include:

- Authentication and authorization
- Access to APIs and configuration
- Changes to cluster objects
- Runtime and network activity anomalies

Regardless of the original log format, the output is a uniform event model
with a mandatory minimum set of attributes.

Key idea: a security event is information from logs of various services,
normalized into a single contract.

## Solution architecture

The architecture is intended to build a unified pipeline for working with security events
extracted from the logs of applications and Kubernetes infrastructure components.
The module performs three processing stages:

1. **Collection** — collecting logs from pod sources and node files, initial selection of records
   that may contain security events.
2. **Processing and enrichment** — parsing logs, extracting event signals,
   transforming into a unified model, and enriching with context.
3. **Delivery** — policy-based filtering and delivery to configured storages.

At the delivery stage, sending to several types of storage and analytics systems is supported
(e.g., Loki, Elasticsearch, Kafka, Splunk, Vector, File).

### Processing pipeline

![Processing pipeline diagram](../../images/architecture/security/security-events-manager.png)

The architecture separates three stages of the pipeline: **collection**, **processing and enrichment**, and **delivery**.

### Collection layer (log-shipper)

Collection is performed via the auxiliary [`log-shipper`](/modules/log-shipper/) module:

- Container sources use application logs within the namespace.
- Cluster sources use node files and system service logs (for example, `/var/log/kube-audit/audit.log`, `/var/log/auth.log`).

A two-tier scheme is used:

- the `log-shipper` module performs preliminary selection of log records using simple comparison
  operators (`In`, `NotIn`, `Regex`, `NotRegex`, `Exists`, `DoesNotExist`) and forwards them to the gateway;
- the `security-events-manager` module (gateway) performs field recognition (parsing),
  further processing, and delivery.

Because log parsing is resource-intensive, it is performed only for records pre-selected as potentially
containing security events, not for all incoming logs. Therefore, the initial selection stage does not perform
deep field-value filtering that requires full content parsing.

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

### Processing and event extraction

After being forwarded to the `gateway`, the log structure is recognized.
Standard processing strategies are supported:

- `JSON` — for structured logs;
- `Regex` — for string formats with predictable patterns;
- `Grok` — for complex, non-unified formats.

The processing result is used to derive event signals and build a unified set of fields.

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

After processing, the record becomes a source of fields for event classification
(code/category/severity/outcome), as well as for building context
(`actor`, network attributes, and source metadata).

### Event transformation and enrichment

Transformation is implemented in two steps:

1. **Transform** — map fields from the original log to the fields of the target event model.
2. **Enrich** — add or refine fields from additional context sources
   (for example, static environment attributes, subject roles, and service indicators).

The order is fixed: `Transform` is applied first, then `Enrich`.
In case of a conflict for a target field, the final value is determined by the enrichment stage.

### Filtering and delivery

After the event is formed, a delivery policy is applied:

- Source filtering
- Filtering by minimum severity
- Routing to one or more destinations

Filtering rules can use both exact source identifiers and source masks,
which makes it possible to manage flows at the level of individual services or entire groups.

Destinations can be storage and processing systems such as cluster Loki, external SIEM/log platforms,
and streaming buses. The delivery scheme supports parallel delivery to multiple target systems.

## Security event model

Below is the structure of the event that is sent to storage.

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

## Minimal sufficient lifecycle

In a practical scenario, the architecture works along the following chain:

1. Log sources are connected to the collection pipeline.
2. An initial selection of potentially relevant records is performed.
3. Records are processed and transformed into unified events.
4. Events are enriched with contextual attributes.
5. Filtering and routing rules are applied.
6. Events are delivered to target storages and analytics systems.

The result is a unified and manageable flow of security events
suitable for monitoring, investigations, and long-term auditing.

## Implemented security events

The module ships with a built-in set of security event detection rules
covering authentication, configuration, RBAC, runtime, and other categories.
For the up-to-date list of implemented security events, their codes, severity levels, and descriptions,
refer to the [`security-events-manager`](/modules/security-events-manager/security_events.html) module documentation.
