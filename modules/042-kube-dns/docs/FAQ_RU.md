---
title: "Модуль kube-dns: FAQ"
search: DNS, domain, домен, clusterdomain
---

## Как поменять домен кластера с минимальным простоем?

Добавляем новый домен и сохраняем предыдущий:

1. В [controlPlaneManager.apiserver](../040-control-plane-manager/configuration.html) выполнить конфигурацию параметров:

- [controlPlaneManager.apiserver.certSANs](../040-control-plane-manager/configuration.html#parameters-apiserver-certsans)
- [apiserver.serviceAccount.additionalAPIAudiences](../040-control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiaudiences)
- [apiserver.serviceAccount.additionalAPIIssuers](../040-control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiissuers)

Пример:

```
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: control-plane-manager
spec:
  enabled: true
  settings:
    apiserver:
      certSANs:
       - kubernetes.default.svc.<старый clusterDomain>
       - kubernetes.default.svc.<новый clusterDomain>
      serviceAccount:
        additionalAPIAudiences:
        - https://kubernetes.default.svc.<старый clusterDomain>
        - https://kubernetes.default.svc.<новый clusterDomain>
        additionalAPIIssuers:
        - https://kubernetes.default.svc.<старый clusterDomain>
        - https://kubernetes.default.svc.<новый clusterDomain>
```


1. В [kubeDns.clusterDomainAliases](configuration.html#параметры) указать:
    - старый clusterDomain;
    - новый clusterDomain.
1. Дождаться переката kube-apiserver.
1. Поменять `clusterDomain` на новый в `dhctl config edit cluster-configuration`.

**Важно!** Если версия вашего Kubernetes 1.20 и выше, контроллеры для работы с apiserver гарантированно используют [расширенные токены для ServiceAccount'ов](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection). Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`). При смене `clusterDomain` apiserver начнет выдвать токены с новым `service-account-issuer`, но багодаря произведенной конфигурации `additionalAPIAudiences` и `additionalAPIIssuers`, по прежнему будет принимать старые токены. По истечению 48 минут (80% от 3607 секнуд) kubernetes начнет обновлять выпущенные токены, при обновлении будет использован новый `service-account-issuer`. Через 90 минут (3607 секунд и еще немного) после переката kube-apiserver вы можете удалить конфигурацию serviceAccount из конфигурации control-plane-manager.

**Важно!** Если вы используете модуль [istio](../../modules/110-istio/), после смены `clusterDomain` обязательно потребуется рестарт всех прикладных подов под управлением Istio.
