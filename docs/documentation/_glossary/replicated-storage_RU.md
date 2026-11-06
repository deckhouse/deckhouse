---
title: Реплицируемое хранилище
description: Хранилище, в котором данные реплицируются между несколькими узлами и остаются доступными при отказе одного из них.
lang: ru
---

Реплицируемые блочные StorageClass создаются модулем [`sds-replicated-volume`](/modules/sds-replicated-volume/), который основан на Distributed Replicated Block Device (DRBD). Реплицируемое хранилище относится к [программно-определяемым хранилищам](glossary.html#sds). Его пулы создаются на группах томов LVM как [thick](glossary.html#thick-volume) или [thin](glossary.html#thin-volume) pool.

Подробнее в разделе [«Настройка реплицируемого хранилища на основе DRBD»](admin/configuration/storage/sds/lvm-replicated.html).
