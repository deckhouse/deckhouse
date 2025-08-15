---
title: "Добавление и удаление узла"
permalink: ru/virtualization-platform/documentation/admin/platform-management/node-management/adding-node.html
lang: ru
---

## Добавление статического узла в кластер

<span id="добавление-узла-в-кластер"></span>

Добавление статического узла можно выполнить вручную или с помощью Cluster API Provider Static.

### Добавление статического узла вручную

Чтобы добавить bare-metal сервер в кластер как статический узел, выполните следующие шаги:

1. Используйте существующий custom resource [NodeGroup](../../../../reference/cr/nodegroup.html) или создайте новый, задав для параметра [`nodeType`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-nodetype) значение `Static` или `CloudStatic`.

   Пример ресурса NodeGroup с именем `worker`:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: worker
   spec:
     nodeType: Static
   ```

1. Получите код скрипта в кодировке Base64 для добавления и настройки узла.

   Пример получения кода скрипта в кодировке Base64 для добавления узла в NodeGroup `worker`:

   ```shell
   NODE_GROUP=worker
   d8 k -n d8-cloud-instance-manager get secret manual-bootstrap-for-${NODE_GROUP} -o json | jq '.data."bootstrap.sh"' -r
   ```

1. Выполните предварительную настройку нового узла в соответствии с особенностями вашего окружения:

- добавьте необходимые точки монтирования в файл `/etc/fstab` (NFS, Ceph и т. д.);
- установите необходимые пакеты;
- настройте сетевую связность между новым узлом и остальными узлами кластера.

1. Подключитесь на новый узел по SSH и выполните следующую команду, вставив полученную в п.2 Base64-строку:

   ```shell
   echo <Base64-КОД-СКРИПТА> | base64 -d | bash
   ```

### Добавление статического узла с помощью CAPS

Чтобы узнать об инструменте Cluster API Provider Static (CAPS) подробнее,
обратитесь к разделу [Настройка узла через CAPS](node-group.html#настройка-узла-через-caps).

Пример добавления статического узла в кластер с помощью CAPS:

1. **Выделите сервер с установленной операционной системой (ОС)** и настройте сетевую связность.
   При необходимости установите специфические для ОС пакеты и добавьте точки монтирования, которые потребуются на узле.

   * Создайте пользователя (в примере — `caps`) с возможностью выполнять `sudo`, выполнив на сервере следующую команду:

     ```shell
     useradd -m -s /bin/bash caps 
     usermod -aG sudo caps
     ```

   * Разрешите пользователю выполнять команды через `sudo` без пароля. Для этого на сервере внесите следующую строку в конфигурацию `sudo` (отредактировав файл `/etc/sudoers`, выполнив команду `sudo visudo` или другим способом):

     ```text
     caps ALL=(ALL) NOPASSWD: ALL
     ```

   * Сгенерируйте на сервере пару SSH-ключей с пустой парольной фразой:

     ```shell
     ssh-keygen -t rsa -f caps-id -C "" -N ""
     ```

     Публичный и приватный ключи пользователя `caps` будут сохранены в файлах `caps-id.pub` и `caps-id` в текущей директории на сервере.

   * Добавьте полученный публичный ключ в файл `/home/caps/.ssh/authorized_keys` пользователя `caps`, выполнив в директории с ключами на сервере следующие команды:

     ```shell
     mkdir -p /home/caps/.ssh 
     cat caps-id.pub >> /home/caps/.ssh/authorized_keys 
     chmod 700 /home/caps/.ssh 
     chmod 600 /home/caps/.ssh/authorized_keys
     chown -R caps:caps /home/caps/
     ```

   * В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

     ```shell
     pdpl-user -i 63 caps
     ```

1. **Создайте в кластере ресурс SSHCredentials.**

   * Для доступа к добавляемому серверу, компоненту CAPS необходим приватный ключ сервисного пользователя `caps`. Ключ в формате Base64 добавляется в ресурс SSHCredentials.

     В директории с ключами пользователя на сервере выполните следующую команду для получения закрытого ключа в формате Base64:

     ```shell
     base64 -w0 caps-id
     ```

   * На любом компьютере, настроенном на управление кластером, создайте переменную окружения с приватным ключом в формате Base64, полученным на предыдущем шаге (в начале команды добавьте пробел, чтобы ключ не сохранился в истории команд):

     ```shell
     CAPS_PRIVATE_KEY_BASE64=<ЗАКРЫТЫЙ_КЛЮЧ_В_BASE64>
     ```

   * Создайте ресурс SSHCredentials с именем сервисного пользователя и его приватным ключом:

     ```shell
     d8 k create -f - <<EOF
     apiVersion: deckhouse.io/v1alpha1
     kind: SSHCredentials
     metadata:
       name: static-0-access
     spec:
       user: caps
       privateSSHKey: "${CAPS_PRIVATE_KEY_BASE64}"
     EOF
     ```

1. **Создайте в кластере ресурс StaticInstance**:

   Ресурс StaticInstance определяет IP-адрес сервера статического узла и данные для доступа к серверу:

   ```shell
   d8 k create -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: StaticInstance
   metadata:
     name: static-0
   spec:
     # Укажите IP-адрес сервера статического узла.
     address: "<SERVER-IP>"
     credentialsRef:
       kind: SSHCredentials
       name: static-0-access
   EOF
   ```

1. **Создайте в кластере ресурс NodeGroup**:

   ```shell
   d8 k create -f - <<EOF
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

