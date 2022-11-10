---
title: "Управление узлами: FAQ"
search: добавить ноду в кластер, добавить узел в кластер, настроить узел с GPU, эфемерные узлы
---
{% raw %}

## Как добавить статичный узел в кластер?

Чтобы добавить новый статичный узел (выделенная ВМ или железный сервер, например) в кластер, необходимо:

1. Использовать существующую или создать новую `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `CloudStatic`). Например, создадим [`NodeGroup` с именем `worker`](examples.html#пример-описания-статичной-nodegroup).
2. Получить скрипт для установки и настройки узла:

   ```shell
   kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."bootstrap.sh"' -r
   ```

3. Перед настройкой Kubernetes на узле убедиться, что выполнены все необходимые действия для корректной работы узла в кластере:
   - В `/etc/fstab` добавлены все необходимые точки монтирования (NFS, Ceph, ...);
   - На узел установлен пакет `ceph-common` нужной версии или другие необходимые пакеты;
   - Между узлами кластера настроена сетевая связанность.
4. Зайти на новый узел по SSH и выполнить команду из Secret'а: `echo <base64> | base64 -d | bash`.

## Как добавить несколько статических узлов в кластер?

Если у вас в кластере не созданы `NodeGroup`, то вы можете ознакомиться с информацией, как это сделать в [этом разделе](#как-добавить-статичный-узел-в-кластер).
Если у вас уже созданы `NodeGroup`, то вы можете автоматизировать процесс добавления узлов с помощью любой предпочитаемой платформы автоматизации. Для примера мы будем использовать Ansible.

1. Получите один из адресов Kubernetes API-сервера. Обратите внимание, что IP-адрес должен быть доступен с узлов, которые добавляются в кластер:

   ```shell
   kubectl get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

2. Получите Kubernetes API-токен для специального `ServiceAccount`, которым управляет Deckhouse:

   ```shell
   kubectl -n d8-cloud-instance-manager get $(kubectl -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
     -o json | jq '.data.token' -r | base64 -d && echo ""
   ```

