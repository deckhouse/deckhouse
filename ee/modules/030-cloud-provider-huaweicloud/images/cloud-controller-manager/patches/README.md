## 001-fix-empty-shared-bandwidth-id-sending.patch

Prevent sending empty shared bandwidth identifier in HTTP requests to create EIP.

## 001-go-mod.patch

Update dependencies

### 002-fix-cluster-name-handling.patch

For each `Service`, two load balancers were provisioned: one with the correct cluster name and another with the default cluster name. It is incorrect to have two load balancers for the same service. This patch fixes the issue by ensuring that only one load balancer is created with the correct cluster name.
