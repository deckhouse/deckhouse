# Архитектура взаимодействия

Документ описывает архитектуру взаимодействия модуля `registry` с ключевыми
подсистемами Deckhouse Kubernetes Platform (DKP).

## Как было

Изначально управление registry выполнялось через единый секрет `deckhouse-registry`.
Этот секрет одновременно конфигурировал две разные подсистемы:

- **global** — рендеринг манифестов модулей с конфигурацией из
  `deckhouse-registry`;
- **node-manager** — рендеринг bashible-бандла с конфигурацией registry в containerd на узлах.

### Проблемы такой схемы

1. **Смешение двух разных контуров доступа в одном секрете.**

2. **Отсутствие оркестрации и этапности.**
   Любое изменение секрета приводило к одновременному (параллельному) применению новой
   конфигурации сразу во всех компонентах. Не было управляемого поэтапного перехода.
   Из-за этого некорректное изменение `deckhouse-registry` могло привести к поломке кластера:
   deckhouse брал новые параметры, перерендеривал манифесты и себя, но из-за отсутствия конфигураций на узлах впоследствии падал с `ImagePullBackOff`.
   Сам bashible не мог раскатить новые конфиги, так как ждал пробуждения deckhouse.

```mermaid
flowchart LR
  subgraph W["Without the registry module<br/>Рендеринг шаблонов"]
    direction LR

    S["Secret: deckhouse-registry"]
    G["Module Global"]

    D["Module Deckhouse"]
    R["Module Registry"]
    N["Module Node-Manager"]
    O["Module .... other"]

    MD["Manifests..."]
    MR["Manifests..."]
    MN["Manifests..."]
    MO["Manifests..."]

    N2["Module Node-Manager"]
    C["Containerd: registry config"]

    S --> G

    G --> D
    G --> R
    G --> N
    G --> O

    D --> MD
    R --> MR
    N --> MN
    O --> MO

    S --> N2
    N2 --> C
  end

  classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
  classDef manifest fill:#e6e6e6,stroke:#999,color:#333;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;

  class S secret;
  class G,D,R,N,O,N2 module;
  class MD,MR,MN,MO manifest;
  class C cri;
```

## Как стало

Модуль `registry` разделяет конфигурацию global и node-manager и вводит управляемый,
поэтапный переход между режимами работы registry:

1. **Разделение контуров доступа.**
   Введён отдельный секрет `registry-bashible-config` для конфигурации узлов. Таким образом:
   - для **API-доступа** (in-cluster) + рендеринга шаблонов используется `deckhouse-registry`;
   - для **CRI-доступа** (containerd на узлах) используется `registry-bashible-config`.

   Если модуль `registry` не используется, поведение остаётся обратно совместимым: node-manager
   конфигурирует containerd по `deckhouse-registry` (как раньше).

2. **Оркестрация и этапность.**
   Модуль `registry` содержит **orchestrator** — стейт-машину, которая управляет переходом
   между режимами (Direct / Proxy / Local / Unmanaged). Переход выполняется поэтапно.

Общая картина «с модулем registry» разбита на четыре части — по одной на каждый модуль. Они соединяются через секреты:
- модуль `deckhouse` создаёт `registry-config`;
- модуль `registry` считывает его и публикует `deckhouse-registry` и `registry-bashible-config`;
- модули `node-manager` и `global` используют полученные от модуля `registry` секреты.

**Module Deckhouse**

```mermaid
flowchart LR
  subgraph DECKHOUSE["Module Deckhouse"]
    direction LR

    MC["ModuleConfig"]

    MDH["Module Deckhouse"]
    VH["ValidationHook"]

    SRC_TOP["Secret: registry-config"]

    MREG_TOP["Module Registry"]

    MC --> MDH
    MDH --> VH
    MDH --> SRC_TOP
    VH --> MC
    SRC_TOP --> MREG_TOP
  end

  classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
  classDef config fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  classDef hook fill:#ffd6e7,stroke:#c4477f,color:#52102e;

  class MC config;
  class SRC_TOP secret;
  class MDH,MREG_TOP module;
  class VH hook;
```

Модуль `deckhouse` выполняет:

