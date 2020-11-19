---
title: "Управление узлами: FAQ"
search: добавить ноду в кластер, настроить ноду с GPU, эфемерные узлы
---

## Как автоматически добавить статичный узел в кластер?

Чтобы добавить новую ноду в статичный кластер необходимо:
- Создать `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `Hybrid`) или использовать уже существующую. К примеру создадим `NodeGroup` с именем `example`.
- Получить скрипт для установки и настройки ноды: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-example -o json | jq '.data."bootstrap.sh"' -r`
- Перед настройкой kubernetes на ноде убедитесь, что вы выполнили все необходимые действия для корректной работы узла в кластере:
  - Добавили в `/etc/fstab` все необходимые маунты (nfs, ceph, ...)
  - Установили на ноду `ceph-common` нужной версии или еще какие-то пакеты
  - Настроили сеть для коммуникации узлов в кластере
- Зайти на новую ноду по ssh и выполнить команду из секрета: `echo <base64> | base64 -d | bash`

## Как завести ноду под управление node-manager?

Чтобы завести ноду под управление `node-manager`:
- Создать `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `Hybrid`) или использовать уже существующую. К примеру создадим `NodeGroup` с именем `nodes`.
- Получить скрипт для установки и настройки моды: `kubectl -n d8-cloud-instance-manager  get secret manual-bootstrap-for-nodes-o json | jq '.data."adopt.sh"' -r`
- Зайти на новую ноду по ssh и выполнить команду из секрета: `echo <base64> | base64 -d | bash`

## Как изменить node-group у статичного узла?

Чтобы перенести существующий статичный узел из одной node-group в другую, необходимо изменить у узла лейбл группы:
```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<group_name>
```

Изменения не будут применены мгновенно. Обновлением состояния объектов NodeGroup занимается один из хуков deckhouse, который подписывается на изменения нод.

## Как вывести ноду из-под управления node-manager?

- Остановить сервис и таймер bashible: `systemctl stop bashible.timer bashible.service`
- Удалить скрипты bashible: `rm -rf /var/lib/bashible`
- Удалить с ноды аннотации и лейблы:
```shell
kubectl annotate node <node_name> node.deckhouse.io/configuration-checksum- update.node.deckhouse.io/waiting-for-approval- update.node.deckhouse.io/disruption-approved- update.node.deckhouse.io/disruption-required- update.node.deckhouse.io/approved- update.node.deckhouse.io/draining- update.node.deckhouse.io/drained-
kubectl label node <node_name> node.deckhouse.io/group-
```

## Как зачистить ноду для последующего ввода в кластер?

1. Установим kubeadm нужной версии и выполним reset.

    ```shell
    apt install kubeadm=1.16* -y
    kubeadm reset
    ```

2. Остановим все сервисы и удалим все директории.

    ```shell
    systemctl disable --now kubernetes-api-proxy
    systemctl disable --now bashible.timer
    systemctl disable --now bashible
    systemctl disable --now kubelet
    systemctl disable --now kubelet-face-slapper.timer
    systemctl disable --now kubelet-face-slapper.service
    rm -rf /var/lib/bashible/ /etc/kubernetes/deckhouse/ /etc/kubernetes/kubernetes-api-proxy/ /etc/systemd/system/kubernetes-api-proxy.service
    systemctl daemon-reload
    systemctl restart kubernetes-api-proxy
    ```

3. [Запустим](#как-автоматически-добавить-статичный-узел-в-кластер) `bootstrap.sh`.
4. Включим все сервисы обратно.

    ```shell
    systemctl enable --now kubernetes-api-proxy
    systemctl enable --now bashible.timer
    systemctl enable --now bashible
    systemctl enable --now kubelet
    systemctl enable --now kubelet-face-slapper.timer
    systemctl enable --now kubelet-face-slapper.service
    ```

## Как понять, что что-то пошло не так?

Модуль `node-manager` создает на каждой ноде сервис `bashible`, и его логи можно посмотреть при помощи: `journalctl -fu bashible`.

## Как посмотреть, что в данный момент выполняется на ноде при ее создании?

Если мы хотим узнать, что происходит на ноде (к примеру она долго создается), то можно посмотреть логи `cloud-init` для этого необходимо:
- Найти ноду, которая сейчас бутстрапится: `kubectl -n d8-cloud-instance-manager  get machine | grep Pending`
- Посмотреть информацию о `machine`: `kubectl -n d8-cloud-instance-manager describe machine kube-2-worker-01f438cf-757f758c4b-r2nx2`
В дескрайбе мы увидим такую информацию:
```shell
Status:
  Bootstrap Status:
    Description:   Use 'nc 192.168.199.115 8000' to get bootstrap logs.
    Tcp Endpoint:  192.168.199.115
