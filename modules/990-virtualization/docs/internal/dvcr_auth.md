# DVCR auth

## VM

NodeGroupConfiguration is used to setup access to DVCR for containerd.
Hook 'discovery_clusterip_service_for_dvcr.py' detects ClusterIP of DVCR and containerd uses it as mirror.

## CVMI/VMI/VMD

virtualization-controller distributes auth secret on demand to support accessing DVCR.

DVCR_AUTH_SECRET, DVCR_AUTH_SECRET_NAMESPACE variables determine a Secret
with auth credentials. It should be of type 'kubernetes.io/dockerconfigjson'.

When DVCR is a destination (all of CVMIs, some VMI and VMD):

- If DVCR_AUTH_SECRET_NAMESPACE equals to vmi/vmd/cvmi namespace, the Secret is used as is. Its name passed as IMPORTER_DESTINATION_SECRET variable.
- If namespaces are different, virtualization-controller makes a copy of the auth Secret into:
  - VMI: vmi-dvcr-auth-<VMI-name> in VMI namespace
  - VMD: vmd-dvcr-auth-<VMD-name> in VMD namespace
  - CVMI: cvmi-dvcr-auth-<CVMI-name> in virtualization-controller Pod namespace
- virtualization-controller mounts this copy into dvcr-importer/uploader Pod
- dvcr-importer/uploader extracts user and password from /dvcr-auth/.dockerconfigjson file.

When DVCR is a source for PVC:

- DataVolume is used to import from DVCR registry into PVC.
- CDI expects Opaque Secret with accessKeyId and secretKey fields, so virtualization-controller makes a copy:
    - VMI: vmi-dvcr-auth-dv-<VMI-name> in VMI namespace.
    - VMD: vmd-dvcr-auth-dv-<VMD-name> in VMD namespace.


virtualization-controller should delete these Secrets when import is done.

