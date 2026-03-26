### 001-our-machinery.patch

Files:

- controllers/cluster_scripts/cloud_init.tmpl
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.
- Update klog to klog/v2 in controllers/vcdcluster_controller.go and controllers/vcdmachine_controller.go

### 002-patch-webhook-server-port.patch

Files:

- main.go

Changes:

- Change webhook server port to 4201
- Update klog to klog/v2 in main.go
- Set zap as klog logger

### 003-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update dependencies

### 004-capi-v1.12.3-update-dependencies.patch

Files:

- go.mod
- go.sum

Changes:

- Update CAPI from v1.7.4 to v1.12.3
- Update controller-runtime from v0.17.3 to v0.22.5
- Update kubernetes dependencies from v0.29.3 to v0.34.3
- Update all transitive dependencies to compatible versions

### 005-capi-v1.12.3-update-imports.patch

Files:

- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1beta2/vcdcluster_types.go
- api/v1beta2/vcdmachine_types.go
- api/v1beta2/vcdmachinetemplate_types.go
- api/v1beta2/zz_generated.deepcopy.go
- controllers/capi_objects_utils.go
- controllers/condition_consts.go
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go
- main.go
- tests/e2e/utils/cluster_upgrade_utils.go
- tests/e2e/utils/node_pool_scaling_utils.go
- tests/e2e/workload_cluster_resize_test.go
- tests/e2e/workload_cluster_upgrade_test.go

Changes:

- Update all CAPI package imports to v1.12.3 structure:
  - api/v1beta1 → api/core/v1beta2
  - bootstrap/kubeadm/api/v1beta1 → api/bootstrap/kubeadm/v1beta2
  - controlplane/kubeadm/api/v1beta1 → api/controlplane/kubeadm/v1beta2
  - exp/addons/api/v1beta1 → api/addons/v1beta2

### 006-capi-v1.12.3-api-compatibility.patch

Files:

- api/v1beta2/vcdcluster_types.go
- api/v1beta2/vcdcluster_webhook.go
- api/v1beta2/vcdmachine_types.go
- api/v1beta2/vcdmachine_webhook.go
- api/v1beta2/zz_generated.deepcopy.go
- controllers/capi_objects_utils.go
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go
- go.mod
- go.sum
- main.go
- tests/e2e/utils/node_pool_scaling_utils.go

Changes:

- Update controller-runtime webhook interfaces:
  - webhook.Defaulter → admission.CustomDefaulter
  - webhook.Validator → admission.CustomValidator
  - Add context.Context parameter to all webhook methods
  - Return type changed to (admission.Warnings, error)
- Add V1Beta1 condition compatibility methods to VCDCluster and VCDMachine:
  - GetV1Beta1Conditions() and SetV1Beta1Conditions() for deprecated conditions package
- Update to deprecated conditions package: sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1
- Replace boolean status fields with condition checks:
  - cluster.Status.ControlPlaneReady → conditions.IsTrue(cluster, clusterv1.ClusterControlPlaneAvailableCondition)
  - cluster.Status.InfrastructureReady → conditions.IsTrue(cluster, clusterv1.ClusterInfrastructureReadyCondition)
- Update main.go for new manager options:
  - DiagnosticsOptions → ManagerOptions
  - Use webhook.NewServer with combined TLS options and port configuration
- Update controller-runtime v0.22.5 imports:
  - Added sigs.k8s.io/controller-runtime/pkg/event
  - Added sigs.k8s.io/controller-runtime/pkg/predicate

### 007-add-vcdmachine-spec-template-org.patch

Files:

- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1beta2/vcdcluster_types.go
- api/v1beta2/vcdcluster_webhook.go
- api/v1beta2/vcdmachine_types.go
- api/v1beta2/vcdmachine_webhook.go
- api/v1beta2/vcdmachinetemplate_types.go
- api/v1beta2/zz_generated.deepcopy.go
- controllers/capi_objects_utils.go
- controllers/cluster_scripts/cloud_init.tmpl
- controllers/condition_consts.go
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go
- go.mod
- go.sum
- main.go
- tests/e2e/utils/cluster_upgrade_utils.go
- tests/e2e/utils/node_pool_scaling_utils.go
- tests/e2e/workload_cluster_resize_test.go
- tests/e2e/workload_cluster_upgrade_test.go

Changes:

- Add TemplateOrg field to VCDMachineSpec
- Update AddNewVM call to use TemplateOrg parameter and add guestCustScript parameter
- Note: This patch also restores changes from patch 001 that were inadvertently removed by patch 006
- The patch includes all changes from 001-our-machinery.patch that are needed for proper VM creation

### 008-add-metadata.patch

Files:

- api/v1beta2/vcdmachine_types.go
- api/v1beta2/zz_generated.deepcopy.go
- controllers/vcdmachine_controller.go

Changes:

- Add VCDMachineMetadata type with constants for metadata types (String, Number, Boolean, DateTime)
- Add VCDMachineMetadata struct with fields: Key, Value, Type, UserAccess, IsSystem
- Add Metadata field to VCDMachineSpec as array of VCDMachineMetadata
- Implement metadata application logic in vcdmachine_controller after VM creation
- Add convertMetadataType helper function to convert metadata types to VCD SDK types
- Add DeepCopy methods for VCDMachineMetadata in generated code
