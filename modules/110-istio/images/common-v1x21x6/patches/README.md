# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 002-istio-gomod_gosum.patch

Fix CVE

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

## 004-istio-sidecar-to-deckhouse-user.patch

Change default user from 1337 to 64535 in istio-proxy sidecar

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 001-kiali-go-mod.patch

Fix CVE
