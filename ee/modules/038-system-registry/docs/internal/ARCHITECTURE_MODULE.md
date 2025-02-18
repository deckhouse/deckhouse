---
title: "Module embedded-registry: module architecture"
description: ""
---

The module consists of the `embedded-registry-manager` daemonset, which includes:

- **Leader**: The manager, only one is elected.
- **Executer**: Runs on each pod of the daemonset.

During the manager's startup:
- It checks for the presence of secrets for `ro` and `rw` users. If they don't exist, it creates them.
- It checks for the `pki` secret with CA certificates for the internal registry. If it's not present, it creates it.
- It checks for `pki` secrets with certificates for each static pod. If they don't exist, it creates them.
- It subscribes to the above secrets and `moduleConfig`, monitoring their changes.

Depending on the configuration, it creates static pods on the master nodes.

```mermaid
graph TD;
    subgraph Cluster
        %% DaemonSetEmbRegistry[Daemonset: manager]
        %% DaemonSetEmbRegistry --> PodEmbRegistry1
        %% DaemonSetEmbRegistry --> PodEmbRegistry2
        %% DaemonSetEmbRegistry --> PodEmbRegistry3
        subgraph MasterNode1[Master node 1]
            subgraph PodEmbRegistry1[Pod: manager-1]
                PodEmbRegistryLeader[leader]
                PodEmbRegistryExecuter1[executer]
            end
            PodRegistry1[StaticPod: embedded-registry-1]
            PodEmbRegistryLeader --> PodEmbRegistryExecuter1
            PodEmbRegistryExecuter1 --> PodRegistry1
        end
        subgraph MasterNode2[Master node 2]
            subgraph PodEmbRegistry2[Pod: manager-2]
                PodEmbRegistryExecuter2[executer]
            end
            PodRegistry2[StaticPod: embedded-registry-2]
            PodEmbRegistryLeader --> PodEmbRegistryExecuter2
            PodEmbRegistryExecuter2 --> PodRegistry2
        end
        subgraph MasterNode3[Master node 3]
            subgraph PodEmbRegistry3[Pod: manager-3]
                PodEmbRegistryExecuter3[executer]
            end
            PodRegistry3[StaticPod: embedded-registry-3]
            PodEmbRegistryLeader --> PodEmbRegistryExecuter3
            PodEmbRegistryExecuter3 --> PodRegistry3
        end
    end
```

To access the registry from within the cluster, the service `embedded-registry.d8-system.svc` is used.

```mermaid
graph TD;
    subgraph Cluster
        ServiceEmbRegistry[Service: embedded-registry.d8-system.svc, port: 5001]
        ServiceEmbRegistry --> PodRegistry1
        ServiceEmbRegistry --> PodRegistry2
        ServiceEmbRegistry --> PodRegistry3
        subgraph MasterNode1[Master node 1]
            PodRegistry1[StaticPod: embedded-registry-1]
        end
        subgraph MasterNode2[Master node 2]
            PodRegistry2[StaticPod: embedded-registry-2]
        end
        subgraph MasterNode3[Master node 3]
            PodRegistry3[StaticPod: embedded-registry-3]
        end
    end
```
