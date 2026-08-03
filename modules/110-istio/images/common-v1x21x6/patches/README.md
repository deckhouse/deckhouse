# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 002-istio-gomod_gosum.patch

Fix CVE (bump `golang.org/x/{crypto,net,sys,text}`).

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 001-kiali-go-mod.patch

Fix CVE (bump `golang.org/x/{crypto,net,sys,text}`).

## 004-istio-init-readonly-rootfs.patch

Set `readOnlyRootFilesystem: true` for the `istio-init` container in the sidecar injection template (`InitContainer` mode).
Required for clusters that enforce read-only root filesystem via SecurityPolicy/PSS (e.g. CSE). Safe for the standard Deckhouse `proxyv2` image: `iptables-wrapper` selects nft in the pod network namespace, so `/run/xtables.lock` is not required.
