---
title: "Модуль kube-dns: FAQ"
type:

- instruction search: DNS, domain, домен, clusterdomain

---

## Как «поменять» домен кластера без простоя?

Добавляем новый домен и сохраняем предыдущий:

1. В `controlPlaneManager.apiserver.certSANs` прописать
    - `kubernetes.default.svc.<старый clusterDomain>`
    - `kubernetes.default.svc.<новый clusterDomain>`
1. В [kubeDns.clusterDomainAliases](./index.html#clusterDomainAliases) указать
    - старый clusterDomain.
    - новый clusterDomain.
1. Дождаться переката kube-apiserver.
1. Поменять clusterDomain на новый в `candictl config edit cluster-configuration`
