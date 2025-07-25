---
title: "Модуль kube-dns: FAQ"
search: DNS, domain, домен, clusterdomain
---

## Как поменять домен кластера с минимальным простоем?

Добавьте новый домен и сохраните предыдущий. Для этого измените конфигурацию параметров:

1. В [controlPlaneManager.apiserver](../control-plane-manager/configuration.html):

   - [controlPlaneManager.apiserver.certSANs](../control-plane-manager/configuration.html#parameters-apiserver-certsans),
   - [apiserver.serviceAccount.additionalAPIAudiences](../control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiaudiences),
   - [apiserver.serviceAccount.additionalAPIIssuers](../control-plane-manager/configuration.html#parameters-apiserver-serviceaccount-additionalapiissuers).

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: control-plane-manager
   spec:
     version: 1
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

1. В [kubeDns.clusterDomainAliases](configuration.html#параметры):

   Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: kube-dns
   spec:
     version: 1
     enabled: true
     settings:
       clusterDomainAliases:
         - <старый clusterDomain>
         - <новый clusterDomain>
   ```

1. Дождитесь перезапуска `kube-apiserver`.
1. Поменяйте `clusterDomain` на новый. Для этого выполните команду:

   ```bash
   d8 platform edit cluster-configuration
   ```

**Важно!** Если версия вашего Kubernetes 1.20 и выше, контроллеры для работы с API-server гарантированно используют [расширенные токены для ServiceAccount'ов](https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/#service-account-token-volume-projection). Это означает, что каждый такой токен содержит дополнительные поля `iss:` и `aud:`, которые включают в себя старый `clusterDomain` (например, `"iss": "https://kubernetes.default.svc.cluster.local"`).
При смене `clusterDomain` API-server начнет выдавать токены с новым `service-account-issuer`, но благодаря произведенной конфигурации `additionalAPIAudiences` и `additionalAPIIssuers` по-прежнему будет принимать старые токены. По истечении 48 минут (80% от 3607 секунд) Kubernetes начнет обновлять выпущенные токены, при обновлении будет использован новый `service-account-issuer`. Через 90 минут (3607 секунд и немного больше) после перезагрузки kube-apiserver можете удалить конфигурацию `serviceAccount` из конфигурации `control-plane-manager`.

**Важно!** Если вы используете модуль [istio](../../modules/istio/), после смены `clusterDomain` обязательно потребуется рестарт всех прикладных подов под управлением Istio.

## Как увеличить количество подов kube-dns?

Deckhouse распределяет поды kube-dns по следующему принципу: выполняется поиск узлов с метками node-role.deckhouse.io/ и node-role.kubernetes.io/, затем применяются следующие правила:

* Если в кластере есть узлы с ролью kube-dns, количество реплик вычисляется как сумма таких узлов и мастеров.
* Если узлы kube-dns отсутствуют, производится поиск узлов с ролью system, и тогда количество реплик определяется как сумма system-узлов и мастеров.
* Если в кластере присутствуют только мастер-узлы, количество реплик kube-dns будет равно числу мастеров.
