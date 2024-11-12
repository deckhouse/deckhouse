---
title: "Добавление и удаление узла"
permalink: ru/virtualization-platform/documentation/admin/platform-management/node-management/adding-node.html
lang: ru
---

## Добавление статического узла в кластер

<span id="добавление-узла-в-кластер"></span>

Добавление статического узла можно выполнить вручную или с помощью Cluster API Provider Static.

### Вручную

Чтобы добавить bare-metal сервер в кластер как статический узел, выполните следующие шаги:

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

В операционных системах семейства Astra Linux, при использовании модуля мандатного контроля целостности Parsec, сконфигурируйте максимальный уровень целостности для пользователя `caps`:

     ```shell
     pdpl-user -i 63 caps
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

TODO не хватает какой-то команды, чтобы проверить, что всё ок.

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






{% raw %}



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


## Удаление узла из кластера

<span id='как-вывести-узел-из-под-управления-node-manager'></span>

> Инструкция справедлива как для узла, настроенного вручную (с помощью bootstrap-скрипта), так и для узла, настроенного с помощью CAPS.

Чтобы вывести из кластера узел и очистить сервер (ВМ), выполните следующую команду на узле:

```shell
bash /var/lib/bashible/cleanup_static_node.sh --yes-i-am-sane-and-i-understand-what-i-am-doing
```


### Как зачистить узел для последующего ввода в кластер?

Это необходимо только в том случае, если нужно переместить статический узел из одного кластера в другой. Имейте в виду, что эти операции удаляют данные локального хранилища. Если необходимо просто изменить `NodeGroup`, следуйте [этой инструкции](#как-изменить-nodegroup-у-статического-узла).

> **Внимание!** Если на зачищаемом узле есть пулы хранения LINSTOR/DRBD, то предварительно выгоните ресурсы с узла и удалите узел LINSTOR/DRBD, следуя [инструкции](/modules/sds-replicated-volume/stable/faq.html#как-выгнать-ресурсы-с-узла).

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
TODO зачем это тут? Это уже про добавление в новый кластер?

## FAQ

### Можно ли удалить StaticInstance?

`StaticInstance`, находящийся в состоянии `Pending` можно удалять без каких-либо проблем.

Чтобы удалить `StaticInstance` находящийся в любом состоянии, отличном от `Pending` (`Running`, `Cleaning`, `Bootstraping`):
1. Добавьте лейбл `"node.deckhouse.io/allow-bootstrap": "false"` в `StaticInstance`.
1. Дождитесь, пока `StaticInstance` перейдет в статус `Pending`.
1. Удалите `StaticInstance`.
1. Уменьшите значение параметра `NodeGroup.spec.staticInstances.count` на 1.

### Как изменить IP-адрес StaticInstance?

Изменить IP-адрес в ресурсе `StaticInstance` нельзя. Если в `StaticInstance` указан ошибочный адрес, то нужно [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

### Как мигрировать статический узел настроенный вручную под управление CAPS?

Необходимо выполнить [очистку узла](#как-вручную-очистить-статический-узел), затем [добавить](#как-добавить-статический-узел-в-кластер-cluster-api-provider-static) узел под управление CAPS.

### Как изменить NodeGroup у статического узла?

<span id='как-изменить-nodegroup-у-статичного-узла'><span>

Если узел находится под управлением [CAPS](./#cluster-api-provider-static), то изменить принадлежность к `NodeGroup` у такого узла **нельзя**. Единственный вариант — [удалить StaticInstance](#можно-ли-удалить-staticinstance) и создать новый.

Чтобы перенести существующий статический узел созданный [вручную](./#работа-со-статическими-узлами) из одной `NodeGroup` в другую, необходимо изменить у узла лейбл группы:

```shell
kubectl label node --overwrite <node_name> node.deckhouse.io/group=<new_node_group_name>
kubectl label node <node_name> node-role.kubernetes.io/<old_node_group_name>-
```

Применение изменений потребует некоторого времени.


### Как посмотреть, что в данный момент выполняется на узле при его создании?

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


### Что такое ресурс Instance?

Ресурс Instance содержит описание объекта эфемерной машины, без учета ее конкретной реализации. Например, машины созданные MachineControllerManager или Cluster API Provider Static будут иметь соответcтвующий ресурс Instance.

Объект не содержит спецификации. Статус содержит:
1. Ссылку на InstanceClass, если он существует для данной реализации.
1. Ссылку на объект Node Kubernetes.
1. Текущий статус машины.
1. Информацию о том, как посмотреть [логи создания машины](#как-посмотреть-что-в-данный-момент-выполняется-на-узле-при-его-создании) (появляется на этапе создания машины).

При создании/удалении машины создается/удаляется соответствующий объект Instance.
Самостоятельно ресурс Instance создать нельзя, но можно удалить. В таком случае машина будет удалена из кластера (процесс удаления зависит от деталей реализации).

{% endraw %}

### Когда требуется перезагрузка узлов?

Некоторые операции по изменению конфигурации узлов могут потребовать перезагрузки.

Перезагрузка узла может потребоваться при изменении некоторых настроек sysctl, например, при изменении параметра `kernel.yama.ptrace_scope` (изменяется при использовании команды `astra-ptrace-lock enable/disable` в Astra Linux).
