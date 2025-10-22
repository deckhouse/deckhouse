---
title: "Cloud provider — Базис.DynamiX: FAQ"
---
### Как настроить INTERNAL LoadBalancer?

Для настройки **INTERNAL** LoadBalancer’а установите в манифесте Service следующую аннотацию:

- `dynamix.cpi.flant.com/internal-network-name: <internal_name>`

### Как настроить EXTERNAL LoadBalancer?

Для настройки **EXTERNAL** LoadBalancer’а установите в манифесте Service следующую аннотацию:

- `dynamix.cpi.flant.com/external-network-name: <external_name>`
