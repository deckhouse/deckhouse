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

### 005-our-machinery.patch

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
- api/v1beta1/vcdcluster_types.go
- api/v1beta1/vcdmachine_types.go
- api/v1beta1/zz_generated.conversion.go
- api/v1beta1/zz_generated.deepcopy.go
- api/v1alpha4/vcdcluster_types.go
- api/v1alpha4/vcdmachine_types.go
- api/v1alpha4/zz_generated.conversion.go
- api/v1alpha4/zz_generated.deepcopy.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.
- Update klog to klog/v2 in controllers and main.go
- Update CAPI imports from v1beta1 to v1beta2 (api/core/v1beta2, api/bootstrap/kubeadm/v1beta2, etc.) in api/v1beta2
- Update CAPI imports in api/v1beta1 files: change `sigs.k8s.io/cluster-api/api/v1beta1` to `sigs.k8s.io/cluster-api/api/core/v1beta2` (v1beta1 path no longer exists in CAPI v1.12.3)
- Update CAPI imports in api/v1alpha4 files: change `sigs.k8s.io/cluster-api/api/v1alpha4` to `sigs.k8s.io/cluster-api/api/core/v1beta2` (v1alpha4 path no longer exists in CAPI v1.12.3)
- Fix zz_generated.deepcopy.go: import CAPI v1beta2 with alias, replace v1beta1.Conditions and v1beta1.MachineAddress with v1beta2 types
- Fix webhook files: remove webhook import, add admission import, replace webhook.Defaulter/Validator with admission.CustomDefaulter/CustomValidator, update ValidateCreate/ValidateUpdate/ValidateDelete signatures to return (admission.Warnings, error) instead of just error (required for controller-runtime v0.22.5)
- Add TemplateOrg field to VCDMachine spec to specify the organization of the template OVA
- Add metadata support for VCDMachine (metadata types, structure and field in Spec)

### 006-capi-v1beta2-compat.patch

Files:

- controllers/capi_objects_utils.go

Changes:

- Fix CAPI v1.12.3 API compatibility issues in controller code
- Update `kcp.Spec.MachineTemplate.InfrastructureRef` to `kcp.Spec.MachineTemplate.Spec.InfrastructureRef` (InfrastructureRef moved to Spec)
- Replace `vcdMachineTemplateRef.Namespace` with `kcp.Namespace` or `md.Namespace` (ContractVersionedObjectReference no longer has Namespace field)
- Add explicit v1.ObjectReference conversion from ContractVersionedObjectReference with Kind, Name, and Namespace fields
- Fix ReadyReplicas pointer dereference: `md.Status.ReadyReplicas` is now `*int32` instead of `int32`, add nil check
- Fix machine.Spec.Version comparison: now `string` instead of `*string`, change checks from `!= nil && *Version` to `!= ""`
- Add `kubeadmbootstrapv1` alias for `sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2` import
- Replace all `v1beta1.KubeadmConfigTemplate` with `kubeadmbootstrapv1.KubeadmConfigTemplate`
