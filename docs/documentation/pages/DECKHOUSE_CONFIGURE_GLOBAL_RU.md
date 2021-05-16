---
title: "Глобальная конфигурация"
permalink: ru/deckhouse-configure-global.html
lang: ru
---

## Что нужно настроить?

Нужно обязательно настроить `project` и `clusterName`. Также, желательно настроить `modules.publicDomainTemplate`.

```yaml
global: |
  project: projectname
  clusterName: main
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

## Параметры

* `project` (обязательно) — имя проекта.
* `clusterName` (обязательно) — имя кластера.
* `modules` — параметры для служебных компонентов;
  * `publicDomainTemplate` (желательно) — шаблон c ключом `%s` в качестве динамической части строки. Шаблон будет использоваться при образовании служебных DNS-записей, необходимых для внутренних нужд Deckhouse и работы модулей. **Нельзя** использовать в кластере (создавать Ingress-ресурсы) DNS-имена подпадающие под указанный шаблон, во избежание пересечений с создаваемыми Deckouse Ingress-ресурсами. Пример шаблона - `%s.kube.company.my`. Если параметр не указан, то Ingress-ресурсы создаваться не будут.
  * `ingressClass` — класс Ingress-контроллера, который используется для служебных компонентов.
    * По умолчанию `nginx`.
  * `placement` — настройки, определяющие расположение компонентов Deckhouse.
    * `customTolerationKeys` — список ключей пользовательских taint'ов, необходимо указывать, чтобы позволить выезжать на выделенные ноды критическим add-on'ам, таким как например cni и csi.
      * Пример:

        ```yaml
        customTolerationKeys:
        - dedicated.example.com
        - node-dedicated.example.com/master
        ```
  * `https` — способ реализации HTTPS, используемый служебными компонентами.
    * `mode` — режим работы HTTPS:
      * `Disabled` — в данном режиме все служебные компоненты будут работать только по http (некоторые модули могут не работать, например [user-authn]({{"/modules/150-user-authn/" | true_relative_url }} ));
      * `CertManager` — все служебные компоненты будут работать по https и заказывать сертификат с помощью clusterissuer заданном в параметре `certManager.clusterIssuerName`;
      * `CustomCertificate` — все служебные компоненты будут работать по https используя сертификат из namespace `d8-system`;
      * `OnlyInURI` — все служебные компоненты будут работать по http (подразумевая, что перед ними стоит внешний https-балансер, который терминирует https).
      * По умолчанию `CertManager`.
    * `certManager`
      * `clusterIssuerName` — указываем, какой ClusterIssuer использовать для служебных компонентов (в данный момент доступны `letsencrypt`, `letsencrypt-staging`, `selfsigned`, но вы можете определить свои).
        * По умолчанию `letsencrypt`.
    * `customCertificate`
      * `secretName` - указываем имя secret'а в namespace `d8-system`, который будет использоваться для системных компонентов (данный секрет должен быть в формате [kubernetes.io/tls](https://kubernetes.github.io/ingress-nginx/user-guide/tls/#tls-secrets).
        * По умолчанию `false`.
  * `resourcesRequests` - количество CPU и memory, выделяемые для работы служебных компонентов.
    * `everyNode` - системные компоненты, работающие на каждом узле кластера (обычно это DaemonSet'ы).
      * `cpu` – сумма CPU, выделяемая для всех компонентов на каждом узле (по умолчанию: 300m)
      * `memory` – суммарный объем памяти, выделяемый для всех компонентов на каждом узле (по умолчанию: 512Mi)
    * `masterNode` - системные компоненты (control plane и системные компоненты на мастер-узлах).
      * `cpu` – сумма CPU, выделяемая для системных компонентов на мастер-узлах сверх `everyNode`.
        * Для кластера, управляемого Deckhouse, значение по умолчанию определяется автоматически: `.status.allocatable.cpu` минимального мастер-узла (но не более 4 ядер) минус `everyNode`.
        * Для managed-кластера значение по умолчанию - 1 ядро минус `everyNode`.
      * `memory` – суммарный объем памяти, выделяемый для системных компонентов на мастер-узлах сверх `everyNode`.
        * Для кластера, управляемого Deckhouse, значение по умолчанию определяется автоматически: `.status.allocatable.memory` минимального мастер-узла (но не более 8 Гб) минус `everyNode`.
        * Для managed-кластера значение по умолчанию - 1 Гб минус `everyNode`.
      * **Внимание!** В случае managed-кластера Deckhouse не управляет control-plane компонентами, поэтому все ресурсы отдаются системным компонентам.
* `storageClass` — имя storage class, который будет использоваться для всех служебных компонентов (prometheus, grafana, openvpn, ...).
    * По умолчанию — null, а значит служебные будут использовать `cluster.defaultStorageClass` (который определяется автоматически), а если такого нет — `emptyDir`.
    * Этот параметр имеет смысл использовать только в исключительных ситуациях.
* `highAvailability` — глобальный включатель режима отказоустойчивости для модулей, которые это поддерживают. По умолчанию не определён и решение принимается на основе autodiscovery-параметра `global.discovery.clusterControlPlaneIsHighlyAvailable`.

