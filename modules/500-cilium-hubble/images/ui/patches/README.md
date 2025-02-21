# Patches

## 001-frontend--vm-name-icon.patch

Improved VM pod appearance in hubble-ui. Now it isn't an "Unknown App", but some VM with name and proper icon.

Hubble UI:

- Uses the label value `vm.kubevirt.internal.virtualization.deckhouse.io/name=<name>` as the name.
- Uses the presence of the `label kubevirt.internal.virtualization.deckhouse.io=virt-launcher` to change the icon.

> **NOTE:**  There is a SVG-file in the patch.

## 002-backend--gomod-gosum.patch

Updated go dependencies to fix vulnerabilities.
