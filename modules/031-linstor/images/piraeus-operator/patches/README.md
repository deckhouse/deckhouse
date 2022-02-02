## Patches

## Fix conflicting names

LinstorSattelliteSet and LinstorController are having conflicting entity names.
Eg. if you install LinstorController with name `linstor` and LinstorSatelliteSet with name `linstor `operator will be failed to upgrade ownerRefference field for `linstor-config` configmap.
https://github.com/piraeusdatastore/piraeus-operator/pull/268
