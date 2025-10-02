CDI works only with PVC as destination. PVCs are not good for VM images,
we need a separate storage for them. CDI supports import from container image
to PVC, so idea is to use container registry as storage for VM images.

This storage is DVCR: Deckhouse Virtualization Container Registry.

Additional importer and uploader are implemented to import into DVCR instead PVC.

## Supported Data Sources

- HTTP (equals to http source in DataVolume)
- ContainerImage (equals to registry source in DataVolume)
- Upload (equals to upload source in DataVolume)
- VirtualImage (import from DVCR)
- ClusterVirtualImage (import from DVCR)
- VirtualDisk (import from DVCR)
- VirtualMachineDiskSnapshot - not implemented yet
- PersistentVolumeClaim - not implemented yet

## Supported storages (destinations)

- PersistentVolumeClaim - import into PVC.
- ContainerRegistry - import into DVCR.

## Possible import paths
- From Data Source to DVCR: controller will run dvcr-importer or dvcr-uploader.
- From Data Source to PVC: controller will start a 2-phase import:
  - First import into DVCR using dvcr-importer (or dvcr-uploader).
  - Then import DVCR image to the PVC using DataVolume.
- From DVCR to DVCR: controller will run dvcr-importer with custom 'dvcr' source.
- From DVCR to PVC: controller will create DataVolume with the 'registry' source and copy auth Secret and CA bundle ConfigMap.

### Import to DVCR
cvmi_importer.go, vmi_importer.go, vmd_importer.go

### Import to PVC
vmd_datavolume.go, vmi_datavolume.go

### Supplemental Secrets and ConfigMaps

In order to provide auth credentials and CA bundle to the importer Pod and DataVolume, virtualization-controller can
make additional Secrets and ConfigMaps.

#### dataSource HTTP and ContainerImage

HTTP and ContainerImage data sources may provide a caBundle string. Importer Pod expects ca bundle as
the file in mounted ConfigMap. virtualization-controller creates ConfigMap with ca bundle in the ca.crt field.

#### storage ContainerRegistry (DVCR)

When importing into DVCR, importer Pod expects auth Secret.
- Auth Secret is distributed across namespaces via the module hook.

Note: CA Bundle for DVCR is not implemented yet, DVCR is an internal Service, we use INSECURE_TLS=true to satisfy rbac-proxy.

#### storage PersistentVolumeClaim (import into PVC)

When importing into PVC, DataVolume use DVCR as registry source, so it expects auth credentials in the Secret, and ca bundle in the ConfigMap.
virtualization-controller creates copies of these resources:
- auth credentials are copied from the Secret specified in DVCR_AUTH_SECRET, DVCR_AUTH_SECRET_NAMESPACE variables.
  - username and password are extracted from .dockerconfigjson to accessKeyId and secretKey fields, as expected by DataVolume.
- ca bundle is copied from the Secret specified in DVCR_CERTS_SECRET, DVCR_CERTS_SECRET_NAMESPACE variables.
  - ca.pem field is extracted into ca.crt field in the new ConfigMap.
