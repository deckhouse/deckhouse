---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If Kubernetes cluster nodes are analyzed by security scanners (antivirus tools), you may need to configure them to avoid false positives.

Deckhouse Kubernetes Platform (DKP) uses the following directories during operation ([download in CSV](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Security Software

### KESL

This section provides recommendations for configuring Kaspersky Endpoint Security for Linux (KESL) to ensure proper functionality with the Deckhouse Kubernetes Platform, regardless of its edition.

To ensure compatibility with DKP, the following tasks must be disabled on the KESL side:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat Protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
The list of tasks may differ in future versions of KESL.
{% endalert %}

Ensure that your Kubernetes nodes meet the minimum resource requirements specified for [DKP](https://deckhouse.io/products/kubernetes-platform/guides/production.html#resource-requirements) and [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/197642.htm).

For the combined use of KESL and Deckhouse, performance optimization may be required according to [Kaspersky's recommendations](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/206054.htm).
