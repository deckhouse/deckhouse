---
title: Integrity control
permalink: en/architecture/security/integrity-control.html
---

Integrity control is a set of mechanisms for verifying containers to ensure their security
and compliance with the specified configuration.

In Deckhouse Kubernetes Platform (DKP), integrity control works:

- When application containers are started
- While application containers are running

![Integrity control in DKP](../../images/architecture/security/integrity-control-en.png)

## Integrity control when starting containers

DKP provides integrity control of application containers at the container runtime (CRI) level.

After downloading an application image, DKP verifies its integrity by checking the SHA-256 hash.  
A container can only be started if the checksum verification succeeds.

The sequence of integrity checks at startup:

1. The image is loaded into the node's local storage.
1. Image metadata is extracted, including the SHA-256 hash.
1. SHA-256 hash is verified by comparing it with the reference value.
1. If the hashes match, the check passes. If they don't match, the image is not started.

To enhance security, you can also configure image pull policies
using [security policies](../../admin/configuration/security/policies.html) to ensure
that only up-to-date image versions are used for container startup.

## Integrity control of running containers

Security event auditing in DKP includes analyzing Linux kernel events and Kubernetes API audit events.
This makes it possible to track whether applications in pods are running unchanged, conform to their expected state,
and have not been modified.

Auditing uses:

- Built-in rules
- Custom rules that can be added using the [Falco condition syntax](https://falco.org/docs/concepts/rules/conditions/)

Integrity control of running containers can detect threats such as launching command-line shells inside containers or pods,
discovering containers running in privileged mode, mounting insecure paths into containers, or attempts to read sensitive data.

For more details on configuring security audits, refer to [Security event audit](../../admin/configuration/security/events/runtime-audit.html).
