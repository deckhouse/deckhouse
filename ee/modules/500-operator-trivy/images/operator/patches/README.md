# Patches

## 001-add-registry-secret-as-dockerconfigjson.patch

This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

## 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.


## 003-fix-node-selector.patch

If there aren't any tolerations for node taints because of usage of manual scheduling, `kube-controller-manager` would recursively create and delete `node-collector` pods. This patch is fixing this behaviour.

[PR](https://github.com/aquasecurity/trivy-kubernetes/pull/217)
