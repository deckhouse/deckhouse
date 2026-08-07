---
title: No VNC access, but the serial console works — what's wrong?
section: vm_operations
lang: en
---

You can connect to a VM via the serial console ([`d8 v console`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-console)) or VNC ([`d8 v vnc`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-vnc)). These methods use different communication channels with the guest OS and depend on its configuration.

VNC displays the guest OS screen and requires virtual terminal support in the kernel. The serial console works independently of the graphics subsystem.

Check in the guest OS whether virtual terminal support is enabled in the kernel configuration:

```bash
cat /boot/config-$(uname -r) | grep CONFIG_VT
```

The output should show `CONFIG_VT=y`:

```console
CONFIG_VT=y
```

If the output shows `CONFIG_VT is not set`, rebuild the kernel with the option enabled or use an OS image with a suitable kernel configuration.
