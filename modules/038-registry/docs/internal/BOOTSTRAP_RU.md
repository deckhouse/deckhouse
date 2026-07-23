# Bootstrap в разных режимах registry

Bootstrap кластера DKP поддерживается в режимах `Direct`, `Unmanaged`, `Proxy` и `Local`.
Параметры registry и режим работы во время установки кластера настраиваются через
ModuleConfig `deckhouse` (`spec.settings.registry`), который передаётся в `dhctl bootstrap` через `--config`.

---

## Режим `Unmanaged` (конфигурируемый)

Создайте `config.yaml` с настройками режима `Unmanaged` для ModuleConfig `deckhouse`:

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
        license: "<LICENSE_KEY>" # Замените на ваш лицензионный ключ
```

Запустите установку:

```bash
dhctl bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml      # конфиг с mc/deckhouse.spec.settings.registry.mode: Unmanaged
```

---

## Режим `Direct`

Создайте `config.yaml` с настройками режима `Direct` для ModuleConfig `deckhouse`:

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
        license: "<LICENSE_KEY>" # Замените на ваш лицензионный ключ
```

Запустите установку:

```bash
dhctl bootstrap \
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml      # конфиг с mc/deckhouse.spec.settings.registry.mode: Direct
```

---

## Режим `Proxy`

Создайте `config.yaml` с настройками режима `Proxy` для ModuleConfig `deckhouse`:

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
        license: "<LICENSE_KEY>" # Замените на ваш лицензионный ключ
```

Запустите установку на static-ноду:

```bash
dhctl bootstrap \
  --ssh-host=<address> \ # static нода
  --ssh-user=ubuntu \
  --ssh-agent-private-keys=~/.ssh/id_rsa \
  --config=/config.yaml  # конфиг с mc/deckhouse.spec.settings.registry.mode: Proxy
```

---

## Режим `Local`

При bootstrap в режиме `Local` образы DKP подготавливаются заранее в виде bundle (`d8 mirror pull`)
и передаются в `dhctl bootstrap` через `--img-bundle-path`.

1. Спулите bundle с образами:

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

1. Создайте `config.yaml` с настройками режима `Local` для ModuleConfig `deckhouse`:

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

1. Запустите установку на static-ноду:

   ```bash
   dhctl bootstrap \
     --ssh-host=<address> \     # static нода
     --ssh-user=ubuntu \
     --ssh-agent-private-keys=~/.ssh/id_rsa \
     --config=/config.yaml \    # ранее настроенный конфиг с mc/deckhouse.spec.settings.registry.mode: Local
     --img-bundle-path=./bundle # путь до папки с bundle
   ```
