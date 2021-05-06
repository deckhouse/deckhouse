---
title: "Модуль kube-dns: FAQ"
search: DNS, domain, домен, clusterdomain
---

## Как «поменять» домен кластера без простоя?

Добавляем новый домен и сохраняем предыдущий:

1. В `controlPlaneManager.apiserver.certSANs` прописать
    - `kubernetes.default.svc.<старый clusterDomain>`
    - `kubernetes.default.svc.<новый clusterDomain>`
1. В [kubeDns.clusterDomainAliases](configuration.html#параметры) указать
    - старый clusterDomain.
    - новый clusterDomain.
1. Дождаться переката kube-apiserver.
1. Поменять clusterDomain на новый в `dhctl config edit cluster-configuration`
