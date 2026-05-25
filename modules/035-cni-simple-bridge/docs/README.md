---
title: "The cni-simple-bridge module"
description: "Ensuring network operation in the Deckhouse Kubernetes Platform cluster with limited functionality."
---

The module does not have any settings.

It is automatically enabled for the following cloud providers:

- [AWS](/modules/cloud-provider-aws/)
- [Azure](/modules/cloud-provider-azure/)
- [GCP](/modules/cloud-provider-gcp/)
- [Yandex](/modules/cloud-provider-yandex/)

{% alert level="info" %}
Starting with DKP version 1.77 for AWS, Azure, and GCP, and 1.76 for Yandex Cloud, the default CNI for new clusters is `cilium`. Existing clusters keep the current CNI configuration.
{% endalert %}
