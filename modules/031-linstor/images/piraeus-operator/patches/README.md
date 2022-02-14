## Patches

## Fix conflicting names

LinstorSattelliteSet and LinstorController are having conflicting entity names.
Eg. if you install LinstorController with name `linstor` and LinstorSatelliteSet with name `linstor `operator will be failed to upgrade ownerRefference field for `linstor-config` configmap.
https://github.com/piraeusdatastore/piraeus-operator/pull/268

## Disable finalizers

This is our internal patch to disable finalizers logic for piraeus-operator custom resources.
It was the simpliest way to avoid dependency problem while deleting operator and custom resources at one time.
It makes no sense for us since all the resources are deployed in single namespace and managed together as one.
