---
title: "Cloud provider — Dynamix: FAQ"
---
### How to configure an INTERNAL LoadBalancer?

To configure an **INTERNAL** LoadBalancer, add the following annotation to your Service manifest:

- `dynamix.cpi.flant.com/internal-network-name: <internal_name>`

### How to configure an EXTERNAL LoadBalancer?

To configure an **EXTERNAL** LoadBalancer, add the following annotation to your Service manifest:

- `dynamix.cpi.flant.com/external-network-name: <external_name>`
