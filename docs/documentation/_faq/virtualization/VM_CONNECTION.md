---
title: Why is only one connection method available for a virtual machine?
subsystems:
- virtualization
lang: en
---

You can connect to a VM via the serial console ([`d8 v console`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-console)) or VNC ([`d8 v vnc`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-vnc)).
These methods use different communication channels with the guest OS and depend on its configuration.
For more details on connecting, see the [Connecting to a virtual machine](user/virtualization/vm-access.html#connecting-to-a-virtual-machine) section.

The sections below describe common situations where only one connection method works.

#### Why does VNC not work when the serial console is available?

VNC displays the guest OS screen and requires virtual terminal support in the kernel.
The serial console works independently of the graphics subsystem.

Check in the guest OS whether virtual terminal support is enabled in the kernel configuration:

```bash
cat /boot/config-$(uname -r) | grep CONFIG_VT
```

The output should show `CONFIG_VT=y`:

```console
CONFIG_VT=y
```

If the output shows `CONFIG_VT is not set`, rebuild the kernel with the option enabled or use an OS image with a suitable kernel configuration.

#### Why does the serial console not work when VNC is available?

The serial console connects to the `ttyS0` port in the guest OS.
If the `getty` service for this port is not running, `d8 v console` will not show a login prompt even though VNC continues to work.

In the guest OS, enable and start the `serial-getty` service for `ttyS0`:

```bash
sudo systemctl enable --now serial-getty@ttyS0.service
```

Then connect to the serial console again.
