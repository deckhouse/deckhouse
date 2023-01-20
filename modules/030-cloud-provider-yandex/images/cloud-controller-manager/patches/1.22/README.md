# Patches

## 001-lock-route-table-operations.patch

Lock route tables operations, since we perform whole route table update on each call from cloud-provider controller.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/48)
