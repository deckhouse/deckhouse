## Istio image build targets

The module builds version-specific images for the supported Istio releases:

- `common`, `cni`, `istioctl`, `kiali`, `pilot`, and `proxyv2` for 1.25.2, 1.27.9, and 1.29.6;
- `operator` for the operator-backed Istio 1.25.2 release.

Each target is defined in its corresponding `*-v1x<minor>x<patch>/werf.inc.yaml` directory. Images are built from the upstream Istio sources pinned by that target, with Deckhouse patches and vulnerability metadata stored alongside the build definition.
