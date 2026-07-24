# Patches

## 001-istio-go-mod.patch

Fix sail-operator CVE vulnerabilities (bump `golang.org/x/{crypto,net,sys,text}`,
`github.com/containerd/containerd`, `oras.land/oras-go/v2`).

## 002-istio-init-readonly-rootfs.patch

Set `readOnlyRootFilesystem: true` for the `istio-init` container in the sidecar injection template (`InitContainer` mode).
Required for clusters that enforce read-only root filesystem via SecurityPolicy/PSS (e.g. CSE). Safe for the standard Deckhouse `proxyv2` image: `iptables-wrapper` selects nft in the pod network namespace, so `/run/xtables.lock` is not required.
