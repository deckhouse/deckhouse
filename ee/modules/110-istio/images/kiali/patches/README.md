# Patches

## 001-fix-fetching-Istiod-resource-threholds.patch

Remove functionality to get limits for Kiali as we do not use limits
https://github.com/kiali/kiali/issues/5742
When the issue will be solved, it is important to revert our previous Dockerfile (https://github.com/deckhouse/deckhouse/blob/4832238d9d4bcda31f520fc1707f0e2a4ced2610/ee/modules/110-istio/images/kiali/Dockerfile).
