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

### 004-capi-v1.12.3-compatibility.patch

Files:

- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1beta2/vcdcluster_webhook.go
- api/v1beta2/vcdmachine_webhook.go
- controllers/capi_objects_utils.go

Changes:

- Update CAPI imports from api/v1beta1 to api/core/v1beta2 for CAPI v1.12.3 compatibility
- Update webhook interfaces from webhook.Defaulter/Validator to admission.CustomDefaulter/CustomValidator
- Add context.Context parameter to all webhook methods
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

- api/v1beta2/vcdmachine_types.go
- api/v1beta2/zz_generated.deepcopy.go
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachines.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachinetemplates.yaml
- controllers/vcdmachine_controller.go

Changes:

- Added logic for adding additional metadata for the virtual machine
