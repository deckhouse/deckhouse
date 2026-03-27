### Patches

This patch is for our case when we want to have a static Nodes in the cluster, managed by vSphere cloud provider.

> Consider implementing a flag in CCM config and sending as a PR to the upstream.

### 001-ignore-static-nodes.patch

Files:

- pkg/ccm/instances.go

Changes:

- Ignore static nodes in providerID

### 002-add-vapptemplate-search-by-vapptemplate-id.patch

Files:

- pkg/vcdsdk/vapp.go

Changes:

- Add ability to search vApp template by id

### 003-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update go-vcloud-director SDK: v2.14.0-rc.3 → v2.21.0 (required for patch 005 which uses QueryCatalogRecords method introduced in v2.21.0)

### 004-klog.patch

Files:

- cmd/ccm/main.go
- pkg/ccm/cloud.go
- pkg/ccm/loadbalancer.go
- pkg/ccm/vminfocache.go
- pkg/config/cloudconfig.go
- pkg/cpisdk/rde.go
- pkg/testingsdk/k8sclient.go
- pkg/vcdsdk/auth.go
- pkg/vcdsdk/client.go
- pkg/vcdsdk/defined_entity.go
- pkg/vcdsdk/gateway.go
- pkg/vcdsdk/ipam.go

Changes:

- Update klog to klog/v2 in other files

### 005-add-vapptemplate-search-by-org.patch

Files:

- pkg/vcdsdk/vapp.go
- go.mod
- go.sum

Changes:

- Add support for searching vAppTemplates in a given org

### 006-fix-lb-health-monitor.patch

Files:

- pkg/ccm/loadbalancer.go

Changes:

- Fixes TCP health monitors removal during an update of the pool

### 007-fix-ccm-command-signature.patch

Files:

- cmd/ccm/main.go

Changes:

- Fix NewCloudControllerManagerCommand call signature for k8s.io v0.34.3
- In k8s.io v0.34.3, the function signature changed to include an additional `map[string]string` parameter for feature gates between `DefaultInitFuncConstructors` and `NamedFlagSets`
