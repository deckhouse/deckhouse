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
