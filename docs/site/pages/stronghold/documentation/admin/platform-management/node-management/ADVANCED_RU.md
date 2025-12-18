---
title: "Расширенные настройки"
permalink: ru/stronghold/documentation/admin/platform-management/node-management/advanced.html
lang: ru
---

{% raw %}

## Восстановление master-узла, если kubelet не может загрузить компоненты control plane

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы
компонентов control plane (например, удалена директория `/var/lib/containerd`). В этом случае kubelet при рестарте не сможет скачать образы компонентов `control plane`, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.ru`.

Ниже инструкция по восстановлению master-узла.

### containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
d8 k -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.ru".auth'
```

Вывод команды нужно скопировать и присвоить переменной AUTH на поврежденном master-узле.
Далее на поврежденном master-узле нужно загрузить образы компонентов `control-plane`:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить kubelet.

## Изменение CRI для NodeGroup

> **Внимание!** Возможен переход только с `Containerd` на `NotManaged` и обратно (параметр [cri.type](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type)).

Установить параметр [cri.type](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type) в `Containerd` или в `NotManaged`.

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
  d8 k patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"Containerd"}}}'
  ```

* Для `NotManaged`:

  ```shell
  d8 k patch nodegroup <имя NodeGroup> --type merge -p '{"spec":{"cri":{"type":"NotManaged"}}}'
  ```

> **Внимание!** При смене `cri.type` для NodeGroup, созданных с помощью `dhctl`, нужно менять ее в `dhctl config edit provider-cluster-configuration` и настройках объекта NodeGroup.

После настройки нового CRI для NodeGroup модуль `node-manager` по одному drain'ит узлы и устанавливает на них новый CRI. Обновление узла
сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup модуль `node-manager` либо автоматически разрешает обновление
узлов, либо требует ручного подтверждения.

## Изменение CRI для всего кластера

> **Внимание!** Возможен переход только с `Containerd` на `NotManaged` и обратно (параметр [cri.type](/modules/node-manager/cr.html#nodegroup-v1-spec-cri-type)).

Необходимо с помощью утилиты `dhctl` отредактировать параметр `defaultCRI` в конфиге `cluster-configuration`.

Также возможно выполнить эту операцию с помощью патча. Пример:
* Для `Containerd`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/NotManaged/Containerd/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p '{"data":{"cluster-configuration.yaml":"'${data}'"}}'
  ```

* Для `NotManaged`:

  ```shell
  data="$(d8 k -n kube-system get secret d8-cluster-configuration -o json | jq -r '.data."cluster-configuration.yaml"' | base64 -d | sed "s/Containerd/NotManaged/" | base64 -w0)"
  d8 k -n kube-system patch secret d8-cluster-configuration -p '{"data":{"cluster-configuration.yaml":"'${data}'"}}'
  ```

Если необходимо какую-то NodeGroup оставить на другом CRI, перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [здесь](#как-изменить-cri-для-nodegroup).

> **Внимание!** Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
> Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера!
> Предпочтительный вариант — сделать multimaster и поменять тип CRI!

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Deckhouse обновляет узлы в master NodeGroup по одному, поэтому необходимо определить, какой узел на данный момент обновляется:

   ```shell
   d8 k get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердить disruption для master-узла, полученного на предыдущем шаге:

   ```shell
   d8 k annotate node <имя master-узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождаться перехода обновленного master-узла в `Ready`. Выполнить итерацию для следующего master'а.

## Как использовать containerd с поддержкой Nvidia GPU?

Необходимо создать отдельную NodeGroup для GPU-узлов.

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
                runtime_type = "io.containerd.runc.v2"
                [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.nvidia.options]
                  BinaryName = "/usr/bin/nvidia-container-runtime"
                  SystemdCgroup = false
    EOF
  nodeGroups:
  - gpu
  weight: 31
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
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
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
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
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
$ d8 k logs job/nvidia-cuda-test
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
$ d8 k logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## Как добавить несколько статических узлов в кластер вручную?

Используйте существующий или создайте новый custom resource [NodeGroup](/modules/node-manager/cr.html#nodegroup) ([пример](examples.html#пример-описания-статической-nodegroup) NodeGroup с именем `worker`).

Автоматизировать процесс добавления узлов можно с помощью любой платформы автоматизации. Далее приведен пример для Ansible.

1. Получите один из адресов Kubernetes API-сервера. Обратите внимание, что IP-адрес должен быть доступен с узлов, которые добавляются в кластер:

   ```shell
   d8 k -n default get ep kubernetes -o json | jq '.subsets[0].addresses[0].ip + ":" + (.subsets[0].ports[0].port | tostring)' -r
   ```

   Проверьте версию K8s. Если версия >= 1.25, создайте токен `node-group`:

   ```shell
   d8 k create token node-group --namespace d8-cloud-instance-manager --duration 1h
   ```

   Сохраните полученный токен, и добавьте в поле `token:` playbook'а Ansible на дальнейших шагах.

1. Если версия Kubernetes меньше 1.25, получите Kubernetes API-токен для специального ServiceAccount'а, которым управляет Deckhouse:

   ```shell
   d8 k -n d8-cloud-instance-manager get $(d8 k -n d8-cloud-instance-manager get secret -o name | grep node-group-token) \
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

1. Определите дополнительную переменную `node_group`. Значение переменной должно совпадать с именем NodeGroup, которой будет принадлежать узел. Переменную можно передать различными способами, например с использованием inventory-файла:

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

## Как заставить werf игнорировать состояние Ready в группе узлов?

[werf](https://ru.werf.io) проверяет состояние `Ready` у ресурсов и в случае его наличия дожидается, пока значение станет `True`.

Создание (обновление) ресурса NodeGroup в кластере может потребовать значительного времени на развертывание необходимого количества узлов. При развертывании такого ресурса в кластере с помощью werf (например, в рамках процесса CI/CD) развертывание может завершиться по превышении времени ожидания готовности ресурса. Чтобы заставить werf игнорировать состояние `nodeGroup`, необходимо добавить к `nodeGroup` следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

{% endraw %}
