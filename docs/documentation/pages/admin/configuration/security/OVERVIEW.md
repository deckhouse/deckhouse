---
title: Overview
permalink: en/admin/configuration/security/
description: "Configure security features in Deckhouse Kubernetes Platform including certificates, audit logging, runtime security, scanning, and security policies. Complete security hardening guide."
---

The "Security" section covers security features in Deckhouse Kubernetes Platform.
It contains recommendations, instructions, and configuration examples for built-in protection mechanisms,
as well as integration with external systems.

In this section, you will find information on:

- Security event audit:
  - How to enable and configure Kubernetes API event audit.
  - How to collect security events at the kernel and Kubernetes API levels
    using the platform's built-in capabilities (Falco).
  - How to configure audit rules and receive alerts on suspicious activity.

- Security policies:
  - Support for Pod Security Standards.
  - Configuring operational and advanced security policies using Gatekeeper.
  - Verifying container image signatures.
  - Working with custom policies and exceptions.

- Image vulnerability scanning:
  - How to set up regular scanning of container images.
  - How to view scan results and manually trigger rescans.

- Certificate management:
  - Issuing, renewing, and managing TLS certificates using the [`cert-manager`](/modules/cert-manager/) module.
  - Examples of using Let’s Encrypt, HashiCorp Vault, self-signed, and external CAs.
  - Support for `HTTP-01` and `DNS-01` validation types.

- Integration with external monitoring and security systems:
  - Sending logs to Kaspersky Unified Monitoring and Analysis Platform (KUMA).
  - Configuring exclusions for antivirus solutions, using Kaspersky Endpoint Security for Linux (KESL) as an example.
