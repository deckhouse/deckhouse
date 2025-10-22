---
title: "Модуль cilium-hubble"
description: "Визуализация сетевого стека кластера Deckhouse Kubernetes Platform с помощью Cilium Hubble."
webIfaces:
- name: hubble
---

Модуль `cilium-hubble` обеспечивает визуализацию сетевого стека кластера, если включен Cilium CNI.

## Требования

Для работы модуля `cilium-hubble` необходимы:

- Версия ядра Linux >= 5.8 с поддержкой eBPF.
- Поддержка формата метаданных [BPF Type Format (BTF)](https://www.kernel.org/doc/html/v5.8/bpf/btf.html). Проверить можно следующими способами:
  - выполнить команду `ls -lah /sys/kernel/btf/vmlinux` — наличие файла подтверждает поддержку BTF;
  - выполнить команду `grep -E "CONFIG_DEBUG_INFO_BTF=(y|m)" /boot/config-*` — если параметр включён, BTF поддерживается.
