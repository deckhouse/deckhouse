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

Adding logic to create an additional JWT token at the time of the main one's regeneration to avoid downtime of services.

## 001-kiali-go-mod.patch

Fix CVE
