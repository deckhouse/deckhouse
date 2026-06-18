# RUNBOOK

## Что делать, если этап не прошел

### `RegistryContainsRequiredImages`

Необходимо посмотреть на статус переключения. В статусе будет отображена ошибка проверки. Пример ошибки недоступности внешнего registry:
```bash
$ d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)
...
- lastTransitionTime: "2026-06-18T08:41:23Z"
  message: |-
    Mode: Default
    some-nexus.io: 0 of 182 items processed, 182 items with errors:
    - source: deckhouse/containers/deckhouse
      image: some-nexus.io/nexus/internal/registry/path:release-1.76
      error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
    - source: module/terraform-manager/terraform-manager-dvp
      image: some-nexus.io/nexus/internal/registry/path@sha256:0429bcb05580b5b8a55242953dcacc4f150a8d757844a184bc2c5295d9de6d03
      error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
    - source: module/cloud-provider-vcd/cloud-data-discoverer-legacy
      image: some-nexus.io/nexus/internal/registry/path@sha256:04f22995347e40b5d64ef2b898ecc3eced40367ab0ee312222150cd5e6dd46a4
      error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
    - source: module/control-plane-manager/kube-controller-manager133
      image: some-nexus.io/nexus/internal/registry/path@sha256:05bdde23b414ed662946bbfda8c611240f2df17c40ee4af297ba7318a0caad81
      error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
    - source: module/cloud-provider-gcp/cloud-controller-manager131
      image: some-nexus.io/nexus/internal/registry/path@sha256:071f70dd9cc6c38c8d62fd9a26ae885d5be6a1892ca89ebda3df6c90ce4a6880
      error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host

      ...and more
  reason: Processing
  status: "False"
  type: RegistryContainsRequiredImages
...
```

Для исправления ошибки:
1. Проверьте доступ до registry с узлов кластера.
2. Проверьте корректность ввода параметров registry в `mc/deckhouse`. Если параметры указаны неправильно — исправьте их на корректные. Дождитесь выполнения проверки с новыми параметрами.

### `ContainerdConfigPreflightReady`

Необходимо посмотреть статус переключения. Если в статусе отображается информация вида:
```bash
$ d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)
...
- lastTransitionTime: "2026-06-18T08:41:23Z"
  message: |
    Check current nodes configuration
    2/2 node(s) Unready:
    - master-0: has custom toml merge containerd configuration
    - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
...
```

Значит на узлах имеются кастомные конфиги containerd. Необходимо выполнить миграцию на новый формат конфигов, на который будет в дальнейшем переключен кластер.

### `TransitionContainerdConfigReady`

<!-- TODO: описать диагностику и действия при зависании раскатки переходного конфига containerd. -->

### `InClusterProxyReady`

<!-- TODO: описать диагностику и действия, если `registry-incluster-proxy` не поднимается. -->

### `NodeServicesReady`

<!-- TODO: описать диагностику и действия, если раскатка `registry-nodeservices-manager` и static pod-ов `registry-nodeservices-<node>` не завершается. -->

### `DeckhouseRegistrySwitchReady`

<!-- TODO: описать диагностику и действия, если переключение DKP на новый registry не завершается. -->

### `CleanupNodeServices`

<!-- TODO: описать диагностику и действия, если node-services не удаляются. -->

### `CleanupInClusterProxy`

<!-- TODO: описать диагностику и действия, если `registry-incluster-proxy` не удаляется. -->

### `FinalContainerdConfigReady`

<!-- TODO: описать диагностику и действия при зависании раскатки финального конфига containerd. -->
