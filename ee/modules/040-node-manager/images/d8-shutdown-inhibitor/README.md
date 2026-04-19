# Deckhouse shutdown inhibitor

## Why?

We need to delay system shutdown until Pods with label are gone from the Node.

## Implementation

d8-shutdown-inhibitor runs as daemon which lock system shutdown with systemd inhibitors, receives shutdown event and wait until Pods are gone.

## Development

Testing and debugging may require reboot, so better use cluster made from
virtual machines, e.g. nested cluster on DVP.

Copy .env.example to .env and change variables to access your node which you use for tests.

Use `task` utility to build and deploy.

`task build` builds binary.
`task deploy` builds binary and transfers to selected Node.

### Useful commands

```shell
systemd-inhibit --list
  List all inhibitor locks.
  
systemd-analyze cat-config systemd/logind.conf
  Check logind configuration, e.g. get InhibitDelayMaxSec.
  
systemctl poweroff --check-inhibitors=yes
systemctl reboot --check-inhibitors=yes
  Use these commands to shutdown/reboot the Node.
```

## Tests

### simulate power button device removal

```shell
ls /sys/bus/acpi/drivers/button
echo PNP0C0C:00 > /sys/bus/acpi/drivers/button/unbind
```

### simulate power button press

```shell
python3 - <<'PY'  
import libevdev, os
dev = libevdev.Device()
dev.name = "d8-shutdown-inhibitor-test"
dev.enable(libevdev.EV_KEY.KEY_POWER)
uinput = dev.create_uinput_device()
uinput.send_events([
    libevdev.InputEvent(libevdev.EV_KEY.KEY_POWER, 1),
    libevdev.InputEvent(libevdev.EV_SYN.SYN_REPORT, 0)])
uinput.send_events([
    libevdev.InputEvent(libevdev.EV_KEY.KEY_POWER, 0),
    libevdev.InputEvent(libevdev.EV_SYN.SYN_REPORT, 0)])
PY
```

