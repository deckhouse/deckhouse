![resources](/images/gs/cloud-provider-openstack/openstack-standard.png)
<!--- Source: https://docs.google.com/drawings/d/1hjmDn2aJj3ru3kBR6Jd6MAW3NWJZMNkend_K43cMN0w/edit --->

In this scheme, an internal cluster network is created with a gateway to the public network; the nodes do not have public IP addresses. Note that the floating IP is assigned to the master node.

> **Caution!**
> If the provider does not support SecurityGroups, all applications running on nodes with floating IPs assigned will be available at a public IP. For example, kube-apiserver on master nodes will be available on port 6443. To avoid this, we recommend using the SimpleWithInternalNetwork placement strategy.
