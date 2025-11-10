# Patches

### 001-add-registry-secret-as-dockerconfigjson.patch

This patch adds docker auth config via kubernetes volume/volumeMount to scanjobs in Standalone mode so that trivy init container can download trivy-db from a private registry. ClientServer mode doesn't have to download trivy-db on its own.
This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

### 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.

### 004-scan-job-registry-ca.patch

This patch adds the ability to specify CA for scan jobs via environment variables.

[Issue](https://github.com/deckhouse/deckhouse/issues/4950)

### 005-cis-benchmark-on-startup.patch

The first check begins instantly when the operator starts.

[Issue](https://github.com/deckhouse/deckhouse/issues/5174)

### 006-new-metrics.patch

This patch adds primaryLink metric for reports.

### 007-fix-custom-volumes.patch

[PR](https://github.com/aquasecurity/trivy-operator/pull/2241)
[Issue](https://github.com/aquasecurity/trivy-operator/issues/2240)

### 008-bump-dependencies.patch

This patch updates vulnerable dependencies.

### 009-fix-policies-cache.patch

The operator of v0.22.0 cannot re-download policies if the image of the policies has been changed, it tries to download the old one.

### 010-use-local-policies.patch

Uses policies from the "/local/policies" directory if "OPERATOR_USE_LOCAL_POLICIES" is set to "true"

### 011-support-new-jobs.patch

The current version of operator does not support kubernetes v1.31+ because of new job resources, this patch fixes it.

[PR](https://github.com/aquasecurity/trivy-operator/pull/2292)
[Issue](https://github.com/aquasecurity/trivy-operator/issues/2251)

### 012-proxy-for-jobs-and-security-context-forced.patch

Forced security context on scanning job pods that is similar to `helm_lib_module_container_security_context_pss_restricted_flexible`.
Passed the trivy-operator's HTTP(S)_PROXY and NO_PROXY environment variables to the scanning pods.

### 013-fix-cve.patch

Updated some operator dependencies.

### 014-fix-cve2.patch

Fixes CVEs:
- CVE-2025-52881
- CVE-2024-25621