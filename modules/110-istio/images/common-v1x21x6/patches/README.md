# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 002-istio-gomod_gosum.patch

Fix CVE

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

## 004-istio-pilot-secrets_restrict.patch

Restricting the secrets request from a local and remote cluster in multicluster mode

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 001-kiali-go-mod.patch

Fix CVE
