---
title: bpfLB
description: Режим BPF Load Balancer модуля [`cni-cilium`](/modules/cni-cilium/), который обрабатывает внешний трафик для сервисов типов NodePort и LoadBalancer. Режим задаётся параметром bpfLBMode.
lang: ru
---

В режиме `SNAT` BPF-балансировщик полностью повторяет логику `kube-proxy`: IP-адрес источника в пакетах, передаваемых подам на других узлах, заменяется на InternalIP узла. В режиме [DSR](glossary.html#dsr) входящий трафик проходит через балансировщик, а исходящий идёт напрямую с узла целевого пода, что увеличивает общую пропускную способность.

Подробнее в разделе [«Режимы работы bpfLB»](admin/configuration/network/other/bpflb.html).
