### 001-our-machinery.patch

Files:

- controllers/cluster_scripts/cloud_init.tmpl
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go
- main.go
- controllers/condition_consts.go
- controllers/capi_objects_utils.go
- api/v1beta2/vcdcluster_types.go
- api/v1beta2/vcdmachinetemplate_types.go
- api/v1beta2/vcdmachine_types.go
- api/v1beta2/zz_generated.deepcopy.go
- api/v1beta2/vcdcluster_webhook.go
- api/v1beta2/vcdmachine_webhook.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.
- Update klog to klog/v2 in controllers and main.go
- Update CAPI imports from v1beta1 to v1beta2 (api/core/v1beta2, api/bootstrap/kubeadm/v1beta2, etc.) in all files
- Fix zz_generated.deepcopy.go: import CAPI v1beta2 with alias, replace v1beta1.Conditions and v1beta1.MachineAddress with v1beta2 types
- Fix webhook files: remove webhook import, add admission import, replace webhook.Defaulter/Validator with admission.Defaulter/Validator, update ValidateCreate/ValidateUpdate/ValidateDelete signatures to return (admission.Warnings, error) instead of just error (required for controller-runtime v0.22.5)
- Add TemplateOrg field to VCDMachine spec to specify the organization of the template OVA
- Add metadata support for VCDMachine (metadata types, structure and field in Spec)

### 002-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update dependencies
- Update cluster-api from v1.4.0 to v1.12.3 (v1beta2 contract)
- Update kubernetes dependencies to v0.34.3
- Update controller-runtime to v0.22.5
- Add google.golang.org/genproto excludes to resolve dependency conflicts

### 003-klog.patch

Files:

- pkg/capisdk/defined_entity.go

Changes:

- Update klog to klog/v2 in other files

### 006-update-api-imports.patch

Files:

- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1alpha4/vcdcluster_types.go
- api/v1alpha4/vcdmachine_types.go
- api/v1alpha4/zz_generated.conversion.go
- api/v1alpha4/zz_generated.deepcopy.go

Changes:

- Update CAPI imports in api/v1beta1 and api/v1alpha4 files from v1beta1 to v1beta2 paths
- Required for backward compatibility API conversion to work with CAPI v1.12.3

### 007-update-test-imports.patch

Files:

- tests/e2e/utils/cluster_upgrade_utils.go
- tests/e2e/utils/node_pool_scaling_utils.go
- tests/e2e/workload_cluster_upgrade_test.go
- tests/e2e/workload_cluster_resize_test.go

Changes:

- Update CAPI imports in test files from v1beta1 to v1beta2 paths
