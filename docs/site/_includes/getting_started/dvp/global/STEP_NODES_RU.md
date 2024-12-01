<script type="text/javascript" src='{{ assets["getting-started.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["getting-started-access.js"].digest_path }}'></script>
<script type="text/javascript" src='{{ assets["bcrypt.js"].digest_path }}'></script>

На данном этапе вы создали кластер, который состоит из **единственного** узла — master-узла. На master-узле по умолчанию работает только ограниченный набор системных компонентов. Для полноценной работы кластера необходимо либо добавить в кластер хотя бы один worker-узел, либо разрешить остальным компонентам Deckhouse работать на master-узле.

<p>Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте в <a href="../documentation/admin/platform-management/node-management/adding-node.html#добавление-статического-узла-в-кластер">документации</a>):</p>

Добавьте узел в кластер (подробнее о добавлении статического узла в кластер читайте в [документации](../documentation/admin/platform-management/node-management/adding-node.html)):

- Подготовьте сервер, который будет worker-узлом кластера.

- Создайте [NodeGroup](../../reference/cr/nodegroup.html) `worker`. Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
sudo d8 k create -f - << EOF
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
  {% endsnippetcut %}
  
- Сгенерируйте SSH-ключ с пустой парольной фразой. Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
```
ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
```
  {% endsnippetcut %}

- Создайте в кластере ресурс [SSHCredentials](../../reference/cr/sshcredentials.html). Для этого выполните на **master-узле** следующую команду:

  {% snippetcut %}
  ```shell
kubectl create -f - <<EOF
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
```
cat /dev/shm/caps-id.pub
```
{% endsnippetcut %}

- **На подготовленной виртуальной машине** создайте пользователя `caps`. Для этого выполните следующую команду, указав публичную часть SSH-ключа, полученную на предыдущем шаге:

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
```
pdpl-user -i 63 caps
```
  {% endsnippetcut %}

- Создайте [StaticInstance](../../reference/cr/staticinstance.html) для добавляемого узла. Для этого выполните на **master-узле** следующую команду, указав IP-адрес добавляемого узла:

  {% snippetcut %}
  ```shell
export NODE=<NODE-IP-ADDRESS> # Укажите IP-адрес узла, который необходимо подключить к кластеру.
  kubectl create -f - <<EOF
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
  {% endsnippetcut %}

- Убедитесь, что все узлы кластера находятся в статусе `Ready`.
  Выполните на **master-узле** следующую команду, чтобы получить список узлов кластера:

  {% snippetcut %}
  ```shell
  sudo kubectl get no
  ```
  {% endsnippetcut %}
