---
title: "The loki module"
---

The module implements log storage.

The module uses the Grafana Loki project.

The module deploys log storage based on Grafana Loki, configures the [log-shipper](../log-shipper/) module to use the loki module if necessary, and adds the corresponding data source to Grafana.

{% alert level="warning" %}
The module works only in standalone mode and doesn't support high availability for now. Thus, its usage is limited, and it's recommended to push crucial logs to other storage.
{% endalert %}
