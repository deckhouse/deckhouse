---
title: "Сloud provider — Yandex.Cloud: FAQ"
---

## How do I set up the INTERNAL LoadBalancer?

Attach the following annotation to the service:
```
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```
The annotation links the LB with the appropriate Subnet.
