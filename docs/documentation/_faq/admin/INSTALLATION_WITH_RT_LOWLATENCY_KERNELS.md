---
title: Does Deckhouse support realtime (rt) and low-latency Linux kernels?
subsystems:
- deckhouse
lang: en
---

In general, realtime (rt) and lowlatency kernels are supported with no extra configuration needed for Deckhouse. Such kernels have been tested with Deckhouse EE Stable v1.75.7 on:

- CentOS 9 Stream, linux-5.14.0-706.el9.x86_64+rt
- RedOS 8, linux-6.12.85-1.red80.x86_64-rt
- Astra Linux 1.7.5, linux-5.15-lowlatency
