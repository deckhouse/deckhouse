# Patches

## 001-istio-go-mod.patch

Fix Istio CVE vulnerabilities

## 002-istio-operato-cni_status_restrict.patch

Fix Sails operator check status about CNI

## 003-change-to-deckhouse-user.patch

Change runAsUser from 1337 to 64535 in istio templates, changed istio-init.iptables user arg to both 1337 and 64535 UIDs in injection-template.yaml

## 004-mark-interception.patch

Apply the same mark-based interception changes (patch 004 from common) to the sail-operator's vendored copy of injection-template.yaml. Adds `--outbound-mark` flag, `ISTIO_META_OUTBOUND_MARK` env, and conditional `CAP_NET_RAW` + `allowPrivilegeEscalation: true` when `traffic.sidecar.istio.io/outboundSocketMark` annotation is set (`allowPrivilegeEscalation: true` is required because the capability is granted via `setcap` on the `envoy` binary, which the kernel ignores at exec under `no_new_privs` otherwise — see common-v1x25x2/patches/README.md for why an ambient-capabilities approach without it doesn't work).

Applies on top of 003.

## 005-markimage-values.patch

Add the `markImage` field to the `ProxyConfig` type in `api/v1/values_types.gen.go` (and the matching `DeepCopyInto` in `api/v1/zz_generated.deepcopy.go`). This is the sail-operator equivalent of putting `markImage` into the validated proxy schema: in 1.25 the operator parses `spec.values` into a plain Go struct (`values_types.gen.go`, controller-gen), so adding the field here makes `markImage` a known/validated value instead of an unknown-field validation error. It is rendered by the injection template (see 004-mark-interception.patch) for pods annotated with `traffic.sidecar.istio.io/outboundSocketMark`.

For istio 1.21 the same effect is achieved without a patch, by placing `markImage` under `spec.unvalidatedValues` in the IstioOperator CR (see `templates/control-plane/iop/iop.yaml`), because in 1.21 `ProxyConfig` is a proto message and `spec.values` is validated strictly against the proto descriptor.

Applies on top of 004.
