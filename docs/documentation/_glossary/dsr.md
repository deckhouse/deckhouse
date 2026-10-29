---
title: DSR (Direct Server Return)
description: A traffic processing method in which incoming packets pass through the load balancer and outgoing packets go directly from the servers that handle the request.
lang: en
---

Responses are usually much larger than requests, so DSR unloads the balancer and increases the overall throughput. In Deckhouse Platform, DSR is one of the [bpfLB](glossary.html#bpflb) modes of the [`cni-cilium`](/modules/cni-cilium/) module and is used instead of SNAT.

For details, see [BpfLB operation modes](admin/configuration/network/other/bpflb.html).
