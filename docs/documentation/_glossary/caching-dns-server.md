---
title: Caching DNS server
description: A DNS server that the [`node-local-dns`](/modules/node-local-dns/) module runs on every cluster node. It caches DNS responses and removes the network round trip to the cluster DNS for repeated requests.
lang: en
---

On each node the module configures an interface with the IP address of the [`kube-dns`](/modules/kube-dns/) service, starts a caching CoreDNS on that address and adds an iptables rule that redirects DNS traffic to it while its socket is open. Otherwise the standard routing through the cluster DNS is used. External requests that are not cached yet are still resolved through the chain of internal zones first, but under load the cache alone improves resolution performance noticeably.

For details, see [Caching DNS server in a cluster](architecture/network/dns-caching.html).
