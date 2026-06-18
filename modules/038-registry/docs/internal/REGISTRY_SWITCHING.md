# Процесс переключения

Внутренний механизм переключения выполнен в виде finite state machine (orchestrator).
Orchestrator на каждой итерации вызова продвигает переключение на один шаг и фиксирует результат в conditions секрета
`registry-state`.
Пока текущий шаг не готов (`status: "False"`), orchestrator не переходит к следующему шагу.

## Компоненты и conditions

| Компонент         | Описание                                                                                | В каких режимах используется |
| ----------------- | --------------------------------------------------------------------------------------- | ---------------------------- |
| `incluster-proxy` | `Deployment registry-incluster-proxy` — proxy для in-cluster обращения к registry       | `Direct`                     |
| `node-services`   | реестр на узлах (`registry-nodeservices-<node>`)                                        | `Proxy`, `Local`             |
| `proxy`           | proxy на каждом узле, балансирует запросы на `registry-nodeservices-<node>`             | `Proxy`, `Local`             |
| `service`         | сервис `registry.d8-system.svc:5001` — точка входа для in-cluster обращения к registry  | `Direct`, `Proxy`, `Local`   |
| `ingress`         | публичный доступ к локальному реестру (`registry.<PUBLIC_DOMAIN>`) для `d8 mirror push` | `Local`                      |
| `checker`         | проверка наличия требуемых образов в целевом реестре                                    | все режимы                   |


| Condition                         | Описание                                                                                 |
| --------------------------------- | ---------------------------------------------------------------------------------------- |
| `RegistryContainsRequiredImages`  | проверка реестра на наличие образов DKP                                                  |
| `ContainerdConfigPreflightReady`  | preflight проверка, на узлах нет кастомных конфигов containerd                           |
| `NodeServicesReady`               | раскатка `registry-nodeservices-manager` и static pod-ов `registry-nodeservices-<node>`  |
| `InClusterProxyReady`             | раскатка `registry-incluster-proxy` deployment-а                                         |
| `TransitionContainerdConfigReady` | bashible (transition) на узлы раскатан **переходный** конфиг containerd (старый + новый) |
| `DeckhouseRegistrySwitchReady`    | переключение DKP на новый registry (`deckhouse-registry` обновлён)                       |
| `FinalContainerdConfigReady`      | bashible (finalize) на узлах остался **только** новый источник, старый удалён            |
| `CleanupNodeServices`             | удалены `registry-nodeservices-manager` и static pod-ы `registry-nodeservices-<node>`    |
| `CleanupInClusterProxy`           | удален `registry-incluster-proxy` deployment                                             |
| `Ready`                           | итог, переход завершён, `mode == target_mode`                                            |
| `ErrTransitionNotSupported`       | ошибка, запрошен недопустимый переход (например `Proxy` → `Local`)                       |


## Переключение в режим Direct или смена параметров режима Direct

Обращение containerd идёт напрямую в registry через виртуальный адрес `registry.d8-system.svc:5001/system/deckhouse`.
Это позволяет задействовать механизм mirroring в containerd. Обращение к `registry.d8-system.svc:5001/system/deckhouse` транслируется в upstream registry.

```toml
[host]
  [host."https://some-nexus.io"]
    capabilities = ["pull", "resolve"]
    [host."https://some-nexus.io".auth]
      username = "admin"
      password = "admin123"
    [[host."https://some-nexus.io".rewrite]]
      regex = "^system/deckhouse"
      replace = "nexus/internal/registry/path"
```

In-cluster обращение выполняется через внутренний некешируемый proxy-сервис `registry-incluster-proxy`. Обращение к нему выполняется через реальный сервис `registry.d8-system.svc`. На уровне `registry-incluster-proxy` обращение конвертируется аналогично mirror-ингу в containerd. Обращение к `registry.d8-system.svc:5001/system/deckhouse` транслируется в upstream registry.

Переключение в режим Direct выполняется со следующей последовательностью этапов:

```mermaid
flowchart TD
    A["RegistryContainsRequiredImages<br/>проверка внешнего реестра"]
    B["ContainerdConfigPreflightReady<br/>preflight узлов"]
    C["TransitionContainerdConfigReady<br/>containerd: старый + новый источник"]
    D["InClusterProxyReady<br/>раскатка registry-incluster-proxy"]
    E["DeckhouseRegistrySwitchReady<br/>переключение DKP на новый registry"]
    F["FinalContainerdConfigReady<br/>containerd: только новый источник"]
    G["CleanupNodeServices<br/>остановка node-services (если были)"]
    H["Ready<br/>mode = Direct"]

    A --> B --> C --> D --> E --> F --> G --> H

    classDef cond fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
    class A,B,C,D,E,F,G,H cond;
```

