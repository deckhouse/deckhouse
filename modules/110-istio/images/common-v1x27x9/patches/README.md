# Patches

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 001-kiali-gomod_gosum.patch

Fix Kiali CVE vulnerabilities

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 003-change-to-deckhouse-user.patch

Change istio-proxy UID/GID from 1337 to 64535 (Deckhouse reserved UID), keeping 1337 in iptables/nftables owner rules. 
> [!WARNING]
> **Skips `istio-discovery/files/*` (operator/injector templates are managed by Deckhouse separately).**
