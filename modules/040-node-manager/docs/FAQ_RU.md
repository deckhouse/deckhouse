---
title: "Управление узлами: FAQ"
description: Управление узлами кластера Kubernetes. Добавление, удаление узлов в кластере. Изменение CRI узла.
search: добавить ноду в кластер, добавить узел в кластер, настроить узел с GPU, эфемерные узлы
---
{% raw %}

## Как добавить master-узлы в облачном кластере (single-master в multi-master)?

Инструкция в [FAQ модуля control-plane-manager...](../040-control-plane-manager/faq.html#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master)

## Как уменьшить число master-узлов в облачном кластере (multi-master в single-master)?

Инструкция в [FAQ модуля control-plane-manager...](../040-control-plane-manager/faq.html#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master)

## Статические узлы

<span id='как-добавить-статический-узел-в-кластер'></span>
<span id='как-добавить-статичный-узел-в-кластер'></span>

Добавить статический узел в кластер можно вручную ([пример](examples.html#вручную)) или с помощью [Cluster API Provider Static](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static).

### Как добавить статический узел в кластер (Cluster API Provider Static)?

Чтобы добавить статический узел в кластер (сервер bare metal или виртуальную машину), выполните следующие шаги:

1. Подготовьте необходимые ресурсы — серверы/виртуальные машины, установите специфические пакеты ОС, добавьте точки монтирования, настройте сетевую связанность и т.п.
1. Создайте ресурс [SSHCredentials](cr.html#sshcredentials).
1. Создайте ресурс [StaticInstance](cr.html#staticinstance).
1. Создайте ресурс [NodeGroup](cr.html#nodegroup) с [nodeType](cr.html#nodegroup-v1-spec-nodetype) `Static`, указав [желаемое количество узлов](cr.html#nodegroup-v1-spec-staticinstances-count) в группе и, при необходимости, [фильтр](cr.html#nodegroup-v1-spec-staticinstances-labelselector) выбора `StaticInstance`.

[Пример](examples.html#с-помощью-cluster-api-provider-static) добавления статического узла.

### Как добавить несколько статических узлов в кластер вручную?

Используйте существующий или создайте новый custom resource [NodeGroup](cr.html#nodegroup) ([пример](examples.html#пример-описания-статической-nodegroup) `NodeGroup` с именем `worker`).

Автоматизировать процесс добавления узлов можно с помощью любой платформы автоматизации. Далее приведен пример для Ansible.

1. Получите один из адресов Kubernetes API-сервера. Обратите внимание, что IP-адрес должен быть доступен с узлов, которые добавляются в кластер:

   ```shell
   kubectl -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Проверьте версию K8s. Если версия >= 1.25, создайте токен `node-group`:

   ```shell
   kubectl create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Сохраните полученный токен, и добавьте в поле `token:` playbook'а Ansible на дальнейших шагах.

1. Если версия Kubernetes меньше 1.25, получите Kubernetes API-токен для специального ServiceAccount'а, которым управляет Deckhouse:

   ```shell
   kubectl -n d8-cloud-instance-manager get $(kubectl -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

1. Создайте Ansible playbook с `vars`, которые заменены на полученные на предыдущих шагах значения:

   ```yaml
   - hosts: all
     become: yes
     gather_facts: no
     vars:
       kube_apiserver: <KUBE_APISERVER>
       token: <TOKEN>
     tasks:
       - name: Check if node is already bootsrapped
         stat:
           path: /var/lib/bashible
         register: bootstrapped
       - name: Get bootstrap secret
         uri:
           url: "https://{{ kube_apiserver }}/api/v1/namespaces/d8-cloud-instance-manager/secrets/manual-bootstrap-for-{{ node_group }}"
           return_content: yes
           method: GET
           status_code: 200
           body_format: json
           headers:
             Authorization: "Bearer {{ token }}"
           validate_certs: no
         register: bootstrap_secret
         when: bootstrapped.stat.exists == False
       - name: Run bootstrap.sh
         shell: "{{ bootstrap_secret.json.data['bootstrap.sh'] | b64decode }}"
         args:
           executable: /bin/bash
         ignore_errors: yes
         when: bootstrapped.stat.exists == False
       - name: wait
         wait_for_connection:
           delay: 30
         when: bootstrapped.stat.exists == False
   ```

1. Определите дополнительную переменную `node_group`. Значение переменной должно совпадать с именем `NodeGroup`, которой будет принадлежать узел. Переменную можно передать различными способами, например с использованием inventory-файла:

   ```text
   [system]
   system-0
   system-1

   [system:vars]
   node_group=system

   [worker]
   worker-0
   worker-1

   [worker:vars]
   node_group=worker
   ```

1. Выполните playbook с использованием inventory-файла.

### Как вручную очистить статический узел?

<span id='как-вывести-узел-из-под-управления-node-manager'></span>

> Инструкция справедлива как для узла, настроенного вручную (с помощью bootstrap-скрипта), так и для узла, настроенного с помощью CAPS.

Чтобы вывести из кластера узел и очистить сервер (ВМ), выполните следующую команду на узле:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

### Можно ли удалить StaticInstance?

`StaticInstance`, находящийся в состоянии `Pending` можно удалять без каких-либо проблем.

Чтобы удалить `StaticInstance` находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstraping`):
1. Добавьте лейбл `"node.deckhouse.io/allow-bootstrap": "false"` в `StaticInstance`.
1. Дождитесь, пока `StaticInstance` перейдет в статус `Pending`.
1. Удалите `StaticInstance`.
1. Уменьшите значение параметра `NodeGroup.spec.staticInstances.count` на 1.

### Как изменить IP-адрес StaticInstance?

Изменить IP-адрес в ресурсе `StaticInstance` нельзя. Если в `StaticInstance` указан ошибочный адрес, то нужно [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

### Как мигрировать статический узел настроенный вручную под управление CAPS?

Необходимо выполнить [очистку узла](#как-вручную-очистить-статический-узел), затем [добавить](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static) узел под управление CAPS.

## Как изменить NodeGroup у статического узла?

<span id='как-изменить-nodegroup-у-статичного-узла'><span>

Если узел находится под управлением [CAPS](./#cluster-api-provider-static), то изменить принадлежность к `NodeGroup` у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

Чтобы перенести существующий статический узел созданный [вручную](./#работа-со-статическими-узлами) из одной `NodeGroup` в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.

## Как зачистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить `NodeGroup`, следуйте [этой инструкции](#как-изменить-nodegroup-у-статического-узла).

> **Внимание!** Если на зачищаемом узле есть пулы хранения LINSTOR/DRBD, то предварительно выгоните ресурсы с узла и удалите узел LINSTOR/DRBD, следуя [инструкции](/modules/sds-replicated-volume/stable/faq.html#как-выгнать-ресурсы-с-узла).

1. Удалите узел из кластера Kubernetes:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Запустите на узле скрипт очистки:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. После перезагрузки узла [запустите](#как-добавить-статический-узел-в-кластер) скрипт `bootstrap.sh`.

## Как понять, что что-то пошло не так?

Если узел в NodeGroup не обновляется (значение `UPTODATE` при выполнении команды `kubectl get nodegroup` меньше значения `NODES`) или вы предполагаете какие-то другие проблемы, которые могут быть связаны с модулем `node-manager`, нужно посмотреть логи сервиса `bashible`. Сервис `bashible` запускается на каждом узле, управляемом модулем `node-manager`.

Чтобы посмотреть логи сервиса `bashible`, выполните на узле следующую команду:

```shell
journalctl -fu bashible
```

Пример вывода, когда все необходимые действия выполнены:

```console
May 25 04:39:16 kube-master-0 systemd[1]: Started Bashible service.
May 25 04:39:16 kube-master-0 bashible.sh[1976339]: Configuration is in sync, nothing to do.
May 25 04:39:16 kube-master-0 systemd[1]: bashible.service: Succeeded.
```

## Как посмотреть, что в данный момент выполняется на узле при его создании?

Если необходимо узнать, что происходит на узле (к примеру, он долго создается), можно посмотреть логи `cloud-init`. Для этого выполните следующие шаги:
1. Найдите узел, который сейчас бутстрапится:

   ```shell
   kubectl get instances | grep Pending
   ```

   Пример:

   ```shell
   $ kubectl get instances | grep Pending
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Получите информацию о параметрах подключения для просмотра логов:

   ```shell
   kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   Пример:

   ```shell
   $ kubectl get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   bootstrapStatus:
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. Выполните полученную команду (в примере выше — `nc 192.168.199.178 8000`), чтобы увидеть логи `cloud-init` и на чем зависла настройка узла.

Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.

## Как обновить ядро на узлах?

### Для дистрибутивов, основанных на Debian

Создайте ресурс `NodeGroupConfiguration`, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="5.15.0-53-generic"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-apt-install "linux-image-${desired_version}"
```

### Для дистрибутивов, основанных на CentOS

Создайте ресурс `NodeGroupConfiguration`, указав в переменной `desired_version` shell-скрипта (параметр `spec.content` ресурса) желаемую версию ядра:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-kernel.sh
spec:
  bundles:
    - '*'
  nodeGroups:
    - '*'
  weight: 32
  content: |
    # Copyright 2022 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    desired_version="3.10.0-1160.42.2.el7.x86_64"

    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      bb-log-info "Setting reboot flag due to kernel was updated"
      bb-flag-set reboot
    }

    version_in_use="$(uname -r)"

    if [[ "$version_in_use" == "$desired_version" ]]; then
      exit 0
    fi

    bb-deckhouse-get-disruptive-update-approval
    bb-yum-install "kernel-${desired_version}"
```

## Какие параметры NodeGroup к чему приводят?

| Параметр NG                           | Disruption update          | Перезаказ узлов   | Рестарт kubelet |
|---------------------------------------|----------------------------|-------------------|-----------------|
| chaos                                 | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |
| cloudInstances.maxSurgePerZone        | -                          | -                 | -               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| disruptions                           | -                          | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

Подробно о всех параметрах можно прочитать в описании custom resource [NodeGroup](cr.html#nodegroup).

В случае изменения параметров `InstanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит. Количество заказываемых одновременно `MachineDeployment` определяется параметром `cloudInstances.maxSurgePerZone`.

При disruption update выполняется evict подов с узла. Если какие-либо поды не удалось evict'нуть, evict повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После этого поды, которые не удалось evict'нуть, удаляются.

## Как пересоздать эфемерные машины в облаке с новой конфигурацией?

При изменении конфигурации Deckhouse (как в модуле `node-manager`, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Пересоздание происходит только после изменения ресурсов `InstanceClass` или `NodeGroup`.

Чтобы принудительно пересоздать все узлы, связанные с ресурсом `Machines`, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки?

> **Внимание!** Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов **Deckhouse**. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.

Для решений данной задачи существуют два механизма:

1. Установка меток в `NodeGroup` `spec.nodeTemplate.labels` для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
2. Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

> Deckhouse по умолчанию tolerate'ит ключ `dedicated`, поэтому рекомендуется использовать ключ `dedicated` с любым `value` для taint'ов на ваших выделенных узлах.️
> Если необходимо использовать произвольные ключи для `taints` (например, `dedicated.client.com`), значение ключа нужно добавить в параметр [modules.placement.customTolerationKeys](../../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys). Таким образом мы разрешим системным компонентам (например, `cni-flannel`) выезжать на эти выделенные узлы.

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

`NodeGroup` для компонентов подсистем Deckhouse будут с такими параметрами:

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

Самое действенное — держать в кластере некоторое количество *подогретых* узлов, которые позволят новым репликам ваших приложений запускаться мгновенно. Очевидным минусом данного решения будут дополнительные расходы на содержание этих узлов.

Необходимые настройки целевой `NodeGroup` будут следующие:

1. Указать абсолютное количество *подогретых* узлов (или процент от максимального количества узлов в этой группе) в параметре `cloudInstances.standby`.
1. При наличии дополнительных служебных компонентов (не обслуживаемых Deckhouse, например DaemonSet `filebeat`) для этих узлов — задать их суммарное потребление ресурсов в параметре `standbyHolder.notHeldResources`.
1. Для работы этой функции требуется, чтобы как минимум один узел из группы уже был запущен в кластере. Иными словами, либо должна быть доступна одна реплика приложения, либо количество узлов для этой группы `cloudInstances.minPerZone` должно быть `1`.

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

> **Внимание!** Использовать эту настройку допустимо только тогда, когда вы четко понимаете, зачем это необходимо.

Установить параметр:

```yaml
mcmEmergencyBrake: true
```

## Как восстановить master-узел, если kubelet не может загрузить компоненты control plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы
компонентов control plane (например, удалена директория `/var/lib/containerd`). В этом случае kubelet при рестарте не сможет скачать образы компонентов `control plane`, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Ниже инструкция по восстановлению master-узла.

### containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать и присвоить переменной AUTH на поврежденном master-узле.
Далее на поврежденном master-узле нужно загрузить образы компонентов `control-plane`:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить kubelet.

## Как изменить CRI для NodeGroup?

> **Внимание!** Возможен переход только с `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).

Установить параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type) в `Containerd` или в `NotManaged`.

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

Также эту операцию можно выполнить с помощью патча:

* Для `Containerd`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Для `NotManaged`:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

> **Внимание!** При смене `cri.type` для NodeGroup, созданных с помощью `dhctl`, нужно менять ее в `dhctl config edit provider-cluster-configuration` и настройках объекта `NodeGroup`.

После настройки нового CRI для NodeGroup модуль `node-manager` по одному drain'ит узлы и устанавливает на них новый CRI. Обновление узла
сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup модуль `node-manager` либо автоматически разрешает обновление
узлов, либо требует ручного подтверждения.

## Как изменить CRI для всего кластера?

> **Внимание!** Возможен переход только с `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).

Необходимо с помощью утилиты `dhctl` отредактировать параметр `defaultCRI` в конфиге `cluster-configuration`.

Также возможно выполнить эту операцию с помощью `kubectl patch`. Пример:
* Для `Containerd`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* Для `NotManaged`:

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

Если необходимо какую-то NodeGroup оставить на другом CRI, перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [здесь](#как-изменить-cri-для-nodegroup).

> **Внимание!** Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
> Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера!
> Предпочтительный вариант — сделать multimaster и поменять тип CRI!

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Deckhouse обновляет узлы в master NodeGroup по одному, поэтому необходимо определить, какой узел на данный момент обновляется:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердить disruption для master-узла, полученного на предыдущем шаге:

   ```shell
   kubectl annotate node <имя master-узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождаться перехода обновленного master-узла в `Ready`. Выполнить итерацию для следующего master'а.

## Как добавить шаг для конфигурации узлов?

Дополнительные шаги для конфигурации узлов задаются с помощью custom resource [NodeGroupConfiguration](cr.html#nodegroupconfiguration).

## Как использовать containerd с поддержкой Nvidia GPU?

Необходимо создать отдельную NodeGroup для GPU-нод.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

Далее необходимо создать NodeGroupConfiguration для NodeGroup `gpu` для конфигурации containerd:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
  - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/nvidia_gpu.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".containerd]
          default_runtime_name = "nvidia"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = false
    EOF
  nodeGroups:
  - gpu
  weight: 49
```

Далее необходимо добавить NodeGroupConfiguration для установки драйверов Nvidia для NodeGroup `gpu`.

### Ubuntu

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - ubuntu-lts
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/apt/sources.list.d/nvidia-container-toolkit.list" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
      curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    fi
    bb-apt-install nvidia-container-toolkit nvidia-driver-535-server
  nodeGroups:
  - gpu
  weight: 30
```

### Centos

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - centos
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    if [ ! -f "/etc/yum.repos.d/nvidia-container-toolkit.repo" ]; then
      distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
      curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    fi
    bb-yum-install nvidia-container-toolkit nvidia-driver
  nodeGroups:
  - gpu
  weight: 30
```

После этого выполните бутстрап и ребут узла.

### Как проверить, что все прошло успешно?

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: nvidia-cuda-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: nvidia-cuda-test
          image: nvidia/cuda:11.6.2-base-ubuntu20.04
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

И посмотрите логи:

```shell
$ kubectl logs job/nvidia-cuda-test
Tue Jan 24 11:36:18 2023
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 525.60.13    Driver Version: 525.60.13    CUDA Version: 12.0     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla T4            Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   45C    P0    25W /  70W |      0MiB / 15360MiB |      0%      Default |
|                               |                      |                  N/A |
+-------------------------------+----------------------+----------------------+

+-----------------------------------------------------------------------------+
| Processes:                                                                  |
|  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
|        ID   ID                                                   Usage      |
|=============================================================================|
|  No running processes found                                                 |
+-----------------------------------------------------------------------------+
```

Создайте в кластере Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gpu-operator-test
  namespace: default
spec:
  completions: 1
  template:
    spec:
      restartPolicy: Never
      nodeSelector:
        node.deckhouse.io/group: gpu
      containers:
        - name: gpu-operator-test
          image: nvidia/samples:vectoradd-cuda10.2
          imagePullPolicy: "IfNotPresent"
```

И посмотрите логи:

```shell
$ kubectl logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## Как развернуть кастомный конфиг containerd?

Bashible на узлах мержит основной конфиг containerd для Deckhouse с  конфигами из `/etc/containerd/conf.d/*.toml`.

### Как добавить авторизацию в дополнительный registry?

Разверните скрипт `NodeGroupConfiguration`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2023 Flant JSC
    #
    # Licensed under the Apache License, Version 2.0 (the "License");
    # you may not use this file except in compliance with the License.
    # You may obtain a copy of the License at
    #
    #     http://www.apache.org/licenses/LICENSE-2.0
    #
    # Unless required by applicable law or agreed to in writing, software
    # distributed under the License is distributed on an "AS IS" BASIS,
    # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
    # See the License for the specific language governing permissions and
    # limitations under the License.

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << "EOF"
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."artifactory.proxy"]
              endpoint = ["https://artifactory.proxy"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."artifactory.proxy".auth]
              auth = "AAAABBBCCCDDD=="
    EOF
  nodeGroups:
    - "*"
  weight: 49
```

## Как использовать NodeGroup с приоритетом?

С помощью параметра [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) custom resource'а `NodeGroup` можно задавать порядок заказа узлов в кластере.
Например, можно сделать так, чтобы сначала заказывались узлы типа *spot-node*, а если они закончились — обычные узлы. Или чтобы при наличии ресурсов в облаке заказывались узлы большего размера, а при их исчерпании — узлы меньшего размера.

Пример создания двух `NodeGroup` с использованием узлов типа spot-node:

```yaml
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker-spot
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-spot
    maxPerZone: 5
    minPerZone: 0
    priority: 50
  nodeType: CloudEphemeral
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker
    maxPerZone: 5
    minPerZone: 0
    priority: 30
  nodeType: CloudEphemeral
```

В приведенном выше примере `cluster-autoscaler` сначала попытается заказать узел типа spot-node. Если в течение 15 минут его не получится добавить в кластер, NodeGroup `worker-spot` будет поставлена *на паузу* (на 20 минут) и `cluster-autoscaler` начнет заказывать узлы из NodeGroup `worker`.
Если через 30 минут в кластере возникнет необходимость развернуть еще один узел, `cluster-autoscaler` сначала попытается заказать узел из NodeGroup `worker-spot` и только потом — из NodeGroup `worker`.

После того как NodeGroup `worker-spot` достигнет своего максимума (5 узлов в примере выше), узлы будут заказываться из NodeGroup `worker`.

Шаблоны узлов (labels/taints) для NodeGroup `worker` и `worker-spot` должны быть одинаковыми или как минимум подходить для той нагрузки, которая запускает процесс увеличения кластера.

## Как интерпретировать состояние группы узлов?

**Ready** — группа узлов содержит минимально необходимое число запланированных узлов с состоянием ```Ready``` для всех зон.

Пример 1. Группа узлов в состоянии ```Ready```:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Пример 2. Группа узлов в состоянии ```Not Ready```:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating** — группа узлов содержит как минимум один узел, в котором присутствует аннотация с префиксом ```update.node.deckhouse.io``` (например, ```update.node.deckhouse.io/waiting-for-approval```).

**WaitingForDisruptiveApproval** — группа узлов содержит как минимум один узел, в котором присутствует аннотация ```update.node.deckhouse.io/disruption-required``` и
отсутствует аннотация ```update.node.deckhouse.io/disruption-approved```.

**Scaling** — рассчитывается только для групп узлов с типом ```CloudEphemeral```. Состояние ```True``` может быть в двух случаях:

1. Когда число узлов меньше *желаемого числа узлов в группе, то есть когда нужно увеличить число узлов в группе*.
1. Когда какой-то узел помечается к удалению или число узлов больше *желаемого числа узлов*, то есть когда нужно уменьшить число узлов в группе.

*Желаемое число узлов* — это сумма всех реплик, входящих в группу узлов.

Пример. Желаемое число узлов равно 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error** — содержит последнюю ошибку, возникшую при создании узла в группе узлов.

## Как заставить werf игнорировать состояние Ready в группе узлов?

[werf](https://ru.werf.io) проверяет состояние ```Ready``` у ресурсов и в случае его наличия дожидается, пока значение станет ```True```.

Создание (обновление) ресурса [nodeGroup](cr.html#nodegroup) в кластере может потребовать значительного времени на развертывание необходимого количества узлов. При развертывании такого ресурса в кластере с помощью werf (например, в рамках процесса CI/CD) развертывание может завершиться по превышении времени ожидания готовности ресурса. Чтобы заставить werf игнорировать состояние `nodeGroup`, необходимо добавить к `nodeGroup` следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

## Что такое ресурс Instance?

Ресурс Instance содержит описание объекта эфемерной машины, без учета ее конкретной реализации. Например, машины созданные MachineControllerManager или Cluster API Provider Static будут иметь соответcтвующий ресурс Instance.

Объект не содержит спецификации. Статус содержит:
1. Ссылку на InstanceClass, если он существует для данной реализации.
1. Ссылку на объект Node Kubernetes.
1. Текущий статус машины.
1. Информацию о том, как посмотреть [логи создания машины](#как-посмотреть-что-в-данный-момент-выполняется-на-узле-при-его-создании) (появляется на этапе создания машины).

При создании/удалении машины создается/удаляется соответствующий объект Instance.
Самостоятельно ресурс Instance создать нельзя, но можно удалить. В таком случае машина будет удалена из кластера (процесс удаления зависит от деталей реализации).

{% endraw %}

## Когда требуется перезагрузка узлов?

Некоторые операции по изменению конфигурации узлов могут потребовать перезагрузки.

Перезагрузка узла может потребоваться при изменении некоторых настроек sysctl, например при изменении параметра `kernel.yama.ptrace_scope` (изменяется при использовании команды `astra-ptrace-lock enable/disable` в Astra Linux).
