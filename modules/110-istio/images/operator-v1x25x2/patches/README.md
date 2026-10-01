# Patches

## 001-istio-go-mod.patch

Fix Istio CVE vulnerabilities

## 002-istio-operato-cni_status_restrict.patch

Fix Sails operator check status about CNI

## 003-istio-init-readonly-rootfs.patch

Set `readOnlyRootFilesystem: true` for the `istio-init` container in the sidecar injection template (`InitContainer` mode).
Required for clusters that enforce read-only root filesystem via SecurityPolicy/PSS (e.g. CSE). Safe for the standard Deckhouse `proxyv2` image: `iptables-wrapper` selects nft in the pod network namespace, so `/run/xtables.lock` is not required.

## 004-istio-operator-unlock-pending-helm-release.patch

Backport of two upstream sail-operator fixes, neither of which is in 1.25.2:

- [#971](https://github.com/istio-ecosystem/sail-operator/pull/971) (`4a908c07`, "Try to uninstall releases in `uninstalling` status");
- [#1103](https://github.com/istio-ecosystem/sail-operator/pull/1103) (`2fb9356e`, "unlock release stuck in pending state").

The resulting `UpgradeOrInstallChart` logic is identical to upstream after `2fb9356e`. The `GetRelease`/`UpdateRelease` helpers and the integration test from #1103 are not backported. A unit test for a `pending-rollback` release with a previous revision is added instead.

If the operator exits in the middle of a Helm operation (crash, OOM, lost leader lease, API server outage), the release is left in a `pending-*` or `uninstalling` state. Without this patch, a release left in `pending-rollback` or `uninstalling` is reported as "unrecoverable"/"unexpected", and the operator stops applying the chart for good: changes to the `Istio` resource (e.g. `pilot.cni.enabled`) never reach istiod and the sidecar injector. With the patch:

- a `pending-*` release is marked `failed` first, then fixed by the usual rollback followed by an upgrade (or, for the first revision, by an uninstall followed by a fresh install);
- an `uninstalling` release is uninstalled again, then installed from scratch.
