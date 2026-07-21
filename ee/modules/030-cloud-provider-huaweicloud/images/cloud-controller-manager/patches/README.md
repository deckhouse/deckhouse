## 001-fix-empty-shared-bandwidth-id-sending.patch

Prevent sending empty shared bandwidth identifier in HTTP requests to create EIP.

## 001-go-mod.patch

Update dependencies

### 002-fix-cluster-name-handling.patch

For each `Service`, two load balancers were provisioned: one with the correct cluster name and another with the default cluster name. It is incorrect to have two load balancers for the same service. This patch fixes the issue by ensuring that only one load balancer is created with the correct cluster name.

### 003-enterprise-project-id.patch

Add supported enterprise-project-id

### 004-fix_providerid_and_exclude_loopback_in_node_IP_selection.patch

Fix providerID format and exclude 127.0.0.0/8 in node IP selections

### 005-default-lb-class-and-algorithm.patch

Add default values for `elb.class` (`shared`) and `lb-algorithm` (`ROUND_ROBIN`) so that LoadBalancer services work without requiring these annotations to be set explicitly.

### 006-ignore-static-nodes.patch

Static nodes are registered with `providerID` set to `static://`, which does not match the cloud provider's expected instance ID format. Without this patch, the CCM fails to resolve such nodes in `InstanceExistsByProviderID`, `InstanceExists`, `InstanceShutdownByProviderID`, `NodeAddressesByProviderID` and `InstanceMetadata`. In particular, a failing `InstanceMetadata` call prevents the CCM from ever removing the `node.cloudprovider.kubernetes.io/uninitialized` taint from a freshly added static node, so the node never finishes initialization and gets recreated once the StaticMachine bootstrap timeout is reached. This patch makes the CCM treat any `static://` provider ID as a node the cloud provider does not manage, returning safe defaults instead of errors.
