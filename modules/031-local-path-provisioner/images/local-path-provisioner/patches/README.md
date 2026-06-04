## Patches

### 001-fix-directory-or-create.patch

Use `type: Directory` instead of `type: DirectoryOrCreate` for created PVs
to avoid the situations when initial storage is broken and unmounted.
https://github.com/rancher/local-path-provisioner/pull/224

### 002-workspace-emptydir.patch

Adds support for clusters where containerd forces `readOnlyRootFilesystem` for every Pod in `d8-*` namespaces (CSE).

* introduces an `emptyDir` volume mounted into the container at `/workspace`;
* mounts the hostPath parent directory under `/workspace/<parentDir>` instead of its original absolute path;
* therefore the helper-pod can still create or remove directories even though the container root filesystem is read-only.

The patch touches `provisioner.go`.

### 003-allow-root-helperpod.patch

Deckhouse-specific deviation on top of the upstream HelperPod template
validation that landed in `v0.0.36` (CVE-2026-44543,
[GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4),
CVSS 8.7 High). Upstream rejects pods that set `spec.securityContext.runAsUser`
or `spec.securityContext.runAsGroup` to `0` unless the operator-level flag
`--allow-unsafe-helper-pod-template` is passed, which disables **all**
validation and would re-introduce the CVE.

This patch removes only those two specific checks so that all other
security-sensitive fields stay forbidden (`initContainers`,
`ephemeralContainers`, extra containers, custom `volumes`/`volumeMounts`,
host namespaces, `nodeName`/`serviceAccountName`, `envFrom`/`env.valueFrom`,
container lifecycle/probes, `sysctls`, `privileged: true`, `capabilities.add`,
`allowPrivilegeEscalation: true`).

`runAsUser=0` / `runAsGroup=0` must remain allowed because the helper container
manages host-path directories that were originally created by root and
therefore must run as root itself.

The patch touches `util.go`.
