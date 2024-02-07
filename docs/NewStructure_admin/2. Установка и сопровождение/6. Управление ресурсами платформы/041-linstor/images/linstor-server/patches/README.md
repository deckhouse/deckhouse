## Patches

### Automatically fix symlinks for devices

This change is workaround for specific set of issues, often related to udev,
which lead to the disappearance of symlinks for LVM devices on a working system.
These issues commonly manifest during device resizing and deactivation,
causing LINSTOR exceptions when accessing DRBD super-block of volume.

- Upstream: https://github.com/LINBIT/linstor-server/pull/370
