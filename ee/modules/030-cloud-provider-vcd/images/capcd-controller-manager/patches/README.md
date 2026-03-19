### 001-our-machinery.patch

Files:

- controllers/cluster_scripts/cloud_init.tmpl
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.
- Update klog to klog/v2 in controllers
- Update CAPI imports from v1beta1 to v1beta2 (api/core/v1beta2, api/bootstrap/kubeadm/v1beta2, etc.)

### 002-patch-webhook-server-port.patch

Files:

- main.go

Changes:

- Change webhook server port to 4201
- Update klog to klog/v2 in main.go
- Update CAPI imports from v1beta1 to v1beta2
- Set zap as klog logger

### 003-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update dependencies 
- Update cluster-api from v1.7.4 to v1.12.3 (v1beta2 contract)
- Update kubernetes dependencies to v0.34.3
- Update controller-runtime to v0.22.5
- Add google.golang.org/genproto excludes to resolve dependency conflicts

### 004-klog.patch

Files:

- pkg/capisdk/defined_entity.go

Changes:

- Update klog to klog/v2 in other files

### 005-add-vcdmachine-spec-template-org.patch

Files:

- api/v1beta2/vcdmachine_types.go

Changes:

- Update CAPI imports from v1beta1 to v1beta2 in vcdmachine_types.go
- Add TemplateOrg field to VCDMachine spec to specify the organization of the template OVA
- Add metadata support for VCDMachine (metadata types, structure and field in Spec)
- Allows adding custom metadata to virtual machines for organizing and categorizing inventory

### 006-update-api-v1beta1-imports.patch

Files:

- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go

Changes:

- Update CAPI imports in api/v1beta1 files from v1beta1 to v1beta2 paths
- Required for backward compatibility API conversion to work with CAPI v1.12.3

### 007-update-test-imports.patch

Files:

- tests/e2e/utils/cluster_upgrade_utils.go
- tests/e2e/utils/node_pool_scaling_utils.go
- tests/e2e/workload_cluster_upgrade_test.go
- tests/e2e/workload_cluster_resize_test.go

Changes:

- Update CAPI imports in test files from v1beta1 to v1beta2 paths
