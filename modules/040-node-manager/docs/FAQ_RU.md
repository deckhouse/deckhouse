---
title: "Управление узлами: FAQ"
description: Управление узлами кластера Kubernetes. Добавление, удаление узлов в кластере. Изменение CRI узла.
search: добавить ноду в кластер, добавить узел в кластер, настроить узел с GPU, эфемерные узлы
---

<div id='как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master'></div>

## Как добавить master-узлы в облачном кластере?

Как конвертировать кластер с одним master-узлом в мультикластерный описано [в FAQ модуля control-plane-manager](../control-plane-manager/faq.html#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master).

<div id='как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master'></div>

## Как уменьшить число master-узлов в облачном кластере?

Как конвертировать мультимастерный кластер в кластер с одним master-узлом описано [в FAQ модуля control-plane-manager](../control-plane-manager/faq.html#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master).

## Статические узлы

<span id='как-добавить-статический-узел-в-кластер'></span>
<span id='как-добавить-статичный-узел-в-кластер'></span>

Добавить статический узел в кластер можно вручную ([пример](examples.html#вручную)) или с помощью [Cluster API Provider Static](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static).

### Как добавить статический узел в кластер (Cluster API Provider Static)?

Чтобы добавить статический узел в кластер (сервер bare-metal или виртуальную машину), выполните следующие шаги:

1. Подготовьте необходимые ресурсы:

   - Выделите сервер или виртуальную машину и убедитесь, что узел имеет необходимую сетевую связанность с кластером.

   - При необходимости установите дополнительные пакеты ОС и настройте точки монтирования, которые будут использоваться на узле.

1. Создайте пользователя с правами `sudo`:

   - Добавьте нового пользователя (в данном примере — `caps`) с правами выполнения команд через `sudo`:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   - Разрешите пользователю выполнять команды через `sudo` без ввода пароля. Для этого отредактируйте конфигурацию `sudo` (отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```shell
     caps ALL=(ALL) NOPASSWD: ALL
     ```

1. На сервере откройте файл `/etc/ssh/sshd_config` и убедитесь, что параметр `UsePAM` установлен в значение `yes`. Затем перезапустите службу `sshd`:

   ```shell
   sudo systemctl restart sshd
   ```

1. Сгенерируйте на сервере пару SSH-ключей с пустой парольной фразой:

   ```shell
   ssh-keygen -t rsa -f caps-id -C "" -N ""
   ```

   Приватный и публичный ключи будут сохранены в файлах `caps-id` и `caps-id.pub` соответственно в текущей директории.

1. Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами на сервере следующие команды:

   ```shell
   mkdir -p /home/caps/.ssh 
   cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
   chmod 700 /home/caps/.ssh 
   chmod 600 /home/caps/.ssh/authorized_keys
   chown -R caps:caps /home/caps/
   ```

1. Создайте ресурс [SSHCredentials](cr.html#sshcredentials).
1. Создайте ресурс [StaticInstance](cr.html#staticinstance).
1. Создайте ресурс [NodeGroup](cr.html#nodegroup) с [nodeType](cr.html#nodegroup-v1-spec-nodetype) `Static`, указав [желаемое количество узлов](cr.html#nodegroup-v1-spec-staticinstances-count) в группе и, при необходимости, [фильтр](cr.html#nodegroup-v1-spec-staticinstances-labelselector) выбора `StaticInstance`.

[Пример](examples.html#с-помощью-cluster-api-provider-static) добавления статического узла.

### Как добавить несколько статических узлов в кластер вручную?

Используйте существующий или создайте новый кастомный ресурс (Custom Resource) [NodeGroup](cr.html#nodegroup) ([пример](examples.html#пример-описания-статической-nodegroup) `NodeGroup` с именем `worker`).

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

{% raw %}

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

{% endraw %}

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

1. Запустите выполнение playbook'а с использованием inventory-файла.

### Как вручную очистить статический узел?

<span id='как-вывести-узел-из-под-управления-node-manager'></span>

{% alert level="info" %}
Инструкция справедлива как для узла, настроенного вручную (с помощью бутстрап-скрипта), так и для узла, настроенного с помощью CAPS.
{% endalert %}

Чтобы вывести из кластера узел и очистить сервер (ВМ), выполните следующую команду на узле:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

### Можно ли удалить StaticInstance?

`StaticInstance`, находящийся в состоянии `Pending` можно удалять без каких-либо проблем.

Чтобы удалить `StaticInstance` находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstrapping`), выполните следующие шаги:

1. Добавьте метку `"node.deckhouse.io/allow-bootstrap": "false"` в `StaticInstance`.
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

## Как очистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить `NodeGroup`, следуйте [этой инструкции](#как-изменить-nodegroup-у-статического-узла).

{% alert level="warning" %}
Если на зачищаемом узле есть пулы хранения LINSTOR/DRBD, то предварительно перенесите ресурсы с узла и удалите узел LINSTOR/DRBD, следуя [инструкции](/modules/sds-replicated-volume/stable/faq.html#как-выгнать-ресурсы-с-узла).
{% endalert %}

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

Если узел в NodeGroup не обновляется (значение `UPTODATE` при выполнении команды `kubectl get nodegroup` меньше значения `NODES`) или вы предполагаете какие-то другие проблемы, которые могут быть связаны с модулем `node-manager`, нужно проверить логи сервиса `bashible`. Сервис `bashible` запускается на каждом узле, управляемом модулем `node-manager`.

Чтобы проверить логи сервиса `bashible`, выполните на узле следующую команду:

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

Если необходимо узнать, что происходит на узле (например, узел долго создается), можно проверить логи `cloud-init`. Для этого выполните следующие шаги:

1. Найдите узел, который находится в стадии бутстрапа:

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

1. Выполните полученную команду (в примере выше — `nc 192.168.199.178 8000`), чтобы просмотреть логи `cloud-init` и определить, на каком этапе остановилась настройка узла.

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
    bb-dnf-install "kernel-${desired_version}"
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

Подробно о всех параметрах можно прочитать в описании кастомного ресурса [NodeGroup](cr.html#nodegroup).

В случае изменения параметров `InstanceClass` или `instancePrefix` в конфигурации Deckhouse не будет происходить `RollingUpdate`. Deckhouse создаст новые `MachineDeployment`, а старые удалит. Количество заказываемых одновременно `MachineDeployment` определяется параметром `cloudInstances.maxSurgePerZone`.

При обновлении, которое требует прерывания работы узла (disruption update), выполняется процесс вытеснения подов с узла. Если какой-либо под не может быть вытеснен, попытка повторяется каждые 20 секунд до достижения глобального таймаута в 5 минут. После истечения этого времени, поды, которые не удалось вытеснить, удаляются принудительно.

## Как пересоздать эфемерные машины в облаке с новой конфигурацией?

При изменении конфигурации Deckhouse (как в модуле `node-manager`, так и в любом из облачных провайдеров) виртуальные машины не будут перезаказаны. Пересоздание происходит только после изменения ресурсов `InstanceClass` или `NodeGroup`.

Чтобы принудительно пересоздать все узлы, связанные с ресурсом `Machines`, следует добавить/изменить аннотацию `manual-rollout-id` в `NodeGroup`: `kubectl annotate NodeGroup имя_ng "manual-rollout-id=$(uuidgen)" --overwrite`.

## Как выделить узлы под специфические нагрузки?

{% alert level="warning" %}
Запрещено использование домена `deckhouse.io` в ключах `labels` и `taints` у `NodeGroup`. Он зарезервирован для компонентов Deckhouse. Следует отдавать предпочтение в пользу ключей `dedicated` или `dedicated.client.com`.
{% endalert %}

Для решений данной задачи существуют два механизма:

1. Установка меток в `NodeGroup` `spec.nodeTemplate.labels` для последующего использования их в `Pod` [spec.nodeSelector](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) или [spec.affinity.nodeAffinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity). Указывает, какие именно узлы будут выбраны планировщиком для запуска целевого приложения.
1. Установка ограничений в `NodeGroup` `spec.nodeTemplate.taints` с дальнейшим снятием их в `Pod` [spec.tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/). Запрещает исполнение не разрешенных явно приложений на этих узлах.

{% alert level="info" %}
Deckhouse по умолчанию поддерживает использование taint'а с ключом `dedicated`, поэтому рекомендуется применять этот ключ с любым значением для taints на ваших выделенных узлах.

Если требуется использовать другие ключи для taints (например, `dedicated.client.com`), необходимо добавить соответствующее значение ключа в параметр [modules.placement.customTolerationKeys](../../deckhouse-configure-global.html#parameters-modules-placement-customtolerationkeys). Это обеспечит разрешение системным компонентам, таким как `cni-flannel`, использовать эти узлы.
{% endalert %}

Подробности [в статье на Habr](https://habr.com/ru/company/flant/blog/432748/).

## Как выделить узлы под системные компоненты?

### Фронтенд

Для Ingress-контроллеров используйте `NodeGroup` со следующей конфигурацией:

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

Для компонентов подсистем Deckhouse параметр `NodeGroup` будет настроен с параметрами:

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

Самое действенное — держать в кластере некоторое количество предварительно подготовленных узлов, которые позволят новым репликам ваших приложений запускаться мгновенно. Очевидным минусом данного решения будут дополнительные расходы на содержание этих узлов.

Необходимые настройки целевой `NodeGroup` будут следующие:

1. Указать абсолютное количество предварительно подготовленных узлов (или процент от максимального количества узлов в этой группе) в параметре `cloudInstances.standby`.
1. При наличии на узлах дополнительных служебных компонентов, не обслуживаемых Deckhouse (например, DaemonSet `filebeat`), задать их процентное потребление ресурсов узла можно в параметре `standbyHolder.overprovisioningRate`.
1. Для работы этой функции требуется, чтобы как минимум один узел из группы уже был запущен в кластере. Иными словами, либо должна быть доступна одна реплика приложения, либо количество узлов для этой группы `cloudInstances.minPerZone` должно быть `1`.

Пример:

```yaml
cloudInstances:
  maxPerZone: 10
  minPerZone: 1
  standby: 10%
  standbyHolder:
    overprovisioningRate: 30%
```

## Как выключить machine-controller-manager/CAPI в случае выполнения потенциально деструктивных изменений в кластере?

{% alert level="danger" %}
Использовать эту настройку допустимо только тогда, когда вы четко понимаете, зачем это необходимо.
{% endalert %}

Для того чтобы временно отключить machine-controller-manager (MCM) и предотвратить его автоматические действия, которые могут повлиять на инфраструктуру кластера (например, удаление или пересоздание узлов), установите следующий параметр в конфигурации:

```yaml
mcmEmergencyBrake: true
```

Для отключения CAPI установите следующий параметр в конфигурации:

```yaml
capiEmergencyBrake: true
```

## Как восстановить master-узел, если kubelet не может загрузить компоненты control plane?

Подобная ситуация может возникнуть, если в кластере с одним master-узлом на нем были удалены образы компонентов control plane (например, удалена директория `/var/lib/containerd`).
В этом случае kubelet при рестарте не сможет скачать образы компонентов `control plane`, поскольку на master-узле нет параметров авторизации в `registry.deckhouse.io`.

Далее приведена инструкция по восстановлению master-узла.

### containerd

Для восстановления работоспособности master-узла нужно в любом рабочем кластере под управлением Deckhouse выполнить команду:

```shell
kubectl -n d8-system get secrets deckhouse-registry -o json |
jq -r '.data.".dockerconfigjson"' | base64 -d |
jq -r '.auths."registry.deckhouse.io".auth'
```

Вывод команды нужно скопировать и присвоить переменной `AUTH` на поврежденном master-узле.

Далее на поврежденном master-узле нужно загрузить образы компонентов `control-plane`:

```shell
for image in $(grep "image:" /etc/kubernetes/manifests/* | awk '{print $3}'); do
  crictl pull --auth $AUTH $image
done
```

После загрузки образов необходимо перезапустить `kubelet`.

## Как изменить CRI для NodeGroup?

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для NodeGroup, установите параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type) в `Containerd` или в `NotManaged`.

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

{% alert level="warning" %}
 При изменении `cri.type` для NodeGroup, созданных с помощью `dhctl`, необходимо обновить это значение в `dhctl config edit provider-cluster-configuration` и настройках объекта NodeGroup.
{% endalert %}

После изменения CRI для NodeGroup модуль `node-manager` будет поочередно перезагружать узлы, применяя новый CRI.  Обновление узла сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup, модуль `node-manager` либо автоматически выполнит обновление узлов, либо потребует подтверждения вручную.

## Как изменить CRI для всего кластера?

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр [cri.type](cr.html#nodegroup-v1-spec-cri-type)).
{% endalert %}

Для изменения CRI для всего кластера, необходимо с помощью утилиты `dhctl` отредактировать параметр `defaultCRI` в конфигурационном файле `cluster-configuration`.

Также возможно выполнить эту операцию с помощью `kubectl patch`.

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

Если необходимо, чтобы отдельные NodeGroup использовали другой CRI, перед изменением `defaultCRI` необходимо установить CRI для этой NodeGroup,
как описано [в документации](#как-изменить-cri-для-nodegroup).

{% alert level="danger" %}
Изменение `defaultCRI` влечет за собой изменение CRI на всех узлах, включая master-узлы.
Если master-узел один, данная операция является опасной и может привести к полной неработоспособности кластера.
Рекомендуется использовать multimaster-конфигурацию и менять тип CRI только после этого.
{% endalert %}

При изменении CRI в кластере для master-узлов необходимо выполнить дополнительные шаги:

1. Чтобы определить, какой узел в текущий момент обновляется в master NodeGroup, используйте следующую команду:

   ```shell
   kubectl get nodes -l node-role.kubernetes.io/control-plane="" -o json | jq '.items[] | select(.metadata.annotations."update.node.deckhouse.io/approved"=="") | .metadata.name' -r
   ```

1. Подтвердите остановку (disruption) для master-узла, полученного на предыдущем шаге:

   ```shell
   kubectl annotate node <имя master-узла> update.node.deckhouse.io/disruption-approved=
   ```

1. Дождитесь перехода обновленного master-узла в `Ready`. Выполните итерацию для следующего master-узла.

## Как добавить шаг для конфигурации узлов?

Дополнительные шаги для конфигурации узлов задаются с помощью кастомного ресурса [NodeGroupConfiguration](cr.html#nodegroupconfiguration).

## Как автоматически проставить на узел кастомные лейблы?

1. На узле создайте каталог `/var/lib/node_labels`.

1. Создайте в нём файл или файлы, содержащие необходимые лейблы. Количество файлов может быть любым, как и вложенность подкаталогов, их содержащих.

1. Добавьте в файлы нужные лейблы в формате `key=value`. Например:

   ```console
   example-label=test
   ```

1. Сохраните файлы.

При добавлении узла в кластер указанные в файлах лейблы будут автоматически проставлены на узел.

{% alert level="warning" %}
Обратите внимание, что добавить таким образом лейблы, использующиеся в DKP, невозможно. Работать такой метод будет только с кастомными лейблами, не пересекающимися с зарезервированными для Deckhouse.
{% endalert %}

## Как использовать containerd с поддержкой Nvidia GPU?

Необходимо создать отдельную NodeGroup для GPU-узлов:

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

Далее создайте NodeGroupConfiguration для NodeGroup `gpu` для конфигурации containerd:

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

Добавьте NodeGroupConfiguration для установки драйверов Nvidia для NodeGroup `gpu`.

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
    bb-dnf-install nvidia-container-toolkit nvidia-driver
    nvidia-ctk config --set nvidia-container-runtime.log-level=error --in-place
  nodeGroups:
  - gpu
  weight: 30
```

После того как конфигурации будут применены, необходимо провести бутстрап и перезагрузить узлы, чтобы применить настройки и установить драйвера.

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

Проверьте логи командой:

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

Проверьте логи командой:

```shell
$ kubectl logs job/gpu-operator-test
[Vector addition of 50000 elements]
Copy input data from the host memory to the CUDA device
CUDA kernel launch with 196 blocks of 256 threads
Copy output data from the CUDA device to the host memory
Test PASSED
Done
```

## Как развернуть кастомный конфигурационный файл containerd?

{% alert level="info" %}
Пример `NodeGroupConfiguration` основан на функциях, заложенных в скрипте [032_configure_containerd.sh](./#особенности-написания-скриптов).
{% endalert %}

{% alert level="danger" %}
Добавление кастомных настроек вызывает перезапуск сервиса containerd.
{% endalert %}

Bashible на узлах объединяет конфигурацию containerd для Deckhouse с конфигурацией из файла `/etc/containerd/conf.d/*.toml`.

{% alert level="warning" %}
Вы можете переопределять значения параметров, которые заданы в файле `/etc/containerd/deckhouse.toml`, но их работу придётся обеспечивать самостоятельно. Также, лучше изменением конфигурации не затрагивать master-узлы (nodeGroup `master`).
{% endalert %}

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-option-config.sh
spec:
  bundles:
    - '*'
  content: |
    # Copyright 2024 Flant JSC
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
    bb-sync-file /etc/containerd/conf.d/additional_option.toml - << EOF
    oom_score = 500
    [metrics]
    address = "127.0.0.1"
    grpc_histogram = true
    EOF
  nodeGroups:
    - "worker"
  weight: 31
```

### Как добавить конфигурацию для дополнительного registry?

В containerd существует два способа описания конфигурации registry: **старый** и **новый**.

Для проверки наличия **старого** способа конфигурации выполните на узлах кластера следующие команды:

```bash
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.mirrors'
cat /etc/containerd/config.toml | grep 'plugins."io.containerd.grpc.v1.cri".registry.configs'

# Пример вывода:
# [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
#   [plugins."io.containerd.grpc.v1.cri".registry.mirrors."<REGISTRY_URL>"]
# [plugins."io.containerd.grpc.v1.cri".registry.configs]
#   [plugins."io.containerd.grpc.v1.cri".registry.configs."<REGISTRY_URL>".auth]
```

Для проверки наличия **нового** способа конфигурации выполните на узлах кластера следующую команду:

```bash
cat /etc/containerd/config.toml | grep '/etc/containerd/registry.d'

# Пример вывода:
# config_path = "/etc/containerd/registry.d"
```

#### Старый способ

{% alert level="warning" %}
Этот формат конфигурации containerd устарел (deprecated).
{% endalert %}

{% alert level="info" %}
Используется в containerd v1, если Deckhouse не управляется модулем Registry (режим [`Unmanaged`](/products/kubernetes-platform/documentation/v1/modules/deckhouse/configuration.html#parameters-registry)).
{% endalert %}

Конфигурация описывается в основном конфигурационном файле containerd `/etc/containerd/config.toml`.

Пользовательская конфигурация добавляется через механизм `toml merge`. Конфигурационные файлы из директории `/etc/containerd/conf.d` объединяются с основным файлом `/etc/containerd/config.toml`. Применение merge происходит на этапе выполнения скрипта `032_configure_containerd.sh`, поэтому соответствующие файлы должны быть добавлены заранее.

Пример конфигурационного файла для директории `/etc/containerd/conf.d/`:

```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
          endpoint = ["https://${REGISTRY_URL}"]
      [plugins."io.containerd.grpc.v1.cri".registry.configs]
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
          auth = "${BASE_64_AUTH}"
          username = "${USERNAME}"
          password = "${PASSWORD}"
        [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
          ca_file = "${CERT_DIR}/${CERT_NAME}.crt"
          insecure_skip_verify = true
```

{% alert level="danger" %}
Добавление кастомных настроек через механизм `toml merge` вызывает перезапуск сервиса containerd.
{% endalert %}

##### Как добавить авторизацию в дополнительный registry (старый способ)?

Пример добавления авторизации в дополнительный registry при использовании **старого** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
            [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
              endpoint = ["https://${REGISTRY_URL}"]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
              username = "username"
              password = "password"
              # OR
              auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
    EOF
```

##### Как настроить сертификат для дополнительного registry (старый способ)?

Пример настройки сертификата для дополнительного registry при использовании **старого** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example
    CERT_FILE_NAME=${REGISTRY_URL}
    CERTS_FOLDER="/var/lib/containerd/certs/"


    mkdir -p ${CERTS_FOLDER}
    bb-sync-file "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              ca_file = "${CERTS_FOLDER}/${CERT_FILE_NAME}.crt"
    EOF
```

{% alert level="info" %}
Помимо сontainerd, сертификат можно [добавить в операционную систему](examples.html#добавление-корневого-сертификата-в-хост).
{% endalert %}

##### Как добавить TLS skip verify (старый способ)?

Пример добавления TLS skip verify при использовании **старого** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # Для добавления файла перед шагом '032_configure_containerd.sh'
  weight: 31
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p /etc/containerd/conf.d
    bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
    [plugins]
      [plugins."io.containerd.grpc.v1.cri"]
        [plugins."io.containerd.grpc.v1.cri".registry]
          [plugins."io.containerd.grpc.v1.cri".registry.configs]
            [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".tls]
              insecure_skip_verify = true
    EOF
```

После применения конфигурационного файла проверьте доступ к registry с узлов, используя команду:

```bash
# Через cri-интерфейс
crictl pull private.registry.example/image/repo:tag
```

#### Новый способ

{% alert level="info" %}
Используется в containerd v2.  

Используется в containerd v1, если управление осуществляется через модуль Registry (например, в режиме [`Direct`](/products/kubernetes-platform/documentation/v1/modules/deckhouse/configuration.html#parameters-registry)).
{% endalert %}

Конфигурация описывается в каталоге `/etc/containerd/registry.d` и задаётся через создание подкаталогов с именами, соответствующими адресу registry:

```bash
/etc/containerd/registry.d
├── private.registry.example:5001
│   ├── ca.crt
│   └── hosts.toml
└── registry.deckhouse.ru
    ├── ca.crt
    └── hosts.toml
```

Пример содержимого файла `hosts.toml`:

```toml
[host]
  # Mirror 1
  [host."https://${REGISTRY_URL_1}"]
    capabilities = ["pull", "resolve"]
    ca = ["${CERT_DIR}/${CERT_NAME}.crt"]

    [host."https://${REGISTRY_URL_1}".auth]
      username = "${USERNAME}"
      password = "${PASSWORD}"

  # Mirror 2
  [host."http://${REGISTRY_URL_2}"]
    capabilities = ["pull", "resolve"]
    skip_verify = true
```

{% alert level="info" %}
Изменения конфигураций не приводят к перезапуску сервиса containerd.
{% endalert %}

##### Как добавить авторизацию в дополнительный registry (новый способ)?

Пример добавления авторизации в дополнительный registry при использовании **нового** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-auth.sh
spec:
  # Шаг может быть любой, т.к. не требуется перезапуск сервиса containerd
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        [host."https://${REGISTRY_URL}".auth]
          username = "username"
          password = "password"
    EOF
```

##### Как настроить сертификат для дополнительного registry (новый способ)?

Пример настройки сертификата для дополнительного registry? при использовании **нового** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-tls.sh
spec:
  # Шаг может быть любой, тк не требуется перезапуск сервиса containerd
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt" - << EOF
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
    EOF

    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        ca = ["/etc/containerd/registry.d/${REGISTRY_URL}/ca.crt"]
    EOF
```

{% alert level="info" %}
Помимо containerd, сертификат можно [добавить в операционную систему](examples.html#добавление-корневого-сертификата-в-хост).
{% endalert %}

##### Как добавить TLS skip verify (новый способ)?

Пример добавления TLS skip verify при использовании **нового** способа конфигурации:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: containerd-additional-config-skip-tls.sh
spec:
  # Шаг может быть любой, тк не требуется перезапуск сервиса containerd
  weight: 0
  bundles:
    - '*'
  nodeGroups:
    - "*"
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
    
    REGISTRY_URL=private.registry.example

    mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
    bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
    [host]
      [host."https://${REGISTRY_URL}"]
        capabilities = ["pull", "resolve"]
        skip_verify = true
    EOF
```

После применения конфигурационного файла проверьте доступ к registry с узлов, используя команды:

```bash
# Через cri интерфейс
crictl pull private.registry.example/image/repo:tag

# Через ctr с указанием директории с конфигурациями
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/image/repo:tag

# Через ctr для http репозитория
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/image/repo:tag
```

## Как использовать NodeGroup с приоритетом?

С помощью параметра [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority) кастомного ресурса `NodeGroup` можно задавать порядок заказа узлов в кластере.
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

В приведенном выше примере, `cluster-autoscaler` сначала попытается заказать узел типа *_spot-node*. Если в течение 15 минут его не получится добавить в кластер, NodeGroup `worker-spot` будет поставлен на паузу (на 20 минут) и `cluster-autoscaler` начнет заказывать узлы из NodeGroup `worker`.
Если через 30 минут в кластере возникнет необходимость развернуть еще один узел, `cluster-autoscaler` сначала попытается заказать узел из NodeGroup `worker-spot` и только потом — из NodeGroup `worker`.

После того как NodeGroup `worker-spot` достигнет своего максимума (5 узлов в примере выше), узлы будут заказываться из NodeGroup `worker`.

Шаблоны узлов (labels/taints) для NodeGroup `worker` и `worker-spot` должны быть одинаковыми, или как минимум подходить для той нагрузки, которая запускает процесс увеличения кластера.

## Как интерпретировать состояние группы узлов?

**Ready** — группа узлов содержит минимально необходимое число запланированных узлов с состоянием `Ready` для всех зон.

Пример 1. Группа узлов в состоянии `Ready`:

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

Пример 2. Группа узлов в состоянии `Not Ready`:

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

**Updating** — группа узлов содержит как минимум один узел, в котором присутствует аннотация с префиксом `update.node.deckhouse.io` (например, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval** — группа узлов содержит как минимум один узел, в котором присутствует аннотация `update.node.deckhouse.io/disruption-required` и
отсутствует аннотация `update.node.deckhouse.io/disruption-approved`.

**Scaling** — рассчитывается только для групп узлов с типом `CloudEphemeral`. Состояние `True` может быть в двух случаях:

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

[werf](https://ru.werf.io) проверяет состояние `Ready` у ресурсов и в случае его наличия дожидается, пока значение станет `True`.

Создание (обновление) ресурса [nodeGroup](cr.html#nodegroup) в кластере может потребовать значительного времени на развертывание необходимого количества узлов. При развертывании такого ресурса в кластере с помощью werf (например, в рамках процесса CI/CD) развертывание может завершиться по превышении времени ожидания готовности ресурса. Чтобы заставить werf игнорировать состояние `nodeGroup`, необходимо добавить к `nodeGroup` следующие аннотации:

```yaml
metadata:
  annotations:
    werf.io/fail-mode: IgnoreAndContinueDeployProcess
    werf.io/track-termination-mode: NonBlocking
```

## Что такое ресурс Instance?

Ресурс `Instance` в Kubernetes представляет собой описание объекта эфемерной виртуальной машины, но без конкретной реализации. Это абстракция, которая используется для управления машинами, созданными с помощью таких инструментов, как MachineControllerManager или Cluster API Provider Static.

Объект не содержит спецификации. Статус содержит:

1. Ссылку на `InstanceClass`, если он существует для данной реализации.
1. Ссылку на объект Node Kubernetes.
1. Текущий статус машины.
1. Информацию о том, как проверить [логи создания машины](#как-посмотреть-что-в-данный-момент-выполняется-на-узле-при-его-создании) (появляется на этапе создания машины).

При создании или удалении машины создается или удаляется соответствующий объект Instance.
Самостоятельно ресурс `Instance` создать нельзя, но можно удалить. В таком случае машина будет удалена из кластера (процесс удаления зависит от деталей реализации).

## Когда требуется перезагрузка узлов?

Некоторые операции по изменению конфигурации узлов могут потребовать перезагрузки.

Перезагрузка узла может потребоваться при изменении некоторых настроек sysctl, например, при изменении параметра `kernel.yama.ptrace_scope` (изменяется при использовании команды `astra-ptrace-lock enable/disable` в Astra Linux).
