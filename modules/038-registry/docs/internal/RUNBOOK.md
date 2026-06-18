# RUNBOOK

Документ описывает диагностику и действия при ошибках переключения registry.

Дополнительные команды проверок:

**Проверка статуса переключения**
```bash
watch -c "kubectl -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller module values registry | yq '.internal.orchestrator.state.conditions // []'"
```

**Проверка очереди deckhouse**
```bash
watch kubectl -n d8-system exec -i svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue list
```

## Диагностика этапов переключения

### `RegistryContainsRequiredImages`

1. Проверьте очередь deckhouse. В очереди не должно быть ошибок.
2. Проверьте статус переключения. В статусе будет указана ошибка доступности registry и образов в нем:
  ```Yaml
  ...
  - lastTransitionTime: "2026-06-18T08:41:23Z"
    message: |-
      Mode: Default
      some-nexus.io: 0 of 182 items processed, 182 items with errors:
      - source: deckhouse/containers/deckhouse
        image: some-nexus.io/nexus/deckhouse/path:release-1.76
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/terraform-manager/terraform-manager-dvp
        image: some-nexus.io/nexus/deckhouse/path@sha256:0429bcb05580b5b8a55242953dcacc4f150a8d757844a184bc2c5295d9de6d03
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/cloud-provider-vcd/cloud-data-discoverer-legacy
        image: some-nexus.io/nexus/deckhouse/path@sha256:04f22995347e40b5d64ef2b898ecc3eced40367ab0ee312222150cd5e6dd46a4
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/control-plane-manager/kube-controller-manager133
        image: some-nexus.io/nexus/deckhouse/path@sha256:05bdde23b414ed662946bbfda8c611240f2df17c40ee4af297ba7318a0caad81
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host
      - source: module/cloud-provider-gcp/cloud-controller-manager131
        image: some-nexus.io/nexus/deckhouse/path@sha256:071f70dd9cc6c38c8d62fd9a26ae885d5be6a1892ca89ebda3df6c90ce4a6880
        error: Get "https://some-nexus.io/v2/": dial tcp: lookup some-nexus.io on 10.222.0.10:53: no such host

        ...and more
    reason: Processing
    status: "False"
    type: RegistryContainsRequiredImages
  ...
  ```

3. Если ошибка связана с доступность registry:
   1. Проверьте, доступен ли registry с узлов кластера. Пример команды для выполнения проверки: `ctr images pull --tlscacert=./path/to/ca --user="name:pass" --http-dump some-nexus.io/deckhouse/path:release-1.76`;
   2. Проверьте корректность введенных параметров в `mc/deckhouse`. Если параметры введены неверно - исправьте их;

4. Если ошибка связана с образами (свое хранилище образов):
   1. Проверьте, загружен ли образ в локальное хранилище образов `ctr images pull --tlscacert=./path/to/ca --user="name:pass" --http-dump some-nexus.io/deckhouse/path:release-1.76`
   2. Проверьте, нет ли в локальном хранилище образов ошибок (логи хранилища);


