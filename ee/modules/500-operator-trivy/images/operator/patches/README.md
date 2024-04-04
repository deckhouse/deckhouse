# Patches

### 001-add-registry-secret-as-dockerconfigjson.patch

This patch adds docker auth config via kubernetes volume/volumeMount to scanjobs in Standalone mode so that trivy init container can download trivy-db from a private registry. ClientServer mode doesn't have to download trivy-db on its own.
This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

### 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.


### 003-aws-ecr.patch

In ClientServer mode scan pods can't retrive images from Amazon Elastic Container Registry, because `AWS_REGION` environment variable is absent.
This patch is fixing this behaviour.

[PR](https://github.com/aquasecurity/trivy-operator/pull/1613)


### 004-scan-job-registry-ca.patch

This patch adds the ability to specify CA for scan jobs via environment variables.

[Issue](https://github.com/deckhouse/deckhouse/issues/4950)


### 005-cis-benchmark-on-startup.patch

The first check begins instantly when the operator starts.

[Issue](https://github.com/deckhouse/deckhouse/issues/5174)

### 006-new-metrics.patch

This patch adds new prometheus metrics for reports.
