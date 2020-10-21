---
title: "Управление control plane: настройки"
---

Управление компонентами control plane кластера осуществляется с помощью модуля `control-plane-manager`, а параметры кластера, влияющие на управление control plane, берутся из Custom Resource `ClusterConfiguration` (создается при инсталляции).

Модуль по умолчанию **включен**. Выключить можно стандартным способом:

```yaml
controlPlaneManagerEnabled: "false"
```

## Параметры

* `apiserver` — параметры `kube-apiserver`.
  * `bindToWildcard` — bool, слушать ли на `0.0.0.0`.
    * По умолчанию, `false`.
    * По умолчанию apiserver слушает на hostIP, который обычно соответствует Internal адресу узла, но это зависит от типа кластера (Static или Cloud) и выбранной схемы размещения (layout'а).
  * `certSANs` — массив строк, список дополнительных [SANs](https://en.wikipedia.org/wiki/Subject_Alternative_Name), с которыми будет сгенерирован сертификат apiserver'а.
    * Кроме переданного списка, всегда используется и следующий список:
      * kubernetes
      * kubernetes.default
      * kubernetes.default.svc
      * kubernetes.default.svc.cluster.local
      * 192.168.0.1
      * 127.0.0.1
      * **current_hostname**
      * **hostIP**
  * `authn` — опциональные параметры аутентификации клиентов Kubernetes API. По умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authn`](/modules/150-user-authn)
    * `oidcIssuerURL` — строка, адрес OIDC провайдера.
    * `oidcCA` — строка, CA OIDC провайдера.
  * `authz` — параметры авторизации клиентов Kubernetes API. По умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authz`](/modules/140-user-authz)
    * `webhookURL` — строка, authorization webhook'а.
    * `webhookCA` — строка, CA authorization webhook'a.
  * `loadBalancer` — если указано, будет создан сервис с типом `LoadBalancer` (d8-control-plane-apiserver в ns kube-system):
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
      * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет удалить и добавить параметр `apiserver.loadBalancer`.
    * `sourceRanges` — список CIDR, которым разрешен доступ к API.
      * Облачный провайдер может не поддерживать данную опцию и игнорировать её.
  * `auditPolicyEnabled` — bool, включение [аудита событий](faq.html#как-включить-аудит-событий) с конфигурацией из `Secret` (audit-policy в ns kube-system)
* `etcd` — параметры `etcd`.
  * `externalMembersNames` – массив имен внешних etcd member'ов (эти member'ы не будут удалятся).
