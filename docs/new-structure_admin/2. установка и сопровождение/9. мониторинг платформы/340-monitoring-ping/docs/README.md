---
title: "The monitoring-ping module"
---

## Description

This module monitors network connectivity between cluster nodes and external nodes (optionally).

Each node sends ICMP packets to all other cluster nodes (as well as to optional external nodes) twice per second and exports the data to `Prometheus`.
It is bundled with a dashboard for `Grafana` that displays the corresponding graphs.

## How does it work?

The module tracks the node's `.status.addresses` field for changes. Upon detecting changes, it invokes a hook that collects a complete list of node names/addresses and passes it to a DaemonSet (the latter recreates the Pods). As a result, ping checks the always up-to-date list of nodes.
