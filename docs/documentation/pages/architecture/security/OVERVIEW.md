---
title: Security subsystem
permalink: en/architecture/security/
search: security, security subsystem
description: Architecture of the Security subsystem in Deckhouse Kubernetes Platform.
extractedLinksOnlyMax: 0
extractedLinksMax: 0
---

This subsection describes the architecture of the Security subsystem of Deckhouse Kubernetes Platform (DKP).

The Security subsystem includes the following modules:

* [`admission-policy-engine`](/modules/admission-policy-engine/): Lets you use security policies in the cluster according to Kubernetes [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/). The module uses [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/docs/) to enforce these policies.
* [`runtime-audit-engine`](/modules/runtime-audit-engine/): Implements an internal threat detection system.
* [`operator-trivy`](/modules/operator-trivy/): Performs periodic vulnerability scanning of the DKP cluster.
* [`cert-manager`](/modules/cert-manager/): Manages TLS certificates in the cluster.
* [`secrets-store-integration`](/modules/secrets-store-integration/): Delivers secrets to Kubernetes applications by integrating secrets, keys, and certificates stored in external secret stores.
* [`secret-copier`](/modules/secret-copier/): Automatically copies secrets to cluster namespaces.

The following Security subsystem components are currently described in this subsection:

* [Integrity control](integrity-control.html)
* [Security event auditing](runtime-audit.html)