1. **Дождитесь, когда ресурс NodeGroup перейдёт в состояние `Ready`**.
   Чтобы проверить состояние ресурса, выполните следующую команду:

   ```shell
   d8 k get ng worker
   ```

   В статусе NodeGroup в колонке `READY` должен появиться 1 узел:

   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   worker   Static   1       1       1                                                                 15m   True
   ```

### Добавление статического узла с помощью Cluster API Provider Static и фильтров в label selector

<span id="caps-with-label-selector"></span>

Чтобы подключить разные StaticInstance в разные NodeGroup можно использовать label selector, указываемый в NodeGroup и в метаданных StaticInstance.

Для примера разберём задачу распределения 3 статических узлов по 2 NodeGroup: 1 узел добавим в группу worker и 2 узла в группу front.

1. Подготовьте необходимые ресурсы (3 сервера) и создайте для них ресурсы SSHCredentials, аналогично п.1 и п.2 [предыдущего примера](#добавление-статического-узла-с-помощью-caps).

1. Создайте в кластере два ресурса NodeGroup:

   Укажите `labelSelector`, чтобы в NodeGroup подключались только сервера, совпадающие с ним.

   ```shell
   d8 k create -f - <<EOF
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

1. Создайте в кластере ресурсы StaticInstance

   Укажите актуальные IP-адреса серверов и задайте лейбл `role` в метаданных:

   ```shell
   d8 k create -f - <<EOF
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
       name: front-1-credentials
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
       name: front-2-credentials
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
       name: worker-1-credentials
   EOF
   ```

1. Чтобы проверить результат, выполните следующую команду:

   ```shell
   d8 k get ng
   ```

   В результате будет выведен список созданных ресурсов NodeGroup с распределенными между ними статическими узлами:

   ```console
   NAME     TYPE     READY   NODES   UPTODATE   INSTANCES   DESIRED   MIN   MAX   STANDBY   STATUS   AGE    SYNCED
   master   Static   1       1       1                                                               1h     True
   front    Static   2       2       2                                                               1h     True
   ```

## Как понять, что что-то пошло не так?

Если узел в NodeGroup не обновляется (значение `UPTODATE` при выполнении команды `d8 k get nodegroup` меньше значения `NODES`) или вы предполагаете какие-то другие проблемы, которые могут быть связаны с модулем `node-manager`, нужно посмотреть логи сервиса `bashible`. Сервис `bashible` запускается на каждом узле, управляемом модулем `node-manager`.

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

