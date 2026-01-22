---
title: "Module registry: module architecture"
description: ""
---

<!--
The module consists of the `registry-manager` daemonset, which includes:

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
        %% DaemonSetRegistry[Daemonset: manager]
        %% DaemonSetRegistry ==> PodRegistry1
        %% DaemonSetRegistry ==> PodRegistry2
        %% DaemonSetRegistry ==> PodRegistry3
        subgraph MasterNode1[Master node 1]
            subgraph PodRegistry1[Pod: manager-1]
                PodRegistryLeader[leader]
                PodRegistryExecuter1[executer]
            end
            PodRegistry1[StaticPod: registry-1]
            PodRegistryLeader ==> PodRegistryExecuter1
            PodRegistryExecuter1 ==> PodRegistry1
        end
        subgraph MasterNode2[Master node 2]
            subgraph PodRegistry2[Pod: manager-2]
                PodRegistryExecuter2[executer]
            end
            PodRegistry2[StaticPod: registry-2]
            PodRegistryLeader ==> PodRegistryExecuter2
            PodRegistryExecuter2 ==> PodRegistry2
        end
        subgraph MasterNode3[Master node 3]
            subgraph PodRegistry3[Pod: manager-3]
                PodRegistryExecuter3[executer]
            end
            PodRegistry3[StaticPod: registry-3]
            PodRegistryLeader ==> PodRegistryExecuter3
            PodRegistryExecuter3 ==> PodRegistry3
        end
    end

```

To access the registry from within the cluster, the service `registry.d8-system.svc` is used.

```mermaid
graph TD;
    subgraph Cluster
        ServiceRegistry[Service: registry.d8-system.svc, port: 5001]
        ServiceRegistry ==> PodRegistry1
        ServiceRegistry ==> PodRegistry2
        ServiceRegistry ==> PodRegistry3
        subgraph MasterNode1[Master node 1]
            PodRegistry1[StaticPod: registry-1]
        end
        subgraph MasterNode2[Master node 2]
            PodRegistry2[StaticPod: registry-2]
        end
        subgraph MasterNode3[Master node 3]
            PodRegistry3[StaticPod: registry-3]
        end
    end
```
-->
