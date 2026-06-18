# Architecture

This document describes the architecture of the `registry` module's interaction with the key
subsystems of the Deckhouse Kubernetes Platform (DKP).

## How it was before

Originally, registry management was performed through a single `deckhouse-registry` secret.
This secret simultaneously configured two different subsystems:

- **global** — rendering of module manifests using the configuration from
  `deckhouse-registry`;
- **node-manager** — rendering of the bashible bundle with the registry configuration for containerd on the nodes.

### Problems of the old switching mechanism

1. **Mixing two different access circuits in a single secret.**

2. **Lack of orchestration and staging.**
   Any change to the secret led to the new configuration being applied simultaneously (in parallel)
   across all components at once. There was no managed, staged transition.
   Because of this, an incorrect change to `deckhouse-registry` could break the cluster:
   deckhouse picked up the new parameters, re-rendered the manifests and itself, but because of the absence of configurations on the nodes it subsequently crashed with `ImagePullBackOff`.
   Bashible itself could not roll out the new configs, because it was waiting for deckhouse to wake up.

```mermaid
flowchart LR
  subgraph W["Without the registry module<br/>Template rendering"]
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

## How it is now

The `registry` module separates the global and node-manager configuration and introduces a managed,
staged transition between registry operating modes:

1. **Separation of access circuits.**
   A dedicated `registry-bashible-config` secret has been introduced for node configuration. As a result:
   - for **API access** (in-cluster) + template rendering, `deckhouse-registry` is used;
   - for **CRI access** (containerd on the nodes), `registry-bashible-config` is used.

   If the `registry` module is not used, the behavior remains backward compatible: node-manager
   configures containerd using `deckhouse-registry` (as before).

2. **Orchestration and staging.**
   The `registry` module contains an **orchestrator** — a state machine that manages the transition
   between modes (Direct / Proxy / Local / Unmanaged). The transition is performed in stages.

The overall picture "with the registry module" is split into four parts — one per module. They are connected through secrets:
- the `deckhouse` module creates `registry-config`;
- the `registry` module reads it and publishes `deckhouse-registry` and `registry-bashible-config`;
- the `node-manager` and `global` modules use the secrets received from the `registry` module.

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

The `deckhouse` module performs:

- creation of the `registry-config` secret — rendering the secret from the registry parameters passed in `mc/deckhouse`. Rendering allows filling in default parameter values (defaults in the openapi spec of `mc/deckhouse`);
- creation of a **validation webhook** — a hook that validates the input parameters. Additionally, there is a go-hook
  that extracts the current mode from the registry in order to build a validation hook that checks
  the admissibility of editing `mc/deckhouse` and switching modes.

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

For the `registry` module, the `registry-config` secret is an **input** parameter (created
by the `deckhouse` module).

**Input parameters (orchestrator snapshots):**

- `registry-config` (secret) — configuration from deckhouse;
- `registry-init` (secret) — bootstrap configuration;
- `registry-state` (secret) — saved state of the state machine;
- `deckhouse-registry` (secret) — current registry parameters;
- `registry-pki`, `registry-user-*` (secrets) — state secrets for PKI;
- `incluster-proxy`, `node-services` — module components.

**Output parameters:**

- `incluster-proxy`, `node-services`, etc. — registry components;
- `registry-bashible-config` (secret) — CRI configuration for node-manager (bashible);
- `deckhouse-registry` (secret) — API access parameters for global.

The **orchestrator** implements a state machine that manages the transition between modes
(`Direct`, `Proxy`, `Local`, `Unmanaged`).

```mermaid
flowchart TD
    subgraph inputs["Input data"]
        cfg[("registry-config")]
        init[("registry-init")]
        st[("registry-state")]
        dr_in[("deckhouse-registry")]
    end

    subgraph orch["orchestrator (state machine)"]
        initialize["initialize<br/>(bootstrap)"]
        process["process<br/>(reconciling the state to the expected one)"]
        initialize --> process
    end

    inputs --> orch

    orch -->|"CRI"| bc[("registry-bashible-config")]
    orch -->|"API"| dr_out[("deckhouse-registry")]
    orch --> comps["Registry components:<br/>- incluster-proxy;<br/> - node-services<br/> - ..."]
    orch --> state_out[("registry-state<br/>(state + conditions)")]

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

`Node-manager` receives the registry parameters and renders the bashible bundle with the prepared containerd configuration.

The rule for selecting the secret for manifest rendering:
- if `registry-bashible-config` exists — it is used;
- otherwise — `deckhouse-registry` is used (backward compatibility).

