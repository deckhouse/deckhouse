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

- Add TemplateOrg field to VCDMachine spec to specify the organization of the template OVA

### 006-add-metadata.patch

Files:

- api/v1beta2/zz_generated.deepcopy.go
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachines.yaml
- config/crd/bases/infrastructure.cluster.x-k8s.io_vcdmachinetemplates.yaml

Changes:

- Add metadata support for VCDMachine (generated deepcopy code and CRD updates)
- Allows adding custom metadata to virtual machines for organizing and categorizing inventory
