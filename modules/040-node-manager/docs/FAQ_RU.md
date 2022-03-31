---
title: "Управление узлами: FAQ"
search: добавить ноду в кластер, добавить узел в кластер, настроить узел с GPU, эфемерные узлы
---

## Как добавить статичный узел в кластер?

Чтобы добавить новый статичный узел (выделенная ВМ или железный сервер, например) в кластер, необходимо:

- Использовать существующую или создать новую `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `CloudStatic`). Например, создадим [`NodeGroup` с именем `worker`](usage.html#пример-описания-статичной-nodegroup).
- Получить скрипт для установки и настройки узла: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r`.
- Перед настройкой Kubernetes на узле убедитесь, что вы выполнили все необходимые действия для корректной работы узла в кластере:
  - Добавили в `/etc/fstab` все необходимые точки монтирования (nfs, ceph, ...);
  - Установили на узел пакет `ceph-common` нужной версии, или другие необходимые пакеты;
  - Настроили сетевую связанность между узлами кластера.
- Зайти на новый узел по ssh и выполнить команду из секрета: `echo <base64> | base64 -d | bash`.

## Как завести существующий узел кластера под управление node-manager?

Чтобы завести существующий узел кластера под управление `node-manager`, необходимо:

- Использовать существующую или создать новую `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `CloudStatic`). Например, создадим [`NodeGroup` с именем `worker`](usage.html#пример-описания-статичной-nodegroup).
- Получить скрипт для установки и настройки узла: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."adopt.sh"' -r`.
- Зайти на новый узел по ssh и выполнить команду из секрета: `echo <base64> | base64 -d | bash`.

## Как изменить node-group у статичного узла?

Чтобы перенести существующий статичный узел из одной node-group в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Изменения не будут применены мгновенно. Обновлением состояния объектов NodeGroup занимается один из хуков Deckhouse, который подписывается на изменения узлов.

## Как вывести узел из-под управления node-manager?

- Остановить сервис и таймер bashible: `systemctl stop bashible.timer bashible.service`
- Удалить скрипты bashible: `rm -rf /var/lib/bashible`
- Удалить с узла аннотации и лейблы:
  ```shell
  kubectl annotate node <node_name> node.deckhouse.io/configuration-checksum- update.node.deckhouse.io/waiting-for-approval- update.node.deckhouse.io/disruption-approved- update.node.deckhouse.io/disruption-required- update.node.deckhouse.io/approved- update.node.deckhouse.io/draining- update.node.deckhouse.io/drained-
  kubectl label node <node_name> node.deckhouse.io/group-
  ```

## Как зачистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если вам нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если вам просто нужно изменить NodeGroup, вы должны следовать [этой инструкции](#как-изменить-node-group-у-статичного-узла).

1. Удалить ноду из кластера Kubernetes:
    ```shell
    kubectl drain <node> --ignore-daemonsets --delete-local-data
    kubectl delete node <node>
    ```
1. Остановить все сервисы и запущенные контейнеры:
    ```shell
    systemctl stop kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
    systemctl stop bashible.service bashible.timer
    systemctl stop kubelet.service
    systemctl stop containerd
    systemctl list-units --full --all | grep -q docker.service && systemctl stop docker
    kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
    ```
1. Удалить маунты:
   ```shell
   for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done
   ```
1. Удалить директории и файлы:
   ```shell
   rm -rf /var/lib/bashible
   rm -rf /var/cache/registrypackages
   rm -rf /etc/kubernetes
   rm -rf /var/lib/kubelet
   rm -rf /var/lib/docker
   rm -rf /var/lib/containerd
   rm -rf /etc/cni
   rm -rf /var/lib/cni
   rm -rf /var/lib/etcd
   rm -rf /etc/systemd/system/kubernetes-api-proxy*
   rm -rf /etc/systemd/system/bashible*
   rm -rf /etc/systemd/system/sysctl-tuner*
   rm -rf /etc/systemd/system/kubelet*
   ```
1. Удалить интерфейсы:
   ```shell
   ifconfig cni0 down
   ifconfig flannel.1 down
   ifconfig docker0 down
   ip link delete cni0
   ip link delete flannel.1
   ```
1. Очистить systemd:
   ```shell
   systemctl daemon-reload
   systemctl reset-failed
   ```
1. Запустить обратно CRI:
   ```shell
   systemctl start containerd
   systemctl list-units --full --all | grep -q docker.service && systemctl start docker
   ```
