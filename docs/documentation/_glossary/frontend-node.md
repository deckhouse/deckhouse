---
title: Frontend node
description: A node that balances incoming traffic. Ingress controllers run on frontend nodes.
lang: en
---

A cluster needs more than one frontend node, and the nodes must handle the traffic when at least one of them fails. With two frontend nodes each must carry the full load, with three each must withstand a load increase of at least 150%. Frontend nodes usually form the `front` [node group](glossary.html#node-group) and run the [Ingress controllers](glossary.html#ingress-controller).

For details, see [Recommendations for configuring cluster nodes](admin/configuration/high-reliability-and-availability/recommendations.html).
