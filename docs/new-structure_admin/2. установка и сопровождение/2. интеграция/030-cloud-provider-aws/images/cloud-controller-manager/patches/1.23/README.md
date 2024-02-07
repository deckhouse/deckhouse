# Patches

## 001-identify-instances-by-name.patch

Find nodes in the cloud by the `Name` tag containing Node privateDNSName.

This is required by our version of machine-controller-manager: <https://github.com/deckhouse/mcm/commit/f1608dd44075ffb861e668f5672e0b02f2a1fdef>

## 002-non-type-lb.patch

Adds an ability to create LoadBalancer with type `none`. LoadBalancers with this type will have managed target groups,
which allows you to create ApplicationLoadBalancer with automatically managed targets.

Upstream [PR](https://github.com/kubernetes/cloud-provider-aws/pull/429)

## 003-dont-delete-ingress-sg-rules-elb.patch

We shouldn't delete Ingress SG rule, if it allows access from configured "ElbSecurityGroup", so that we won't disrupt access to Nodes from other ELBs.

Upstream [PR](https://github.com/kubernetes/kubernetes/pull/105194)
