---
title: "The loki module"
---

The module organizes log storage.

The module uses the [Grafana Loki](https://grafana.com/oss/loki/) project.

The module deploys log storage based on Grafana Loki, configures the [log-shipper](../460-log-shipper) module to use loki if necessary, and adds the corresponding data source to Grafana.
