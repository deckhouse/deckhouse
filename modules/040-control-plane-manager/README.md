# Модуль control-plane-manager

## Принцип работы

С помощью DaemonSet `control-plane-manager` запускается на всех master нодах (лейбл `node-role.kubernetes.io/master: ""`) кластера.

1. Обновляет сертификаты всех control plane компонентов.
2. Добавляет и удаляет новых etcd member'ов в etcd кластере.
3. Согласно [политикам](#Политики-обновления-control-plane-компонентов) делает upgrade или downgrade компонентов Kubernetes control plane.

## Политики обновления control plane компонентов

* **Patch версии** (1.x.**x**). Для каждой minor версии (1.**x**.x) в модуле явно указана patch версия Kubernetes, эти версии могут меняться при релизе Deckhouse.
* **Minor версии** (1.**x**.x). Модуль обновляется по одной minor версии вверх или вниз, когда соблюдаются следующие политики для каждой итерации обновления.
    * Upgrade:
        1. Все kubelet'ы на нодах кластера должны быть обновлены на текущую minor версию Kubernetes.
    * Downgrade
        1. Новая версия может быть только на одну меньше, чем *максимальная версия control plane компонентов, когда либо использовавшаяся в данном кластере*.
            * Например, `maxUsedControlPlaneVersion = 1.16`. Минимально возможная версия control plane компонентов в кластере — `1.15`.
        2. Все kubelet'ы на нодах кластера должны быть предварительно downgrade'нуты.

## Конфигурация

Все глобальные параметры кластера модуль берёт из `ClusterConfiguration`, который создается при инсталляции.

### Включение модуля

Модуль по-умолчанию **включен**. Выключить можно стандартным способом:

```yaml
controlPlaneManager: "false"
```

### Параметры модуля

* `apiserver` — параметры `kube-apiserver`.
  * `bindToWildcard` — bool, слушать ли на `0.0.0.0`.
    * По-умолчанию, `false`.
    * По-умолчанию apiserver слушает на hostIP, который обычно соответствует Internal адресу узла, но это зависит от типа кластера (Static или Cloud) и выбранной схемы размещения (layout'а).
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
  * `authn` — опциональные параметры аутентификации клиентов Kubernetes API. По-умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authn`](modules/150-user-authn)
    * `oidcIssuerURL` — строка, адрес OIDC провайдера.
    * `oidcCA` — строка, CA OIDC провайдера.
  * `authz` — параметры авторизации клиентов Kubernetes API. По-умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authz`](modules/140-user-authz)
    * `webhookURL` — строка, authorization webhook'а.
    * `webhookCA` — строка, CA authorization webhook'a.
  * `loadBalancer` — если указано, будет создан сервис с типом `LoadBalancer` (d8-control-plane-apiserver в ns kube-system):
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
      * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет удалить и добавить параметр `apiserver.loadBalancer`.
    * `sourceRanges` — список CIDR, которым разрешен доступ к API.
      * Облачный провайдер может не поддерживать данную опцию и игнорировать её.

#### Пример конфигурации модуля

```yaml
controlPlaneManagerEnabled: "true"
controlPlaneManager: |
  bindToWildcard: true
  certSANs:
  - bakery.infra
  - devs.infra
  loadBalancer: {}
```

## Что делать, если что-то пошло не так?

1. В процессе работы control-plane-manager оставляет резервные копии в `/etc/kubernetes/deckhouse/backup`, они могут помочь.
2. Удачи.

### Радикальное восстановление etcd кластера

1. Остановить (удалить `/etc/kubernetes/manifests/etcd.yaml`) etcd на всех нодах, кроме одной. С неё мы начнём восстановление multi-master'а.
2. На оставшейся ноде указать следующий параметр командной строки: `--force-new-cluster`.
3. После успешного подъёма кластера, удалить параметр `--force-new-cluster`.

**Внимание!** Операция деструктивна, полностью уничтожает консенсус и запускает etcd кластер с состояния, которое сохранилось на ноде. Любые pending записи пропадут.
