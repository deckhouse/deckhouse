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

### 004-capi-v1.12.3-compatibility.patch

Files:

- api/v1alpha4/vcdcluster_types.go
- api/v1alpha4/vcdmachine_types.go
- api/v1alpha4/zz_generated.conversion.go
- api/v1alpha4/zz_generated.deepcopy.go
- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1beta2/vcdcluster_webhook.go
- api/v1beta2/vcdmachine_webhook.go
- controllers/capi_objects_utils.go

Changes:

- Update CAPI imports from api/v1alpha4 and api/v1beta1 to api/core/v1beta2 for CAPI v1.12.3 compatibility
- Update webhook interfaces from webhook.Defaulter/Validator to admission.CustomDefaulter/CustomValidator
- Add context.Context parameter to all webhook methods and return (admission.Warnings, error)
- Fix InfrastructureRef path: kcp.Spec.MachineTemplate.InfrastructureRef -> kcp.Spec.MachineTemplate.Spec.InfrastructureRef
- Handle ReadyReplicas as pointer type (*int32)
- Handle machine.Spec.Version as string instead of *string

### 005-add-vcdmachine-spec-template-org.patch

Files:

- controllers/vcdmachine_controller.go
- api/v1beta2/vcdmachine_types.go

Changes:

- Add TemplateOrg field to VCDMachine spec

### 006-add-metadata.patch

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

- Add metadata field for VM
