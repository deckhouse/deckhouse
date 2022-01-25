---
title: "Cloud provider — VMware vSphere: настройки провайдера"
---

## VsphereClusterConfiguration
Схема размещения (layout) описывается структурой `VsphereClusterConfiguration`:
* `layout` — название схемы размещения.
  * Варианты — `Standard` (описание ниже).
* `provider` — параметры подключения к vCenter.
  * `server` — хост или IP vCenter сервера.
  * `username` — логин.
  * `password` — пароль.
  * `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
    * Формат — bool.
    * Опциональный параметр. По умолчанию `false`.
* `masterNodeGroup` — описание master NodeGroup.
  * `replicas` — сколько master-узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass](cr.html#vsphereinstanceclass). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * `mainNetworkIPAddresses` — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master-узлам в основной сети (параметр `mainNetwork`).
      * Опциональный параметр. По умолчанию, включается DHCP клиент.
      * `address` — IP адрес с CIDR префиксом.
        * Пример: `10.2.2.2/24`.
      * `gateway` — IP адрес шлюза по умолчанию. Должен находится в подсети, указанной в `address`.
        * Пример: `10.2.2.254`.
      * `nameservers`
        * `addresses` — список dns-серверов.
          * Пример: `- 8.8.8.8`
        * `search` — список DNS search domains.
          * Пример: `- tech.lan`
* `nodeGroups` — массив дополнительных NG для создания статичных узлов (например, для выделенных фронтов или шлюзов). Настройки NG:
  * `name` — имя NG, будет использоваться для генерации имени узлов.
  * `replicas` — сколько узлов создать.
  * `zones` — узлы будут создаваться только в перечисленных зонах.
  * `instanceClass` — частичное содержимое полей [VsphereInstanceClass](cr.html#vsphereinstanceclass). Обязательными параметрами являются `numCPUs`, `memory`, `template`, `mainNetwork`, `datastore`.
    * `numCPUs`
    * `memory`
    * `template`
    * `mainNetwork`
    * `additionalNetworks`
    * `datastore`
    * `rootDiskSize`
    * `resourcePool`
    * `runtimeOptions`
      * `nestedHardwareVirtualization`
      * `cpuShares`
      * `cpuLimit`
      * `cpuReservation`
      * `memoryShares`
      * `memoryLimit`
      * `memoryReservation`
    * `mainNetworkIPAddresses` — список статических адресов (с CIDR префиксом), назначаемых (по-очереди) master-узлам в основной сети (параметр `mainNetwork`).
      * Опциональный параметр. По умолчанию, включается DHCP клиент.
      * `address` — IP адрес с CIDR префиксом.
        * Пример: `10.2.2.2/24`.
      * `gateway` — IP адрес шлюза по умолчанию. Должен находится в подсети, указанной в `address`.
        * Пример: `10.2.2.254`.
      * `nameservers`
        * `addresses` — массив dns-серверов.
          * Пример: `- 8.8.8.8`
        * `search` — массив DNS search domains.
          * Пример: `- tech.lan`
  * `nodeTemplate` — настройки Node-объектов в Kubernetes, которые будут добавлены после регистрации узла.
    * `labels` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.labels`
      * Пример:
        ```yaml
        labels:
          environment: production
          app: warp-drive-ai
        ```
    * `annotations` — аналогично стандартному [полю](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#objectmeta-v1-meta) `metadata.annotations`
      * Пример:
        ```yaml
        annotations:
          ai.fleet.com/discombobulate: "true"
        ```
    * `taints` — аналогично полю `.spec.taints` из объекта [Node](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#taint-v1-core). **Внимание!** Доступны только поля `effect`, `key`, `values`.
      * Пример:

        ```yaml
        taints:
        - effect: NoExecute
          key: ship-class
          value: frigate
        ```
* `internalNetworkCIDR` — подсеть для master-узлов во внутренней сети. Адреса выделяются с десятого адреса. Например, для подсети `192.168.199.0/24` будут использованы адреса начиная с `192.168.199.10`. Будет использоваться при использовании `additionalNetworks` в `masterInstanceClass`.
* `vmFolderPath` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
  * Пример — `dev/test`
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
  * Формат — string.
  * Опциональный параметр. По умолчанию `k8s-region`.
* `zoneTagCategory` — имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
  * Формат — string.
  * Опциональный параметр. По умолчанию `k8s-zone`.
* `disableTimesync` — отключить ли синхронизацию времени со стороны vSphere. **Внимание!** это не отключит NTP демоны в гостевой ОС, а лишь отключит "подруливание" временем со стороны ESXi.
  * Формат — bool.
  * Опциональный параметр. По умолчанию `true`.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `baseResourcePool` — относительный (от vSphere Cluster) путь до существующего родительского `resourcePool` для всех создаваемых (в каждой зоне) `resourcePool`'ов.
* `useNestedResourcePool` - создавать вложенный пул (`true`) или использовать основной пул (`false`). По-умолчанию - `true`.
* `sshPublicKey` — публичный ключ для доступа на узлы.
* `externalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления ExternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

    ```yaml
    externalNetworkNames:
    - MAIN-1
    - public
    ```

    * Опциональный параметр.
* `internalNetworkNames` — имена сетей (не полный путь, а просто имя), подключённые к VirtualMachines, и используемые vsphere-cloud-controller-manager для проставления InternalIP в `.status.addresses` в Node API объект.
  * Формат — массив строк. Например,

    ```yaml
    internalNetworkNames:
    - KUBE-3
    - devops-internal
    ```

  * Опциональный параметр.
* `zones` — ограничение набора зон, в которых разрешено создавать узлы.
  * Обязательный параметр.
  * Формат — массив строк.
