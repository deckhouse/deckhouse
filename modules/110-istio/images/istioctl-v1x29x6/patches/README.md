# Patches

## 001-deckhouse-default-istio-namespace.patch

Set `constants.IstioSystemNamespace` to the Deckhouse Istio namespace (`d8-istio`).

## 002-deckhouse-default-istio-revision.patch

Force the Deckhouse control-plane revision of this image (`v1x29`) for istioctl commands that talk to the control plane:

- default value of `--revision` / `-r`;
- user-provided `--revision` is overridden in `PersistentPreRunE`;
- `CLIClientWithRevision` always uses `v1x29`;
- `RevisionOrDefault` always returns `v1x29` (ignores revision tags and user input).
