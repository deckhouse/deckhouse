---
title: "The cilium-hubble module"
description: "Visualization of the Deckhouse Kubernetes Platform cluster network stack using Cilium Hubble."
webIfaces:
- name: hubble
---

The `cilium-hubble` module provides visualization of the cluster network stack if the cilium CNI is enabled.

## Requirements

The module requires eBPF support in the Linux kernel.
In addition, for Hubble to function properly, the BPF Type Format (BTF) metadata format must be supported.

Requirements for the `cilium-hubble` module:

- Linux kernel version >= 5.8.
- [BPF Type Format (BTF)](https://www.kernel.org/doc/html/v5.8/bpf/btf.html) support enabled. You can verify it as follows:
  - Run `ls -lah /sys/kernel/btf/vmlinux` — if the file exists, BTF is supported.
  - Run `grep -E "CONFIG_DEBUG_INFO_BTF=(y|m)" /boot/config-*` — if the parameter is enabled, BTF is supported.
