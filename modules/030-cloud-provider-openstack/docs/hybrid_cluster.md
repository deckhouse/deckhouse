---
title: "Hybrid кластер в OpenStack" 
---

## Требования
Hybrid кластер представляет собой объединённые в один кластер bare metal ноды и ноды openstack. Для создания такого кластера
необходимо наличие L2 сети между всеми нодами кластера.

## Параметры конфигурации

> **Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `OpenStackInstanceClass`. См. подробнее в документации модуля [node-manager]({{ site.baseurl }}/modules/040-node-manager/#как-мне-перекатить-машины-с-новой-конфигурацией).
Для настройки аутентификации с помощью модуля `user-authn` необходимо в Crowd'е проекта создать новое `Generic` приложение.

* `connection` - Параметры подключения к api cloud provider'a
    * `authURL` — OpenStack Identity API URL.
    * `caCert` — если OpenStack API имеет self-signed сертификат, можно указать CA x509 сертификат, использовавшийся для подписи.
        * Формат — строка. Сертификат в PEM формате.
        * Опциональный параметр.
    * `domainName` — имя домена.
    * `tenantName` — имя проекта.
        * Не может использоваться вместе с `tenantID`.
    * `tenantID` — id проекта.
        * Не может использоваться вместе с `tenantName`.
    * `username` — имя пользователя с полными правами на проект.
    * `password` — пароль к пользователю.
    * `region` — регион OpenStack, где будет развёрнут кластер.
* `internalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,

        ```yaml
        internalNetworkNames:
        - KUBE-3
        - devops-internal
        ```
* `externalNetworkNames` — имена сетей, подключённые к виртуальной машине, и используемые cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
    * Формат — массив строк. Например,

        ```yaml
        externalNetworkNames:
        - KUBE-3
        - devops-internal
        ```
* `podNetworkMode` - определяет способ организации трафика в той сети, которая используется для коммуникации между подами (обычно это internal сеть, но бывают исключения).
    * Допустимые значение:
      * `DirectRouting` – между узлами работает прямая маршрутизация.
      * `DirectRoutingWithPortSecurityEnabled` - между узлами работает прямая маршрутизация, но только если в OpenStack явно разрешить на Port'ах диапазон адресов используемых во внутренней сети.
          * **Внимание!** Убедитесь, что у `username` есть доступ на редактирование AllowedAddressPairs на Port'ах, подключенных в сеть `internalNetworkName`. Обычно, в OpenStack, такого доступа нет, если сеть имеет флаг `shared`.
      * `VXLAN` – между узлами НЕ работает прямая маршрутизация, необходимо использовать VXLAN.
    * Опциональный параметр. По-умолчанию `DirectRoutingWithPortSecurityEnabled`.
* `instances` — параметры instances, которые используются при создании:
    * `sshKeyPairName` — имя OpenStack ресурса `keypair`, который будет использоваться при заказе instances.
    * `securityGroups` — Список securityGroups, которые нужно прикрепить к заказанным instances. Используется для задания firewall правил по отношению к заказываемым instances.
        * Формат — массив строк.
* `loadBalancer` - параметры Load Balancer
    * `subnetID` - ID Neutron subnet, в котором создать load balancer virtual IP.
        * Формат — строка.
        * Опциональный параметр.
    * `floatingNetworkID` - ID external network, который будет использоваться для заказа floating ip
        * Формат — строка.
        * Опциональный параметр.
* `zones` - список зон, в котором по-умолчанию заказывать инстансы. Может быть переопределён индивидуально для каждой NodeGroup'ы
    * Формат — массив строк.

#### Пример конфигурации

```yaml
cloudProviderOpenstack: |
  connection:
    authURL: https://test.tests.com:5000/v3/
    domainName: default
    tenantName: default
    username: jamie
    password: nein
    region: HetznerFinland
  externalNetworkNames:
  - public
  internalNetworkNames:
  - kube
  instances:
    sshKeyPairName: my-ssh-keypair
    securityGroups:
    - default
    - allow-ssh-and-icmp
  zones:
  - zone-a
  - zone-b
```

## Как мне поднять гибридный (вручную заведённые ноды) кластер?

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. [Включить](#пример-конфигурации) модуль и прописать ему необходимые для работы параметры.

**Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел кубернетес запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

## Подключение storage в гибридном кластере

Если вам требуются PersistentVolumes на нодах, подключаемых к кластеру из openstack, то необходимо создать StorageClass с нужным OpenStack volume type. Получить список типов можно командой `openstack volume type list`.
Например, для volume type `ceph-ssd`:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ceph-ssd
provisioner: csi-cinderplugin # обязательно такой
parameters:
  type: ceph-ssd
volumeBindingMode: WaitForFirstConsumer
```
