# Patches for capv-controller-manager

This directory contains patches applied to the upstream `kubernetes-sigs/cluster-api-provider-vsphere`
source during the `capv-controller-manager` image build.

No patches are shipped yet. Common reasons a patch may be added later:

- **CVE bumps in `go.mod`** — align transitive dependencies with the Deckhouse security baseline.
- **Trim supervisor-only controllers** — Deckhouse only uses the `govmomi` flavor; the
  vSphere-with-Kubernetes (`supervisor`) reconcilers can be removed to shrink the binary.
- **IPAM integration** — if the upstream `IPAddressClaim` watch conflicts with Deckhouse's own
  IPAM flow, disable it (mirrors the OpenStack `001-disable-floating-ip-pool-controller.patch`).