3. Создайте Ansible playbook с `vars`, которые заменены на полученные на предыдущих шагах значения:

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
         ignore_errors: yes
         when: bootstrapped.stat.exists == False
       - name: wait
         wait_for_connection:
           delay: 30
         when: bootstrapped.stat.exists == False
   ```

4. Вам также необходимо определить дополнительную переменную `node_group`. Значение переменной должно совпадать с именем `NodeGroup`, которой будет принадлежать узел. Переменную можно передать разными способами, ниже пример с использованием файла инвентаря.

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

5. Теперь вы можете выполнить playbook с использованием файла инвентаря.

## Как завести существующий узел кластера под управление node-manager?

Чтобы завести существующий узел кластера под управление `node-manager`, необходимо:

1. Использовать существующую или создать новую `NodeGroup` с необходимыми параметрами (`nodeType` может быть `Static` или `CloudStatic`). Например, создадим [`NodeGroup` с именем `worker`](examples.html#пример-описания-статичной-nodegroup).
2. Получить скрипт для установки и настройки узла: `kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-worker -o json | jq '.data."adopt.sh"' -r`.
3. Зайти на новый узел по SSH и выполнить команду из Secret'а: `echo <base64> | base64 -d | bash`.

## Как изменить NodeGroup у статичного узла?

Чтобы перенести существующий статичный узел из одной NodeGroup в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Изменения не будут применены мгновенно. Обновлением состояния объектов NodeGroup занимается один из хуков Deckhouse, который подписывается на изменения узлов.

## Как вывести узел из-под управления node-manager?

Чтобы вывести узел из-под управления `node-manager`, необходимо:

1. Остановить сервис и таймер bashible: `systemctl stop bashible.timer bashible.service`.
2. Удалить скрипты bashible: `rm -rf /var/lib/bashible`.
3. Удалить с узла аннотации и лейблы:

   ```shell
   kubectl annotate node <node_name> node.deckhouse.io/configuration-checksum- update.node.deckhouse.io/waiting-for-approval- update.node.deckhouse.io/disruption-approved- update.node.deckhouse.io/disruption-required- update.node.deckhouse.io/approved- update.node.deckhouse.io/draining- update.node.deckhouse.io/drained-
   kubectl label node <node_name> node.deckhouse.io/group-
   ```

## Как зачистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить NodeGroup, следуйте [этой инструкции](#как-изменить-nodegroup-у-статичного-узла).

1. Удалите узел из кластера Kubernetes:

   ```shell
   kubectl drain <node> --ignore-daemonsets --delete-local-data
   kubectl delete node <node>
   ```

1. Остановите все сервисы и запущенные контейнеры:

   ```shell
   systemctl stop kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl stop bashible.service bashible.timer
   systemctl stop kubelet.service
   systemctl stop containerd
   systemctl list-units --full --all | grep -q docker.service && systemctl stop docker
   kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
   ```

1. Удалите точки монтирования:

   ```shell
   for i in $(mount -t tmpfs | grep /var/lib/kubelet | cut -d " " -f3); do umount $i ; done
   ```

1. Удалите директории и файлы:

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

1. Удалите интерфейсы:

   ```shell
   ifconfig cni0 down
   ifconfig flannel.1 down
   ifconfig docker0 down
   ip link delete cni0
   ip link delete flannel.1
   ```

1. Очистите systemd:

   ```shell
   systemctl daemon-reload
   systemctl reset-failed
   ```

1. Перезагрузите узел.

1. Запустите обратно CRI:

   ```shell
   systemctl start containerd
   systemctl list-units --full --all | grep -q docker.service && systemctl start docker
   ```

1. [Запустите](#как-добавить-статичный-узел-в-кластер) скрипт `bootstrap.sh`.
1. Включите все сервисы обратно:

   ```shell
   systemctl start kubelet.service
   systemctl start kubernetes-api-proxy.service kubernetes-api-proxy-configurator.service kubernetes-api-proxy-configurator.timer
   systemctl start bashible.service bashible.timer
   ```

## Как понять, что что-то пошло не так?

Для этого необходимо посмотреть логи сервиса `bashible`, который модуль `node-manager` создает на каждом узле.

Посмотреть логи сервиса `bashible` можно командой:

```shell
journalctl -fu bashible
```

## Как посмотреть, что в данный момент выполняется на узле при его создании?

Если необходимо узнать, что происходит на узле (к примеру он долго создается), то можно посмотреть логи `cloud-init`. Для этого необходимо:
1. Найти узел, который сейчас бутстрапится: `kubectl -n d8-cloud-instance-manager get machine | grep Pending`;
1. Посмотреть информацию о `machine`: `kubectl -n d8-cloud-instance-manager describe machine kube-2-worker-01f438cf-757f758c4b-r2nx2`. Пример результата:

   ```shell
   Status:
     Bootstrap Status:
       Description:   Use 'nc 192.168.199.115 8000' to get bootstrap logs.
       Tcp Endpoint:  192.168.199.115
   ```

1. Выполнить команду `nc 192.168.199.115 8000`, чтобы увидеть логи `cloud-init` и на чем зависла настройка узла.

Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.

## Как настроить узел с GPU?

Если у вас есть узел с GPU и вы хотите настроить Docker для работы с `node-manager`, то вам необходимо выполнить все настройки на узле [по документации](https://github.com/NVIDIA/k8s-device-plugin#quick-start).

Создать `NodeGroup` с такими параметрами:

```yaml
cri:
  type: NotManaged
operatingSystem:
  manageKernel: false
```

После чего добавить узел под управление `node-manager`.

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

Подробно о всех параметрах можно прочитать в описании custom resource [NodeGroup](cr.html#nodegroup).

В случае изменения параметра `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит.

При disruption update выполняется evict Pod'ов с узла. Если какие-либо Pod'ы не удалось evict'нуть, evict повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После этого Pod'ы, которые не удалось evict'нуть, удаляются.