- создание секрета `registry-config` — рендеринг секрета из переданных в `mc/deckhouse` параметров registry. Рендеринг позволяет заполнять параметры по-умолчанию (default-ы в openapi спеке `mc/deckhouse`);
- создание **validation webhook** — хук валидации входных параметров. Дополнительно есть go-хук,
  который извлекает текущий режим из registry, чтобы построить validation-хук, проверяющий
  допустимость редактирования `mc/deckhouse` и смену режимов.

**Module Registry**

```mermaid
flowchart LR
  subgraph REGISTRY["Module Registry"]
    direction LR

    SRC["Secret: registry-config"]

    MR["Module Registry"]

    STATE["Secret: registry-state"]

    DEPLOY["Deployment: registry-incluster-proxy<br/>(Direct)"]

    DS["Daemonset: registry-nodeservices-manager<br/>(Proxy/Local)"]
    STATIC_NODE["Static pod: registry-nodeservices-&lt;node&gt;"]

    SDR["Secret: deckhouse-registry"]
    MGLOBAL_FROM_REG["Module Global"]

    SBASH["Secret: registry-bashible-config"]
    MNM_FROM_REG["Module Node-Manager"]

    SRC --> MR

    MR --> STATE
    STATE --> MR

    MR --> DEPLOY

    MR --> DS
    DS --> STATIC_NODE

    MR --> SDR
    SDR --> MGLOBAL_FROM_REG
    SDR --> MNM_FROM_REG

    MR --> SBASH
    SBASH --> MNM_FROM_REG
  end

  classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
  classDef workload fill:#cdebc5,stroke:#4c9a3f,color:#16400d;

  class SRC,STATE_OTHER,STATE,SDR,SBASH secret;
  class MR,MGLOBAL_FROM_REG,MNM_FROM_REG module;
  class DEPLOY,DS,STATIC_NODE workload;
```

Для модуля `registry` секрет `registry-config` является **входным** параметром (создаётся
модулем `deckhouse`).

**Входные параметры (snapshots orchestrator):**

- `registry-config` (secret) — конфигурация из deckhouse;
- `registry-init` (secret) — bootstrap-конфигурация;
- `registry-state` (secret) — сохранённое состояние стейт-машины;
- `deckhouse-registry` (secret) — текущие параметры registry;
- `registry-pki`, `registry-user-*` (secrets) — секреты состояния для PKI;
- `incluster-proxy`, `node-services` — компоненты модуля.

**Выходные параметры:**

- `incluster-proxy`, `node-services` и т. д. — компоненты registry;
- `registry-bashible-config` (secret) — конфигурация CRI для node-manager (bashible);
- `deckhouse-registry` (secret) — параметры API-доступа для global.

**Orchestrator** реализует стейт-машину, которая управляет переходом между режимами
(`Direct`, `Proxy`, `Local`, `Unmanaged`).

```mermaid
flowchart TD
    subgraph inputs["Входные данные"]
        cfg[("registry-config")]
        init[("registry-init")]
        st[("registry-state")]
        dr_in[("deckhouse-registry")]
    end

    subgraph orch["orchestrator (стейт-машина)"]
        initialize["initialize<br/>(bootstrap)"]
        process["process<br/>(приведение стейта к ожидаемому состоянию)"]
        initialize --> process
    end

    inputs --> orch

    orch -->|"CRI"| bc[("registry-bashible-config")]
    orch -->|"API"| dr_out[("deckhouse-registry")]
    orch --> comps["Компоненты registry:<br/>- incluster-proxy;<br/> - node-services<br/> - ..."]
    orch --> state_out[("registry-state<br/>(состояние + conditions)")]

    classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
    classDef stage fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
    classDef workload fill:#cdebc5,stroke:#4c9a3f,color:#16400d;

    class cfg,init,st,dr_in,bc,dr_out,state_out secret;
    class initialize,process,bashible stage;
    class comps workload;
```

**Module Node-Manager**

```mermaid
flowchart LR
  subgraph NODE_MANAGER["Module Node-Manager"]
    direction LR

    SDR_NM["Secret: deckhouse-registry"]
    SBASH_NM["Secret: registry-bashible-config"]

    MNM["Module Node-Manager"]

    CONTAINERD["Containerd: registry config"]
    STATIC_PROXY["Static pod: registry-proxy"]

    SDR_NM --> MNM
    SBASH_NM --> MNM

    MNM --> CONTAINERD
    MNM --> STATIC_PROXY
  end

  classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
  classDef workload fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;

  class SDR_NM,SBASH_NM secret;
  class MNM module;
  class CONTAINERD cri;
  class STATIC_PROXY workload;
```

