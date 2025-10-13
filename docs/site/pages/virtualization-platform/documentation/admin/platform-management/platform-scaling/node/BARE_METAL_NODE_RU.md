---
title: "Добавление и управление bare-metal узлами"
permalink: ru/virtualization-platform/documentation/admin/platform-management/platform-scaling/node/bare-metal-node.html
lang: ru
---

## Добавление узлов в bare-metal кластере

### Ручной способ

1. Включите модуль [`node-manager`](/modules/node-manager/cr.html).

1. Создайте объект [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом `Static`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

   В спецификации этого ресурса укажите тип узлов `Static`. Для всех объектов NodeGroup в кластере автоматически создаётся скрипт `bootstrap.sh`, с помощью которого узлы добавляются в группы. Когда узлы добавляются вручную, необходимо скопировать этот скрипт на сервер и выполнить.

   Скрипт можно получить в веб-интерфейсе Deckhouse на вкладке «Группы узлов → Скрипты» или командой `d8 k`:

   ```shell
   d8 k -n d8-cloud-instance-manager get secrets manual-bootstrap-for-worker -ojsonpath="{.data.bootstrap\.sh}"
   ```

   Скрипт нужно раскодировать из Base64, а затем выполнить от `root`.

1. Когда скрипт выполнится, сервер добавится в кластер в качестве узла той группы, для которой был использован скрипт.

### Автоматический способ

В DVP возможно автоматическое добавление физических (bare-metal) серверов в кластер без ручного запуска установочного скрипта на каждом узле. Для этого необходимо:

1. Подготовить сервер (ОС, сеть):
   - установить поддерживаемую ОС;
   - настроить сеть и убедиться, что сервер доступен по SSH;
   - создать системного пользователя (например, `ubuntu`), от имени которого будет выполняться подключение по SSH;
   - убедиться, что пользователь может выполнять команды через `sudo`.

1. Создать объект [SSHCredentials](/modules/node-manager/cr.html#sshcredentials) с доступом к серверу. DVP использует объект SSHCredentials для подключения к серверам по SSH. В нём указывается:
   - приватный ключ;
   - пользователь ОС;
   - порт SSH;
   - (опционально) пароль для `sudo`, если требуется.

     Пример:

     ```yaml
     apiVersion: deckhouse.io/v1alpha1
     kind: SSHCredentials
     metadata:
       name: static-nodes
     spec:
       privateSSHKey: |
         -----BEGIN OPENSSH PRIVATE KEY-----
         LS0tLS1CRUdJlhrdG...................VZLS0tLS0K
         -----END OPENSSH PRIVATE KEY-----
       sshPort: 22
       sudoPassword: password
       user: ubuntu
     ```

     > **Важно**. Приватный ключ должен соответствовать открытому ключу, добавленному в `~/.ssh/authorized_keys` на сервере.

1. Создать объект [StaticInstance](/modules/node-manager/cr.html#staticinstance) для каждого сервера:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
     labels:
       static-node: auto
   spec:
     address: 192.168.1.10
     credentialsRef:
       apiVersion: deckhouse.io/v1alpha1
       kind: SSHCredentials
       name: static-nodes
   ```

   Под каждый сервер необходимо создавать отдельный ресурс [StaticInstance](/modules/node-manager/cr.html#staticinstance), но можно использовать одни и те же SSHCredentials для доступа на разные серверы.

   Возможные состояния ресурсов StaticInstance:

   - `Pending` — сервер ещё не настроен, в кластере отсутствует соответствующий узел;
   - `Bootstrapping` — выполняется настройка сервера и подключение узла в кластер;
   - `Running` — сервер успешно настроен, узел подключён к кластеру;
   - `Cleaning` — выполняется очистка сервера и удаление узла из кластера.

   Эти состояния отображают текущий этап управления узлом. CAPS автоматически переводит StaticInstance между этими состояниями в зависимости от необходимости добавить или удалить узел из группы.

1. Создать [NodeGroup](/modules/node-manager/cr.html#nodegroup) с описанием, как DVP будет использовать эти серверы:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
     staticInstances:
       count: 3
       labelSelector:
         matchLabels:
           static-node: auto
     nodeTemplate:
       labels:
         node-role.deckhouse.io/worker: ""
   ```

   Здесь добавляются параметры, которые описывают использование StaticInstances:

   - `count` указывает, сколько узлов будет добавлено в эту группу;
   - в `labelSelector` прописываются правила для создания выборки узлов.

   При использовании Cluster API Provider Static (CAPS) важно правильно задать параметры `nodeType`: `Static` и секцию `staticInstances` в объекте NodeGroup:

   - Если параметр `labelSelector` не задан, CAPS будет использовать любые ресурсы StaticInstance, доступные в кластере.
   - Один StaticInstance может использоваться в нескольких группах при совпадении лейблов.
   - CAPS будет автоматически поддерживать количество узлов в группе, заданное в параметре `count`.
   - При удалении узла CAPS выполнит его очистку и отключение, а соответствующий StaticInstance перейдёт в статус `Pending` и может быть использован повторно.

После создания группы узлов появится скрипт для добавления серверов в группу. DVP будет ожидать появления требуемого числа объектов StaticInstance, подходящих по лейблам. Как только такой объект появится, DVP использует указанный IP и параметры для подключения по SSH, выполнит скрипт `bootstrap.sh` и добавит сервер в группу.

## Изменение конфигурации статического кластера

Настройки статического кластера хранятся в структуре [StaticClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#staticclusterconfiguration).

Чтобы изменить параметры статического кластера, выполните команду:

```shell
d8 platform edit static-cluster-configuration
```

## Перемещение статического узла между NodeGroup

{% alert level="warning" %}
В процессе переноса статических узлов между [NodeGroup](/modules/node-manager/cr.html#nodegroup) выполняется очистка и повторный bootstrap узла, объект `Node` пересоздаётся.
{% endalert %}

1. Создайте новый ресурс NodeGroup, например, с именем `front`, который будет управлять статическим узлом с лейблом `role: front`:

   ```yaml
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: front
   spec:
     nodeType: Static
     staticInstances:
       count: 1
       labelSelector:
         matchLabels:
           role: front
   ```

1. Измените лейбл `role` у существующего [StaticInstance](/modules/node-manager/cr.html#staticinstance) с `worker` на `front`. Это позволит новой NodeGroup `front` начать управлять этим узлом:

   ```shell
   d8 k label staticinstance static-worker-1 role=front --overwrite
   ```

1. Обновите ресурс NodeGroup `worker`, уменьшив значение параметра `count` с `1` до `0`:

   ```shell
   d8 k patch nodegroup worker -p '{"spec": {"staticInstances": {"count": 0}}}' --type=merge
   ```

### Ручная очистка статического узла

Для отключения узла из кластера и очистки сервера используйте скрипт `/var/lib/bashible/cleanup_static_node.sh`, который заранее размещён на каждом статическом узле.

Пример отключения узла кластера и очистки сервера:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

{% alert level="info" %}
Инструкция справедлива как для узла, настроенного вручную (с помощью бутстрап-скрипта), так и для узла, настроенного с помощью CAPS.
{% endalert %}

## Пример описания NodeGroup

### Пример описания NodeGroup для статических узлов

Для виртуальных машин на гипервизорах или физических серверов используйте статические узлы, указав `nodeType: Static` в [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
```

Узлы в такую группу добавляются вручную с помощью подготовленных скриптов или автоматически с помощью Cluster API Provider Static (CAPS).

## Изменение CRI для NodeGroup

CRI (Container Runtime Interface) — стандартный интерфейс между kubelet и программой исполнения контейнеров (runtime).

{% alert level="warning" %}
Смена CRI возможна только между `Containerd` на `NotManaged` и обратно (параметр `cri.type`).
{% endalert %}

Для изменения CRI для [NodeGroup](/modules/node-manager/cr.html#nodegroup), установите параметр `cri.type` в `Containerd` или в `NotManaged`.

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

{% alert level="warning" %}
 При изменении `cri.type` для NodeGroup, созданных с помощью `dhctl`, необходимо обновить это значение в `dhctl config edit provider-cluster-configuration` и настройках объекта NodeGroup.
{% endalert %}

После изменения CRI для NodeGroup модуль `node-manager` будет поочередно перезагружать узлы, применяя новый CRI.  Обновление узла сопровождается простоем (disruption). В зависимости от настройки `disruption` для NodeGroup, модуль `node-manager` либо автоматически выполнит обновление узлов, либо потребует подтверждения вручную.

## Изменение NodeGroup у статического узла

Если узел находится под управлением [CAPS](#автоматический-способ), то изменить принадлежность к NodeGroup у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#удаление-staticinstance) и создать новый.

Чтобы перенести существующий статический узел созданный [вручную](#ручной-способ) из одной NodeGroup в другую, необходимо изменить у узла лейбл группы:

```shell
d8 k label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
d8 k label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.

## Изменение IP-адреса в StaticInstance

Изменить IP-адрес в ресурсе [StaticInstance](/modules/node-manager/cr.html#staticinstance) нельзя. Если в StaticInstance указан ошибочный адрес, то нужно [удалить StaticInstance](#удаление-staticinstance) и создать новый.

## Удаление StaticInstance

[StaticInstance](/modules/node-manager/cr.html#staticinstance), находящийся в состоянии `Pending` можно удалять без ограничений.

Чтобы удалить StaticInstance находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstrapping`), выполните следующие шаги:

1. Добавьте лейбл `"node.deckhouse.io/allow-bootstrap": "false"` в StaticInstance.

   Пример команды для добавления лейбла:

   ```shell
   d8 k label staticinstance d8cluster-worker node.deckhouse.io/allow-bootstrap=false
   ```

1. Дождитесь, пока StaticInstance перейдет в статус `Pending`.

   Для проверки статуса StaticInstance используйте команду:

   ```shell
   d8 k get staticinstances
   ```

1. Удалите StaticInstance.

   Пример команды для удаления StaticInstance:

   ```shell
   d8 k delete staticinstance d8cluster-worker
   ```

1. Уменьшите значение параметра `NodeGroup.spec.staticInstances.count` на 1.
