---
title: What should I do if DKP is not receiving updates from the configured channel?
subsystems:
  - deckhouse
lang: en
---

- Ensure the [correct release channel](../admin/configuration/update/configuration.html#checking-the-current-update-mode) is configured.
- Check that DNS resolution for the Deckhouse image registry is working correctly.
  
  Get and compare the IP addresses of `registry.deckhouse.io` from both a node and the `deckhouse` Pod.
  They must match.

  Example of obtaining an IP of `registry.deckhouse.io` from a node:

  ```shell
  getent ahosts registry.deckhouse.io
  ```

  Example output:

  ```console
  185.193.90.38    STREAM registry.deckhouse.io
  185.193.90.38    DGRAM
  185.193.90.38    RAW
  ```

  Example of obtaining an IP of `registry.deckhouse.io` from the `deckhouse` Pod:

  ```shell
  d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- getent ahosts registry.deckhouse.io
  ```

  Example output:

  ```console
  185.193.90.38    STREAM registry.deckhouse.io
  185.193.90.38    DGRAM  registry.deckhouse.io
  ```

  If the resulted IPs do not match, check DNS settings on the node.
  Pay attention to the `search` domain list in `/etc/resolv.conf`, which affects name resolution in the `deckhouse` Pod.
  If the `search` parameter in `/etc/resolv.conf` file specifies a domain with wildcard DNS resolution configured,
  this may lead to incorrect IP address resolution for the Deckhouse image registry (see example below).

#### Example DNS settings that may cause issues resolving the DKP image registry IP address

Below is an example of how DNS settings may result in different resolution behavior on the node and in a Kubernetes Pod:

- Example of `/etc/resolv.conf` on the node:

  ```text
  nameserver 10.0.0.10
  search company.my
  ```

  > On nodes, the default `ndot` setting is **1** (`options ndots:1`), while in Kubernetes Pods, it’s **5**.
  > This causes different resolution logic for DNS names with 5 or fewer dots on a node and on the Pod.

- The DNS zone `company.my` has a wildcard entry `*.company.my` that resolves to `10.0.0.100`.
  This means any undefined DNS name in the `company.my` zone resolves to `10.0.0.100`.

Taking into account the `search` parameter in `/etc/resolv.conf`, when accessing `registry.deckhouse.io` from a node,
the system will attempt to resolve the IP address for `registry.deckhouse.io`
(because it considers it fully qualified due to the default `options ndots:1` setting).

However, when accessing `registry.deckhouse.io` from a Kubernetes Pod,
considering the `options ndots:5` setting used by default in Kubernetes and the `search` parameter,
the system will first attempt to resolve the name `registry.deckhouse.io.company.my`.
This name will resolve to the IP address `10.0.0.100` because,
according to the `company.my` DNS zone's wildcard configuration,
`*.company.my` is resolved to `10.0.0.100`.
As a result, the Pod will fail to connect to the `registry.deckhouse.io` host and will be unable to download information about available Deckhouse updates.
