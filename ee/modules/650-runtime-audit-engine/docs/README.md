---
title: "The runtime-audit-engine module"
---

## Overview

The module implements runtime threats detection engine. 
It can collect Linux core system calls and Kubernetes API audit data, enrich these events with metadata from Kubernetes pods


## Architecture

TBA

## How to customize audit rules

There is a custom resource definition to load custom rules: `FalcoAuditRules.` 
Each Falco agent pod has a sidecar container with a [shell-operator](https://github.com/flant/shell-operator) instance.
This sidecar reads rules from custom resources and stores it on Pod's filesystem in `/etc/falco/rules.d/` folder.
Falco will automatically reload the configuration once a new rule appears.

![falco shell-operator](../../images/650-runtime-audit-engine/falco_shop.png)

This schema helps to use the IaC approach to maintain Falco rules.

## Requirements

### OS

The module uses the eBPF Falco driver to ingest syscall data. It works better for environments where loading a kernel module is untrusted or not supported, e.g., GKE, EKS, and other Managed Kubernetes solutions.
Yet there are known limitations for the eBPF driver:
* The eBPF probe may not work for every system.
* At least Linux kernel version 4.14 is required, but the Falco project suggests an LTS kernel of 4.14/4.19 or above.

### CPU / Memory

Falco agents are deployed on every node. Resources consumption of each Pod depends on the number of rules / ingested events.
