# Patches

## 001-add-registry-secret-as-dockerconfigjson.patch

This [issue](https://github.com/aquasecurity/trivy-operator/issues/695) covers both trivy and trivy-operator. We've decided not to pursue patch upstreaming for now.

## 002-skip-some-checks.patch

Skip some defseq checks for proper reports result for deckhouse installation.


## 003-aws-ecr.patch

In ClientServer mode scan pods can't retrive images from Amazon Elastic Container Registry, because `AWS_REGION` environment variable is absent.
This patch is fixing this behaviour.

[PR](https://github.com/aquasecurity/trivy-operator/pull/1613)
