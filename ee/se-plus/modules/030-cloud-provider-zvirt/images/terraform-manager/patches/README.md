## 001-xml-escape_cloud-init_data.patch

TODO: Add description

## 002-detach_disks_on_vm_removal.patch

TODO: Add description

## 003-go-mod.patch

update go.mod and up go version for cve fix

## 004-initialization_dns.patch

add vm.initialization_dns option

## 005-disk_resize_not_found.patch

Let `ovirt_disk_resize` clear its id when the disk it holds has been removed, instead of
answering the 404 with a hard error that fails every refresh from then on and takes plan,
apply and destroy with it. Every other resource in the provider already does this, and so
does `resizeDisk` in the same file.

## 006-initialization_nic_forcenew.patch

Replace the VM on any change to `initialization_nic`. Upstream put `ForceNew` on the
block only, which in the SDK does not reach the attributes inside it, and `vmUpdate`
sends only `name` and `comment` — so editing an address, netmask, gateway or interface
name planned as an in-place update that reached oVirt as nothing at all, leaving state
claiming a value the VM never got. This is how `customNetworkConfig` is delivered, so
every field of it now forces a destructive replacement.
