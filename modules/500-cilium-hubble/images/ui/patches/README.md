# Patches

## 001--vm-name-icon.frontend.patch

Improved VM pod appearance in hubble-ui. Now it isn't an "Unknown App", but some VM with name and proper icon.

Hubble UI:

- Uses the label value `vm.kubevirt.internal.virtualization.deckhouse.io/name=<name>` as the name.
- Uses the presence of the `label kubevirt.internal.virtualization.deckhouse.io=virt-launcher` to change the icon.

> **NOTE:**  There is a SVG-file in the patch.

## 002--gomod-gosum.backend.patch

Updated go dependencies to fix vulnerabilities, e.g.:

- `github.com/cilium/ebpf` -> `v0.22.0` (CVE-2026-10722)
- `github.com/cilium/cilium` -> `v1.19.5` (CVE-2026-56743)
- `google.golang.org/grpc` -> `v1.83.2` (CVE-2026-84303, CVE-2026-84304, CVE-2026-84445)

## 003--auth.backend.patch

Added backend authorization logic.

## 003--auth.frontend.patch

Added frontend authorization logic.
