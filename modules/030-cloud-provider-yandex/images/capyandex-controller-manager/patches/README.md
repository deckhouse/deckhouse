# 001-use-existing-control-plane-endpoint.patch

This directory contains patches applied to the upstream `yandex-cloud/cluster-api-provider-yandex`
source during the `capyandex-controller-manager` image build.

## 001-use-existing-control-plane-endpoint.patch

Adds control plane endpoint management modes:
- `ManagedLoadBalancer` keeps the upstream behavior.
- `External` skips load balancer reconciliation and sets
  `spec.controlPlaneEndpoint` from the controller manager `RESTConfig.Host`.

## 002-support-yandex-machine-metadata.patch

Adds support for propagating `YandexMachine.spec.metadata` into the Yandex Cloud
instance create request. This is required for Deckhouse bootstrap metadata such as
`node-network-cidr`.

## 003-support-capi-v1beta2-initialization-contract.patch

Adds `status.initialization.provisioned` support for `YandexCluster` and
`YandexMachine`, and sets it when the provider marks infrastructure ready. This
is required for the CAPI v1beta2 Machine/Cluster controllers to treat
infrastructure provisioning as completed.

## 004-migrate-to-cluster-api-v1-11-phase-1.patch

Pulls in the essential code changes from upstream PR
`yandex-cloud/cluster-api-provider-yandex#46` to move CAPY to Cluster API 1.11:
- updates dependencies to `sigs.k8s.io/cluster-api v1.11.3`
- switches typed imports to `api/core/v1beta2` and `api/core/v1beta1`
- moves condition helpers to deprecated v1beta1 compatibility package
- updates controller-runtime predicates, watches, and webhook decoder types

This patch intentionally excludes upstream CI, README, tests, and CRD churn. It
only carries the runtime and API changes required by the Deckhouse CAPY fork.
