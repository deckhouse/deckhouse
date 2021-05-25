---
title: "The upmeter module"
---

This module monitors the state of the cluster and displays the status page with the availability level (SLA).

- **agent** — this program periodically performs probes and feeds their results to the aggregator. It runs on master nodes;
- **upmeter** — aggregates the results and implement the API server to retrieve them. Upmeter can link the history of probe results to the Downtime custom resource (where incidents are manually described);
- **front**
    - **status** — shows the current availability level over the previous 10 minutes (this one requires authorization by default, but you can disable it);
    - **web-ui** — displays the availability levels based on probes in time (requires authorization);
- **smoke-mini** — continuous *smoke testing* using a StatefulSet that looks like a real application.
