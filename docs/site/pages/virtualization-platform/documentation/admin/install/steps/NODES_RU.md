---
title: "Добавление узлов"
permalink: ru/virtualization-platform/documentation/admin/install/steps/nodes.html
lang: ru
---

После первоначальной установки кластер состоит только из одного узла — master-узла. Для того чтобы запускать виртуальные машины на подготовленных worker-узлах, их необходимо добавить в кластер.

Далее будет рассмотрено добавление двух worker-узлов. Более подробную информацию о добавлении статических узлов в кластер можно найти [в документации](/products/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/bare-metal-node.html).

{% alert level="info" %}
Для выполнения приведенных ниже команд необходима установленная утилита [d8](/products/kubernetes-platform/documentation/v1/cli/d8/) (Deckhouse CLI) и настроенный контекст kubectl для доступа к кластеру. Также, можно подключиться к master-узлу по SSH и выполнить команду от пользователя `root` с помощью `sudo -i`.
{% endalert %}

## Подготовка узлов

1. Убедитесь, что поддержка виртуализации Intel-VT (VMX) или AMD-V (SVM) включена на уровне BIOS/UEFI на всех узлах кластера.

1. Установите одну из [поддерживаемых операционных систем](../../../about/requirements.html#поддерживаемые-ос-для-узлов-платформы) на каждом узле кластера. Обратите внимание на версию и архитектуру системы.

1. Проверьте доступ к container registry:
   - Убедитесь, что с каждого узла кластера доступен container registry. Установщик по умолчанию использует публичное хранилище `registry.deckhouse.ru`. Настройте сетевое подключение и необходимые политики безопасности для доступа к репозиторию.
   - Чтобы проверить доступ, воспользуйтесь следующей командой:

     ```shell
     curl https://registry.deckhouse.ru/v2/
     ```

     Ожидаемый ответ:

     ```console
     401 Unauthorized
     ```

## Добавление подготовленных узлов

Создайте ресурс [NodeGroup](/modules/node-manager/cr.html#nodegroup) `worker`. Для этого выполните следующую команду:

```shell
d8 k create -f - << EOF
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

Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на **master-узле** следующую команду:

```shell
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```

Создайте в кластере ресурс [SSHCredentials](/modules/node-manager/cr.html#sshcredentials). Для этого выполните на **master-узле** следующую команду:

```yaml
sudo -i d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
kind: SSHCredentials
metadata:
  name: caps
spec:
  user: caps
  privateSSHKey: "`cat /dev/shm/caps-id | base64 -w0`"
EOF
```

Получите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на **master-узле** следующую команду:

```shell
cat /dev/shm/caps-id.pub
```

**На worker-узле** создайте пользователя `caps`. Для этого выполните следующие команды, указав публичную часть SSH-ключа, полученную на предыдущем шаге (выполните эти команды под пользователем `root`):

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

**В операционных системах семейства Astra Linux** при использовании модуля мандатного контроля целостности Parsec сконфигурируйте максимальный уровень целостности для пользователя `caps`:

```shell
sudo -i pdpl-user -i 63 caps
```

Создайте ресурсы [StaticInstance](/modules/node-manager/cr.html#staticinstance). Выполните следующие команды с указанием IP-адреса и уникального имени каждого узла:

```yaml
export NODE_IP=<NODE-IP-ADDRESS> # Укажите IP-адрес узла, который необходимо подключить к кластеру.
export NODE_NAME=<NODE-NAME> # Укажите уникальное имя узла, например, dvp-worker-1.
d8 k create -f - <<EOF
apiVersion: deckhouse.io/v1alpha2
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

Убедитесь, что все узлы кластера находятся в статусе `Ready`.

Выполните следующую команду, чтобы получить список узлов кластера:

```shell
d8 k get no
```

{% offtopic title="Пример вывода..." %}

```console
NAME            STATUS   ROLES                  AGE    VERSION
master-0        Ready    control-plane,master   40m    v1.29.10
dvp-worker-1    Ready    worker                 3m     v1.29.10
dvp-worker-2    Ready    worker                 3m     v1.29.10
```

{% endofftopic %}