## Как пересоздать эфемерные машины в облаке с новой конфигурацией?

При изменении конфигурации Deckhouse (как в модуле node-manager, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Пересоздание происходит только после изменения ресурсов `InstanceClass` или `NodeGroup`.

Для того чтобы принудительно пересоздать все узлы, связанные с ресурсом `Machines`, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки?

> **Внимание!** Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов **Deckhouse**. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.

Для решений данной задачи существуют два механизма:

1. Установка меток в `NodeGroup` `spec.nodeTemplate.labels` для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
2. Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

> Deckhouse по умолчанию tolerate'ит ключ `dedicated`, поэтому рекомендуется использовать ключ `dedicated` с любым `value` для taint'ов на ваших выделенных узлах.️
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
1. При наличии дополнительных служебных компонентов (не обслуживаемых Deckhouse, например, DaemonSet `filebeat`) для этих узлов — задать их суммарное потребление ресурсов в параметре `standbyHolder.notHeldResources`.
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

> **Внимание!** Использовать эту настройку допустимо только тогда, когда вы четко понимаете зачем это необходимо.

Установить параметр:

```yaml
mcmEmergencyBrake: true
```

## Как восстановить master-узел, если kubelet не может загрузить компоненты control plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы
компонентов control plane (например, удалена директория `/var/lib/docker` при использовании Docker или `/var/lib/containerd` при использовании containerd). В этом случае kubelet при рестарте не сможет скачать образы control plane компонентов, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Ниже инструкция по восстановлению master-узла.

### Docker

Для восстановления работоспособности master-узла необходимо в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r 'del(.auths."registry.deckhouse.io".username, .auths."registry.deckhouse.io".password)'
```

Вывод команды нужно скопировать и добавить его в файл `/root/.docker/config.json` на поврежденном master-узле.
Далее на поврежденном master-узле нужно загрузить образы `control-plane` компонентов:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  docker pull $image
done
```

После загрузки образов необходимо перезапустить kubelet.
После восстановления работоспособности master-узла необходимо **убрать внесенные в файл `/root/.docker/config.json` изменения!**

### Containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать и присвоить переменной AUTH на поврежденном master-узле.
Далее на поврежденном master-узле нужно загрузить образы `control-plane` компонентов:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить kubelet.

## Как изменить CRI для NodeGroup?

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

Также эту операцию можно выполнить при помощи патча:

* Для Containerd:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Для Docker:

  ```shell
  kubectl patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Docker"}}}'
  ```

> **Внимание!** Нельзя устанавливать `cri.type` для NodeGroup, созданных при помощи `dhctl` (например, NodeGroup `master`).

После настройки нового CRI для NodeGroup модуль node-manager по одному drain'ит узлы и устанавливает на них новый CRI. Обновление узла
сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup модуль node-manager либо автоматически разрешает обновление
узлов, либо требует ручного подтверждения.

## Как изменить CRI для всего кластера?

> **Внимание!** Docker deprecated, возможен переход только с Docker на Containerd. Переход с Containerd на Docker запрещён.

Необходимо при помощи утилиты `dhctl` отредактировать параметр `defaultCRI` в конфиге `cluster-configuration`.

Также возможно выполнить эту операцию при помощи `kubectl patch`. Пример:
* Для Containerd

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Docker/Containerd/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

* Для Docker

  ```shell
  data="$(kubectl -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/Docker/" | base64 -w0)"
  kubectl -n kube-system patch secret d8-cluster-configuration -p "{\"data\":{\"cluster-configuration.yaml\":\"$data\"}}"
  ```

Если необходимо какую-то NodeGroup оставить на другом CRI, то перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [здесь](#как-изменить-cri-для-nodegroup).

> **Внимание!** Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
> Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера!
> Предпочтительный вариант — сделать multimaster и поменять тип CRI!

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Deckhouse обновляет узлы в master NodeGroup по одному, поэтому необходимо определить, какой узел на данный момент обновляется:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердить disruption для мастера, полученного на предыдущем шаге:

   ```shell
   kubectl annotate node <имя master узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождаться перехода обновленного master-узла в `Ready`. Выполнить итерацию для следующего мастера.

## Как добавить шаг для конфигурации узлов?

Дополнительные шаги для конфигурации узлов задаются при помощи custom resource `NodeGroupConfiguration`.

## Как использовать containerd с поддержкой Nvidia GPU?

Так как для использования Nvidia GPU требуется особая настройка containerd, необходимо создать NodeGroup с типом CRI `Unmanaged`.

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: gpu
spec:
  chaos:
    mode: Disabled
  cri:
    type: NotManaged
  disruptions:
    approvalMode: Automatic
  nodeType: CloudStatic
```

### Debian

Debian-based дистрибутивы содержат пакеты с драйверами Nvidia в базовом репозитории, поэтому нет необходимости подготавливать специальный образ с установленными драйверами.

Разверните скрипты `NodeGroupConfiguration`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-containerd.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 31
  content: |
    # Copyright 2021 Flant JSC
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
        
    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      systemctl daemon-reload
      systemctl enable containerd.service
      systemctl restart containerd.service
    }
        
    # set default
    desired_version={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | quote }}
    allowed_versions_pattern={{ index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "allowedPattern" | quote }}
    
    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "debian" }}
      {{- $debianVersion := toString $key }}
      {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
    if bb-is-debian-version? {{ $debianVersion }} ; then
      desired_version={{ $value.containerd.desiredVersion | quote }}
      allowed_versions_pattern={{ $value.containerd.allowedPattern | quote }}
    fi
      {{- end }}
    {{- end }}
    
    if [[ -z $desired_version ]]; then
      bb-log-error "Desired version must be set"
      exit 1
    fi
    
    should_install_containerd=true
    version_in_use="$(dpkg -l containerd.io 2>/dev/null | grep -E "(hi|ii)\s+(containerd.io)" | awk '{print $2"="$3}' || true)"
    if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
      should_install_containerd=false
    fi
    
    if [[ "$version_in_use" == "$desired_version" ]]; then
      should_install_containerd=false
    fi
    
    if [[ "$should_install_containerd" == true ]]; then
      # set default
      containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%sStretch" (index .k8s .kubernetesVersion "bashible" "debian" "9" "containerd" "desiredVersion" | replace "containerd.io=" "" | replace "." "" | replace "-" "")) }}"
    
    {{- $debianName := dict "9" "Stretch" "10" "Buster" "11" "Bullseye" }}
    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "debian" }}
      {{- $debianVersion := toString $key }}
      if bb-is-debian-version? {{ $debianVersion }} ; then
        containerd_tag="{{- index $.images.registrypackages (printf "containerdDebian%s%s" ($value.containerd.desiredVersion | replace "containerd.io=" "" | replace "." "" | replace "-" "") (index $debianName $debianVersion)) }}"
      fi
    {{- end }}
    
      crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"
    
      bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}"
    fi
    
    # Upgrade containerd-flant-edition if needed
    containerd_fe_tag="{{ index .images.registrypackages "containerdFe1511" | toString }}"
    if ! bb-rp-is-installed? "containerd-flant-edition" "${containerd_fe_tag}" ; then
      systemctl stop containerd.service
      bb-rp-install "containerd-flant-edition:${containerd_fe_tag}"
    
      mkdir -p /etc/systemd/system/containerd.service.d
      bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
    [Service]
    ExecStart=
    ExecStart=-/usr/local/bin/containerd
    EOF
    fi
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-and-start-containerd.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 50
  content: |
    # Copyright 2021 Flant JSC
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
    
    bb-event-on 'bb-sync-file-changed' '_on_containerd_config_changed'
    _on_containerd_config_changed() {
      systemctl restart containerd.service
    }
    
      {{- $max_concurrent_downloads := 3 }}
      {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
      {{- if .images }}
        {{- if .images.common.pause }}
          {{- $sandbox_image = printf "%s%s:%s" .registry.address .registry.path .images.common.pause }}
        {{- end }}
      {{- end }}
    
    systemd_cgroup=true
    # Overriding cgroup type from external config file
    if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
      systemd_cgroup=false
    fi
    
    # generated using `containerd config default` by containerd version `containerd containerd.io 1.4.3 269548fa27e0089a8b8278fc4fc781d7f65a939b`
    bb-sync-file /etc/containerd/config.toml - << EOF
    version = 2
    root = "/var/lib/containerd"
    state = "/run/containerd"
    plugin_dir = ""
    disabled_plugins = []
    required_plugins = []
    oom_score = 0
    [grpc]
      address = "/run/containerd/containerd.sock"
      tcp_address = ""
      tcp_tls_cert = ""
      tcp_tls_key = ""
      uid = 0
      gid = 0
      max_recv_message_size = 16777216
      max_send_message_size = 16777216
    [ttrpc]
      address = ""
      uid = 0
      gid = 0
    [debug]
      address = ""
      uid = 0
      gid = 0
      level = ""
    [metrics]
      address = ""
      grpc_histogram = false
    [cgroup]
      path = ""
    [timeouts]
      "io.containerd.timeout.shim.cleanup" = "5s"
      "io.containerd.timeout.shim.load" = "5s"
      "io.containerd.timeout.shim.shutdown" = "3s"
      "io.containerd.timeout.task.state" = "2s"
    [plugins]
      [plugins."io.containerd.gc.v1.scheduler"]
        pause_threshold = 0.02
        deletion_threshold = 0
        mutation_threshold = 100
        schedule_delay = "0s"
        startup_delay = "100ms"
      [plugins."io.containerd.grpc.v1.cri"]
        disable_tcp_service = true
        stream_server_address = "127.0.0.1"
        stream_server_port = "0"
        stream_idle_timeout = "4h0m0s"
        enable_selinux = false
        selinux_category_range = 1024
        sandbox_image = {{ $sandbox_image | quote }}
        stats_collect_period = 10
        systemd_cgroup = false
        enable_tls_streaming = false
        max_container_log_line_size = 16384
        disable_cgroup = false
        disable_apparmor = false
        restrict_oom_score_adj = false
        max_concurrent_downloads = {{ $max_concurrent_downloads }}
        disable_proc_mount = false
        unset_seccomp_profile = ""
        tolerate_missing_hugetlb_controller = true
        disable_hugetlb_controller = true
        ignore_image_defined_volumes = false
        [plugins."io.containerd.grpc.v1.cri".containerd]
          snapshotter = "overlayfs"
          default_runtime_name = "nvidia"
          no_pivot = false
          disable_snapshot_annotations = true
          discard_unpacked_layers = false
          [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              runtime_type = "io.containerd.runc.v2"
              runtime_engine = ""
              runtime_root = ""
              privileged_without_host_devices = false
              base_runtime_spec = ""
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
                SystemdCgroup = ${systemd_cgroup}
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = ${systemd_cgroup}
        [plugins."io.containerd.grpc.v1.cri".cni]
          bin_dir = "/opt/cni/bin"
          conf_dir = "/etc/cni/net.d"
          max_conf_num = 1
          conf_template = ""
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .registry.address }}"]
              endpoint = ["{{ .registry.scheme }}://{{ .registry.address }}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".auth]
              auth = "{{ .registry.auth | default "" }}"
      {{- if eq .registry.scheme "http" }}
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".tls]
              insecure_skip_verify = true
      {{- end }}
        [plugins."io.containerd.grpc.v1.cri".image_decryption]
          key_model = ""
        [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
          tls_cert_file = ""
          tls_key_file = ""
      [plugins."io.containerd.internal.v1.opt"]
        path = "/opt/containerd"
      [plugins."io.containerd.internal.v1.restart"]
        interval = "10s"
      [plugins."io.containerd.metadata.v1.bolt"]
        content_sharing_policy = "shared"
      [plugins."io.containerd.monitor.v1.cgroups"]
        no_prometheus = false
      [plugins."io.containerd.runtime.v1.linux"]
        shim = "containerd-shim"
        runtime = "runc"
        runtime_root = ""
        no_shim = false
        shim_debug = false
      [plugins."io.containerd.runtime.v2.task"]
        platforms = ["linux/amd64"]
      [plugins."io.containerd.service.v1.diff-service"]
        default = ["walking"]
      [plugins."io.containerd.snapshotter.v1.devmapper"]
        root_path = ""
        pool_name = ""
        base_image_size = ""
        async_remove = false
    EOF
    
    bb-sync-file /etc/crictl.yaml - << "EOF"
    runtime-endpoint: unix:/var/run/containerd/containerd.sock
    image-endpoint: unix:/var/run/containerd/containerd.sock
    timeout: 2
    debug: false
    pull-image-on-create: false
    EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - 'debian'
  nodeGroups:
  - 'gpu'
  weight: 30
  content: |
    # Copyright 2021 Flant JSC
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

    distribution="debian9"
    curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey -o - | apt-key add -
    curl -s -L https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.list -o /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get update
    apt-get install -y nvidia-container-toolkit nvidia-driver-470
```

Для других версий Debian исправьте значение переменной `distribution` в скрипте и версию пакета драйверов Nvidia (в примере выше — `nvidia-driver-470`).

### CentOS

CentOS-based дистрибутивы не содержат драйверы Nvidia в базовых репозиториях.

Установку драйверов Nvidia в CentOS-based дистрибутивах трудно автоматизировать, поэтому желательно иметь подготовленный образ с установленными драйверами.
Как установить драйвера Nvidia написано в [инструкции](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/index.html#redhat-installation).

Разверните скрипты `NodeGroupConfiguration`:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-containerd.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 31
  content: |
    # Copyright 2021 Flant JSC
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
    
    bb-event-on 'bb-package-installed' 'post-install'
    post-install() {
      systemctl daemon-reload
      systemctl enable containerd.service
      systemctl restart containerd.service
    }
        
    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
      {{- $centosVersion := toString $key }}
      {{- if or $value.containerd.desiredVersion $value.containerd.allowedPattern }}
    if bb-is-centos-version? {{ $centosVersion }} ; then
      desired_version={{ $value.containerd.desiredVersion | quote }}
      allowed_versions_pattern={{ $value.containerd.allowedPattern | quote }}
    fi
      {{- end }}
    {{- end }}
    
    if [[ -z $desired_version ]]; then
      bb-log-error "Desired version must be set"
      exit 1
    fi
    
    should_install_containerd=true
    version_in_use="$(rpm -q containerd.io | head -1 || true)"
    if test -n "$allowed_versions_pattern" && test -n "$version_in_use" && grep -Eq "$allowed_versions_pattern" <<< "$version_in_use"; then
      should_install_containerd=false
    fi
    
    if [[ "$version_in_use" == "$desired_version" ]]; then
      should_install_containerd=false
    fi
    
    if [[ "$should_install_containerd" == true ]]; then
    
    {{- range $key, $value := index .k8s .kubernetesVersion "bashible" "centos" }}
      {{- $centosVersion := toString $key }}
      if bb-is-centos-version? {{ $centosVersion }} ; then
        containerd_tag="{{- index $.images.registrypackages (printf "containerdCentos%s" ($value.containerd.desiredVersion | replace "containerd.io-" "" | replace "." "_" | replace "-" "_" | camelcase )) }}"
      fi
    {{- end }}
    
      crictl_tag="{{ index .images.registrypackages (printf "crictl%s" (.kubernetesVersion | replace "." "")) | toString }}"
    
      bb-rp-install "containerd-io:${containerd_tag}" "crictl:${crictl_tag}"
    fi
    
    # Upgrade containerd-flant-edition if needed
    containerd_fe_tag="{{ index .images.registrypackages "containerdFe1511" | toString }}"
    if ! bb-rp-is-installed? "containerd-flant-edition" "${containerd_fe_tag}" ; then
      systemctl stop containerd.service
      bb-rp-install "containerd-flant-edition:${containerd_fe_tag}"
    
      mkdir -p /etc/systemd/system/containerd.service.d
      bb-sync-file /etc/systemd/system/containerd.service.d/override.conf - << EOF
    [Service]
    ExecStart=
    ExecStart=-/usr/local/bin/containerd
    EOF
    fi
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: configure-and-start-containerd.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 50
  content: |
    # Copyright 2021 Flant JSC
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
    
    bb-event-on 'bb-sync-file-changed' '_on_containerd_config_changed'
    _on_containerd_config_changed() {
      systemctl restart containerd.service
    }
    
      {{- $max_concurrent_downloads := 3 }}
      {{- $sandbox_image := "registry.k8s.io/pause:3.2" }}
      {{- if .images }}
        {{- if .images.common.pause }}
          {{- $sandbox_image = printf "%s%s:%s" .registry.address .registry.path .images.common.pause }}
        {{- end }}
      {{- end }}
    
    systemd_cgroup=true
    # Overriding cgroup type from external config file
    if [ -f /var/lib/bashible/cgroup_config ] && [ "$(cat /var/lib/bashible/cgroup_config)" == "cgroupfs" ]; then
      systemd_cgroup=false
    fi
    
    # generated using `containerd config default` by containerd version `containerd containerd.io 1.4.3 269548fa27e0089a8b8278fc4fc781d7f65a939b`
    bb-sync-file /etc/containerd/config.toml - << EOF
    version = 2
    root = "/var/lib/containerd"
    state = "/run/containerd"
    plugin_dir = ""
    disabled_plugins = []
    required_plugins = []
    oom_score = 0
    [grpc]
      address = "/run/containerd/containerd.sock"
      tcp_address = ""
      tcp_tls_cert = ""
      tcp_tls_key = ""
      uid = 0
      gid = 0
      max_recv_message_size = 16777216
      max_send_message_size = 16777216
    [ttrpc]
      address = ""
      uid = 0
      gid = 0
    [debug]
      address = ""
      uid = 0
      gid = 0
      level = ""
    [metrics]
      address = ""
      grpc_histogram = false
    [cgroup]
      path = ""
    [timeouts]
      "io.containerd.timeout.shim.cleanup" = "5s"
      "io.containerd.timeout.shim.load" = "5s"
      "io.containerd.timeout.shim.shutdown" = "3s"
      "io.containerd.timeout.task.state" = "2s"
    [plugins]
      [plugins."io.containerd.gc.v1.scheduler"]
        pause_threshold = 0.02
        deletion_threshold = 0
        mutation_threshold = 100
        schedule_delay = "0s"
        startup_delay = "100ms"
      [plugins."io.containerd.grpc.v1.cri"]
        disable_tcp_service = true
        stream_server_address = "127.0.0.1"
        stream_server_port = "0"
        stream_idle_timeout = "4h0m0s"
        enable_selinux = false
        selinux_category_range = 1024
        sandbox_image = {{ $sandbox_image | quote }}
        stats_collect_period = 10
        systemd_cgroup = false
        enable_tls_streaming = false
        max_container_log_line_size = 16384
        disable_cgroup = false
        disable_apparmor = false
        restrict_oom_score_adj = false
        max_concurrent_downloads = {{ $max_concurrent_downloads }}
        disable_proc_mount = false
        unset_seccomp_profile = ""
        tolerate_missing_hugetlb_controller = true
        disable_hugetlb_controller = true
        ignore_image_defined_volumes = false
        [plugins."io.containerd.grpc.v1.cri".containerd]
          snapshotter = "overlayfs"
          default_runtime_name = "nvidia"
          no_pivot = false
          disable_snapshot_annotations = true
          discard_unpacked_layers = false
          [plugins."io.containerd.grpc.v1.cri".containerd.default_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.untrusted_workload_runtime]
            runtime_type = ""
            runtime_engine = ""
            runtime_root = ""
            privileged_without_host_devices = false
            base_runtime_spec = ""
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
            [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
              runtime_type = "io.containerd.runc.v2"
              runtime_engine = ""
              runtime_root = ""
              privileged_without_host_devices = false
              base_runtime_spec = ""
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
                SystemdCgroup = ${systemd_cgroup}
              [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia]
                privileged_without_host_devices = false
                runtime_engine = ""
                runtime_root = ""
                runtime_type = "io.containerd.runc.v1"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = ${systemd_cgroup}
        [plugins."io.containerd.grpc.v1.cri".cni]
          bin_dir = "/opt/cni/bin"
          conf_dir = "/etc/cni/net.d"
          max_conf_num = 1
          conf_template = ""
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
              endpoint = ["https://registry-1.docker.io"]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."{{ .registry.address }}"]
              endpoint = ["{{ .registry.scheme }}://{{ .registry.address }}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".auth]
              auth = "{{ .registry.auth | default "" }}"
      {{- if eq .registry.scheme "http" }}
            [plugins."io.containerd.grpc.v1.cri".registry.configs."{{ .registry.address }}".tls]
              insecure_skip_verify = true
      {{- end }}
        [plugins."io.containerd.grpc.v1.cri".image_decryption]
          key_model = ""
        [plugins."io.containerd.grpc.v1.cri".x509_key_pair_streaming]
          tls_cert_file = ""
          tls_key_file = ""
      [plugins."io.containerd.internal.v1.opt"]
        path = "/opt/containerd"
      [plugins."io.containerd.internal.v1.restart"]
        interval = "10s"
      [plugins."io.containerd.metadata.v1.bolt"]
        content_sharing_policy = "shared"
      [plugins."io.containerd.monitor.v1.cgroups"]
        no_prometheus = false
      [plugins."io.containerd.runtime.v1.linux"]
        shim = "containerd-shim"
        runtime = "runc"
        runtime_root = ""
        no_shim = false
        shim_debug = false
      [plugins."io.containerd.runtime.v2.task"]
        platforms = ["linux/amd64"]
      [plugins."io.containerd.service.v1.diff-service"]
        default = ["walking"]
      [plugins."io.containerd.snapshotter.v1.devmapper"]
        root_path = ""
        pool_name = ""
        base_image_size = ""
        async_remove = false
    EOF
    
    bb-sync-file /etc/crictl.yaml - << "EOF"
    runtime-endpoint: unix:/var/run/containerd/containerd.sock
    image-endpoint: unix:/var/run/containerd/containerd.sock
    timeout: 2
    debug: false
    pull-image-on-create: false
    EOF
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: install-cuda.sh
spec:
  bundles:
  - 'centos'
  nodeGroups:
  - 'gpu'
  weight: 30
  content: |
    # Copyright 2021 Flant JSC
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

    distribution="centos7"
    curl -s -L https://nvidia.github.io/libnvidia-container/${distribution}/libnvidia-container.repo -o /etc/yum.repos.d/nvidia-container-toolkit.repo
    yum install -y nvidia-container-toolkit
```

### Как проверить что все прошло успешно?

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
          image: docker.io/nvidia/cuda:11.0-base
          imagePullPolicy: "IfNotPresent"
          command:
            - nvidia-smi
```

И посмотрите логи:

```shell
$ kubectl logs job/nvidia-cuda-test
Fri May  6 07:45:37 2022       
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 470.57.02    Driver Version: 470.57.02    CUDA Version: 11.4     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|                               |                      |               MIG M. |
|===============================+======================+======================|
|   0  Tesla V100-SXM2...  Off  | 00000000:8B:00.0 Off |                    0 |
| N/A   32C    P0    22W / 300W |      0MiB / 32510MiB |      0%      Default |
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

{% endraw %}
