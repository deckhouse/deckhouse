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

Note that the failure surfaces as `error reading VPC address ...`: the provider
routes the delete error through `handleNotFoundError`, whose fallback message
mentions reading regardless of the operation that failed.

Remove this patch once the provider handles the release itself.

## 002-vpc-address-delete-retry.patch

Retry `yandex_vpc_address` deletion while the API reports `FailedPrecondition`.

Removing `externalIPAddresses` from a node group makes Terraform update the instance
(dropping NAT) and then destroy the address. The API releases the address
asynchronously, so the delete that follows can still be rejected with
`FailedPrecondition: Address in use`, and the whole converge fails.

Note that the original error surfaces as `error reading VPC address ...`: the
provider routes the delete error through `handleNotFoundError`, whose fallback
message mentions reading regardless of the operation that failed.

Remove this patch once the retry lands upstream.
