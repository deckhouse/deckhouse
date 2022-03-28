## Patches

## Disable finalizers

This is our internal patch to disable finalizers logic for piraeus-operator custom resources.
It was the simpliest way to avoid dependency problem while deleting operator and custom resources at one time.
It makes no sense for us since all the resources are deployed in single namespace and managed together as one.

## RBAC-proxy

Adds extra options to allow deploying with kube-rbac-proxy
https://github.com/piraeusdatastore/piraeus-operator/pull/280
