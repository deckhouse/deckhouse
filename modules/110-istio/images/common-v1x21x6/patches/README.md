# Patches

## 001-istio-apply_go.patch

Fix Istio Operator healt status

## 002-istio-gomod_gosum.patch

Fix CVE

## 003-istio-server_fmtText.patch

Fix use expfmt library in pilot-agent. This library used for format metrics.

> [!WARNING]
> **After update istio to version 1.22.X and above need delete this patch!**

## 004-istio-operator_clusterrole_disable.patch

Fix clusterrole request from operator

## 005-istio-discovery-skip-gateway-crb-for-1x21.patch

For Istio 1.21 (revision 1x21) do not emit `istiod-gateway-controller` ClusterRoleBinding from the discovery chart so the operator does not attempt to create/patch cluster-scoped RBAC (which would fail and can block reconciliation, e.g. ingressgateway stuck in ContainerCreating).

## 001-kiali-go-mod.patch

Fix CVE

