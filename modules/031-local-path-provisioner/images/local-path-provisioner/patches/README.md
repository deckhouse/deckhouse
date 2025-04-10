## Patches

### 001-go-mod.patch

Update dependencies

### 002-fix-directory-or-create.patch

Use `type: Directory` instead of `type: DirectoryOrCreate` for created PVs
to avoid the situations when initial storage is broken and unmounted.
https://github.com/rancher/local-path-provisioner/pull/224
