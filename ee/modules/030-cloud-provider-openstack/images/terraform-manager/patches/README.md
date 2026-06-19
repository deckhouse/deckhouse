## 001-implement_openstack_compute_servergroup_v2_data_source.patch
add data source for openstack_compute_servergroup_v2

## 002-go-mod.patch

Bump go.mod dependencies to fix known CVEs.

## 003-empty-metadata-fix.patch
Empty metadata always create diff. Set empty map instead nil for metadata when read resource.
