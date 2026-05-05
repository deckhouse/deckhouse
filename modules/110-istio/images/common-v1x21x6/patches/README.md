# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 002-istio-gomod_gosum.patch

Fix CVE

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 004-istio-multicluster_regenerate_token_timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 001-kiali-go-mod.patch

Fix CVE
