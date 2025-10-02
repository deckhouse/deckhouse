The following annotations can be used to modify the standard `PersistentVolumeClaim` parameter determination process for `StorageClass` during creation PVC for VD:

| Annotation                                           | Valid values                     |
| ---------------------------------------------------- | -------------------------------- |
| virtualdisk.virtualization.deckhouse.io/volume-mode  | `Block`, `Filesystem`            |
| virtualdisk.virtualization.deckhouse.io/access-mode  | `ReadWriteOnce`, `ReadWriteMany` |
| virtualdisk.virtualization.deckhouse.io/binding-mode | `Immediate`                      |
