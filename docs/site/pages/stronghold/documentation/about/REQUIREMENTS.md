---
title: "Requirements"
permalink: en/stronghold/documentation/about/requirements.html
---

## System requirements

Since the way each customer uses the secrets storage may vary, the following recommendations should be considered as a starting point. The scope of required resources depends on operations performed by the Stronghold cluster.

Below are the two main cluster types considered depending on their purpose:

- **Small clusters**: Suitable for initial deployments, as well as development and testing environments.
- **Large clusters**: Intended for production environments with consistently high workloads. This may involve a large number of transactions, secrets, or a combination of both conditions.

|  | Small cluster | Large cluster |
| :--- | :--- | :--- |
| CPU | 4–8 cores | 8–16 cores |
| Memory | 8–16 GB | 16–32 GB |
| Disk input/output | 3000+ IOPS | 3000+ IOPS |
| Disk input/output | 70+ MB/s | 200+ MB/s |

Depending on the expected number and type of operations, consider the following requirements:

| Operation | 4 cores | 16 cores |
| :--- | :---  | :--- |
| Authorization (token retrieval) | Up to 20 OPS | Up to 100 OPS |
| Reading a key (up to 1 KB) | Up to 500 OPS | Up to 7000 OPS |
| Writing a key | Up to 30 OPS | Up to 150 OPS |

## Supported OS

| Linux distribution          | Supported versions              |
| --------------------------- | ------------------------------- |
| CentOS                      | 7, 8, 9                         |
| Debian                      | 10, 11, 12                      |
| Ubuntu                      | 18.04, 20.04, 22.04, 24.04      |
