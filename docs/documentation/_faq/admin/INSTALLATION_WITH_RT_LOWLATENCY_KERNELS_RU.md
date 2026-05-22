---
title: Поддерживет ли Deckhouse установку на ОС c realtime (rt) и low-latency ядрами?
subsystems:
- deckhouse
lang: ru
---

Deckhouse Kubernetes Platform может устанавливаться на ОС Linux с realtime (rt) и lowlatency ядрами без дополнительной настройки платформы. Работа DKP с такими ядрами была проверена для DKP EE начиная с версии v1.75.7 на следующих ОС и ядрах:

- CentOS 9 Stream, linux-5.14.0-706.el9.x86_64+rt
- RedOS 8, linux-6.12.85-1.red80.x86_64-rt
- Astra Linux 1.7.5, linux-5.15-lowlatency
