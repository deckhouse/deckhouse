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

### 004-go-mod.patch

Maintainers of the cloud-provider-aws forgot to make `go mod tidy` in the v1.22.3 tag, and we have dirty go.mod there.
Probably you don't need this patch in later tags.

(not important, fixed in later releases)