`Node-manager` получает параметры registry и рендерит bashible bundle с подготовленной конфигурацией containerd.

Правило выбора секрета для рендеринга манифестов:
- если есть `registry-bashible-config` — используется он;
- иначе — используется `deckhouse-registry` (обратная совместимость).

Скрипты конфигурации на узле:

- применение настроек registry;
- запуск bashible-api-server;
- создание аннотаций на узле для обратной связи с модулем `registry`:
  - наличие кастомных скриптов в containerd — используется для preflight-проверки, можно ли
    запустить/переключить модуль;
  - применённая версия конфигурации модуля `registry`.

Аннотации на узлах — это канал обратной связи: orchestrator видит фактически применённую на
каждом узле версию и может вести переход поэтапно, не раскатывая новую конфигурацию на все
узлы одновременно.

```mermaid
flowchart TD
    bas["bashible-api-server"]

    bc{{"есть<br/>registry-bashible-config?"}}
    bas --> bc
    bc -->|"да"| use_bc[("registry-bashible-config<br/>(приоритет)")]
    bc -->|"нет"| use_dr[("deckhouse-registry<br/>(обратная совместимость)")]

    use_bc --> scripts["Рендеринг скриптов"]
    use_dr --> scripts

    scripts --> containerd["containerd"]
    scripts --> ann["Аннотации на узле<br/>(получение состояния конфигурации узлов)"]

    ann -.->|"обратная связь"| orch["registry (orchestrator)"]

    classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
    classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
    classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
    classDef decision fill:#ffe9a8,stroke:#d4a72c,color:#3d3000;

    class use_bc,use_dr secret;
    class bas,scripts,orch module;
    class containerd cri;
    class ann module;
    class bc decision;
```

**Module Global**

```mermaid
flowchart LR
  subgraph GLOBAL["Module Global<br/>Рендеринг шаблонов"]
    direction LR

    SDR_GLOBAL["Secret: deckhouse-registry"]
    MG["Module Global"]

    MOD_DECKHOUSE["Module Deckhouse"]
    MOD_REGISTRY["Module Registry"]
    MOD_NODE_MANAGER["Module Node-Manager"]
    MOD_OTHER["Module .... other"]

    MAN_DECKHOUSE["Manifests..."]
    MAN_REGISTRY["Manifests..."]
    MAN_NODE_MANAGER["Manifests..."]
    MAN_OTHER["Manifests..."]

    SDR_GLOBAL --> MG

    MG --> MOD_DECKHOUSE
    MG --> MOD_REGISTRY
    MG --> MOD_NODE_MANAGER
    MG --> MOD_OTHER

    MOD_DECKHOUSE --> MAN_DECKHOUSE
    MOD_REGISTRY --> MAN_REGISTRY
    MOD_NODE_MANAGER --> MAN_NODE_MANAGER
    MOD_OTHER --> MAN_OTHER
  end

  classDef secret fill:#fff3c4,stroke:#d4a72c,color:#3d3000;
  classDef module fill:#cfe8ff,stroke:#3b82c4,color:#0b2f52;
  classDef manifest fill:#e6e6e6,stroke:#999,color:#333;

  class SDR_GLOBAL secret;
  class MG,MOD_DECKHOUSE,MOD_REGISTRY,MOD_NODE_MANAGER,MOD_OTHER module;
  class MAN_DECKHOUSE,MAN_REGISTRY,MAN_NODE_MANAGER,MAN_OTHER manifest;
```

`global` считывает конфигурацию из `deckhouse-registry` и рендерит манифесты модулей всех
компонентов DKP. Дальнейшая работа с `deckhouse-registry` для API-доступа к registry
(operator-trivy, image-availability-exporter и т. д.) выполняется уже независимо другими
модулями.

## Архитектура переключения:

## Взаимодействие компонент модуля registry:

## Bootstrap кластера с модулем registry:

