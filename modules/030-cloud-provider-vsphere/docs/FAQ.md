---
title: "Сloud provider — VMware vSphere: FAQ"
---

## Как мне поднять гибридный (вручную заведённые ноды) кластер?

Hybrid кластер представляет собой объединённые в один кластер bare metal ноды и ноды vSphere. Для создания такого кластера
необходимо наличие L2 сети между всеми нодами кластера.

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. Включить модуль и прописать ему необходимые для работы [параметры](#Параметры-конфигурации).

**Важно!** Cloud-controller-manager синхронизирует состояние между vSphere и Kubernetes, удаляя из Kubernetes те узлы, которых нет в vSphere. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел кубернетес запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).

### Параметры конфигурации

**Внимание!** При изменении конфигурационных параметров приведенных в этой секции (параметров, указываемых в ConfigMap deckhouse) **перекат существующих Machines НЕ производится** (новые Machines будут создаваться с новыми параметрами). Перекат происходит только при изменении параметров `NodeGroup` и `VsphereInstanceClass`. См. подробнее в документации модуля [node-manager](/modules/040-node-manager/faq.html#как-перекатить-эфемерные-машины-в-облаке-с-новой-конфигурацией).

* `host` — домен vCenter сервера.
* `username` — логин.
* `password` — пароль.
* `vmFolderPath` — путь до VirtualMachine Folder, в котором будут создаваться склонированные виртуальные машины.
  * Пример — `dev/test`
* `insecure` — можно выставить в `true`, если vCenter имеет самоподписанный сертификат.
  * Формат — bool.
  * Опциональный параметр. По умолчанию `false`.
* `regionTagCategory`— имя **категории** тэгов, использующихся для идентификации региона (vSphere Datacenter).
  * Формат — string.
  * Опциональный параметр. По умолчанию `k8s-region`.
* `zoneTagCategory` — имя **категории** тэгов, использующихся для идентификации зоны (vSphere Cluster).
    * Формат — string.
    * Опциональный параметр. По умолчанию `k8s-zone`.
* `defaultDatastore` — имя vSphere Datastore, который будет использоваться в качестве default StorageClass.
  * Формат — string.
  * Опциональный параметр. По умолчанию будет использован лексикографически первый Datastore.
* `disableTimesync` — отключить ли синхронизацию времени со стороны vSphere. **Внимание!** это не отключит NTP демоны в гостевой ОС, а лишь отключит "подруливание" временем со стороны ESXi.
  * Формат — bool.
  * Опциональный параметр. По умолчанию `true`.
* `region` — тэг, прикреплённый к vSphere Datacenter, в котором будут происходить все операции: заказ VirtualMachines, размещение их дисков на datastore, подключение к network.
* `sshKeys` — список public SSH ключей в plain-text формате.
  * Формат — массив строк.
  * Опциональный параметр. По умолчанию разрешённых ключей для пользователя по умолчанию не будет.
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

#### Пример

```yaml
cloudProviderVsphereEnabled: "true"
cloudProviderVsphere: |
  host: vc-3.internal
  username: user
  password: password
  vmFolderPath: dev/test
  insecure: true
  region: moscow-x001
  sshKeys:
  - "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD5sAcceTHeT6ZnU+PUF1rhkIHG8/B36VWy/j7iwqqimC9CxgFTEi8MPPGNjf+vwZIepJU8cWGB/By1z1wLZW3H0HMRBhv83FhtRzOaXVVHw38ysYdQvYxPC0jrQlcsJmLi7Vm44KwA+LxdFbkj+oa9eT08nQaQD6n3Ll4+/8eipthZCDFmFgcL/IWy6DjumN0r4B+NKHVEdLVJ2uAlTtmiqJwN38OMWVGa4QbvY1qgwcyeCmEzZdNCT6s4NJJpzVsucjJ0ZqbFqC7luv41tNuTS3Moe7d8TwIrHCEU54+W4PIQ5Z4njrOzze9/NlM935IzpHYw+we+YR+Nz6xHJwwj i@my-PC"
  externalNetworkNames:
  - KUBE-3
  - devops-internal
  internalNetworkNames:
  - KUBE-3
  - devops-internal
```
