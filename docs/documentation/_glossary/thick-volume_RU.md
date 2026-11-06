---
title: Thick-том
description: "Том LVM в thick pool, то есть в пуле типа LVM: место под такой том выделяется целиком при создании. Thick pool даёт производительность, сравнимую с устройством хранения, но не позволяет использовать снимки."
lang: ru
---

Thick-тома создаются в thick pool [локального](glossary.html#local-storage) и [реплицируемого](glossary.html#replicated-storage) хранилищ: режим LVM модуля [`sds-local-volume`](/modules/sds-local-volume/) и группы томов LVM без определения thin pool в модуле [`sds-replicated-volume`](/modules/sds-replicated-volume/). Для томов, которым нужны снимки, используйте [thin-том](glossary.html#thin-volume).
