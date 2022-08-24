# Patches

## 001-netfilter-compatibility-mode.patch

Helps with handling LoadBalancer/NodePort traffic to hostNetwork endpoints.

Taken from <https://github.com/cilium/cilium/pull/17504>

## 002-skip-host-ip-gc.patch

Fixes host connection reset when host policies are enabled and created.

<https://github.com/cilium/cilium/pull/19998>

## 003-fix-bwmap-delete-errors.patch

Fixes errors in metrics from Bandwidth controller. Also adds proper logging for a couple of controllers.

<https://github.com/cilium/cilium/pull/20611>

## 004-health-check-terminating.patch

TODO
