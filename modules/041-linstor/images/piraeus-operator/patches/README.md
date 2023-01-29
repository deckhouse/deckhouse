## Patches

### Disable finalizers

This is our internal patch to disable finalizers logic for piraeus-operator custom resources.
It was the simpliest way to avoid dependency problem while deleting operator and custom resources at one time.
It makes no sense for us since all the resources are deployed in single namespace and managed together as one.

### CSI-controller strategy

This PR allows to specify deployment strategy for linstor-csi-controller
linstor-csi-controller with podAntiAffinity and default strategy will stuck update in case of two nodes.

The linstor-controller deployment is not affected due to hardcoded `strategy: Recreate`

- https://github.com/piraeusdatastore/piraeus-operator/pull/395
