## Patches

## 001-identify-instances-by-name.patch

Find nodes in the cloud by the `Name` tag containing Node privateDNSName.

## 002-non-type-lb.patch

Ability to create LoadBalancer with type `none`. LoadBalancer with this type will have managed target groups,
 which allows you to create ApplicationLoadBalancer with automatically managed targets.

## 003-dont-delete-ingress-sg-rules-elb.patch

We shouldn't delete Ingress SG rule, if it allows access from configured "ElbSecurityGroup", so that we won't disrupt access to Nodes from other ELBs.

## 004-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 005-fix-list-routes-method.patch

Modify `ListRoutes` method to handle errors gracefully without blocking reconcile loop.
The error is logged with `%v`, because klog does not support the `%w` directive (flagged by `go vet`).

## 006-publicNetworkAllowList-for-NLB.patch

Adds support for PublicNetworkAllowList to restrict incoming traffic to NLBs

## Dropped patches

`007-fix-getInstancesByIDs-batcher.patch` is not needed since v1.37.0: upstream fixed the same issue inside
`describeInstanceBatcher.DescribeInstances`, which now splits multi-ID inputs into single-ID batcher submissions
(kubernetes/cloud-provider-aws commit 7e2c8056, "Fix route-controller by handling multi-ID inputs in the batcher").
