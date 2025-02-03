<script type="text/javascript" src='{% javascript_asset_tag getting-started %}[_assets/js/getting-started.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag getting-started-access %}[_assets/js/getting-started-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag bcrypt %}[_assets/js/bcrypt.js]{% endjavascript_asset_tag %}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. На master-узле по умолчанию запускаются только системные компоненты. Для полноценной работы платформы виртуализации необходимо добавить в кластер хотя бы один worker-узел.

Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте [в документации](https://deckhouse.ru/products/virtualization-platform/documentation/admin/platform-management/node-management/adding-node.html)):

- Подготовьте сервер, который будет worker-узлом кластера.

- Создайте [NodeGroup](../../reference/cr/nodegroup.html) `worker`. Для этого выполните на **master-узле** следующую команду:

  ```shell
  sudo -i d8 k create -f - << EOF
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
  
- Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на **master-узле** следующую команду:

  ```shell
  ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
  ```

- Создайте в кластере ресурс [SSHCredentials](../../reference/cr/sshcredentials.html). Для этого выполните на **master-узле** следующую команду:

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

- Выведите публичную часть сгенерированного ранее SSH-ключа (он понадобится на следующем шаге). Для этого выполните на **master-узле** следующую команду:

  ```shell
  cat /dev/shm/caps-id.pub
  ```

- **На подготовленной виртуальной машине** создайте пользователя `caps`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:

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

- **В операционных системах семейства Astra Linux**, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

  ```shell
  pdpl-user -i 63 caps
  ```

- Создайте [StaticInstance](../../reference/cr/staticinstance.html) для добавляемого узла. Для этого выполните на **master-узле** следующую команду, указав IP-адрес добавляемого узла:

  ```shell
  export NODE=<NODE-IP-ADDRESS> # Укажите IP-адрес узла, который необходимо подключить к кластеру.
  sudo -i d8 k create -f - <<EOF
  apiVersion: deckhouse.io/v1alpha1
  kind: StaticInstance
  metadata:
    name: dvp-worker
    labels:
      role: worker
  spec:
    address: "$NODE"
    credentialsRef:
      kind: SSHCredentials
      name: caps
  EOF
  ```

- Убедитесь, что все узлы кластера находятся в статусе `Ready`.
  Выполните на **master-узле** следующую команду, чтобы получить список узлов кластера:

  ```shell
  sudo -i d8 k get no
  ```
