---
title: "Модуль chrony"
description: "Синхронизация времени в кластере Deckhouse Kubernetes Platform."
---

Обеспечивает синхронизацию времени на всех узлах кластера с помощью утилиты [chrony](https://chrony.tuxfamily.org/).

## Как работает

Модуль запускает `chrony` агенты на всех узлах кластера.
По умолчанию используется NTP сервер `pool.ntp.org`. NTP сервер можно изменить через [настройки](https://deckhouse.ru/products/kubernetes-platform/documentation/v1/modules/chrony/configuration.html) модуля.
Для просмотра используемых NTP серверов можно воспользоваться командой:

```bash
kubectl exec -it -n d8-chrony chrony-master-r7v6c -- chronyc -N sources
Defaulted container "chrony" out of: chrony, chrony-exporter, kube-rbac-proxy
MS Name/IP address         Stratum Poll Reach LastRx Last sample
===============================================================================
^* pool.ntp.org.                 2  10   377   171   -502us[ -909us] +/- 5388us
^- pool.ntp.org.                 2  10   377   666  -5317us[-5698us] +/-  103ms
^+ pool.ntp.org.                 2  10   377   938   -201us[ -567us] +/- 5346us
^+ pool.ntp.org.                 2  10   377   843   -159us[ -530us] +/-   12ms
```

`^+` - комбинируемый NTP сервер(`chrony` комбинирует информацию из `combined` серверов для уменьшения неточностей);  
`^*` - текущий NTP сервер;  
`^-` - некомбинируемый NTP сервер.

`chrony` агенты на мастер узлах и на остальных узлах имеют одно главное отличие - на всех узлах, которые не являются мастерами, в списке NTP серверов находятся не только NTP сервера из `module config`, но и адреса всех мастер узлов кластера.  

Таким образом, агенты на мастер узлах синхронизируют время только из списка хостов, указанных в `module config`(по умолчанию с `pool.ntp.org`). А агенты на остальных узлах синхронизируют время со списком NTP серверов из `module config` плюс с `chrony` агентов на мастер узлах.  

Это сделано для того, чтобы в случае недоступности NTP серверов, указанных в `module config`, время синхронизировалось с мастер узлами.