Configuration scripts on the node:

- applying the registry settings;
- starting the bashible-api-server;
- creating annotations on the node for feedback to the `registry` module:
  - the presence of custom scripts in containerd — used for the preflight check of whether the module
    can be started/switched;
  - the applied version of the `registry` module configuration.

Annotations on the nodes are a feedback channel: the orchestrator sees the version actually applied on
each node and can carry out the transition in stages, without rolling out the new configuration to all
nodes simultaneously.

```mermaid
flowchart TD
    bas["bashible-api-server"]

    bc{{"is there a<br/>registry-bashible-config?"}}
    bas --> bc
    bc -->|"yes"| use_bc[("registry-bashible-config<br/>(priority)")]
    bc -->|"no"| use_dr[("deckhouse-registry<br/>(backward compatibility)")]

    use_bc --> scripts["Script rendering"]
    use_dr --> scripts

    scripts --> containerd["containerd"]
    scripts --> ann["Annotations on the node<br/>(obtaining the node configuration state)"]

    ann -.->|"feedback"| orch["registry (orchestrator)"]

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
  subgraph GLOBAL["Module Global<br/>Template rendering"]
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

`global` reads the configuration from `deckhouse-registry` and renders the module manifests for all
DKP components. Further work with `deckhouse-registry` for API access to the registry
(operator-trivy, image-availability-exporter, etc.) is then performed independently by other
modules.

## Interaction of the registry module components:


### Direct

`containerd` accesses the external registry directly via the virtual address `registry.d8-system.svc:5001/system/deckhouse` thanks to the mirroring mechanism in containerd.

In-cluster access is performed through the non-caching proxy `registry-incluster-proxy`, available through the service `registry.d8-system.svc:5001`. At the proxy level the request is translated to the upstream registry.

```mermaid
flowchart TB
  ImageRegistry[("External registry")]

  subgraph DKP["Deckhouse Kubernetes Platform"]
    direction TB

    subgraph K8S["Kubernetes & Scheduling subsystem"]
      direction LR
      Containerd["containerd"]
    end

    subgraph InternalRegistryClients["Internal services accessing the registry API"]
      direction LR
      Deckhouse["deckhouse"]
      OperatorTrivy["operator-trivy"]
    end

    subgraph RegistryModule["registry module"]
      direction LR

      subgraph RegistryInclusterProxyDeploy["registry-incluster-proxy [Deployment, master nodes]"]
        direction TB
        Distribution["distribution"]
        Auth["auth"]
      end
    end
  end

  Containerd -->|"mirroring"| ImageRegistry

  Deckhouse -->|"registry.d8-system.svc:5001"| Distribution

  OperatorTrivy -->|"registry.d8-system.svc:5001"| Distribution

  Distribution -->|"5051"| Auth

  Distribution -->|"proxying"| ImageRegistry

  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  classDef client fill:#e6e6e6,stroke:#999,color:#333;
  class Distribution,Auth work;
  class Containerd cri;
  class Deckhouse,OperatorTrivy client;
  class ImageRegistry ext;
```

### Proxy

`containerd` accesses `127.0.0.1:5001` in the static pod `registry-proxy-<node>`, running on every node. `registry-proxy-<node>` balances requests across the `registry-nodeservices-<node>` components (static pods on master nodes, listening on `internal-ip:5001`), which operate in proxy mode and cache images from the upstream registry into local storage (`/opt/deckhouse/registry`).

In-cluster access is performed through the service `registry.d8-system.svc:5001` directly to the `registry-nodeservices-<node>` components.

```mermaid
flowchart TB
  ImageRegistry[("External registry")]

  subgraph DKP["Deckhouse Kubernetes Platform"]
    direction TB

    subgraph K8S["Kubernetes & Scheduling subsystem"]
      direction LR
      Containerd["containerd"]
    end

    subgraph InternalRegistryClients["Internal services accessing the registry API"]
      direction LR
      Deckhouse["deckhouse"]
      OperatorTrivy["operator-trivy"]
    end

    subgraph RegistryModule["registry module"]
      direction LR

      RegistryProxy["registry-proxy-&lt;node&gt;<br/>[static pod, all nodes]<br/>127.0.0.1:5001"]

      subgraph RegistryNodeservicesStaticPod["registry-nodeservices-&lt;master-node&gt; [static pod, caching]<br/>internal-ip:5001"]
        direction TB
        Distribution["distribution"]
        Auth["auth"]
      end
    end

    RegistryStorage[("/opt/deckhouse/registry")]
  end

  Containerd -->|"127.0.0.1:5001"| RegistryProxy

  RegistryProxy -->|"balancing, internal-ip:5001"| Distribution

  Deckhouse -->|"registry.d8-system.svc:5001"| Distribution

  OperatorTrivy -->|"registry.d8-system.svc:5001"| Distribution

  Distribution -->|"5051"| Auth

  Distribution -->|"image caching"| RegistryStorage

  Distribution -->|"caching proxy"| ImageRegistry

  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  classDef client fill:#e6e6e6,stroke:#999,color:#333;
  class RegistryProxy,Distribution,Auth work;
  class Containerd cri;
  class Deckhouse,OperatorTrivy client;
  class RegistryStorage ext;
  class ImageRegistry ext;
