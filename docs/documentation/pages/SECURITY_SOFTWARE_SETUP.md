---
title: Security software settings for working with Deckhouse
permalink: en/security_software_setup.html
---

If Kubernetes cluster nodes are analyzed by security scanners (antivirus tools), you may need to configure them to avoid false positives.

Deckhouse Kubernetes Platform (DKP) uses the following directories when running ([download in CSV](deckhouse-directories.csv)):

{% include security_software_setup.liquid %}

## Security Software

### KESL

The following are recommendations for configuring Kaspersky Endpoint Security for Linux (KESL) to ensure that it operates smoothly with Deckhouse Kubernetes Platform (whatever edition you choose).

To ensure compatibility with DKP, the following tasks must be disabled on the KESL side:

- `Firewall_Management (ID: 12)`.
- `Web Threat Protection (ID: 14)`.
- `Network Threat Protection (ID: 17)`.
- `Web Control (ID: 26)`.

{% alert level="info" %}
Note that the task list may be different in future KESL versions.
{% endalert %}

Ensure that your Kubernetes nodes meet the minimum resource requirements specified for [DKP](https://deckhouse.io/products/kubernetes-platform/guides/production.html#resource-requirements) and [KESL](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/197642.htm).

If KESL and Deckhouse are run together, you may be required to do some performance tuning as per [Kaspersky recommendations](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/206054.htm).
