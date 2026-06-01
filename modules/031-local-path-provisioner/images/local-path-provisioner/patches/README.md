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

### 003-cve-2026-44543-helperpod-validation.patch

Backports the HelperPod template validation introduced in upstream `v0.0.36`
(CVE-2026-44543, [GHSA-7fxv-8wr2-mfc4](https://github.com/rancher/local-path-provisioner/security/advisories/GHSA-7fxv-8wr2-mfc4),
CVSS 8.7 High) to `v0.0.34`. The provisioner now rejects unsafe security-sensitive
fields in `helperPod.yaml` loaded from the `local-path-config` ConfigMap, so an
attacker with edit permission on that ConfigMap cannot inject a privileged
HelperPod with the host root filesystem mounted.

The following fields are forbidden:

* `initContainers` / `ephemeralContainers`;
* extra containers (only one container is allowed);
* custom `volumes` / `volumeMounts` (the provisioner injects the host-path volume itself);
* `hostNetwork` / `hostPID` / `hostIPC`;
* `spec.nodeName` / `spec.serviceAccountName`;
* `envFrom`, `env.valueFrom`;
* `lifecycle`, `livenessProbe`, `readinessProbe`, `startupProbe`;
* `spec.securityContext.sysctls`;
* `securityContext.privileged: true`, `capabilities.add`, `allowPrivilegeEscalation: true`.

Deckhouse-specific deviation from upstream: `runAsUser=0` / `runAsGroup=0` are
intentionally still allowed, because the helper container manages directories on
the host-path volume that were originally created by root and therefore must run
as root itself.

The patch touches `util.go`.
