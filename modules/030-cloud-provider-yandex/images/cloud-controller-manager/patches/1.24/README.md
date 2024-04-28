# Patches

## 001-lock-route-table-operations.patch

Lock route tables operations, since we perform whole route table update on each call from cloud-provider controller.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/48)

## 002-use-real-internal-ip-for-routes.patch

Use real InternalIP for route table.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/53)

### 003-add-tg-node-annotation.patch

To set node to the non-default target group add annotation yandex.cpi.flant.com/target-group to the node. Yandex CCM creates new target groups with name yandex.cpi.flant.com/target-group annotation value + network id of instance interfaces.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/60)
