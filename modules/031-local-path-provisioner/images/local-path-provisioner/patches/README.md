## Patches

### Add setup/teardown command in config and use them instead of scripts

local-path-provisioner by defaul use scripts for create or delete directory. 
It passes `/bin/sh` command to helper pod, 
but we are using distroless image without shell. 
This patch add parameters with path to binary to configuration.

### Fix DirectoryOrCreate

Use `type: Directory` instead of `type: DirectoryOrCreate` for created PVs
to avoid the situations when initial storage is broken and unmounted.
https://github.com/rancher/local-path-provisioner/pull/224
