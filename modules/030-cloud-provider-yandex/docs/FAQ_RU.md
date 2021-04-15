---
title: "Сloud provider — Yandex.Cloud: FAQ"
---

## Как настроить INTERNAL LoadBalancer?

Установить аннотацию для сервиса:
```
yandex.cpi.flant.com/listener-subnet-id: SubnetID
```
Аннотация указывает, какой Subnet будет слушать LB.
