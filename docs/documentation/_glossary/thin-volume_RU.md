---
title: Thin-том
description: "Том LVM в thin pool, то есть в пуле типа LVMThin: место под такой том выделяется по мере заполнения. Thin pool позволяет использовать снимки, но обладает меньшей производительностью."
lang: ru
---

Thin-тома создаются в thin pool [локального](glossary.html#local-storage) и [реплицируемого](glossary.html#replicated-storage) хранилищ: режим LVMThin модуля [`sds-local-volume`](/modules/sds-local-volume/) и группы томов LVM с определением thin pool в модуле [`sds-replicated-volume`](/modules/sds-replicated-volume/). Там, где снимки не нужны, [thick-том](glossary.html#thick-volume) даёт более высокую производительность.
