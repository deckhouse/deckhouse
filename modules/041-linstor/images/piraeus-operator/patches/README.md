## Patches

### Disable finalizers

This is our internal patch to disable finalizers logic for piraeus-operator custom resources.
It was the simpliest way to avoid dependency problem while deleting operator and custom resources at one time.
It makes no sense for us since all the resources are deployed in single namespace and managed together as one.

### linux-kbuild.patch

Debian 11 explicitly requires files from `/usr/lib/linux-kbuild-5.10`  
This patch passes through `/usr/lib` directory into kernel-module-injector and sets symlinks to allow using it

- Upstream: https://github.com/piraeusdatastore/piraeus-operator/pull/475
