---
title: "The Prometheus operator module: faq"
type:
  - instruction
---

## Tips of installing custom prometheus operator to the cluster

User might need to deploy your own prometheus-operator in order to add prometheus or alertmanagers to the cluster.

1. To avoid interfering with the prometheus-operator from Deckhouse, it is required to set the
   `--deny-namespaces=d8-monitoring` flag for the custom prometheus-operator installation.

2. The prometheus-operator from Deckhouse watches rules and monitors resources only in namespaces
   with the `heritage: deckhouse` label. Do not put this label on custom namespaces.
