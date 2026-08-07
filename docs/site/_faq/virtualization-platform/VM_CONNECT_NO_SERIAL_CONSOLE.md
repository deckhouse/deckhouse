---
title: No serial console access, but VNC works — what's wrong?
section: vm_operations
lang: en
---

You can connect to a VM via the serial console ([`d8 v console`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-console)) or VNC ([`d8 v vnc`](/products/kubernetes-platform/documentation/v1/cli/d8/reference/#d8-v-vnc)). These methods use different communication channels with the guest OS and depend on its configuration.

The serial console connects to the `ttyS0` port in the guest OS. If the `getty` service for this port is not running, `d8 v console` will not show a login prompt even though VNC continues to work.

In the guest OS, enable and start the `serial-getty` service for `ttyS0`:

```bash
sudo systemctl enable --now serial-getty@ttyS0.service
```

Then connect to the serial console again.
