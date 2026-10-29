---
title: Canary deployment
description: An application deployment strategy that rolls out a new version to a small portion of traffic first and shifts more traffic to it as confidence grows, with a quick rollback to the old version if issues arise.
lang: en
---

In Deckhouse Platform a canary deployment is implemented with the [`ingress-nginx`](/modules/ingress-nginx/) module through Ingress annotations or with the [`istio`](/modules/istio/) module through VirtualService and DestinationRule objects (the Istio way is recommended). The new version runs as a separate Deployment and Service next to the stable one.

For details, see [Configuring Canary deployment](user/network/canary-deployment.html).
