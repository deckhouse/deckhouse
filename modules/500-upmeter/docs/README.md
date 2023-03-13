---
title: "The upmeter module"
webIfaces:
- name: status
- name: upmeter
---

The module collects statistics by availability type for cluster components and Deckhouse. It enables evaluating the degree of SLA compliance for these components, presents availability data via a web interface, and provides a web page with the operating statuses of the cluster components.

You can export availability metrics over the [Prometheus Remote Write](https://docs.sysdig.com/en/docs/installation/prometheus-remote-write/) protocol using the [UpmeterRemoteWrite](cr.html#upmeterremotewrite) Custom Resource.

Module composition:
- **agent** — probes the availability of components and sends the results to the server; runs on the master nodes;
- **upmeter** — aggregates the results and implements the API server to retrieve them;
- **front**
  - **status** — shows the current availability level over the previous 10 minutes (this one requires authorization by default, but you can disable it);
  - **webui** — is a dashboard with statistics on probes and availability groups (requires authorization);
- **smoke-mini** — continuous *smoke testing* using a StatefulSet that looks like an actual application.

The module sends about 100 metric readings every 5 minutes. This figure depends on the number of Deckhouse modules enabled.

## Interface

Example of a web interface:
![Example of a web interface](../../images/500-upmeter/image1.png)

Example of Grafana plots based on upmeter metrics:
![Example of Grafana plots based on upmeter metrics](../../images/500-upmeter/image2.png)
