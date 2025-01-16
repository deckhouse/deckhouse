---
title: "The upmeter module"
webIfaces:
- name: status
- name: upmeter
---

The `upmeter` module collects statistics by availability type for cluster components and Deckhouse.

The module also:

* Evaluates the degree of SLA fulfillment on components.
* Displays availability data in the web interface.
* Generates a web page with the status of the cluster components.

You can export availability metrics over the [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/) protocol using the [UpmeterRemoteWrite](cr.html#upmeterremotewrite) custom resource.

Module composition:

- **agent** — runs on master nodes, performs availability tests, and sends results to the server.
- **upmeter** — collects results and supports an API server to retrieve them.
- **front**
  - **status** — shows the availability level for the last 10 minutes (requires authorization, but it can be disabled);
  - **webui** — shows a dashboard with statistics on probes and availability groups (requires authorization).
- **smoke-mini** — supports continuous *smoke testing* using StatefulSet.

The module sends about 100 metric readings every 5 minutes. This figure depends on the number of Deckhouse Kubernetes Platform modules enabled.

## Interface

Example of a web interface:
![Example of a web interface](../../images/upmeter/image1.png)

Example of Grafana plots based on upmeter metrics:
![Example of Grafana plots based on upmeter metrics](../../images/upmeter/image2.png)
