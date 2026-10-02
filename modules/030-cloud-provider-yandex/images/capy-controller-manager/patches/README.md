# 001-use-existing-control-plane-endpoint.patch

This directory contains patches applied to the upstream `yandex-cloud/cluster-api-provider-yandex`
source during the `capy-controller-manager` image build.

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

Pulls in the essential code changes from upstream PR
`yandex-cloud/cluster-api-provider-yandex#46` and the Deckhouse follow-up fixes
required to run CAPY on Cluster API 1.11:
- updates dependencies to `sigs.k8s.io/cluster-api v1.11.3`
- switches typed imports to `api/core/v1beta2` and `api/core/v1beta1`
- updates controller-runtime predicates, watches, and webhook decoder types
- adds `status.initialization.provisioned` support for `YandexCluster` and
  `YandexMachine`
- migrates provider conditions to `[]metav1.Condition` with `conditions.Set(...)`
- adds owned-conditions patching for cluster, machine, and identity scopes
- normalizes legacy stored conditions before patching to preserve upgrade safety

This patch intentionally excludes upstream CI, README, tests, and CRD churn. It
only carries the runtime and API changes required by the Deckhouse CAPY fork.

## 004-go-mod.patch

Bumps `google.golang.org/grpc` to `v1.79.3` and the dependency set required by
that version to fix CVE-2026-33186.

## 005-support-preemptible-machines.patch

Pulls in the runtime and API changes from upstream PR
`yandex-cloud/cluster-api-provider-yandex#50`: adds `Preemptible *bool` to
`YandexMachineSpec` and its deepcopy, and sets `compute.SchedulingPolicy` on
the instance create request in `GetInstanceReq` so a `YandexMachine` with
`spec.preemptible: true` is created as a preemptible Yandex Cloud VM.

This patch intentionally excludes upstream's test, CRD and `templates/`
changes: the test is not exercised by any test gate on this source tree (only
`go build` runs on it), Deckhouse ships its own preserve-unknown-fields CRD
stub for `YandexMachine`/`YandexMachineTemplate`, and the `templates/` files
are upstream's own `clusterctl`/e2e example manifests, never read by
Deckhouse.
