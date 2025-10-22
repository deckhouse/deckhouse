## Patches

### 001-go-mod.patch

Update dependencies

### 002-fix-directory-or-create.patch

Use `type: Directory` instead of `type: DirectoryOrCreate` for created PVs
to avoid the situations when initial storage is broken and unmounted.
https://github.com/rancher/local-path-provisioner/pull/224

### 003-workspace-emptydir.patch

Adds support for clusters where containerd forces `readOnlyRootFilesystem` for every Pod in `d8-*` namespaces (CSE).

* introduces an `emptyDir` volume mounted into the container at `/workspace`;
* mounts the hostPath parent directory under `/workspace/<parentDir>` instead of its original absolute path;
* therefore the helper-pod can still create or remove directories even though the container root filesystem is read-only.

The patch touches `provisioner.go`.

### 004-fix-go-mod.patch

Update dependencies
