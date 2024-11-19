---
title: "Работа с etcd"
permalink: ru/virtualization-platform/documentation/admin/platform-management/control-plane-settings/etcd.html
lang: "ru"
---

## Резервное копирование etcd

### Автоматическое

Deckhouse создаёт CronJob `kube-system/d8-etcd-backup-*`, который срабатывает в 00:00 по UTC+0. Резервная копию данных ectd сохраняется в файл `/var/lib/etcd/etcd-backup.tar.gz` на всех master-узлах.

### Вручную с помощью Deckhouse CLI

В кластерах Deckhouse v1.65 и выше резервную копию данных etcd можно создать одной командой `d8 backup etcd`:

```bash
d8 backup etcd --kubeconfig $KUBECONFIG ./etcd.db
```

В текущей директории будет создан файл `etcd.db` со снимком базы etcd.
Из полученного снимка можно будет восстановить состояние кластера etcd.

Также рекомендуется сделать бэкап директории `/etc/kubernetes`, в которой находятся:
- манифесты и конфигурация компонентов [control-plane](https://kubernetes.io/docs/concepts/overview/components/#control-plane-components);
- [PKI кластера Kubernetes](https://kubernetes.io/docs/setup/best-practices/certificates/).


### Вручную с помощью etcdctl

{% alert level="warning" %}
Не рекомендуется на версиях Deckhouse 1.65 и выше.
{% endalert %}

В кластерах Deckhouse версии v1.64 и ниже запустите следующий скрипт на любом master-узле от пользователя `root`:

```bash
#!/usr/bin/env bash
set -e

pod=etcd-`hostname`
kubectl -n kube-system exec "$pod" -- /usr/bin/etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ snapshot save /var/lib/etcd/${pod##*/}.snapshot && \
mv /var/lib/etcd/"${pod##*/}.snapshot" etcd-backup.snapshot && \
cp -r /etc/kubernetes/ ./ && \
tar -cvzf kube-backup.tar.gz ./etcd-backup.snapshot ./kubernetes/
rm -r ./kubernetes ./etcd-backup.snapshot
```

В текущей директории будет создан файл `kube-backup.tar.gz` со снимком базы etcd одного из членов etcd-кластера.
Из полученного снимка можно будет восстановить состояние кластера etcd.

### Шифрование

Мы рекомендуем хранить резервные копии снимков состояния кластера etcd, а также бэкап директории `/etc/kubernetes/` в зашифрованном виде вне кластера Deckhouse.
Для этого вы можете использовать сторонние инструменты резервного копирования файлов, например [Restic](https://restic.net/), [Borg](https://borgbackup.readthedocs.io/en/stable/), [Duplicity](https://duplicity.gitlab.io/) и т. д.

## Полное восстановление состояния кластера из резервной копии etcd

Далее будут описаны шаги по восстановлению кластера до предыдущего состояния из резервной копии при полной потере данных.

### Восстановление кластера single-master

Для корректного восстановления кластера single-master выполните следующие шаги:

1. Загрузите утилиту [etcdctl](https://github.com/etcd-io/etcd/releases) на сервер (желательно чтобы её версия была такая же, как и версия etcd в кластере).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

   Посмотреть версию etcd в кластере можно выполнив следующую команду:

   ```shell
   d8 k -n kube-system exec -ti etcd-$(hostname) -- etcdctl version
   ```

1. Остановите etcd.

   Etcd запущен в виде статического пода, поэтому достаточно переместить файл манифеста:

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Сохраните текущие данные etcd.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Очистите директорию etcd.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

1. Положите резервную копию etcd в файл `~/etcd-backup.snapshot`.

1. Восстановите базу данных etcd.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

1. Запустите etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

### Восстановление кластера multi-master

Для корректного восстановления кластера multi-master выполните следующие шаги:

1. Явно включите режим High Availability (HA) с помощью глобального параметра [highAvailability](../../deckhouse-configure-global.html#parameters-highavailability). Это нужно, например, чтобы не потерять одну реплику Prometheus и её PVC, поскольку в режиме single-master HA отключен по умолчанию.

1. Переведите кластер в режим single-master, в соответствии с [инструкцией](#как-уменьшить-число-master-узлов-в-облачном-кластере-multi-master-в-single-master) для облачных кластеров или самостоятельно выведите статические master-узлы из кластера.

1. На оставшемся единственном master-узле выполните шаги по восстановлению etcd из резервной копии в соответствии с [инструкцией](#восстановление-кластера-single-master) для single-master.

1. Когда работа etcd будет восстановлена, удалите из кластера информацию об уже удаленных в п.1 master-узлах, воспользовавшись следующей командой (укажите название узла):

   ```shell
   d8 k delete node <MASTER_NODE_I>
   ```

1. Перезапустите все узлы кластера.

1. Дождитесь выполнения заданий из очереди Deckhouse:

   ```shell
   d8 k -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
   ```

1. Переведите кластер обратно в режим multi-master в соответствии с [инструкцией](#как-добавить-master-узлы-в-облачном-кластере-single-master-в-multi-master) для облачных кластеров или [инструкцией](#как-добавить-master-узел-в-статическом-или-гибридном-кластере) для статических или гибридных кластеров.

## Восстановление объекта Kubernetes из резервной копии etcd

Краткий сценарий восстановления отдельных объектов из резервной копии etcd:

1. Получить резервную копию данных.
2. Запустить временный экземпляр etcd.
3. Наполнить его данными из резервной копии.
4. Получить описания нужных объектов с помощью утилиты `etcdhelper`.


### Пример шагов по восстановлению объектов из резервной копии etcd

В примере предполагается:
- `etcd-snapshot.bin` — файл с [резервной копией](#как-сделать-бэкап-etcd-вручную) данных etcd (snapshot)
- `infra-production` — namespace, в котором нужно восстановить объекты.

1. Запустите под с временным экземпляром etcd.

Желательно, чтобы версия запускаемого экземпляра etcd совпадала с версией etcd, из которой создавалась резервная копия. Для простоты экземпляр запускается не локально, а в кластере, т.к. там заведомо есть образ etcd.

  - Подготовьте файл `etcd.pod.yaml` с манифестом пода:

    ```shell
    cat <<EOF >etcd.pod.yaml 
    apiVersion: v1
    kind: Pod
    metadata:
      name: etcdrestore
      namespace: default
    spec:
      containers:
      - command:
        - /bin/sh
        - -c
        - "sleep 96h"
        image: IMAGE
        imagePullPolicy: IfNotPresent
        name: etcd
        volumeMounts:
        - name: etcddir
          mountPath: /default.etcd
      volumes:
      - name: etcddir
        emptyDir: {}
    EOF
    ```
  - 
  - Установите актуальное имя образа etcd:
    ```shell
    IMG=`d8 k -n kube-system get pod -l component=etcd -o jsonpath="{.items[0].spec.containers[*].image}"`
    sed -i -e "s#IMAGE#$IMG#" etcd.pod.yaml
    ```

  - Создайте под:

    ```shell
    d8 k create -f etcd.pod.yaml
    ```

2. Скопируйте `etcdhelper` и снимок etcd в контейнер пода.

   `etcdhelper` можно собрать из [исходного кода](https://github.com/openshift/origin/tree/master/tools/etcdhelper) или скопировать из готового образа (например, из [образа `etcdhelper` на Docker Hub](https://hub.docker.com/r/webner/etcdhelper/tags)).

   Пример:

   ```shell
   d8 k cp etcd-snapshot.bin default/etcdrestore:/tmp/etcd-snapshot.bin
   d8 k cp etcdhelper default/etcdrestore:/usr/bin/etcdhelper
   ```

3. В контейнере установите права на запуск `etcdhelper`, восстановите данные из резервной копии и запустите etcd.

   Пример:

   ```shell
   d8 k -n default exec -it etcdrestore -- sh
   chmod +x /usr/bin/etcdhelper
   etcdctl snapshot restore /tmp/etcd-snapshot.bin
   etcd &
   ```

4. Получите описания нужных объектов кластера, отфильтровав их с помощью `grep`.

   Пример:

   ```shell
   d8 k -n default exec -it etcdrestore -- sh
   mkdir /tmp/restored_yaml
   cd /tmp/restored_yaml
   for o in `etcdhelper -endpoint 127.0.0.1:2379 ls /registry/ | grep infra-production` ; do etcdhelper -endpoint 127.0.0.1:2379 get $o > `echo $o | sed -e "s#/registry/##g;s#/#_#g"`.yaml ; done
   ```

   Замена символов с помощью `sed` в примере позволяет сохранить описания объектов в файлы, именованные подобно структуре реестра etcd. Например: `/registry/deployments/infra-production/supercronic.yaml` → `deployments_infra-production_supercronic.yaml`.

5. Скопируйте полученные описания объектов из пода на master-узел:

   ```shell
   d8 k cp default/etcdrestore:/tmp/restored_yaml restored_yaml
   ```

6. Удалите из полученных описаний объектов информацию о времени создания, UID, status и прочие оперативные данные, после чего восстановите объекты:

   ```shell
   d8 k create -f restored_yaml/deployments_infra-production_supercronic.yaml
   ```

7. Под с временным экземпляром etcd можно удалить:

   ```shell
   d8 k delete -f etcd.pod.yaml
   ```

## Получить список членов кластера etcd

Используйте команду `etcdctl member list`.

Пример:

```shell
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod -l component=etcd,tier=control-plane -o name | head -n1) -- \
etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
--cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
--endpoints https://127.0.0.1:2379/ member list -w table
```

Внимание! Последний параметр в таблице вывода показывает, что член кластера etcd находится в состоянии [**learner**](https://etcd.io/docs/v3.5/learning/design-learner/), а не в состоянии *leader*.

## Получить список членов кластера etcd (Вариант 2)

Используйте команду `etcdctl endpoint status`. Для этой команды, после флага `--endpoints` нужно подставить адрес каждого узла control-plane.

Значение `true` в пятом столбце вывода указывает на лидера.

Пример скрипта, который автоматически передает все адреса узлов control-plane:

```shell
MASTER_NODE_IPS=($(d8 k get nodes -l \
node-role.kubernetes.io/control-plane="" \
-o 'custom-columns=IP:.status.addresses[?(@.type=="InternalIP")].address' \
--no-headers))
unset ENDPOINTS_STRING
for master_node_ip in ${MASTER_NODE_IPS[@]}
do ENDPOINTS_STRING+="--endpoints https://${master_node_ip}:2379 "
done
d8 k -n kube-system exec -ti $(d8 k -n kube-system get pod \
-l component=etcd,tier=control-plane -o name | head -n1) \
-- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt  --cert /etc/kubernetes/pki/etcd/ca.crt \
--key /etc/kubernetes/pki/etcd/ca.key \
$(echo -n $ENDPOINTS_STRING) endpoint status -w table
```

## Пересборка кластера etcd

Пересборка может потребоваться в случае, если etcd-кластер развалился, либо при миграции из multi-master конфигурации в single-master.

1. Выберите узел, с которого начнётся восстановление кластера etcd. В случае миграции в single-master это узел, на котором должен остаться etcd.
2. Остановите etcd на всех остальных узлах. Для этого удалите файл `/etc/kubernetes/manifests/etcd.yaml`.
3. На оставшемся узле в манифесте `/etc/kubernetes/manifests/etcd.yaml` добавьте аргумент `--force-new-cluster` в поле `spec.containers.command`.
4. После успешного запуска кластера удалите параметр `--force-new-cluster`.

> **Внимание!** Операция деструктивна, она полностью уничтожает консенсус и запускает etcd-кластер с состояния, которое сохранилось на выбранном узле. Любые pending-записи пропадут.

## Устранение бесконечного рестарта

Данный вариант может понадобиться, если запуск с аргументом `--force-new-cluster` не восстанавливает работу etcd. Такое может случиться при неудачном converge master-узлов, когда новый master-узел был создан со старым диском etcd, поменял свой адрес из локальной сети, и другие master-узлы отсутствуют. Симптомы, при которых стоит использовать данный способ: контейнер etcd в бесконечном рестарте, в его логе ошибка: `panic: unexpected removal of unknown remote peer`.

1. Установите утилиту [etcdutl](https://github.com/etcd-io/etcd/releases).
1. С текущего локального снапшота базы etcd (`/var/lib/etcd/member/snap/db`) выполните создание нового снапшота:

   ```shell
   ./etcdutl snapshot restore /var/lib/etcd/member/snap/db --name <HOSTNAME> \
   --initial-cluster=HOSTNAME=https://<ADDRESS>:2380 --initial-advertise-peer-urls=https://ADDRESS:2380 \
   --skip-hash-check=true --data-dir /var/lib/etcdtest
   ```

   , где:
- `<HOSTNAME>` — название master-узла;
- `<ADDRESS>` — адрес master-узла.

1. Выполните команды, для использования нового снапшота:

   ```shell
   cp -r /var/lib/etcd /tmp/etcd-backup
   rm -rf /var/lib/etcd
   mv /var/lib/etcdtest /var/lib/etcd
   ```

1. Найдите контейнеры `etcd` и `kube-apiserver`:

   ```shell
   crictl ps -a --name "^etcd|^kube-apiserver"
   ```

1. Удалите найденные контейнеры `etcd` и `kube-apiserver`:

   ```shell
   crictl rm <CONTAINER-ID>
   ```

1. Перезапустите master-узел.
