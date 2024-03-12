---
title: "Модуль cni-flannel"
---

Обеспечивает работу сети в кластере с помощью модуля [flannel](https://github.com/flannel-io/flannel).

---
title: "Управление узлами: примеры"
description: Примеры управления узлами кластера Kubernetes. Примеры создания группы узлов. Примеры автоматизации выполнения произвольных настроек на узле.
---

Ниже представлены несколько примеров описания `NodeGroup`, а также установки плагина cert-manager для kubectl и задания параметра sysctl.

## Примеры описания `NodeGroup`

<span id="пример-описания-nodegroup"></span>

### Облачные узлы

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    zones:
      - eu-west-1a
      - eu-west-1b
    minPerZone: 1
    maxPerZone: 2
    classReference:
      kind: AWSInstanceClass
      name: test
  nodeTemplate:
    labels:
      tier: test
```

### Статические узлы

<span id="пример-описания-статической-nodegroup"></span>

Для виртуальных машин на гипервизорах или физических серверов используйте статические узлы, указав `nodeType: Static` в NodeGroup.

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Узлы в такую группу добавляются [вручную](#вручную) с помощью подготовленных скриптов.

Также можно использовать способ [добавления статических узлов с помощью Cluster API Provider Static](#с-помощью-cluster-api-provider-static).

### Системные узлы

<span id="пример-описания-статичной-nodegroup-для-системных-узлов"></span>

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: Static
```

## Добавление статического узла в кластер

<span id="пример-описания-статичной-nodegroup"></span>

Добавление статического узла можно выполнить вручную или с помощью Cluster API Provider Static.

### Вручную

Чтобы добавить новый статический узел (выделенная ВМ, bare-metal-сервер и т. п.) в кластер вручную, выполните следующие шаги:

1. Для [CloudStatic-узлов](../040-node-manager/cr.html#nodegroup-v1-spec-nodetype) в облачных провайдерах, перечисленных ниже, выполните описанные в документации шаги:
   - [Для AWS](../030-cloud-provider-aws/faq.html#добавление-cloudstatic-узлов-в-кластер)
   - [Для GCP](../030-cloud-provider-gcp/faq.html#добавление-cloudstatic-узлов-в-кластер)
   - [Для YC](../030-cloud-provider-yandex/faq.html#добавление-cloudstatic-узлов-в-кластер)
1. Используйте существующий или создайте новый custom resource [NodeGroup](cr.html#nodegroup) ([пример](#статические-узлы) `NodeGroup` с именем `worker`). Параметр [nodeType](cr.html#nodegroup-v1-spec-nodetype) в custom resource NodeGroup для статических узлов должен быть `Static` или `CloudStatic`.
1. Получите код скрипта в кодировке Base64 для добавления и настройки узла.

   Пример получения кода скрипта в кодировке Base64 для добавления узла в NodeGroup `worker`:

   ```shell
   NODE_GROUP=worker
   kubectl -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Выполните предварительную настройку нового узла в соответствии с особенностями вашего окружения. Например:
   - добавьте необходимые точки монтирования в файл `/etc/fstab` (NFS, Ceph и т. д.);
   - установите необходимые пакеты (например, `ceph-common`);
   - настройте сетевую связанность между новым узлом и остальными узлами кластера.
1. Зайдите на новый узел по SSH и выполните следующую команду, вставив полученную в п. 2 Base64-строку:

   ```shell
   echo <Base64-КОД-СКРИПТА> | base64 -d | bash
   ```

### С помощью Cluster API Provider Static

Простой пример добавления статического узла в кластер с помощью [Cluster API Provider Static (CAPS)](./#cluster-api-provider-static):

1. Подготовьте необходимые ресурсы.

   * Выделите сервер (или виртуальную машину), настройте сетевую связанность и т. п., при необходимости установите специфические пакеты ОС и добавьте точки монтирования которые потребуются на узле.

   * Создайте пользователя (в примере — `caps`) с возможностью выполнять `sudo`, выполнив **на сервере** следующую команду:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Разрешите пользователю выполнять команды через sudo без пароля. Для этого **на сервере** внесите следующую строку в конфигурацию sudo (отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   * Сгенерируйте **на сервере** пару SSH-ключей с пустой парольной фразой:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     Публичный и приватный ключи пользователя `caps` будут сохранены в файлах `caps-id.pub` и `caps-id` в текущей папке на сервере.

   * Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами **на сервере** следующие команды:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

1. Создайте в кластере ресурс [SSHCredentials](cr.html#sshcredentials).

   В директории с ключами пользователя **на сервере** выполните следующую команду для получения закрытого ключа в формате Base64:

   ```shell
   base64 -w0 caps-id
   ```

   На любом компьютере с `kubectl`, настроенным на управление кластером, создайте переменную окружения со значением закрытого ключа созданного пользователя в Base64, полученным на предыдущем шаге:

   ```shell
    CAPS_PRIVATE_KEY_BASE64=<ЗАКРЫТЫЙ_КЛЮЧ_В_BASE64>
   ```

   Выполните следующую команду, для создания в кластере ресурса `SSHCredentials` (здесь и далее также используйте `kubectl`, настроенный на управление кластером):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: SSHCredentials
   metadata:
     name: credentials
   spec:
     user: caps
     privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
   EOF
   ```

1. Создайте в кластере ресурс [StaticInstance](cr.html#staticinstance), указав IP-адрес сервера статического узла:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
   spec:
     # Укажите IP-адрес сервера статического узла.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

1. Создайте в кластере ресурс [NodeGroup](cr.html#nodegroup):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
   EOF
   ```

### С помощью Cluster API Provider Static и фильтрами в label selector

Пример использования фильтров в [label selector](cr.html#nodegroup-v1-spec-staticinstances-labelselector) StaticInstance, для группировки статических узлов и использования их в разных NodeGroup. В примере используются две группы узлов (`front` и `worker`), предназначенные для разных задач, которые должны содержать разные по характеристикам узлы — два сервера для группы `front` и один для группы `worker`.

1. Подготовьте необходимые ресурсы (3 сервера или виртуальные машины) и создайте ресурс `SSHCredentials`, аналогично п.1 и п.2 [примера](#с-помощью-cluster-api-provider-static).

1. Создайте в кластере два ресурса [NodeGroup](cr.html#nodegroup) (здесь и далее используйте `kubectl`, настроенный на управление кластером):

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 2
       labelSelector:
         matchLabels:
           role: front
   ---
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: worker
   EOF
   ```

1. Создайте в кластере ресурсы [StaticInstance](cr.html#staticinstance), указав актуальные IP-адреса серверов:

   ```shell
   kubectl create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-1
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP1>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-front-2
     labels:
       role: front
   spec:
     address: "<SERVER-FRONT-IP2>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   ---
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-worker-1
     labels:
       role: worker
   spec:
     address: "<SERVER-WORKER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: credentials
   EOF
   ```

## Пример описания `NodeUser`

```yaml
apiVersion: deckhouse.io/v1
kind: NodeUser
metadata:
  name: testuser
spec:
  uid: 1100
  sshPublicKeys:
  - "<SSH_PUBLIC_KEY>"
  passwordHash: <PASSWORD_HASH>
  isSudoer: true
```

## Пример описания `NodeGroupConfiguration`

### Установка плагина cert-manager для kubectl на master-узлах

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: add-cert-manager-plugin.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "master"
  content: |
    if [ -x /usr/local/bin/kubectl-cert_manager ]; then
      exit 0
    fi
    curl -L https://github.com/cert-manager/cert-manager/releases/download/v1.7.1/kubectl-cert_manager-linux-amd64.tar.gz -o - | tar -zxvf - kubectl-cert_manager
    mv kubectl-cert_manager /usr/local/bin
```

### Задание параметра sysctl

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: sysctl-tune.sh
spec:
  weight: 100
  bundles:
  - "*"
  nodeGroups:
  - "*"
  content: |
    sysctl -w vm.max_map_count=262144
```

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

Чтобы удалить `StaticInstance` находящийся в любом состоянии отличном от `Pending` (`Runnig`, `Cleaning`, `Bootstraping`), нужно удалить соответствующий ресурс [Instance](cr.html#instance), после чего `StaticInstance` удалится автоматически.

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

> **Внимание!** Если на зачищаемом узле есть пулы хранения LINSTOR, то предварительно выгоните ресурсы с узла и удалите узел из LINSTOR, следуя [инструкции](../041-linstor/faq.html#как-выгнать-ресурсы-с-узла).

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

    distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
    curl -s -L https://nvidia.github.io/libnvidia-container/gpgkey | sudo apt-key add -
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.list | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list
    apt-get update
    apt-get install -y nvidia-container-toolkit nvidia-driver-525-server
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

    distribution=$(. /etc/os-release;echo $ID$VERSION_ID) \
    curl -s -L https://nvidia.github.io/libnvidia-container/$distribution/libnvidia-container.repo | sudo tee /etc/yum.repos.d/nvidia-container-toolkit.repo
    yum install -y nvidia-container-toolkit nvidia-driver
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

---
title: "Модуль snapshot-controller: примеры конфигурации"
---

### Использование снапшотов

Чтобы использовать снапшоты, необходимо указать конкретный `VolumeSnapshotClass`.
Чтобы получить список доступных VolumeSnapshotClass в вашем кластере, выполните:

```shell
kubectl get volumesnapshotclasses.snapshot.storage.k8s.io
```

Затем вы сможете использовать VolumeSnapshotClass для создания снапшота из существующего тома:

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: my-first-snapshot
spec:
  volumeSnapshotClassName: linstor
  source:
    persistentVolumeClaimName: my-first-volume
```

Спустя небольшой промежуток времени снапшот будет готов:

```yaml
$ kubectl describe volumesnapshots.snapshot.storage.k8s.io my-first-snapshot
...
Spec:
  Source:
    Persistent Volume Claim Name:  my-first-snapshot
  Volume Snapshot Class Name:      linstor
Status:
  Bound Volume Snapshot Content Name:  snapcontent-b6072ab7-6ddf-482b-a4e3-693088136d2c
  Creation Time:                       2020-06-04T13:02:28Z
  Ready To Use:                        true
  Restore Size:                        500Mi
```

Вы можете восстановить содержимое этого снапшота, создав новый PVC. Для этого необходимо указать снапшот в качестве источника:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-first-volume-from-snapshot
spec:
  storageClassName: linstor-data-r2
  dataSource:
    name: my-first-snapshot
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

### Клонирование CSI-томов

Основываясь на концепции снапшотов, вы также можете осуществить клонирование Persistent Volumes, а точнее существующих PersistentVolumeClaims (PVC).
Однако спецификация CSI не позволяет производить клонирование томов в пространстве имен и StorageClass'ах, отличных от оригинального PVC
(обратитесь [к документации Kubernetes](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/), чтобы узнать больше об ограничениях).

Чтобы клонировать том, создайте новый PVC и укажите исходный PVC в `dataSource`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-cloned-pvc
spec:
  storageClassName: linstor-data-r2
  dataSource:
    name: my-origin-pvc
    kind: PersistentVolumeClaim
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 500Mi
```

---
title: "Модуль ingress-nginx: FAQ"
---

## Как разрешить доступ к приложению внутри кластера только от ingress controller'ов?

Если вы хотите ограничить доступ к вашему приложению внутри кластера ТОЛЬКО от подов ingress'а, необходимо в под с приложением добавить контейнер с kube-rbac-proxy:

### Пример Deployment для защищенного приложения

{% raw %}

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: my-namespace
spec:
  selector:
    matchLabels:
      app: my-app
  replicas: 1
  template:
    metadata:
      labels:
        app: my-app
    spec:
      serviceAccountName: my-sa
      containers:
      - name: my-cool-app
        image: mycompany/my-app:v0.5.3
        args:
        - "--listen=127.0.0.1:8080"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 443
            scheme: HTTPS
      - name: kube-rbac-proxy
        image: flant/kube-rbac-proxy:v0.1.0 # Рекомендуется использовать прокси из нашего репозитория.
        args:
        - "--secure-listen-address=0.0.0.0:443"
        - "--config-file=/etc/kube-rbac-proxy/config-file.yaml"
        - "--v=2"
        - "--logtostderr=true"
        # Если kube-apiserver недоступен, мы не сможем аутентифицировать и авторизовывать пользователей.
        # Stale Cache хранит только результаты успешной авторизации и используется, только если apiserver недоступен.
        - "--stale-cache-interval=1h30m"
        ports:
        - containerPort: 443
          name: https
        volumeMounts:
        - name: kube-rbac-proxy
          mountPath: /etc/kube-rbac-proxy
      volumes:
      - name: kube-rbac-proxy
        configMap:
          name: kube-rbac-proxy
```

{% endraw %}

Приложение принимает запросы на адресе 127.0.0.1, это означает, что по незащищенному соединению к нему можно подключиться только изнутри пода.
Прокси же слушает на адресе 0.0.0.0 и перехватывает весь внешний трафик к поду.

### Как дать минимальные права для Service Account?

Чтобы аутентифицировать и авторизовывать пользователей с помощью kube-apiserver, у прокси должны быть права на создание `TokenReview` и `SubjectAccessReview`.

В наших кластерах [уже есть готовая ClusterRole](https://github.com/deckhouse/deckhouse/blob/main/modules/002-deckhouse/templates/common/rbac/kube-rbac-proxy.yaml) — **d8-rbac-proxy**.
Создавать ее самостоятельно не нужно! Нужно только прикрепить ее к Service Account'у вашего Deployment'а.
{% raw %}

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-sa
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-namespace:my-sa:d8-rbac-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: d8:rbac-proxy
subjects:
- kind: ServiceAccount
  name: my-sa
  namespace: my-namespace
```

### Конфигурация Kube-RBAC-Proxy

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-rbac-proxy
data:
  config-file.yaml: |+
    excludePaths:
    - /healthz # Не требуем авторизацию для liveness пробы.
    upstreams:
    - upstream: http://127.0.0.1:8081/ # Куда проксируем.
      path: / # Location прокси, с которого запросы будут проксированы на upstream.
      authorization:
        resourceAttributes:
          namespace: my-namespace
          apiGroup: apps
          apiVersion: v1
          resource: deployments
          subresource: http
          name: my-app
```

{% endraw %}
Согласно конфигурации, у пользователя должны быть права на доступ к Deployment с именем `my-app`
и его дополнительному ресурсу `http` в namespace `my-namespace`.

Выглядят такие права в виде RBAC так:
{% raw %}

```yaml
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
rules:
- apiGroups: ["apps"]
  resources: ["deployments/http"]
  resourceNames: ["my-app"]
  verbs: ["get", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: kube-rbac-proxy:my-app
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: kube-rbac-proxy:my-app
subjects:
# Все пользовательские сертификаты ingress-контроллеров выписаны для одной конкретной группы.
- kind: Group
  name: ingress-nginx:auth
```

Для ingress'а ресурса необходимо добавить параметры:

```yaml
nginx.ingress.kubernetes.io/backend-protocol: HTTPS
nginx.ingress.kubernetes.io/configuration-snippet: |
  proxy_ssl_certificate /etc/nginx/ssl/client.crt;
  proxy_ssl_certificate_key /etc/nginx/ssl/client.key;
  proxy_ssl_protocols TLSv1.2;
  proxy_ssl_session_reuse on;
```

{% endraw %}
Подробнее о том, как работает аутентификация по сертификатам, можно прочитать в [документации Kubernetes](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#x509-client-certs).

## Как сконфигурировать балансировщик нагрузки для проверки доступности IngressNginxController?

В ситуации, когда `IngressNginxController` размещен за балансировщиком нагрузки, рекомендуется сконфигурировать балансировщик для проверки доступности
узлов `IngressNginxController` с помощью HTTP-запросов или TCP-подключений. В то время как тестирование с помощью TCP-подключений представляет собой простой и универсальный механизм проверки доступности, мы рекомендуем использовать проверку на основе HTTP-запросов со следующими параметрами:
- протокол: `HTTP`;
- путь: `/healthz`;
- порт: `80` (в случае использования inlet'а `HostPort` нужно указать номер порта, соответствующий параметру [httpPort](cr.html#ingressnginxcontroller-v1-spec-hostport-httpport).

## Как настроить работу через MetalLB с доступом только из внутренней сети?

Пример MetalLB с доступом только из внутренней сети.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    sourceRanges:
    - 192.168.0.0/24
```

## Как добавить дополнительные поля для логирования в nginx-controller?

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  additionalLogFields:
    my-cookie: "$cookie_MY_COOKIE"
```

## Как включить HorizontalPodAutoscaling для IngressNginxController?

> **Важно!** Режим HPA возможен только для контроллеров с inlet'ом `LoadBalancer` или `LoadBalancerWithProxyProtocol`.
> **Важно!** Режим HPA возможен только при `minReplicas` != `maxReplicas`, в противном случае deployment `hpa-scaler` не создается.

HPA выставляется с помощью аттрибутов `minReplicas` и `maxReplicas` в [IngressNginxController CR](cr.html#ingressnginxcontroller).

IngressNginxController разверачивается с помощью DaemonSet. DaemonSet не предоставляет возможности горизонтального масштабирования, поэтому создается дополнительный deployment `hpa-scaler` и HPA resource, который следит за предварительно созданной метрикой `prometheus-metrics-adapter-d8-ingress-nginx-cpu-utilization-for-hpa`. Если CPU utilization превысит 50%, HPA закажет новую реплику для `hpa-scaler` (с учетом minReplicas и maxReplicas).

`hpa-scaler` deployment обладает HardPodAntiAffinity, поэтому он попытается заказать себе новый узел (если это возможно
в рамках своей NodeGroup), куда автоматически будет размещен еще один Ingress-контроллер.

Примечания:
* Минимальное реальное количество реплик IngressNginxController не может быть меньше минимального количества узлов в NodeGroup, в которую разворачивается IngressNginxController.
* Максимальное реальное количество реплик IngressNginxController не может быть больше максимального количества узлов в NodeGroup, в которую разворачивается IngressNginxController.

## Как использовать IngressClass с установленными IngressClassParameters?

Начиная с версии 1.1 IngressNginxController, Deckhouse создает объект IngressClass самостоятельно. Если вы хотите использовать свой IngressClass,
например с установленными IngressClassParameters, достаточно добавить к нему label `ingress-class.deckhouse.io/external: "true"`

```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  labels:
    ingress-class.deckhouse.io/external: "true"
  name: my-super-ingress
spec:
  controller: ingress-nginx.deckhouse.io/my-super-ingress
  parameters:
    apiGroup: elbv2.k8s.aws
    kind: IngressClassParams
    name: awesome-class-cfg
```

В таком случае, при указании данного IngressClass в CRD IngressNginxController, Deckhouse не будет создавать объект, а использует уже существующий.

## Как отключить сборку детализированной статистики Ingress-ресурсов?

По умолчанию Deckhouse собирает подробную статистику со всех Ingress-ресурсов в кластере, что может генерировать высокую нагрузку на систему мониторинга.

Для отключения сбора статистики добавьте label `ingress.deckhouse.io/discard-metrics: "true"` к соответствующему namespace или Ingress-ресурсу.

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов в пространстве имен `review-1`:

```shell
kubectl label ns review-1 ingress.deckhouse.io/discard-metrics=true
```

Пример отключения сбора статистики (метрик) для всех Ingress-ресурсов `test-site` в пространстве имен `development`:

```shell
kubectl label ingress test-site -n development ingress.deckhouse.io/discard-metrics=true
```

---
title: "Модуль ingress-nginx: пример"
---

{% raw %}

## Пример для AWS (Network Load Balancer)

При создании балансировщика будут использованы все доступные в кластере зоны.

В каждой зоне балансировщик получает публичный IP. Если в зоне есть инстанс с Ingress-контроллером, A-запись с IP-адресом балансировщика из этой зоны автоматически добавляется к доменному имени балансировщика.

Если в зоне не остается инстансов с Ingress-контроллером, тогда IP автоматически убирается из DNS.

В том случае, если в зоне всего один инстанс с Ingress-контроллером, при перезапуске пода IP-адрес балансировщика этой зоны будет временно исключен из DNS.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

## Пример для GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
```

> **Внимание!** В GCP на узлах должна присутствовать аннотация, разрешающая принимать подключения на внешние адреса для сервисов с типом NodePort.

## Пример для OpenStack

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

## Пример для bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

## Пример для bare metal (при использовании внешнего балансировщика, например Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и др.)

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

## Пример для bare metal (балансировщик MetalLB)

Модуль `metallb` на текущий момент доступен только в редакции Enterprise Edition Deckhouse.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

В случае использования MetalLB его speaker-поды должны быть запущены на тех же узлах, что и поды Ingress–контроллера.

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создается с параметром `externalTrafficPolicy: Local` (запрещая межузловой SNAT), и для удовлетворения данного параметра MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые поды.

Таким образом, для данного примера [конфигурация модуля `metallb`](../380-metallb/configuration.html) должна быть такой:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

{% endraw %}

---
title: "Модуль pod-reloader: примеры"
---

## Слежение за всеми изменениями во всех подключенных ресурсах: смонтированных как volume или используемых в переменных окружения

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    app: nginx
  annotations:
    pod-reloader.deckhouse.io/auto: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
```

## Слежение за изменениями только в конкретных ресурсах

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/search: "true"
spec:
  template:
    spec:
      containers:
        - name: nginx
          env:
            - name: SECRET_WORD
              valueFrom:
                secretKeyRef:
                  name: nginx-secret-value
                  key: extra
---
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: nginx-secret-value
  annotations:
    pod-reloader.deckhouse.io/match: "true"
```

## Слежение за изменениями в ресурсах из списка

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  annotations:
    pod-reloader.deckhouse.io/configmap-reload: "nginx-config,nginx-pages"
spec:
  template:
    spec:
      containers:
        - name: nginx
          volumeMounts:
            - name: pages
              mountPath: "/usr/share/nginx/pages"
            - name: config
              mountPath: "/etc/nginx/templates"
      volumes:
        - name: pages
          configMap:
            name: nginx-pages
        - name: config
          configMap:
            name: nginx-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-pages
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nginx-config
```

---
title: "Модуль chrony: FAQ"
type:
  - instruction
---

## Как запретить использование chrony и использовать NTP-демоны на узлах?

1. [Выключите](configuration.html) модуль chrony.

1. Создайте `NodeGroupConfiguration` custom step, чтобы включить NTP-демоны на узлах (пример для `systemd-timesyncd`):

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: enable-ntp-on-node.sh
   spec:
     weight: 100
     nodeGroups: ["*"]
     bundles: ["*"]
     content: |
       systemctl enable systemd-timesyncd
       systemctl start systemd-timesyncd
   ```

---
title: "Модуль delivery: примеры конфигурации"
---

## Прежде чем начать

Раздел описывает особенности работы Argo CD в поставке с Deckhouse и предполагает наличие базовых знаний или предварительного знакомства с Argo CD.

Данные, которые используются в примерах ниже:
- для доступа к веб-интерфейсу и API Argo CD выделен домен `argocd` в соответствии с шаблоном имен, определенным в параметре [publicDomainTemplate](../../deckhouse-configure-global.html#parameters-modules-publicdomaintemplate). В примерах ниже используется адрес `argocd.example.com`;
- `myapp` — название приложения;
- `mychart` — название Helm-чарта и werf-бандла. В приведенной схеме они должны совпадать. Для большей ясности название Helm-чарта и werf-бандла выбрано отличным от названия приложения;
- `cr.example.com` — хостнейм OCI-регистри;
- `cr.example.com/myproject` — репозиторий бандла в OCI-регистри.

## Концепция

Модуль предлагает способ развертывания приложений с помощью связки
[werf bundle](https://ru.werf.io/documentation/v1.2/advanced/bundles.html#выкат-бандлов)
и [OCI-based registries](https://helm.sh/docs/topics/registries/).

Преимущество этого подхода заключается в том, что есть единое место доставки артефакта — container
registry. Артефакт содержит в себе как образы контейнеров, так и Helm-чарт. Он используется как для
первичного деплоя приложения, так и для автообновлений по pull-модели.

Используемые компоненты:

- Argo CD;
- Argo CD Image Updater с [патчем для поддержки OCI-репозиториев](https://github.com/argoproj-labs/argocd-image-updater/pull/405);
- werf-argocd-cmp-sidecar, чтобы сохранить аннотации werf во время рендеринга манифестов.

Чтобы использовать OCI-registry как репозиторий, в параметрах репозитория Argo CD нужно использовать
флаг `enableOCI=true`. Модуль `delivery` его устанавливает автоматически.

Чтобы автоматически обновлять приложения в кластере после доставки артефакта, используется Argo CD
Image Updater. В Argo CD Image Updater внесены [изменения](https://github.com/argoproj-labs/argocd-image-updater/pull/405), позволяющие ему работать с werf-бандлами.

В примерах используется схема с шаблоном [«Application of
Applications»](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/#app-of-apps-pattern),
подразумевающая два git-репозитория — отдельный репозиторий для приложения и отдельный репозиторий для инфраструктуры:
![flow](../../images/502-delivery/werf-bundle-and-argocd.svg)

Использование шаблона Application of Applications и отдельного репозитория для инфраструктуры не обязательно, если допускается создавать
ресурсы Application вручную. Для простоты в примерах ниже мы будем придерживаться ручного управления ресурсами Application.

## Конфигурация с WerfSource CRD

Чтобы использовать Argo CD и Argo CD Image Updater, достаточно настроить объект Application и доступ к registry.
Доступ к registry нужен в двух местах — в репозитории Argo CD и в конфигурации Argo CD
Image Updater. Для этого нужно сконфигурировать:

1. Secret для доступа к registry.
2. Объект Application с конфигурацией приложения.
3. Registry для Image Updater в его configMap, в нем будет ссылка на Secret (п. 1) для registry.
4. Secret репозитория Argo CD, в нем будет копия параметров доступа из Secret'а (п. 1).

Модуль `delivery` упрощает конфигурацию для использования werf bundle и Argo CD. Упрощение касается двух
объектов: репозитория Argo CD и конфигурации registry для Image Updater. Эта конфигурация задается
одним ресурсом — *WerfBundle*. Поэтому в рамках модуля нужно определить конфигурацию в трех местах:

1. Secret для доступа к registry в формате `dockerconfigjson`.
2. Объект Application с конфигурацией приложения.
3. Объект WerfSource, содержащий информацию о registry и ссылку на Secret (п. 1) для доступа

Таким образом, для деплоя из OCI-репозитория нужно создать три объекта. Все объекты, предполагающие
область видимости namespace, должны быть созданы в namespace `d8-delivery`.

Пример:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  imageRepo: cr.example.io/myproject  # Репозиторий бандлов и образов.
  pullSecretName: example-registry    # Secret с доступом.
---
apiVersion: v1
kind: Secret
metadata:
  namespace: d8-delivery              # Namespace модуля.
  name: example-registry
type: kubernetes.io/dockerconfigjson  # Поддерживается только этот тип Secret'ов.
data:
  .dockerconfigjson: ...
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 0.0
  name: myapp
  namespace: d8-delivery  # Namespace модуля.
spec:
  destination:
    namespace: myapp
    server: https://kubernetes.default.svc
  project: default
  source:
    chart: mychart                    # Бандл — cr.example.com/myproject/mychart.
    helm: {}
    repoURL: cr.example.com/myproject # Репозиторий Argo CD из WerfBundle.
    targetRevision: 1.0.0
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
    - CreateNamespace=true
```

## Публикация артефакта в registry

OCI-чарт Helm требует, чтобы имя чарта в `Chart.yaml` совпадало с последним элементом пути
в OCI-registry. Поэтому название чарта необходимо использовать в названии бандла:

```sh
werf bundle publish --repo cr.example.com/myproject/mychart --tag 1.0.0
```

Подробнее о бандлах — [в документации werf](https://ru.werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html).

## Автообновление бандла

Argo CD Image Updater используется для автоматического обновления Application из опубликованного
werf-бандла в pull-модели. Image Updater сканирует OCI-репозиторий с заданным интервалом и обновляет
`targetRevision` в Application, посредством чего обновляется все приложение из обновленного
артефакта. Мы используем [измененный Image Updater](https://github.com/argoproj-labs/argocd-image-updater/pull/405), который умеет работать с OCI-registry и werf-бандлами.

### Правила обновлений образов

В Application нужно добавить аннотацию с правилами обновления образа
(подробнее — [в документации werf](https://ru.werf.io/documentation/v1.2/advanced/ci_cd/werf_with_argocd/configure_ci_cd.html#непрерывное-развертывание)).

Пример правила, обновляющего патч-версии приложения (`1.0.*`):

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
```

### Настройки доступа к registry

У сервис-аккаунта `argocd-image-updater` есть права на работу с ресурсами только в namespace
`d8-delivery`, поэтому именно в нем необходимо создать Secret с параметрами доступа к registry, на
который ссылается поле `credentials`.

#### Индивидуальная настройка для Application

Сослаться на параметры доступа можно индивидуально в каждом Application с помощью аннотации `argocd-image-updater.argoproj.io/pull-secret`.

Пример:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

## Особенности аутентификации командной утилиты `argocd`

### Пользователь Argo CD

Задайте `username` и `password` в конфигурации Argo CD или используйте пользователя `admin`. Пользователь `admin` по
умолчанию выключен, поэтому его необходимо включить.

Чтобы включить пользователя `admin`:

1. Откройте конфигурацию модуля `delivery`:

   ```sh
   kubectl edit mc delivery
   ```

1. Установите параметр `spec.settings.argocd.admin.enabled` в `true`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: delivery
     # (...)
   spec:
     enabled: true
     settings:
       argocd:
         admin:
           enabled: true
     version: 1
   ```

### kubectl

Если настроен [внешний доступ к Kubernetes API](../../modules/150-user-authn/configuration.html#parameters-publishapi), `argocd`
может использовать запросы к kube-apiserver:

```sh
argocd login argocd.example.com --core
```

Утилита `argocd` [не позволяет указывать namespace](https://github.com/argoproj/argo-cd/issues/9123)
во время вызова и рассчитывает на установленное значение в `kubectl`. Модуль `delivery`
находится в namespace `d8-delivery`, поэтому на время работы с `argocd` нужно выбрать namespace `d8-delivery` для использования по умолчанию.

Выполните следующую команду для выбора namespace `d8-delivery` в качестве namespace по умолчанию:

```sh
kubectl config set-context --current --namespace=d8-delivery
```

### Dex

Авторизация через Dex **не работает для CLI**, но работает в веб-интерфейсе.

Вот так **не работает**, потому нет возможности зарегистрировать публичного клиента для DexClient, в
роли которого выступает Argo CD:

```sh
argocd login argocd.example.com --sso
```

## Частичное использование WerfSource CRD

### Без репозитория Argo CD

WerfSource отвечает за создание репозитория в Argo CD и внесение registry в конфигурацию
Image Updater. От создания репозитория можно отказаться в WerfSource, для этого нужно установить
параметр `spec.argocdRepoEnabled=false`. Это пригодится, если используется тип репозитория,
отличный от OCI, например Chart Museum или git:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: WerfSource
metadata:
  name: example
spec:
  # ...
  argocdRepoEnabled: false
```

#### Как самостоятельно создать репозиторий Argo CD для OCI-registry

Registry играет роль репозитория бандлов. Чтобы это работало, нужно в репозитории включить режим
OCI. Однако веб-интерфейс не позволяет установить флаг `enableOCI`, поэтому его нужно добавить вне веб-интерфейса.

##### Argo CD CLI

Утилита `argocd` поддерживает флаг `--enable-oci`:

```sh
$ argocd repo add cr.example.com/myproject \
  --enable-oci \
  --type helm \
  --name REPO_NAME \
  --username USERNAME \
  --password PASSWORD
```

##### Веб-интерфейс и kubectl

В существующий репозиторий недостающий флаг можно добавить вручную:

```sh
kubectl -n d8-delivery edit secret repo-....
```

```yaml
apiVersion: v1
kind: Secret
stringData:           # <----- Добавить
  enableOCI: "true"   # <----- и сохранить.
data:
  # (...)
metadata:
  # (...)
  name: repo-....
  namespace: d8-delivery
type: Opaque
```

### Без registry для Image Updater

Registry в `configmap/argocd-image-updater-config` могут быть настроены только через WerfSource,
потому что `deckhouse` генерирует этот ConfigMap, используя объекты WerfSource. Если этот способ не
подходит, конфигурацию Image Updater можно задать с помощью аннотаций в каждом
Application индивидуально:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  annotations:
    argocd-image-updater.argoproj.io/chart-version: ~ 1.0
    argocd-image-updater.argoproj.io/pull-secret: pullsecret:d8-delivery/example-registry
```

---
title: "Модуль delivery: FAQ"
---

## Как сбросить пароль для пользователя admin?

Воспользуйтесь [следующей](https://github.com/argoproj/argo-cd/blob/master/docs/faq.md#i-forgot-the-admin-password-how-do-i-reset-it) инструкцией, заменив namespace `argocd` на `d8-delivery`.

---
title: "Модуль namespace-configurator: примеры"
---

## Пример

Этот пример добавит лейбл `extended-monitoring.deckhouse.io/enabled=true` и аннотацию `foo=bar` к каждому namespace, начинающемуся с `prod-` или `infra-`, за исключением `infra-test`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: namespace-configurator
spec:
  version: 1
  enabled: true
  settings:
    configurations:
    - annotations:
        foo: bar
      labels:
        extended-monitoring.deckhouse.io/enabled: "true"
      includeNames:
      - "^prod"
      - "^infra"
      excludeNames:
      - "infra-test"
```
