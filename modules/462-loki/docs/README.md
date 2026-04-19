---
title: "The loki module"
description: "Log storage in the Deckhouse Kubernetes Platform cluster based on Grafana Loki."
---

In Kubernetes, the system logs on the nodes do not last long and may be lost during a restart or update. This module deploys its own operational log storage based on [Grafana Loki](https://grafana.com/oss/loki/) in the cluster..

Module features:

- system logs are automatically logged into Loki without additional configuration.;
- access to the logs is implemented via Grafana and the Deckhouse Kubernetes Platform web interface (console);
- the module is designed to store logs for a short time. For long-term storage or archiving, it is recommended to use external systems supported via [log-shipper](../log-shipper/).
