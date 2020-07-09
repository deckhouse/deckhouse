---
title: Модуль control-plane-manager
---

## Принцип работы

С помощью DaemonSet `control-plane-manager` запускается на всех master-нодах кластера (лейбл `node-role.kubernetes.io/master: ""`) и:

1. **Управляет всеми сертификатами control-plane** (сертификаты размещаются на узлах согласно [принятым в kubeadm рекомендациям](https://kubernetes.io/docs/setup/best-practices/certificates/#certificate-paths)):
    1. Устанавливает все корневые сертификаты. Сертификаты хранятся в секрете `d8-pki` в `kube-system`.
        * корневой CA kubernetes (`ca.crt` и `ca.key`),
        * корневой CA etcd (`etcd/ca.crt` и `etcd/ca.key`),
        * RSA сертификат и ключ для подписи Service Account'ов (`sa.pub` и `sa.key`),
        * корневой CA для extension API серверов (`front-proxy-ca.key` и `front-proxy-ca.crt`).
    2. Управляет всеми сертификатами узла, необходимыми для control-plane – выписывает, продлевает и перевыписывает, если что-то изменилось (например, список SAN'ов). Сертификаты хранятся только на узлах.
        * серверный сертификат apiserver (`apiserver.crt` и `apiserver.key`),
        * клиентский сертификат для подключения apiserver к kubelet (`apiserver-kubelet-client.crt` и `apiserver-kubelet-client.key`),
        * клиентский сертификат для подключения apiserver к etcd (`apiserver-etcd-client.crt` и `apiserver-etcd-client.key`),
        * клиентский сертификат для подключения apiserver к extension API-серверам (`front-proxy-client.crt` и `front-proxy-client.key`),
        * серверный сертификат etcd (`etcd/server.crt` и `etcd/server.key`),
        * клиентский сертификат для подключения etcd к другим членам кластера (`etcd/peer.crt` и `etcd/peer.key`),
        * клиентский сертификат для подключения kubelet к etcd для helthcheck'ов (`etcd/healthcheck-client.crt` и `etcd/healthcheck-client.key`).
2. **Управляет etcd**:
    * На узлах:
        * Генерирует статический манифест etcd (`/etc/kubernetes/manifests/etcd.yaml`) со всеми необходимыми параметрами (с учетом состояния кластера и настроек модуля).
          * В том числе, всегда указывает `--initial-cluster-state=existing`, чтобы ни при каких обстоятельствах не допустить split-brain.
        * Если отсутствует директория `/var/lib/etcd`, автоматически выполняет join текущего узла в кластер.
        * При изменении любых сертификатов или любых параметров перезапускает etcd, дожидаясь успешного запуска с новыми настройками.
    * Централизовано:
        * Автоматически удаляет членов кластера etcd, для которых не существует одноименный узел (объект Node) в кластере Kubernetes (и которые не указаны в параметре `etcd.externalMembersNames`, см. подробнее ниже).
        * Выполняет upgrade или downgrade согласно [политикам](#политики-обновления-control-plane-компонентов).
3. **Управляет компонентами control-plane** (`kube-apiserver`, `kube-controller-manager`, `kube-scheduler`):
    * На узлах:
        * Генерирует (а также продлевает и обновляет) kubeconfig'и для подключения компонентов к apiserver.
        * Генерирует статические манифесты со всеми необходимыми параметрами (с учетом состояния кластера и настроек модуля).
        * При изменении любых сертификатов, любых конфигов или любых параметров в манифесте перезапускает компонент, дожидаясь успешного запуска с новыми настройками.
    * Централизовано:
        * Выполняет upgrade или downgrade согласно [политикам](#политики-обновления-control-plane-компонентов).
4. **Управляет kubeconfig'ом для cluster-admin** на узлах:
    * Генерирует (а так же продлевает и обновляет) kubeconfig с правами cluster-admin'а (размещает в /etc/kubernetes/admin.yaml).
    * Устанавливает symlink пользователю root, чтобы kubeconfig использовался по-умолчанию.

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
  * `authn` — опциональные параметры аутентификации клиентов Kubernetes API. По-умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authn`]({{ site.baseurl }}/modules/150-user-authn)
    * `oidcIssuerURL` — строка, адрес OIDC провайдера.
    * `oidcCA` — строка, CA OIDC провайдера.
  * `authz` — параметры авторизации клиентов Kubernetes API. По-умолчанию подтягиваются из ConfigMap, устанавливаемого модулем [`user-authz`]({{ site.baseurl }}/modules/140-user-authz)
    * `webhookURL` — строка, authorization webhook'а.
    * `webhookCA` — строка, CA authorization webhook'a.
  * `loadBalancer` — если указано, будет создан сервис с типом `LoadBalancer` (d8-control-plane-apiserver в ns kube-system):
    * `annotations` — аннотации, которые будут проставлены сервису для гибкой настройки балансировщика.
      * **Внимание!** модуль не учитывает особенности указания аннотаций в различных облаках. Если аннотации для заказа load balancer'а применяются только при создании сервиса, то для обновления подобных параметров вам необходимо будет удалить и добавить параметр `apiserver.loadBalancer`.
    * `sourceRanges` — список CIDR, которым разрешен доступ к API.
      * Облачный провайдер может не поддерживать данную опцию и игнорировать её.
* `etcd` — параметры `etcd`.
  * `externalMembersNames` – массив имен внешних etcd member'ов (эти member'ы не будут удалятся).


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

## FAQ

### Как добавить мастер?

Просто поставить на ноду лейбл `node-role.kubernetes.io/master: ""`, все остальное произойдет полностью автоматически.

### Как удалить мастер?

* Если удаление не нарушет кворум в etcd (в корректно функционирущем кластере это все ситуации, кроме перехода 2 -> 1):
    1. Удалить виртуальную машину обычным способом.
* Если удаление нарушает кворум (переход 2 -> 1):
    1. Остановить kubelet на узле (не останавливая контейнер с etcd),
    2. Удалить объект Node из Kubernetes
    3. [Дождаться](#как-посмотреть-список-memberов-в-etcd), пока etcd member будет автоматически удален.
    4. Удалить виртуальную машину обычным способом.

### Как убрать мастер, сохранив узел?

1. Снять лейбл `node-role.kubernetes.io/master: ""` и дождаться, пока etcd member будет удален автоматически.
2. Зайти на узел и выполнить следующие действия:
  ```shell
  rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
  rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
  rm -f /etc/kubernetes/authorization-webhook-config.yaml
  rm -f /etc/kubernetes/admin.conf /root/.kube/config
  rm -rf /etc/kubernetes/deckhouse
  rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
  rm -rf /var/lib/etcd
  ```

### Как посмотреть список member'ов в etcd?

1. Зайти в pod с etcd.
  ```shell
  kubectl -n kube-system exec -ti $(kubectl -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) sh
  ```
2. Выполнить команду.
  ```shell
  etcdctl --ca-file /etc/kubernetes/pki/etcd/ca.crt --cert-file /etc/kubernetes/pki/etcd/ca.crt --key-file /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list
  ```

### Что делать, если что-то пошло не так?

В процессе работы control-plane-manager оставляет резервные копии в `/etc/kubernetes/deckhouse/backup`, они могут помочь.

### Что делать, если кластер etcd развалился?

1. Остановить (удалить `/etc/kubernetes/manifests/etcd.yaml`) etcd на всех нодах, кроме одной. С неё мы начнём восстановление multi-master'а.
2. На оставшейся ноде указать следующий параметр командной строки: `--force-new-cluster`.
3. После успешного подъёма кластера, удалить параметр `--force-new-cluster`.

**Внимание!** Операция деструктивна, полностью уничтожает консенсус и запускает etcd кластер с состояния, которое сохранилось на ноде. Любые pending записи пропадут.
