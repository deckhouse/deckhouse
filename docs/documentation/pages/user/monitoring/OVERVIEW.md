---
title: "Overview"
permalink: en/user/monitoring/
---

Deckhouse Kubernetes Platform (DKP) provides a convenient and ready-to-use Kubernetes cluster monitoring system.

By default, monitoring collects a large number of metrics and contains configured triggers for tracking the general state of applications, as well as provides access to them in the form of convenient dashboards in the Grafana web interface.

It is also possible to configure collection of custom metrics from applications deployed in the cluster.

Key features:

- **Ready-made dashboards** in Grafana with graphs for CPU, memory, disk and network load: Can be viewed by pods, nodes or namespaces.
- **Useful notifications** in Slack/Telegram/email about problems: Service unavailability, disk space shortage, approaching certificate expiration.
- **Simple integration**: To start monitoring your application, it is enough to add a couple of annotations to Pod or Service.

Everything works "out of the box" — no complex configuration is required.
