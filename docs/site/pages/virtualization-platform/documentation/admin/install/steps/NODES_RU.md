---
title: "Добавление узлов"
permalink: ru/virtualization-platform/documentation/admin/install/steps/nodes.html
lang: ru
---

## Добавление узлов

После базовой установки кластер состоит из *единственного* узла — master-узла. Чтобы запускать виртуальные машины на подготовленных worker-узлах, их нужно добавить в кластер.

Далее будет рассмотрено добавление 2 worker-узлов. Подробнее о добавлении статических узлов в кластер читайте в [документации](../../platform-management/node-management/adding-node.html)).

1. Убедитесь, что все шаги подготовки выполнены (см. [подготовка worker-узлов](../prepare.html))

1. Создайте ресурс [NodeGroup](../../reference/cr/nodegroup.html) `worker`. Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
  sudo -i d8 k create -f - << EOF
    apiVersion: deckhouse.io/v1
    kind: NodeGroup
    metadata:
      name: worker
    spec:
      nodeType: Static
      staticInstances:
        count: 2
        labelSelector:
          matchLabels:
            role: worker
  EOF
  ```
  {% endsnippetcut %}

- Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
  ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
  ```
  {% endsnippetcut %}

- Создайте в кластере ресурс [SSHCredentials](../../reference/cr/sshcredentials.html). Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
  sudo -i d8 k create -f - <<EOF
    apiVersion: deckhouse.io/v1alpha1
    kind: SSHCredentials
    metadata:
      name: caps
    spec:
      user: caps
      privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
  EOF
  ```
  {% endsnippetcut %}

- Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
  cat /dev/shm/caps-id.pub
  ```
  {% endsnippetcut %}

- **На worker-узле** создайте пользователя `caps`. Для этого выполните следующие команды, указав публичную часть SSH-ключа, полученную на предыдущем шаге:

  {% snippetcut %}
  ```shell
  export KEY='<SSH-PUBLIC-KEY>' # Укажите публичную часть SSH-ключа пользователя.
  useradd -m -s /bin/bash caps
  usermod -aG sudo caps
  echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
  mkdir /home/caps/.ssh
  echo $KEY >> /home/caps/.ssh/authorized_keys
  chown -R caps:caps /home/caps
  chmod 700 /home/caps/.ssh
  chmod 600 /home/caps/.ssh/authorized_keys
  ```
  {% endsnippetcut %}

- **В операционных системах семейства Astra Linux**, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

  {% snippetcut %}
  ```shell
  pdpl-user -i 63 caps
  ```
  {% endsnippetcut %}

- Создайте ресурcы [StaticInstance](../../reference/cr/staticinstance.html)

  Выполните на **master-узле** следующие команды с указанием IP-адреса и уникального имени каждого узла:

  {% snippetcut %}
  ```shell
  export NODE_IP=<NODE-IP-ADDRESS> # Укажите IP-адрес узла, который необходимо подключить к кластеру.
  export NODE_NAME=<NODE-NAME> # Укажите уникальное имя узла, например, dvp-worker-1.
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: StaticInstance
  metadata:
    name: "$NODE_NAME"
    labels:
      role: worker
  spec:
    address: "$NODE_IP"
    credentialsRef:
      kind: SSHCredentials
      name: caps
  EOF
  ```
  {% endsnippetcut %}

- Убедитесь, что все узлы кластера находятся в статусе `Ready`.

  Выполните на **master-узле** следующую команду, чтобы получить список узлов кластера:

  ```shell
  sudo -i d8 k get no
  ```
  
  Пример вывода:
  {% snippetcut %}
  ```
  user@master-0:~$ sudo -i d8 k get no
  NAME            STATUS   ROLES                  AGE    VERSION
  master-0        Ready    control-plane,master   40m    v1.29.10
  dvp-worker-1    Ready    worker                 3m     v1.29.10
  dvp-worker-2    Ready    worker                 3m     v1.29.10
  ```
  {% endsnippetcut %}