### Этап 1 — `RegistryContainsRequiredImages`

На этом этапе выполняется проверка наличия необходимых (critical) компонентов во внешнем registry.
```bash
$ kubectl get modules -o json | jq -r '.items[] | select(.properties.critical == true and .properties.source == "Embedded") | .metadata.name'
cloud-provider-aws
cni-cilium
deckhouse
node-manager
registry
...
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `RegistryContainsRequiredImages`](RUNBOOK.md#registrycontainsrequiredimages).


### Этап 2 — `ContainerdConfigPreflightReady`

На этом этапе выполняется проверка наличия старой версии кастомных конфигов registry в containerd v1, добавляемых через механизм `toml-merge` в скриптах bashible-бандла.
Проверка наличия конфигов выполняется через проверку лейбла `node.deckhouse.io/containerd-config-registry=custom` на узле, который устанавливает bashible.

Если кастомных конфигов нет — продолжится выполнение следующих шагов.

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `ContainerdConfigPreflightReady`](RUNBOOK.md#containerdconfigpreflightready).


### Этап 3 — `TransitionContainerdConfigReady`
На узлы раскатывается переходный конфиг containerd: активны оба источника — старый и новый.

Взаимодействие выполняется через секрет `registry-bashible-config`. Его конфигурирует оркестратор, который следит за версией конфигурации через аннотацию `registry.deckhouse.io/version=...` на узле.

Данный конфиг получает `bashible-api-server`. Bashible конфигурирует registry-конфиг в containerd и проставляет на узле аннотацию с принятой раскатанной версией конфига.

Если переключение выполнялось из режима Unmanaged, в директории конфигурации будет 2 папки:
```bash
$ ls -alh /etc/containerd/registry.d/
some-nexus.io # конфигурация Unmanaged режима
registry.d8-system.svc:5001 # Конфигурация Direct режима
...
```

Если переключение выполнялось из режима Proxy/Local/Direct в режим Direct, в конфигурации будет 1 папка:
```bash
$ ls -alh /etc/containerd/registry.d/
registry.d8-system.svc:5001 # Конфигурация Old + New
...
```

Внутри будет расположен файл конфигурации `host.toml` с mirror массивом для старой и новой версии конфигурации:
```bash
$ cat /etc/containerd/registry.d/registry.d8-system.svc:5001/host.toml
[host]
  [host."https://old-nexus.io"]
    capabilities = ["pull", "resolve"]
    [host."https://old-nexus.io".auth]
      username = "admin"
      password = "admin123"
    [[host."https://old-nexus.io".rewrite]]
      regex = "^system/deckhouse"
      replace = "nexus/internal/registry/path"

  [host."https://new-nexus.io"]
    capabilities = ["pull", "resolve"]
    [host."https://new-nexus.io".auth]
      username = "admin"
      password = "admin123"
    [[host."https://new-nexus.io".rewrite]]
      regex = "^system/deckhouse"
      replace = "nexus/internal/registry/path"
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `TransitionContainerdConfigReady`](RUNBOOK.md#transitioncontainerdconfigready).


### Этап 4 — `InClusterProxyReady`

На данном этапе на master-узлах кластера поднимается `Deployment` `registry-incluster-proxy`.
Если кластер находится в HA — поднимается несколько экземпляров приложения.

На данном этапе только поднимается компонент. Переключение на его использование пока не выполняется.


```mermaid
flowchart LR
  COND["InClusterProxyReady"]

  subgraph CLUSTER["Кластер"]
    PROXY["registry-incluster-proxy<br/>(Deployment) ✅"]
    SVC(["registry service<br/>registry.d8-system.svc:5001"])
    NODES["Узлы (containerd:<br/>старый + новый источник)"]
  end

  EXT[("Внешний реестр")]

  SVC --> PROXY
  PROXY -.->|"для in-cluster pull"| EXT
  NODES ==>|"CRI pull (во внешний реестр напрямую)"| EXT

  classDef cond fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  class COND cond;
  class PROXY,SVC work;
  class NODES cri;
  class EXT ext;
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `InClusterProxyReady`](RUNBOOK.md#inclusterproxyready).

### Этап 5 — `DeckhouseRegistrySwitchReady`

К данному этапу подготовлены:
- Старый и новый конфиг registry в containerd;
- Incluster proxy компонент для Direct режима;

Здесь выполняется переключение DKP на использование подготовленных компонентов registry.
В момент переключения выполняется:
1. Переключение сервиса `registry.d8-system.svc:5001` с компонентов прошлого режима (например, static pod из режима Proxy/Local) на компонент incluster-proxy.
2. Обновление секрета `deckhouse-registry` (это основная конфигурация registry для всего DKP).

После переключения DKP начинает смотреть на новый сконфигурированный режим/registry:
- внутренние компоненты — через `registry.d8-system.svc:5001` на `incluster-proxy`;
- containerd — использует новую конфигурацию (это либо отдельный конфиг, либо новый настроенный mirror).

Дополнительно запускается механизм ожидания DKP:
- проверка аннотации `registry.deckhouse.io/version=...` на deployment deckhouse (проверка, что deckhouse использует новую версию registry);
- проверка, что deckhouse находится в состоянии ready. Состояние ready описывает, выполнился ли первый прогон хуков и рендеринг манифестов для всех модулей.

После выполнения данного этапа запускается процесс очистки старой конфигурации registry.

```mermaid
flowchart LR
  COND["DeckhouseRegistrySwitchReady →<br/>FinalContainerdConfigReady"]

  subgraph CLUSTER["Кластер"]
    INPULL["In-cluster pull<br/>(controller, trivy, ...)"]
    SVC(["registry service"])
    PROXY["registry-incluster-proxy ✅"]
    NODES["Узлы (containerd:<br/>только внешний реестр)"]
  end

  EXT[("Внешний реестр")]

  INPULL ==>|"in-cluster pull"| SVC --> PROXY
  PROXY ==> EXT
  NODES ==>|"CRI pull (напрямую)"| EXT

  classDef cond fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  class COND cond;
  class PROXY,SVC,INPULL work;
  class NODES cri;
  class EXT ext;
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `DeckhouseRegistrySwitchReady`](RUNBOOK.md#deckhouseregistryswitchready).


### Этап 6 — `CleanupNodeServices`

На данном этапе компоненты режима Local/Proxy уже не используются.
Данные компоненты удаляются из кластера.

Daemonset `registry-nodeservices-manager` удаляет static pod-ы `registry-nodeservices-<node>` с master-узлов. После успешного удаления из кластера удаляется сам `registry-nodeservices-manager`.

```mermaid
flowchart LR
  COND["CleanupNodeServices"]

  subgraph CLUSTER["Кластер"]
    MGR["registry-nodeservices-manager<br/>(DaemonSet) ❌ удаляется"]
    SP["registry-nodeservices-&lt;node&gt;<br/>(static pods) ❌ удаляются"]
  end

  COND --> MGR
  MGR ==>|"удаляет"| SP

  classDef cond fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef gone fill:#f7d4d4,stroke:#c45c5c,color:#5c1010;
  class COND cond;
  class MGR,SP gone;
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `CleanupNodeServices`](RUNBOOK.md#cleanupnodeservices).

### Этап 7 — `FinalContainerdConfigReady`

На данном этапе выполняется очистка старой конфигурации registry в containerd.
На узлы раскатывается финальный конфиг containerd.

Взаимодействие выполняется через секрет `registry-bashible-config`. Его конфигурирует оркестратор, который следит за версией конфигурации через аннотацию `registry.deckhouse.io/version=...` на узле.

Данный конфиг получает `bashible-api-server`. Bashible конфигурирует registry-конфиг в containerd и проставляет на узле аннотацию с принятой раскатанной версией конфига.

На узлах должна остаться одна конфигурация registry:
```bash
$ cat /etc/containerd/registry.d/registry.d8-system.svc:5001/host.toml
[host]
  [host."https://new-nexus.io"]
    capabilities = ["pull", "resolve"]
    [host."https://new-nexus.io".auth]
      username = "admin"
      password = "admin123"
    [[host."https://new-nexus.io".rewrite]]
      regex = "^system/deckhouse"
      replace = "nexus/internal/registry/path"
```

**Что делать, если этап не прошел**: см. [RUNBOOK.md → `FinalContainerdConfigReady`](RUNBOOK.md#finalcontainerdconfigready).

