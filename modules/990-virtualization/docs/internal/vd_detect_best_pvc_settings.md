
When you create a disk, the controller will automatically select the most optimal parameters supported by the storage based on the known data.

The following are the priorities of the `PersistentVolumeClaim` parameter settings when creating a disk by automatically defining the storage features:

- `RWX + Block`
- `RWX + FileSystem`
- `RWO + Block`
- `RWO + FileSystem`

If the storage is unknown and it is impossible to automatically define its parameters, then `RWO + FileSystem` is used.