```

### Local

The network topology is identical to the `Proxy` mode: `containerd` accesses `127.0.0.1:5001` on the `registry-proxy-<node>` on each node, which balances requests across `registry-nodeservices-<node>` (static pods on master nodes, listening on `internal-ip:5001`). The difference is that `registry-nodeservices-<node>` operate in Local mode and serve images from local storage (`/opt/deckhouse/registry`) — there are no requests to the external registry.

Populating the local registry is performed via `ingress` (`registry.<PUBLIC_DOMAIN>`) using the `d8 mirror push` command.

```mermaid
flowchart TB
  MirrorPush["d8 mirror push"]

  subgraph DKP["Deckhouse Kubernetes Platform"]
    direction TB

    subgraph K8S["Kubernetes & Scheduling subsystem"]
      direction LR
      Containerd["containerd"]
    end

    subgraph InternalRegistryClients["Internal services accessing the registry API"]
      direction LR
      Deckhouse["deckhouse"]
      OperatorTrivy["operator-trivy"]
    end

    subgraph RegistryModule["registry module"]
      direction LR

      Ingress(["ingress<br/>registry.&lt;PUBLIC_DOMAIN&gt; :443<br/>→ registry-push :5001"])
      RegistryProxy["registry-proxy-&lt;node&gt;<br/>[static pod, all nodes]<br/>127.0.0.1:5001"]

      subgraph RegistryNodeservicesStaticPod["registry-nodeservices-&lt;master-node&gt; [static pod, local]<br/>internal-ip:5001"]
        direction TB
        Distribution["distribution"]
        Auth["auth"]
      end
    end

    RegistryStorage[("/opt/deckhouse/registry")]
  end

  Containerd -->|"127.0.0.1:5001"| RegistryProxy

  RegistryProxy -->|"balancing, internal-ip:5001"| Distribution

  Deckhouse -->|"registry.d8-system.svc:5001"| Distribution

  OperatorTrivy -->|"registry.d8-system.svc:5001"| Distribution

  Distribution -->|"5051"| Auth

  Distribution -->|"reading images"| RegistryStorage

  MirrorPush -.->|"registry.&lt;PUBLIC_DOMAIN&gt;:443/system/deckhouse"| Ingress
  Ingress -.->|"writing images, :5001"| Distribution

  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  classDef client fill:#e6e6e6,stroke:#999,color:#333;
  class RegistryProxy,Distribution,Auth,Ingress work;
  class Containerd cri;
  class Deckhouse,OperatorTrivy client;
  class RegistryStorage ext;
  class MirrorPush client;
```

### Unmanaged

The internal registry components are not used.
In-cluster access and `containerd` go directly to the external registry.

```mermaid
flowchart TB
  ImageRegistry[("External registry")]

  subgraph DKP["Deckhouse Kubernetes Platform"]
    direction TB

    subgraph K8S["Kubernetes & Scheduling subsystem"]
      direction LR
      Containerd["containerd"]
    end

    subgraph InternalRegistryClients["Internal services accessing the registry API"]
      direction LR
      Deckhouse["deckhouse"]
      OperatorTrivy["operator-trivy"]
    end
  end

  Containerd -->|"direct access"| ImageRegistry

  Deckhouse -->|"direct access"| ImageRegistry

  OperatorTrivy -->|"direct access"| ImageRegistry

  classDef work fill:#cdebc5,stroke:#4c9a3f,color:#16400d;
  classDef cri fill:#ffd9b3,stroke:#d97a2b,color:#5c2e00;
  classDef ext fill:#e2d4f7,stroke:#8b5cc4,color:#2e1052;
  classDef client fill:#e6e6e6,stroke:#999,color:#333;
  class Containerd cri;
  class Deckhouse,OperatorTrivy client;
  class ImageRegistry ext;
```
