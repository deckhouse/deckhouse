---
title: DSR (Direct Server Return)
description: Метод обработки трафика, при котором входящие пакеты проходят через балансировщик нагрузки, а исходящие идут напрямую с серверов, обрабатывающих запрос.
lang: ru
---

Ответы обычно значительно больше запросов, поэтому DSR разгружает балансировщик и увеличивает общую пропускную способность. В Deckhouse Platform DSR — один из режимов [bpfLB](glossary.html#bpflb) модуля [`cni-cilium`](/modules/cni-cilium/), применяется вместо SNAT.

Подробнее в разделе [«Режимы работы bpfLB»](admin/configuration/network/other/bpflb.html).