## Удаление узла из кластера

<span id='как-вывести-узел-из-под-управления-node-manager'></span>

{% alert level="info" %}
Инструкция справедлива как для узла, настроенного вручную (с помощью bootstrap-скрипта), так и для узла, настроенного с помощью CAPS.
{% endalert %}

Чтобы вывести из кластера узел и очистить сервер (ВМ), выполните следующую команду на узле:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```

### Как очистить узел для последующего ввода в кластер?

{% alert level="warning" %}
Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить NodeGroup, следуйте [инструкции по смене NodeGroup](#как-изменить-nodegroup-у-статического-узла).

Если на зачищаемом узле есть пулы хранения LINSTOR/DRBD, чтобы выгнать ресурсы с узла и удалить узел LINSTOR/DRBD, следуйте [соответствующей инструкции модуля `sds-replicated-volume`](https://deckhouse.ru/products/kubernetes-platform/modules/sds-replicated-volume/stable/faq.html#как-выгнать-drbd-ресурсы-с-узла).
{% endalert %}

Чтобы очистить узел для последующего ввода в кластер, выполните следующие шаги:

1. Удалите узел из кластера Kubernetes:

   ```shell
   d8 k drain <node> --ignore-daemonsets --delete-local-data
   d8 k delete node <node>
   ```

1. Запустите на узле скрипт очистки:

   ```shell
   bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
   ```

1. После перезагрузки узел можно добавить в другой кластер.

## FAQ

### Можно ли удалить StaticInstance?

StaticInstance, находящийся в состоянии `Pending`, можно удалять без каких-либо проблем.

Чтобы удалить StaticInstance, находящийся в любом состоянии отличном от `Pending` (`Running`, `Cleaning`, `Bootstrapping`):

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
1. Дождитесь `Ready` состояния NodeGroup.

### Как изменить IP-адрес StaticInstance?

Изменить IP-адрес в ресурсе StaticInstance нельзя. Если в StaticInstance указан ошибочный адрес, то нужно [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

### Как мигрировать статический узел, настроенный вручную, под управление CAPS?

Необходимо выполнить [очистку узла](#удаление-узла-из-кластера), затем [добавить](#добавление-статического-узла-с-помощью-caps) узел под управление CAPS.

### Как изменить NodeGroup у статического узла?

<span id='как-изменить-nodegroup-у-статического-узла'><span>

Если узел находится под управлением CAPS, то изменить принадлежность к NodeGroup у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

Если статический узел был добавлен в кластер вручную, то для перемещения его в другую NodeGroup необходимо изменить лейбл с именем группы и удалить лейбл с ролью:

```shell
d8 k label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
d8 k label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.

### Как посмотреть, что в данный момент выполняется на узле при его создании?

Если необходимо узнать, что происходит на узле (к примеру, он долго создается, завис в состоянии `Pending`), можно посмотреть логи `cloud-init`. Для этого выполните следующие шаги:

1. Найдите узел, который сейчас бутстрапится:

   ```shell
   d8 k get instances | grep Pending
   ```

   Пример вывода команды:

   ```console
   dev-worker-2a6158ff-6764d-nrtbj   Pending   46s
   ```

1. Получите информацию о параметрах подключения для просмотра логов:

   ```shell
   d8 k get instances dev-worker-2a6158ff-6764d-nrtbj -o yaml | grep 'bootstrapStatus' -B0 -A2
   ```

   Пример вывода команды:

   ```console
   bootstrapStatus:
     description: Use 'nc 192.168.199.178 8000' to get bootstrap logs.
     logsEndpoint: 192.168.199.178:8000
   ```

1. Выполните полученную команду (в примере выше — `nc 192.168.199.178 8000`), чтобы получить логи `cloud-init` для последующей диагностики.

   Логи первоначальной настройки узла находятся в `/var/log/cloud-init-output.log`.
