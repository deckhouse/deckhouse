# Managed

Исходник схемы архитектуры текущей реализации. Рендерится в
`docs/images/managed-{en,ru}.png` так же, как лежащие рядом схемы режимов.

## Архитектура, кэш выключен

На каждом узле работает агент. Container runtime направляется на него один раз — на
`127.0.0.1:5001`, через fallback `_default` в containerd — и спрашивает его о любом
registry, передавая рядом исходное имя registry. Куда это уйдёт, агент решает на каждый
запрос.

Именно это делает конфигурацию узла статичной: включение кэша, добавление upstream, смена
учётных данных — ничто из этого не перезаписывает на узле ничего.

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

upstream[("**registry.deckhouse.ru**")]

agent1 ==> upstream
agent2 ==> upstream
controller ==> upstream
operator ==> upstream
exporter ==> upstream
```

## Архитектура, кэш включён

Кэш работает на control-plane узлах и держит свои blob'ы на хосте. Агент обращается к
реплике по адресу узла, а не по имени сервиса: он работает в сетевом пространстве хоста, где
имя cluster DNS не разрешается — `registry.d8-system.svc:5001` идентифицирует набор образов
и из него строятся ссылки на образы, но никто по этому имени не дозванивается.

Upstream остаётся в конфигурации каждого узла как fallback, пока кэш наполняется, поэтому
промах кэша — это более медленная загрузка, а не неудачная.

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

upstream[("**registry.deckhouse.ru**")]

agent1 == "cache first" ==> storage1
agent1 -. "fallback while filling" .-> upstream
syncer1 ==> upstream
storage2 == "replicates from the leader" ==> storage1
```

## Архитектура, изолированный кластер

Если upstream не задан, кэш — единственный источник образов, а `d8 mirror push` —
единственный путь внутрь. Он приходит через endpoint публикации, который помимо учётных
данных требует клиентский сертификат от ingress: это единственный путь, по которому образ
можно заменить, и утёкшего пароля для него быть недостаточно.

Upstream убирается из конфигураций узлов только после того, как лидер кэша держит весь
ожидаемый набор — это единственный условный переход в этом дизайне и единственный, который
мог бы отрезать все узлы от образов.

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
