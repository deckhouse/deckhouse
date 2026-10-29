---
title: Availability probe
description: A check that the upmeter module runs regularly to calculate the availability of Deckhouse Platform components. Probes are grouped, and groups or individual probes can be disabled.
lang: en
---

The upmeter-agent component runs on [master nodes](glossary.html#master-node) and executes probe groups: control-plane (API server availability and controller health), deckhouse (cluster health and the deckhouse controller), extensions, load-balancing, monitoring-and-autoscaling, nginx (every [Ingress controller](glossary.html#ingress-controller) has a `Ready` pod), nodegroups and synthetic (network connectivity between nodes). For example, the control-plane group creates a namespace, a pod and a StatefulSet and checks that Kubernetes processes them.

Probe results are stored by the upmeter controller excluding the downtime intervals defined in [Downtime](/modules/upmeter/cr.html#downtime) objects, shown on the status page and the availability dashboard, and used for [SLA monitoring](glossary.html#sla-monitoring). Probes or probe groups are disabled with the `disabledProbes` parameter of the module.

For details, see [Upmeter module](architecture/observability/upmeter.html).
