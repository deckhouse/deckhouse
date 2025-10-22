# Patches

## 002-openkruise-daemonset-apiversion.patch

TODO

## 003-recommender.patch

This patch is not working for prometheus storage. Only for VPA checkpoints.
Have no idea, what it is for.
As we use Prometheus storage, will not move this patch.

## 004-in-place-metrics.patch

Fix misspelling and wrong prometheus counters for in-place metrics.
https://github.com/kubernetes/autoscaler/pull/8253

## 005-prometheus-bearer-auth.patch

Add support for bearer authentication in the prometheus metrics endpoint.
https://github.com/kubernetes/autoscaler/pull/8263