> [!NOTE]
> Для режима `Local` этап будет в ошибке до тех пор, пока в локальный реестр не будет загружен заранее подготовленный bundle образов командой `d8 mirror push`. Загрузите образы и дождитесь повторной проверки.
> Пример: (../EXAMPLES_RU.md#переключение-на-режим-local)[../EXAMPLES_RU.md#переключение-на-режим-local]

### `ContainerdConfigPreflightReady`

1. Проверьте очередь deckhouse. В очереди не должно быть ошибок.
2. Проверьте статус переключения. В статусе будет указана ошибка выполнения префлай проверки:
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

3. Если в статусе указана ошибка `has custom toml merge containerd configuration`. Небходимо выполнить миграцию. Подробный пример: (../FAQ_RU.md#как-мигрировать-на-модуль-registry)[../FAQ_RU.md#как-мигрировать-на-модуль-registry]


### `TransitionContainerdConfigReady`

Аналогично пункту `FinalContainerdConfigReady`

### `FinalContainerdConfigReady`


1. Проверьте очередь deckhouse. В очереди не должно быть ошибок.
2. Проверьте статус переключения. В статусе будет указан процесс прогона новой версии bashible bundle c новой версией конфигурации registry:
```bash
...
- lastTransitionTime: "2026-06-18T08:41:23Z"
  message: |
    Applying configuration to nodes
    1/3 node(s) ready. Waiting:
    - master-1: "a1b2c3d4..." → "e5f6a7b8..."
    - worker-0: "a1b2c3d4..." → "e5f6a7b8..."
  reason: Processing
  status: "False"
  type: FinalContainerdConfigReady
...
```

3. Если condition долго не проходит:
   1. Проверьте логи bashible на нодах: `journalctl -u bashible.service --no-pager -f`;
   2. Если ошибок нет, проверьте логи компонентов модуля `node-manager` в namespace `d8-cloud-instance-manager`;
4. Убедитесь, что на узлах имеется требуемая конфигурация containerd в директории `/etc/containerd/registry.d`


### `InClusterProxyReady`


1. Проверьте очередь deckhouse. В очереди не должно быть ошибок.
2. Проверьте статус переключения. В статусе будет указана ошибка развертывания компонента `registry-incluster-proxy`.
3. Проверьте статут развертывания deployment `registry-incluster-proxy`. Deployment должен развернуть все поды. В обычном режиме/HA = 1/кол-во мастер узлов. В логах подов не должно быть ошибок:
  ```bash
  $ kubectl -n d8-system get deployment registry-incluster-proxy -o yaml
  $ kubectl -n d8-system describe deployment registry-incluster-proxy
  $ kubectl -n d8-system logs pod registry-incluster-proxy-<replica>
  ```

### `CleanupInClusterProxy`

1. Проверьте статут удаления deployment `registry-incluster-proxy`:
  ```bash
  $ kubectl -n d8-system get deployment registry-incluster-proxy -o yaml
  $ kubectl -n d8-system describe deployment registry-incluster-proxy
  ```
2. Если deployment не удаляется, можно выполнить удаление вручную.
3. Проверьте статус переключения. В статусе должна пропасть ошибка.

### `NodeServicesReady`

1. Проверьте очередь deckhouse. В очереди не должно быть ошибок.
2. Проверьте статус переключения. В статусе будет указана ошибка развертывания компонента `registry-nodeservices`:
  ```yaml
  ...
  - message: |
      1/3 node(s) ready. Waiting:
      - master-1: node is not Ready
      - master-2: services pod(s) is not Ready or config version mismatch (!= "e5f6a7b8...")
    reason: Processing
    status: "False"
    type: NodeServicesReady
  ...
  ```
3. Проверьте статус развертывания daemonset `registry-nodeservices-manager`. Daemonset должен развернуть все поды. Кол-во подов = кол-во мастер узлов. В логах не должно быть ошибок:
  ```bash
  $ kubectl -n d8-system get daemonset registry-nodeservices-manager -o yaml
  $ kubectl -n d8-system describe daemonset registry-nodeservices-manager
  $ kubectl -n d8-system logs pod registry-nodeservices-manager-<master-node>
  ```
4. Проверьте стату развертывания static pod-ов самого registry `registry-nodeservices-<master-node>`. В логах подов не должно быть ошибок:
  ```bash
  $ kubectl -n d8-system get pod registry-nodeservices-<master-node> -o yaml
  $ kubectl -n d8-system describe pod registry-nodeservices-<master-node>
  $ kubectl -n d8-system logs pod registry-nodeservices-manager-<master-node>
  ```
5. Проверьте статус ноды. Нода должна быть в состоянии `Ready`:
  ```bash
  $ kubectl get node <master-node> -o yaml
  $ kubectl describe node <master-node>
  ```

### `CleanupNodeServices`

1. Проверьте состояние daemonset `registry-nodeservices-manager`. Daemonset должен удалить static pods registry `registry-nodeservices-<node>`;
2. Проверьте удалился ли daemonset `registry-nodeservices-manager`.
3. Если на ноде не разворачивается экземпляр `registry-nodeservices-manager` для удаления `registry-nodeservices-<node>`. Удалите static pod вручную:
   ```bash
   mv /etc/kubernetes/manifests/registry-nodeservices.yaml ~/registry-nodeservices.yaml
   mv /etc/kubernetes/manifests/registry ~/registry
   ```
4. Проверьте статус переключения. В статусе должна пропасть ошибка.

### `DeckhouseRegistrySwitchReady`

1. Проверьте статус переключения. В статусе будет указана ошибка развертывания компонента `registry-nodeservices`:
  ```yaml
  ...
  - message: |
      Waiting for deckhouse-controller to become ready
    reason: Processing
    status: "False"
    type: DeckhouseRegistrySwitchReady
  ...
  ```
2. Если ошибка: `Waiting for deckhouse-controller to become ready`:
   1. Проверьте очередь deckhouse. В очереди не должно быть ошибок. Deckhouse должен выполнить все хуки во всех модуля. После выполнения всех хуков и рендеринга всех манифестов, deckhouse перейдет в состояние `Ready`.
   2. Проверьте логи deckhouse - в логах не должно быть ошибок.

### `ErrTransitionNotSupported`

1. Если в conditions появился `ErrTransitionNotSupported` со `status: "True"` и `reason: Error`, был запрошен
неподдерживаемый переход между режимами. Не поддерживаются переходы:
- `Proxy` → `Local`;
- `Local` → `Proxy`;
- `Local` → неконфигурируемый `Unmanaged` (без `imagesRepo`).
1. Для переключения в данные режимы, необходимо выполнить переключение в промежуточный режим `Direct`/`Unmanaged`. Затем, можно выполнить переключение в необходимый режим.
