# Patches

## 001-add-registry-secret-as-dockerconfigjson.patch

This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

## 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.

## 003-fix-defsec-lib-for-cis-compability.patch

Fixing defseq rules for CIS benchmark for `--client-ca-file` and `--etcd-cafile` apiserver arguments.

PR: https://github.com/aquasecurity/defsec/pull/1268

## 004-fix-defsec-lib-for-deckhouse-setup.patch

Fixing rego kubernetes lib for proper find kubernetes system component containers, in particularly for apiserver (we have two containers in our setup with commands `kube-apiserver` and `kube-apiserver-healthcheck`).

PR: https://github.com/aquasecurity/defsec/pull/1269
