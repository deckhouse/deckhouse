---
title: "The virtualization module"
---

The module allows you to manage virtual machines using Kubernetes. The module uses the [kubevirt](https://github.com/kubevirt/kubevirt) project. 

The QEMU (KVM) + libvirtd stack and CNI Cilium are used for virtual machines (the [cni-cilium](../021-cni-cilium/) module is required for the virtualization module to work). It is guaranteed to work with [LINSTOR](../041-linstor) or [Ceph](../099-ceph-csi/) as storage, but other storages are also possible. 

The main advantages of the module:
- Simple interface for working with virtual machines as [Kubernetes primitives](cr.html) (working with VMs is similar to working with Pods);
- High network performance due to the use of CNI cilium with [MacVTap](https://github.com/kvaps/community/blob/macvtap-mode-for-pod-networking/design-proposals/macvtap-mode-for-pod-networking/macvtap-mode-for-pod-networking.md) support (eliminates the overhead of NAT).
