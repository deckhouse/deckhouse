# Patches

## 001-add-registry-secret-as-dockerconfigjson.patch

This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

## 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.

## 003-support-legacy-dockercfg.patch

Add support for `kubernetes.io/dockercfg`(legacy) secret type for `imagePullSecrets` field in scan jobs.

PR: https://github.com/aquasecurity/trivy-operator/pull/1183

