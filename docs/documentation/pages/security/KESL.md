---
title: KESL
permalink: en/security/kesl.html
lang: en
---

## KESL

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

If KESL and DKP are run together, you may be required to do some performance tuning as per [Kaspersky recommendations](https://support.kaspersky.com/KES4Linux/12.1.0/en-US/206054.htm).
