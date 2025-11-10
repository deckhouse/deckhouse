# Patches

## 000-update-go-libraries.patch

Update golang libraries and dependencies.

## 001-add-d8-annotations.patch

Add optional use of “network.deckhouse.io/load-balancer-ips” and “network.deckhouse.io/load-balancer-shared-ip-key” annotations.

## 002-disable-l2.patch

Prohibit initializing the L2 controller.

## 003-disable-new-pool-annotation.patch

Disabling the new annotation `metallb.universe.tf/ip-allocated-from-pool`, as well as warnings about using deprecated service annotations.