```
- Выполнить команду `nc 192.168.199.115 8000`, тогда вы увидите логи `cloud-init` и увидите на чем зависла настройка ноды

Логи первоначальной настройки ноды находятся в `/var/log/cloud-init-output.log`.

## Как настроить ноду с GPU?

Если у вас есть нода с GPU и вы хотите настроить docker для работы с `node-manager`, то вам необходимо выполнить все настройки на ноде по [документации](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

Создать `NodeGroup` с такими параметрами:
```shell
  docker:
    manage: false
  operatingSystem:
    manageKernel: false
```

После чего добавить ноду под управление `node-manager`.

## Какие параметры NodeGroup к чему приводят?

| Параметр NG                   | Disruption update    | Перезаказ нод | Рестарт kubelet |
|-------------------------------|----------------------|---------------|-----------------|
| operatingSystem.manageKernel  | + (true) / - (false) | -             | -               |
| kubelet.maxPods               | -                    | -             | +               |
| kubelet.rootDir               | -                    | -             | +               |
| docker.maxConcurrentDownloads | +                    | -             | +               |
| docker.manage                 | + (true) / - (false) | -             | -               |
| nodeTemplate                  | -                    | -             | -               |
| chaos                         | -                    | -             | -               |
| kubernetesVersion             | -                    | -             | +               |
| static                        | -                    | -             | +               |
| disruptions                   | -                    | -             | -               |
| cloudInstances.classReference | -                    | +             | -               |

Подробно о всех параметрах можно прочитать в описании custom resource [NodeGroup](/modules/040-node-manager/cr.html#nodegroup)

В случае изменения параметра `instancePrefix` в конфигурации deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит.

## Как перекатить эфемерные машины в облаке с новой конфигурацией?

При изменении конфигурации Deckhouse (как в модуле node-manager, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Перекат происходит только после изменения `InstanceClass` или `NodeGroup` объектов.

Для того, чтобы форсированно перекатить все Machines, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки?

> ⛔ Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов **Deckhouse**. Отдайте предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.

Для решений данной задачи существуют два механизма:
- Установка меток в `NodeGroup` `spec.nodeTemplate.labels`, для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает какие именно ноды будут выбраны планировщиком для запуска целевого приложения
- Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints`, с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих нодах.

> ℹ Deckhouse по умолчанию толерейтит ключ `dedicated`, поэтому рекомендуется использовать ключ `dedicated` с любым `value` для тейнтов на ваших выделенных нодах.️
> Если необходимо использовать произвольные ключи для `taints` (например, `dedicated.client.com`), то нужно добавить в `ConfigMap` `d8-system/deckhouse` в секцию `global.modules.placement.customTolerationKeys` значение ключа. Таким образом мы разрешим системным компонентам (например `cni-flannel`) выезжать на эти выделенные ноды.

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
1. Для работы этой функции требуется, чтобы как минимум один узел из группы уже был запущен в кластере. Иными словами должна быть либо одна реплика приложения, либо количество узлов для этой группы `cloudInstances.minPerZone` должно быть `1`.

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
