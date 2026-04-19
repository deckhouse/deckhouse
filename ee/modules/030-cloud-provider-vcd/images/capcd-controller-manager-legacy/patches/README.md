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
