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
- api/v1beta2/zz_generated.deepcopy.go

Changes:

- This patch is for our usage cases of cluster-api cloud provider.
- Update klog to klog/v2 in controllers and main.go
- Update CAPI imports from v1beta1 to v1beta2 (api/core/v1beta2, api/bootstrap/kubeadm/v1beta2, etc.) in all files

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

### 005-add-vcdmachine-spec-template-org.patch

Files:

- api/v1beta2/vcdmachine_types.go

Changes:

- Update CAPI imports from v1beta1 to v1beta2 in vcdmachine_types.go
- Add TemplateOrg field to VCDMachine spec to specify the organization of the template OVA
- Add metadata support for VCDMachine (metadata types, structure and field in Spec)
- Allows adding custom metadata to virtual machines for organizing and categorizing inventory
