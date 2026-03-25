### 001-our-machinery.patch

Files:

- controllers/cluster_scripts/cloud_init.tmpl
- controllers/vcdcluster_controller.go
- controllers/vcdmachine_controller.go
- main.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.

### 002-go-mod.patch

Files:

- go.mod
- go.sum

Changes:

- Update dependencies

### 003-klog.patch

Files:

- capisdk/defined_entity.go

Changes:

- Update klog to klog/v2 in other files

### 004-add-vcdmachine-spec-template-org.patch

Files:

- api/v1beta2/vcdmachine_types.go

Changes:

- Add TemplateOrg field to VCDMachineSpec
- Note: TemplateOrg parameter is not used in AddNewVM call in v1.2.0 as the SDK version doesn't support it yet

### 005-add-metadata.patch

Files:

- api/v1alpha4/zz_generated.deepcopy.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1beta2/vcdmachine_types.go
- api/v1beta2/zz_generated.deepcopy.go
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdclusters.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdclustertemplates.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachines.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachinetemplates.yaml
- config/rbac/role.yaml
- config/webhook/manifests.yaml
- controllers/vcdmachine_controller.go

Changes:

- Add VCDMachineMetadata type with constants for metadata types (String, Number, Boolean, DateTime)
- Add VCDMachineMetadata struct with fields: Key, Value, Type, UserAccess, IsSystem
- Add Metadata field to VCDMachineSpec as array of VCDMachineMetadata
- Implement metadata application logic in vcdmachine_controller after VM creation
- Add convertMetadataType helper function to convert metadata types to VCD SDK types
- Add DeepCopy methods for VCDMachineMetadata in generated code
- Update CRD manifests with metadata field definitions
