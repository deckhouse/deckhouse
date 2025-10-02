# CR handling contract

## Public API

All CRs in virtualization.deckhouse.io group are considered a public API.

Virtualization-controller will not change the main portion of this CRs: labels, annotations and spec.

Allowed changes to CRs are:
- Add and remove finalizers in metadata.finalizers.
- Add and remove owners in metadata.ownerReferences.
- Modify status subresource.

## CR status

Virtualization-controller may change CR status:
- Update info about resource state for humans, e.g. for using in additionalPrinterColumns.
  - These fields are typed and described in OpenAPI schema in CRD.
- Update info about resource state for computers, e.g. for web UI.
  - These fields are typed and described in OpenAPI schema in CRD.
- Store its own internal state viable for object converge, but not meaningful outside the controller.
  - These fields may remain untyped in OpenAPI schema in CRD.
APIService  may change CR status:
- Add external requests (possible kubevirt-style messaging from APIService to CR controller)
  - These fields may remain untyped in OpenAPI schema in CRD.

## Backend API

Virtualization-controller uses kubevirt.io and cdi.kubevirt.io CRs as a backend to manage
images, disks, and VMs.

These resources are fully managed by virtualization-controller. They may
receive additional annotations and labels. Any external changes to these
resources may lead to unpredictable behaviour of the virtualization-controller.
