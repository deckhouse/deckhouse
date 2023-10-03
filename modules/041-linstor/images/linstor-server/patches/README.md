## Patches

### Automatically fix symlinks for devices

Introduce an additional handler that checks for the device path before each such modification.
If the device is not found, it attempts to fix the symlink using dmsetup output.

This change is workaround for specific set of issues, often related to udev,
which lead to the disappearance of symlinks for LVM devices on a working system.
These issues commonly manifest during device resizing and deactivation,
causing LINSTOR expceptions when accessing DRBD super-block of volume.

- Upstream: https://github.com/LINBIT/linstor-server/pull/370
