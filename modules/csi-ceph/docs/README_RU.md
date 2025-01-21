---
title: "Модуль csi-ceph"
moduleStatus: experimental
---

Модуль устанавливает и настраивает CSI-драйвер для RBD и CephFS.

Настройка выполняется посредством [custom resources](cr.html), что позволяет подключить более одного Ceph-кластера (UUID не должны совпадать).
