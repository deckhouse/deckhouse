# Patches

## 001-istio-go-mod.patch

Fix Istio CVE vulnerabilities

## 002-istio-operato-cni_status_restrict.patch

Fix Sails operator check status about CNI

## 003-change-to-deckhouse-user.patch

Change runAsUser from 1337 to 64535 in istio templates, changed istio-init.iptables user arg to both 1337 and 64535 UIDs in injection-template.yaml

## 004-mark-interception.patch

Apply the same mark-based interception changes (patch 004 from common) to the sail-operator's vendored copy of injection-template.yaml. Adds `--outbound-mark` flag, `ISTIO_META_OUTBOUND_MARK` env, and conditional `CAP_NET_ADMIN` capability when `traffic.sidecar.istio.io/outboundSocketMark` annotation is set.

Applies on top of 003.
