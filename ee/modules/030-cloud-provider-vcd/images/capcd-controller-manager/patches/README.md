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

### 004-klog.patch

Files:

- pkg/capisdk/defined_entity.go

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

- api/v1beta2/vcdmachine_types.go
- api/v1beta2/zz_generated.deepcopy.go
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachines.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachinetemplates.yaml
- controllers/vcdmachine_controller.go

Changes:

- Added logic for adding additional metadata for the virtual machine
