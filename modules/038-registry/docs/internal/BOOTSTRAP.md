# Bootstrap in different registry modes

DKP cluster bootstrap is supported in `Direct`, `Unmanaged`, `Proxy`, and `Local` modes.
The registry parameters and the mode used during cluster installation are configured via
the `deckhouse` ModuleConfig (`spec.settings.registry`), which is passed to `dhctl bootstrap` through `--config`.

---

## `Unmanaged` mode (configurable)

Create a `config.yaml` with the `Unmanaged` mode settings for the `deckhouse` ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    logLevel: Info
    registry:
      mode: Unmanaged
      unmanaged:
        imagesRepo: registry.deckhouse.ru/deckhouse/ee
        scheme: HTTPS
        license: "<LICENSE_KEY>" # Replace with your license key
```

Start the installation:

```bash
dhctl bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml      # config with mc/deckhouse.spec.settings.registry.mode: Unmanaged
```

---

## `Direct` mode

Create a `config.yaml` with the `Direct` mode settings for the `deckhouse` ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    logLevel: Info
    registry:
      mode: Direct
      direct:
        imagesRepo: registry.deckhouse.ru/deckhouse/ee
        scheme: HTTPS
        license: "<LICENSE_KEY>" # Replace with your license key
```

Start the installation:

```bash
dhctl bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml      # config with mc/deckhouse.spec.settings.registry.mode: Direct
```

---

## `Proxy` mode

Create a `config.yaml` with the `Proxy` mode settings for the `deckhouse` ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  enabled: true
  settings:
    bundle: Default
    logLevel: Info
    registry:
      mode: Proxy
      proxy:
        imagesRepo: registry.deckhouse.ru/deckhouse/ee
        scheme: HTTPS
        license: "<LICENSE_KEY>" # Replace with your license key
```

Start the installation on a static node:

```bash
dhctl bootstrap \
  --ssh-host=<address> \ # static node
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml  # config with mc/deckhouse.spec.settings.registry.mode: Proxy
```

---

## `Local` mode

When bootstrapping in `Local` mode, DKP images are prepared in advance as a bundle (`d8 mirror pull`)
and passed to `dhctl bootstrap` through `--img-bundle-path`.

1. Pull the image bundle:

   ```bash
   d8 mirror pull \
     --license="..." \
     --source="registry.deckhouse.ru/deckhouse/ee" \
     --include-module=console@v1.45.0 \
     --include-module=pod-reloader@v1.0.7 \
     --include-module=prompp@v3.7.9 \
     --deckhouse-tag="..." \
     ./bundle
   ```

1. Create a `config.yaml` with the `Local` mode settings for the `deckhouse` ModuleConfig:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       bundle: Default
       logLevel: Info
       registry:
         mode: Local
   ```

1. Start the installation on a static node:

   ```bash
   dhctl bootstrap \
     --ssh-host=<address> \     # static node
     --ssh-user=ubuntu \
     --ssh-agent-private-keys=~/.ssh/id_rsa \
     --config=/config.yaml \    # previously configured config with mc/deckhouse.spec.settings.registry.mode: Local
     --img-bundle-path=./bundle # path to the bundle folder
   ```
