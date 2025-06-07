## Patches

## 001-identify-instances-by-name.patch

Find nodes in the cloud by the `Name` tag containing Node privateDNSName.

## 002-non-type-lb.patch

Ability to create LoadBalancer with type `none`. LoadBalancer with this type will have managed target groups,
 which allows you to create ApplicationLoadBalancer with automatically managed targets.

## 003-dont-delete-ingress-sg-rules-elb.patch

We shouldn't delete Ingress SG rule, if it allows access from configured "ElbSecurityGroup", so that we won't disrupt access to Nodes from other ELBs.

## 004-bump-deps.patch

Update dependencies

## 005-fix-list-routes-method.patch

Modify `ListRoutes` method to handle errors gracefully without blocking reconcile loop.
