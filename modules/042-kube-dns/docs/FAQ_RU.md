---
title: "Модуль kube-dns: FAQ"
search: DNS, domain, домен, clusterdomain
---

## Как поменять домен кластера с минимальным простоем?

Добавляем новый домен и сохраняем предыдущий:

1. В [controlPlaneManager.apiserver.certSANs](../040-control-plane-manager/configuration.html#parameters-apiserver-certsans) прописать
    - `kubernetes.default.svc.<старый clusterDomain>`
    - `kubernetes.default.svc.<новый clusterDomain>`
1. В [kubeDns.clusterDomainAliases](configuration.html#параметры) указать
    - старый clusterDomain.
    - новый clusterDomain.
1. Дождаться переката kube-apiserver.
1. Поменять `clusterDomain` на новый в `dhctl config edit cluster-configuration`

**Важно!** Если версия вашего Kubernetes 1.20 и выше, то контроллеры для работы с apiserver гарантированно используют [расширенные токены для ServiceAccount-ов](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection). Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`). При смене `clusterDomain`, apiserver перестанет принимать запросы со старыми токенами, что приведёт к ошибкам в работе всех контроллеров включая deckhouse. Решение — дождаться ротации токенов, которая произойдёт раньше, чем expiration time токена, либо рестартнуть все поды со всеми контроллерами.

**Важно!** Если вы используете модуль [istio](../../modules/110-istio/), то после смены `clusterDomain` обязательно потребуется рестарт всех прикладных Pod'ов под управлением Istio.
