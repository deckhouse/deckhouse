---
title: "Monitoring network interaction between all cluster nodes, as well as (optionally) up to additional external nodes"
description: "Monitoring network interaction between all cluster nodes, as well as (optionally) up to additional external nodes"
---

## Description

The network communication monitoring module provides continuous connectivity verification between all the main and, if necessary, external nodes of the cluster.

Module features:

- automatically checks the availability of all cluster nodes (and, optionally, external systems) using ICMP (ping) — testing is started every two seconds;
- all results are exported in metrics format to the Prometheus monitoring system.;
- included is a ready—made dashboard for Grafana, where current availability, delay schedules, and potential network connectivity issues are visualized in real time;
- allows you to quickly identify nodes with degraded connectivity and speeds up the response to incidents.

## How does it work?

The module tracks the node's `.status.addresses` field for changes. Upon detecting changes, it invokes a hook that collects a complete list of node names/addresses and passes it to a DaemonSet (the latter recreates the Pods). As a result, ping checks the always up-to-date list of nodes.
