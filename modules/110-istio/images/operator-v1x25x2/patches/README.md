# Patches

## 001-istio-go-mod.patch

Fix Istio CVE vulnerabilities

## 002-istio-operato-cni_status_restrict.patch

Fix Sails operator check status about CNI

## 003-istio-init-readonly-rootfs.patch

Set `readOnlyRootFilesystem: true` for the `istio-init` container in the sidecar injection template (`InitContainer` mode).
Required for clusters that enforce read-only root filesystem via SecurityPolicy/PSS (e.g. CSE). Safe for the standard Deckhouse `proxyv2` image: `iptables-wrapper` selects nft in the pod network namespace, so `/run/xtables.lock` is not required.
