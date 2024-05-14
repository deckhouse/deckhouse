# Patches

## 001-use-real-internal-ip-for-routes.patch

Use real InternalIP for route table.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/53)

### 002-add-tg-node-annotation.patch

To set node to the non-default target group add annotation yandex.cpi.flant.com/target-group to the node. Yandex CCM creates new target groups with name yandex.cpi.flant.com/target-group annotation value + network id of instance interfaces.

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/60)

### 003-internal-lb.patch

Added the ability to create an internal load balancer using the annotation yandex.cpi.flant.com/loadbalancer-internal: "".

Upstream [PR](https://github.com/deckhouse/yandex-cloud-controller-manager/pull/61)
