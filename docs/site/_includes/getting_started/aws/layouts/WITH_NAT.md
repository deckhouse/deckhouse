![resources](/images/gs/cloud-provider-aws/aws-withnat.png)
<!--- source: https://docs.google.com/drawings/d/1UPzygO3w8wsRNHEna2uoYB-69qvW6zDYB5s1OumUOes/edit --->

In this Layout, a bastion host is created together with the cluster. Access to the cluster nodes will be possible through the bastion.

Virtual machines access the Internet using a NAT Gateway with a shared (and single) source IP.

> **Caution!** The NAT Gateway is always created in zone `a` in this layout. If cluster nodes are placed in other zones, then if there are problems in zone `a`, they will also be unavailable. In other words, when choosing the `WithNat` layout, the availability of the entire cluster will depend on the availability of zone `a`.
