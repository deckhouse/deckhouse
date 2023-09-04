## Patches

### Disable finalizers

This is our internal patch to disable finalizers logic for piraeus-operator custom resources.
It was the simpliest way to avoid dependency problem while deleting operator and custom resources at one time.
It makes no sense for us since all the resources are deployed in single namespace and managed together as one.

### linux-kbuild.patch

Debian 11 explicitly requires files from `/usr/lib/linux-kbuild-5.10`  
This patch passes through `/usr/lib` directory into kernel-module-injector and sets symlinks to allow using it

- Upstream: https://github.com/piraeusdatastore/piraeus-operator/pull/475

### Add metrics port

Add the securedMetricsPort parameter to the linstorcontrollers.piraeus.linbit.com crd. If this parameter is set, then port for scraping metrics will be added to linstor-controller service and K8S_AWAIT_ELECTION_SERVICE_PORTS_JSON env var of the linstor-controller deployment. This is required when using service monitor to monitor the linstor controller in HA mode.

- Upstream: https://github.com/piraeusdatastore/piraeus-operator/pull/495
