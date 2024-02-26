---
title: "Модуль kube-dns: FAQ"
search: DNS, domain, домен, clusterdomain
---

## Как поменять домен кластера с минимальным простоем?

Добавьте новый домен и сохраните предыдущий:

1. В [controlPlaneManager.apiserver.certSANs](../040-control-plane-manager/configuration.html#parameters-apiserver-certsans) пропишите:
    - `kubernetes.default.svc.<старый clusterDomain>`;
    - `kubernetes.default.svc.<новый clusterDomain>`.
1. В [kubeDns.clusterDomainAliases](configuration.html#параметры) укажите:
    - старый clusterDomain;
    - новый clusterDomain.
1. Дождитесь переката kube-apiserver.
1. Замените `clusterDomain` на новый в `dhctl config edit cluster-configuration`.

**Важно!** Если версия вашего Kubernetes 1.20 и выше, контроллеры для работы с apiserver гарантированно используют [расширенные токены для ServiceAccount'ов](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection). Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`). При смене `clusterDomain` apiserver перестанет принимать запросы со старыми токенами, что приведет к ошибкам в работе всех контроллеров, включая deckhouse. Решение — дождаться ротации токенов, которая произойдет раньше, чем expiration time токена, либо рестартнуть все поды со всеми контроллерами.

**Важно!** Если используется модуль [istio](../../modules/110-istio/), после смены `clusterDomain` обязательно потребуется рестарт всех прикладных подов под управлением Istio.
