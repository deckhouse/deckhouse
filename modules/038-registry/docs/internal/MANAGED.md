# Managed

Source for the architecture diagram of the current implementation. Rendered to
`docs/images/managed-{en,ru}.png` the same way the mode diagrams beside it are.

## Architecture, with the cache off

Every node runs an agent. The container runtime is pointed at it once — at
`127.0.0.1:5001`, through containerd's `_default` fallback — and asks it about every
registry, passing the original registry name along. The agent decides per request where
that goes.

This is what makes the node configuration static: turning the cache on, adding an upstream,
changing credentials — none of it rewrites anything on a node.

```mermaid
---
title: Managed, cache off
---
flowchart TD
subgraph Cluster["Deckhouse Kubernetes Cluster"]
subgraph Node1["Node 1"]
kubelet1[Kubelet]
containerd1[Containerd]
agent1["registry-agent **(127.0.0.1:5001)**"]
kubelet1 ==> containerd1
containerd1 == "every registry, via _default" ==> agent1
end
subgraph Node2["Node 2"]
kubelet2[Kubelet]
containerd2[Containerd]
agent2["registry-agent **(127.0.0.1:5001)**"]
kubelet2 ==> containerd2
containerd2 ==> agent2
end
subgraph InCluster["In-cluster Components"]
controller[deckhouse-controller]
operator[operator-trivy]
exporter[image-availability-exporter]
end
end

upstream[("**registry.deckhouse.io**")]

agent1 ==> upstream
agent2 ==> upstream
controller ==> upstream
operator ==> upstream
exporter ==> upstream
```

## Architecture, with the cache on

The cache runs on the control-plane nodes and keeps its blobs on the host. An agent reaches
a replica by node address, not by the service name: it runs in the host network, where a
cluster DNS name does not resolve — `registry.d8-system.svc:5001` identifies the image set
and is what image references are built from, but nothing dials it.

The upstream stays in each node's layout as a fallback while the cache is filling, so a
cache miss is a slower pull rather than a failed one.

```mermaid
---
title: Managed, cache on
---
flowchart TD
subgraph Cluster["Deckhouse Kubernetes Cluster"]
subgraph Node1["Worker node"]
kubelet1[Kubelet]
containerd1[Containerd]
agent1["registry-agent"]
kubelet1 ==> containerd1
containerd1 ==> agent1
end
subgraph Master1["Master 1"]
storage1["registry-storage **(leader)**"]
syncer1[syncer]
data1[("/opt/deckhouse/registry")]
syncer1 -. fills .-> storage1
storage1 --- data1
end
subgraph Master2["Master 2"]
storage2["registry-storage **(follower)**"]
data2[("/opt/deckhouse/registry")]
storage2 --- data2
end
end

upstream[("**registry.deckhouse.io**")]

agent1 == "cache first" ==> storage1
agent1 -. "fallback while filling" .-> upstream
syncer1 ==> upstream
storage2 == "replicates from the leader" ==> storage1
```

## Architecture, air-gapped

With no upstream configured the cache is the only source of images, and `d8 mirror push`
is the only way in. It arrives through the publication endpoint, which requires a client
certificate from the ingress on top of credentials: this is the one path that can replace
an image, so a leaked password must not be enough to use it.

The upstream is removed from the node layouts only once the cache leader holds the whole
expected set — that is the single conditional transition in this design, and the only one
that could cut every node off from images.

```mermaid
---
title: Managed, air-gapped
---
flowchart TD
operator(["d8 mirror push"])
ingress["Ingress **(registry-push)**"]

subgraph Cluster["Deckhouse Kubernetes Cluster"]
subgraph Master1["Master 1"]
storage1["registry-storage **(leader)**"]
data1[("/opt/deckhouse/registry")]
storage1 --- data1
end
subgraph Master2["Master 2"]
storage2["registry-storage **(follower)**"]
data2[("/opt/deckhouse/registry")]
storage2 --- data2
end
subgraph Node1["Worker node"]
containerd1[Containerd]
agent1["registry-agent"]
containerd1 ==> agent1
end
end

operator ==> ingress
ingress == "client certificate + write credentials" ==> storage1
storage2 == replicates ==> storage1
agent1 ==> storage1
agent1 ==> storage2
```
