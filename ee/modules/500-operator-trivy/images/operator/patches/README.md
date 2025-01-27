# Patches

### 001-add-registry-secret-as-dockerconfigjson.patch

This patch adds docker auth config via kubernetes volume/volumeMount to scanjobs in Standalone mode so that trivy init container can download trivy-db from a private registry. ClientServer mode doesn't have to download trivy-db on its own.
This issue covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

### 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.


### 004-scan-job-registry-ca.patch

This patch adds the ability to specify CA for scan jobs via environment variables.

Issue


### 005-cis-benchmark-on-startup.patch

The first check begins instantly when the operator starts.

Issue

### 006-new-metrics.patch

This patch adds primaryLink metric for reports.

### 007-fix-custom-volumes.patch

PR
Issue

### 008-bump-dependencies.patch

This patch updates vulnerable dependencies.

### 009-fix-policies-cache.patch

The operator of v0.22.0 cannot re-download policies if the image of the policies has been changed, it tries to download the old one.

### 010-use-local-policies.patch

Uses policies from the "/local/policies" directory if "OPERATOR_USE_LOCAL_POLICIES" is set to "true"
