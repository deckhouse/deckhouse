# Patches

## 001-kiali-go-mod.patch

Fix Kiali CVE vulnerabilities

## 001-istio-gomod_gosum.patch

Fix Istio CVE vulnerabilities

## 002-istio-multicluster-regenerate-token-timeout.patch

Implement graceful transition for remote multicluster secrets. To prevent connectivity gaps during secret rotation, the old secret is no longer dismissed immediately. Instead, it remains active until the new secret is processed and all associated metadata is synced.
Adopted upstream pr https://github.com/istio/istio/pull/58567.

## 003-change-to-deckhouse-user.patch

Change default user from 1337 to 64535 in istio containers
