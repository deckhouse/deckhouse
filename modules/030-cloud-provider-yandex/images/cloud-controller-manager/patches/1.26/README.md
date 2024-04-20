# Patches

### 001-add-tg-node-annotation.patch

To set node to the non-default target group add annotation yandex.cpi.flant.com/target-group to the node. Yandex CCM creates new target groups with name yandex.cpi.flant.com/target-group annotation value + network id of instance interfaces.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/60)
