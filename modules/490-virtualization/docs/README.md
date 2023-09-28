---
title: "The virtualization module"
---

{% alert level="danger" %}
The current module version is no longer under development and will be replaced by a new one. The new module version is not guaranteed to be compatible with the current one. We do not recommend using the current module version for new projects.
{% endalert %}

The module allows you to manage virtual machines using Kubernetes. The module uses the [kubevirt](https://github.com/kubevirt/kubevirt) project. 

The QEMU (KVM) + libvirtd stack and CNI Cilium are used for virtual machines (the [cni-cilium](../021-cni-cilium/) module is required for the virtualization module to work). It has been tested to work well with [LINSTOR](../041-linstor) or [Ceph](../031-ceph-csi/) storage, but you can use other storage management systems as well. 

The main advantages of the module:
- Easy-to-use and intuitive interface for working with virtual machines like with [Kubernetes primitives](cr.html) (operating VMs is now as easy as running Pods);
- High network performance thanks to Cilium CNI with [MacVTap](https://github.com/kvaps/community/blob/macvtap-mode-for-pod-networking/design-proposals/macvtap-mode-for-pod-networking/macvtap-mode-for-pod-networking.md) support (eliminates the NAT overhead).
