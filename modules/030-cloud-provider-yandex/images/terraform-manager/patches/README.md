# Patches

## 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 002-vpc-address-detach-before-delete.patch

Release a `yandex_vpc_address` from the instance holding it before deleting it.

Removing `externalIPAddresses` from a node group asks Terraform to drop NAT from
the instance and to destroy the reserved address. Terraform offers no way to
guarantee that the instance update happens first, and in practice it does not:
the address destroy runs while the instance still holds the address, and the API
answers `FailedPrecondition: Address in use`. Retrying the delete does not help,
because nothing releases the address in the meantime.

The provider now detaches the one-to-one NAT that occupies the address and
retries the delete once. The VPC API does not report which resource holds an
address, so instances in the address folder are scanned for a matching NAT.
The detach is performed without confirmation: removing the address from
configuration is treated as consent to drop the NAT that keeps it busy.

Only instances are scanned. Addresses held by a load balancer or protected by
`deletion_protection` are not released; in that case the delete fails and the
original `FailedPrecondition` is included in the returned error so the operator
can see which resource still holds it.

Detach + retry runs under its own 5-minute timeout (`vpcAddressReleaseTimeout`)
so that instance listing, `RemoveOneToOneNat`, and the second `Delete` are not
capped by the 30-second default that governs the initial delete attempt.

Note that the failure surfaces as `error reading VPC address ...`: the provider
routes the delete error through `handleNotFoundError`, whose fallback message
mentions reading regardless of the operation that failed.

Remove this patch once the provider handles the release itself.
