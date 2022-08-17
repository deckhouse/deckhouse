## Patches

## 001-identify-instances-by-name.patch

Find nodes in the cloud by the `Name` tag containing Node privateDNSName.

(Deckhouse only feature)

## 002-non-type-lb.patch

Ability to create LoadBalancer with type `none`. LoadBalancer with this type will have managed target groups,
 which allows you to create ApplicationLoadBalancer with automatically managed targets.

Upstream [PR](https://github.com/kubernetes/cloud-provider-aws/pull/429)

## 003-dont-delete-ingress-sg-rules-elb.patch

We shouldn't delete Ingress SG rule, if it allows access from configured "ElbSecurityGroup", so that we won't disrupt access to Nodes from other ELBs.

Upstream [PR](https://github.com/kubernetes/kubernetes/pull/105194)
