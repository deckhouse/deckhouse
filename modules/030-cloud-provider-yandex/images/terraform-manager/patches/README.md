# Patches

## 001-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

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