1. [Запустить](#как-добавить-статичный-узел-в-кластер) скрипт `bootstrap.sh`.
1. Включить все сервисы обратно:
   ```shell
   systemctl start kubelet.service
   systemctl start kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl start bashible.service bashible.timer
   ```

## Как понять, что что-то пошло не так?

Нужно посмотреть логи сервиса `bashible`, который модуль `node-manager` создает на каждом узле. 

Посмотреть логи сервиса `bashible`:
```shell
journalctl -fu bashible
```

## Как посмотреть, что в данный момент выполняется на узле при его создании?

Если мы хотим узнать, что происходит на узле (к примеру он долго создается), то можно посмотреть логи `cloud-init`. Для этого необходимо:
- Найти узел, который сейчас бутстрапится: `kubectl -n d8-cloud-instance-manager get machine | grep Pending`;
- Посмотреть информацию о `machine`: `kubectl -n d8-cloud-instance-manager describe machine kube-2-worker-01f438cf-757f758c4b-r2nx2`. Пример результата:
  ```shell
  Status:
    Bootstrap Status:
      Description:   Use 'nc 192.168.199.115 8000' to get bootstrap logs.
      Tcp Endpoint:  192.168.199.115
  ```

- Выполнить команду `nc 192.168.199.115 8000`. Вы увидите логи `cloud-init` и увидите на чем зависла настройка узла.

Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.

## Как настроить узел с GPU?

Если у вас есть узел с GPU и вы хотите настроить Docker для работы с `node-manager`, то вам необходимо выполнить все настройки на узле по [документации](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

Создать `NodeGroup` с такими параметрами:

```shell
  cri:
    type: NotManaged
  operatingSystem:
    manageKernel: false
```

После чего, добавить узел под управление `node-manager`.

## Какие параметры NodeGroup к чему приводят?

| Параметр NG                           | Disruption update          | Перезаказ узлов   | Рестарт kubelet |
|---------------------------------------|----------------------------|-------------------|-----------------|
| operatingSystem.manageKernel          | + (true) / - (false)       | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.docker.maxConcurrentDownloads     | +                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| nodeTemplate                          | -                          | -                 | -               |
| chaos                                 | -                          | -                 | -               |
| kubernetesVersion                     | -                          | -                 | +               |
| static                                | -                          | -                 | +               |
| disruptions                           | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |

Подробно о всех параметрах можно прочитать в описании custom resource [NodeGroup](cr.html#nodegroup)

В случае изменения параметра `instancePrefix` в конфигурации Deckhouse, не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит.

При disruption update выполняется evict подов с ноды. Если какие-либо поды не удалось evict'нуть, evict повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После этого поды, которые не удалось evict'нуть удаляются. 

## Как перекатить эфемерные машины в облаке с новой конфигурацией?

При изменении конфигурации Deckhouse (как в модуле node-manager, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Перекат происходит только после изменения ресурсов `InstanceClass` или `NodeGroup`.

Для того, чтобы принудительно перекатить все узлы, связанные с ресурсом `Machines`, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки?

> ⛔ Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов **Deckhouse**. Отдайте предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.

Для решений данной задачи существуют два механизма:

- Установка меток в `NodeGroup` `spec.nodeTemplate.labels`, для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
- Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints`, с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

> ℹ Deckhouse по умолчанию толерейтит ключ `dedicated`, поэтому рекомендуется использовать ключ `dedicated` с любым `value` для тейнтов на ваших выделенных узлах.️
> Если необходимо использовать произвольные ключи для `taints` (например, `dedicated.client.com`), то нужно добавить в `ConfigMap` `d8-system/deckhouse` в секцию `global.modules.placement.customTolerationKeys` значение ключа. Таким образом мы разрешим системным компонентам (например `cni-flannel`) выезжать на эти выделенные узлы.

Подробности [в статье на Habr](https://habr.com/ru/company/flant/blog/432748/).

## Как выделить узлы под системные компоненты?

### Фронтенд

Для **Ingress**-контроллеров используйте `NodeGroup` со следующей конфигурацией:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/frontend: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

### Системные

`NodeGroup` для компонентов подсистем Deckhouse, будут с такими параметрами:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```

## Как ускорить заказ узлов в облаке при горизонтальном масштабировании приложений?

Самое действенное — держать в кластере некоторое количество "подогретых" узлов, которые позволят новым репликам ваших приложений запускаться мгновенно. Очевидным минусом данного решения будут дополнительные расходы на содержание этих узлов.

Необходимые настройки целевой `NodeGroup` будут следующие:

1. Указать абсолютное количество "подогретых" узлов (или процент от максимального количества узлов в этой группе) в параметре `cloudInstances.standby`.
1. При наличии, дополнительных служебных компонентов (не обслуживаемых Deckhouse, например DaemonSet `filebeat`) для этих узлов — задать их суммарное потребление ресурсов в параметре `standbyHolder.notHeldResources`.
1. Для работы, этой функции требуется чтобы как минимум один узел из группы уже был запущен в кластере. Иными словами, либо должна быть доступна одна реплика приложения, либо количество узлов для этой группы `cloudInstances.minPerZone` должно быть `1`.

Пример:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    notHeldResources:
      cpu: 300m
      memory: 2Gi
```

## Как выключить machine-controller-manager в случае выполнения потенциально деструктивных изменений в кластере?

> ⛔ **_Внимание!!!_** Использовать эту настройку только тогда, когда вы четко понимаете зачем это необходимо !!!

Установить параметр:

```yaml
mcmEmergencyBrake: true
```

## Как восстановить master-узел, если kubelet не может загрузить компоненты control-plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы
компонентов `control-plane` (например, удалена директория `/var/lib/docker` при использовании docker или `/var/lib/containerd` при использовании containerd). В этом случае, `kubelet` при рестарте  не сможет скачать образы `control-plane`-компонентов, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Как восстановить:

### Docker

Для восстановления работоспособности master-узла, нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r 'del(.auths."registry.deckhouse.io".username, .auths."registry.deckhouse.io".password)'
```

Вывод команды нужно скопировать, и добавить его в файл `/root/.docker/config.json` на поврежденном master-узле.
Далее, на поврежденном master-узле нужно загрузить образы `control-plane` компонентов:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  docker pull $image
done
```

После загрузки образов необходимо перезапустить `kubelet`.
После восстановления работоспособности master-узла, **необходимо убрать внесенные в файл `/root/.docker/config.json` изменения!*

### Containerd

Для восстановления работоспособности master-узла, нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать, и присвоить переменной AUTH на поврежденном master-узле.
Далее, на поврежденном master-узле нужно загрузить образы `control-plane` компонентов:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов, необходимо перезапустить `kubelet`.

## Как изменить CRI для node-group?

Установить параметр `cri.type` в `Docker` или в `Containerd`.
Пример YAML-манифеста NodeGroup:
```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  cri:
    type: Containerd
```

Также, эту операцию можно выполнить при помощи патча:

* Containerd:
  ```shell
  kubectl patch nodegroup <имя node-group> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Docker:
  ```shell
  kubectl patch nodegroup <имя node-group> --type merge -p '{"spec":{"cri":{"type":"Docker"}}}'
  ```

> ⛔ **_Внимание!!!_** Нельзя устанавливать `cri.type` для node-group, созданных при помощи `dhctl`, например, node-group `master`.

После настройки нового CRI для NodeGroup, модуль node-manager по одному drain'ит узлы и устанавливает на них новый CRI. Обновление узла
сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup, модуль node-manager либо автоматически разрешает обновление
узлов, либо требует ручного подтверждения.

## Как изменить CRI для всего кластера?
Необходимо при помощи утилиты `dhctl` отредактировать параметр `defaultCRI` в конфиге `cluster-configuration`.

Также, возможно выполнить эту операцию при помощи `kubectl patch`. Пример:
* Containerd
```shell
data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Docker/Containerd/" | base64 -w0)"
kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
```
* Docker
```shell
data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/Docker/" | base64 -w0)"
kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
```

Если необходимо какую-то node-group оставить на другом CRI, то перед изменением `defaultCRI` необходимо установить CRI для этой node-group,
как описано [здесь](#как-изменить-cri-для-node-group).

> ⛔ **_Внимание!!!_** Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master узлы!!!
> Если master узел один, данная операция является опасной и может привести к полной неработоспособности кластера!!!
> Предпочтительный вариант - сделать multimaster и поменять тип CRI!!!

При изменении CRI в кластере для master узлов необходимы дополнительные шаги:

* Docker -> Containerd

* Для каждого master узла по очереди необходимо будет:
1. В случае, если для master node-group `approvalMode` установлен в `Manual`, подтвердить disruption:
```shell
kubectl annotate node <имя master узла> update.node.deckhouse.io/disruption-approved=
```
2. Дождаться перехода обновленного master узла в Ready.
* Containerd -> Docker

Перед изменением `defaultCRI` необходимо на каждом master узле сформировать docker config:
```shell
mkdir -p ~/docker && kubectl -n d8-system get secret deckhouse-registry -o json | jq -r '.data.".dockerconfigjson"' | base64 -d > ~/.docker/config.json
```

Для каждого master узла по очереди необходимо будет:
1. В случае, если для master node-group `approvalMode` установлен в `Manual`, подтвердить disruption:
```shell
kubectl annotate node <имя master узла> update.node.deckhouse.io/disruption-approved=
```
2. После обновления CRI и ребута выполнить команду:
```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  docker pull $image
done
```
3. Дождаться перехода обновленного master узла в Ready.
4. Удалить на обновленном master узле docker config:
```shell
rm -f ~/.docker/config.json
```

## Как добавить шаг для конфигурации узлов?
Дополнительные шаги для конфигурации узлов задаются при помощи CR `NodeGroupConfiguration`.
