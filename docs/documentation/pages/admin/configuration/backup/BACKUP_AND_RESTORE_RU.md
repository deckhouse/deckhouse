---
title: Backup and restore
permalink: ru/admin/configuration/backup/backup-and-restore.html
lang: ru
---

## Ручное восстановление кластера

### Восстановление кластера с одним control-plane узлом

Для корректного восстановления кластера выполните следующие шаги на master-узле:

1. Подготовьте утилиту `etcdctl`. Найдите и скопируйте исполняемый файл на узле:

   ```shell
   cp $(find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/ \
   -name etcdctl -print | tail -n 1) /usr/local/bin/etcdctl
   ```

   Проверьте версию `etcdctl`:

   ```shell
   etcdctl version
   ```

   Убедитесь, что результат команды `etcdctl version` отображается без ошибок.

   **При отсутствии** `etcdctl` скачайте исполняемый файл с [официального репозитория etcd](https://github.com/etcd-io/etcd/releases), выбрав версию, которая соответствует версии etcd в кластере:

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.16/etcd-v3.5.16-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.16-linux-amd64.tar.gz && mv etcd-v3.5.16-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Проверьте версию etcd в кластере (при доступном Kubernetes API) выполнив команду:

   ```shell
   kubectl -n kube-system exec -ti etcd-$(hostname) -- etcdctl version
   ```

   Если команда выполнится успешно, вы увидите актуальную версию etcd.

1. Остановите etcd. Переместите манифест etcd, чтобы kubelet прекратил запуск соответствующего пода с помощью команды:

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Убедитесь, что под etcd остановлен:

   ```shell
   crictl ps | grep etcd
   ```

   Если команда не возвращает данных о запущенном поде etcd, значит он успешно остановлен.

1. Сохраните текущие данные etcd. Создайте резервную копию текущего состояния каталога `member`:

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

   В случае непредвиденных обстоятельств вы сможете вернуться к этим файлам.

1. Очистите директорию etcd. Удалите старые данные, чтобы подготовить etcd к восстановлению из резервной копии:

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

   Проверьте, что каталог `/var/lib/etcd/member/` теперь пуст или отсутствует:

   ```shell
   ls -la /var/lib/etcd/member/
   ```

1. Поместите файл резервной копии etcd. Скопируйте или перенесите файл снапшота `etcd-backup.snapshot` в домашнюю директорию текущего пользователя (root):

   ```shell
   cp /путь/до/резервной/копии/etcd-backup.snapshot ~/etcd-backup.snapshot
   ```

   Убедитесь, что файл доступен для чтения:

   ```shell
   ls -la ~/etcd-backup.snapshot
   ```

1. Восстановите базу данных etcd из резервной копии. Воспользуйтесь `etcdctl` для восстановления:

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

   После завершения команды проверьте, что в каталоге `/var/lib/etcd/` появились файлы, соответствующие восстановленному состоянию.

1. Запустите etcd. Верните манифест etcd в рабочую директорию, чтобы kubelet вновь запустил под etcd:

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. Дождитесь, пока под будет создан и перейдёт в состояние `Running`. Убедитесь, что он действительно запущен:

   ```shell
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

   Процесс запуска может занять некоторое время. После успешного старта etcd кластер будет восстановлен из резервной копии.

### Восстановление мультимастерного кластера

Для корректного восстановления мультимастерного кластера выполните следующие шаги:

1. Активируйте режим High Availability (HA). Для этого используйте параметр [highAvailability](TODO). Это необходимо, чтобы сохранить хотя бы одну реплику Prometheus и его PVC, поскольку в кластере с одним master-узлом HA по умолчанию отключён.

1. Переведите кластер в режим с одним master-узлом:

   - В облачном кластере воспользуйтесь [инструкцией].
   - В статическом кластере удалите лишние master-узлы вручную.

1. Восстановите etcd из резервной копии на единственном оставшемся master-узле. Следуйте [инструкции](#восстановление-кластера-с-одним-control-plane-узлом) для кластера с одним control-plane узлом.

1. Когда работа etcd будет восстановлена, удалите из кластера информацию об уже удаленных в первом пункте master-узлах, воспользовавшись следующей командой (укажите название узла):

   ```shell
   kubectl delete node <ИМЯ_MASTER_УЗЛА>
   ```

1. Перезапустите все узлы кластера. Убедитесь, что после перезагрузки все узлы доступны и работают корректно.

1. Дождитесь выполнения заданий из очереди Deckhouse:

   ```shell
   kubectl -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller queue main
   ```

1. Переведите кластер обратно в мультимастерный режим:

   - Для облачных кластеров используйте [инструкцию](TODO).
   - Для статических и гибридных кластеров воспользуйтесь [инструкцией](TODO).

   После этих шагов кластер будет успешно восстановлен в мультимастерной конфигурации.

## Восстановление отдельных объектов

### Восстановление объектов Kubernetes из резервной копии etcd

Чтобы восстановить отдельные объекты кластера (например, конкретные Deployment, Secret или ConfigMap) из резервной копии etcd, выполните следующие шаги:

1. Запустите временный экземпляр etcd. Создайте отдельную копию etcd, которая будет работать независимо от основного кластера.
1. Загрузите данные из резервного снимка. Используйте существующий файл снимка (snapshot) etcd, чтобы заполнить временный экземпляр нужными данными.
1. Выгрузите необходимые объекты в формате JSON. Выберите конкретные ресурсы (по их ключам в etcd), а затем сохраните их описания в JSON-файлах.

Данные действия можно произвести как [с помощью скрипта](#автоматизированная-выгрузка-объектов), так и [вручную](#ручная-выгрузка-объектов).

### Автоматизированная выгрузка объектов

Чтобы автоматически выгрузить объекты кластера из резервной копии etcd, воспользуйтесь представленным скриптом. Перед запуском убедитесь, что заданы необходимые настройки:  

- Путь до резервного снимка (snapshot) etcd. Укажите в скрипте корректное расположение файла `etcd-backup.snapshot`.
- Директория для выгружаемых JSON-файлов. Определите, куда именно будут сохраняться выгруженные манифесты объектов кластера.
- Фильтр отбора объектов (grep). При необходимости задайте строку для фильтрации путей в etcd, чтобы выгрузить лишь нужные ресурсы, например по названию пространства имён или типу ресурса.

{% offtopic title="Скрипт выгрузки объектов" %}
```shell
BACKUP_OUTPUT_DIR="/tmp/etc_restore" # Путь до директории куда будут выгружены объекты (создастся автоматически).
ETCD_SNAPSHOT_PATH="./etcd-backup.snapshot" # Путь до резервного снимка (snapshot) etcd.
FILTER="verticalpodautoscalers" # Фильтр отбора объектов (grep) для отбора записей .

IMG=$(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')

kubectl delete po etcd-restore --force || true
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: etcd-restore
spec:
  volumes:
  - name: shared-data
    emptyDir: {}  
  - name: etcddir
    emptyDir: {}
  containers:
  - name: etcd
    image: $IMG
    volumeMounts:
    - name: shared-data
      mountPath: /etcd-backup
    - name: etcddir
      mountPath: /default.etcd      
  
  - name: ubuntu
    image: ubuntu:latest
    command: ["/bin/sh", "-c", "sleep 100h"]
    volumeMounts:
    - name: shared-data
      mountPath: /etcd-backup

  restartPolicy: Never
EOF

kubectl wait --for=condition=Ready pod etcd-restore
kubectl cp  $ETCD_SNAPSHOT_PATH etcd-restore:/etcd-backup -c ubuntu
kubectl exec -t etcd-restore -c etcd -- etcdctl snapshot restore /etcd-backup/etcd-backup.snapshot --data-dir=./default.etcd_new
kubectl exec -t etcd-restore -c etcd -- etcd   --name temp   --data-dir /default.etcd_new   --advertise-client-urls http://localhost:12379   --listen-client-urls http://localhost:12379 --listen-peer-urls http://localhost:12380 &

mkdir -p $BACKUP_OUTPUT_DIR
files=($(kubectl exec -t etcd-restore -c etcd  -- etcdctl  --endpoints=localhost:12379 get / --prefix --keys-only | grep "$FILTER" )) 
for file in "${files[@]}"
do
  OBJECT=$(kubectl exec -t etcd-restore -c etcd  -- etcdctl  --endpoints=localhost:12379 get "$file" --write-out=json)
  VALUE=$(echo $OBJECT | jq -r '.kvs[0].value' | base64 --decode | jq )
  DIR=$(dirname "$file")
  FILE=$(basename "$file")
  mkdir -p "$BACKUP_OUTPUT_DIR/$DIR"
  echo "$VALUE" > "$BACKUP_OUTPUT_DIR/$DIR/$FILE.json"
  echo $BACKUP_OUTPUT_DIR/$DIR/$FILE.json
done
kubectl delete po etcd-restore --force
```
{% endofftopic %}

### Ручная выгрузка объектов

Нижеописанные шаги позволят вручную запустить временный экземпляр etcd, восстановить в него данные из снапшота (snapshot) и выгрузить в JSON-файлы только те объекты, которые вам необходимы.

1. Подготовьте временный под с контейнерами `etcd` и `ubuntu` с помощью шаблона `etcd.pod.yaml`. Шаблон содержит два контейнера:

   - `etcd` — должен соответствовать версии etcd, из которой был создан резервный снимок (snapshot).
   - `ubuntu` — вспомогательный контейнер для отладочных целей (в современных образах etcd может отсутствовать оболочка `bash` или `sh`).

   Подставьте в шаблон актуальную версию образа etcd (аналогичную оригинальному кластеру) и создайте под с помощью команды:

   ```shell
   IMG=$(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
   sed -i -e "s#ETCD_IMAGE#$IMG#" etcd.pod.yaml
   kubectl create -f etcd.pod.yaml
   ```

   {% offtopic title="Пример шаблона" %}
   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: etcd-restore
   spec:
     volumes:
     - name: shared-data
       emptyDir: {}  
     - name: etcddir
       emptyDir: {}
     containers:
     - name: etcd
       image: ETCD_IMAGE
       volumeMounts:
       - name: shared-data
         mountPath: /etcd-backup
       - name: etcddir
         mountPath: /default.etcd      
  
     - name: ubuntu
       image: ubuntu:latest
       command: ["/bin/sh", "-c", "sleep 100h"]
       volumeMounts:
       - name: shared-data
         mountPath: /etcd-backup

     restartPolicy: Never
   ```
   {% endofftopic %}

1. Скопируйте резервный снимок etcd в контейнер временного пода. Используйте команду `kubectl cp`, чтобы передать файл снапшота (например, `etcd-snapshot.bin`) в контейнер ubuntu:

   ```shell
   kubectl cp etcd-snapshot.bin etcd-restore:/etcd-backup -c ubuntu
   ```

   Теперь файл резервной копии будет доступен контейнеру etcd.

1. Восстановите данные из снапшота в новую директорию. Запустите команду внутри контейнера etcd, указав путь к загруженному файлу и новую директорию для восстановления:

   ```shell
   kubectl exec -t etcd-restore -c etcd -- etcdctl snapshot restore /etcd-backup/etcd-backup.snapshot --data-dir=./default.etcd_new
   ```

   По завершении команды данные из снапшота будут развернуты в директории `./default.etcd_new`.

1. Запустите дополнительный экземпляр etcd на нестандартном порту на основе восстановленных данных. Чтобы не конфликтовать с основным etcd в кластере, используйте другую директорию данных и порты:

   ```shell
   kubectl exec -t etcd-restore -c etcd -- etcd   --name temp   --data-dir /default.etcd_new   --advertise-client-urls http://localhost:12379   --listen-client-urls http://localhost:12379 --listen-peer-urls http://localhost:12380 &
   ```

   > **Внимание.** В примере сервис запускается в фоновом режиме напрямую из командной строки. Используйте такой запуск только для процесса восстановления данных.

1. Выберите и выгрузите нужные объекты кластера:

   - Определите фильтр для `grep`, чтобы выгрузить только нужные ресурсы (например, определённый namespace или тип ресурсов).
   - Создайте каталог для сохранения JSON-файлов и выгрузите объекты в цикле:

      ```shell
      FILTER="verticalpodautoscalers"
      BACKUP_OUTPUT_DIR="/tmp/etc_restore"
      mkdir -p $BACKUP_OUTPUT_DIR
      files=($(kubectl exec -t etcd-restore -c etcd  -- etcdctl  --endpoints=localhost:12379 get / --prefix --keys-only | grep "$FILTER" )) 
      for file in "${files[@]}"
      do
        OBJECT=$(kubectl exec -t etcd-restore -c etcd  -- etcdctl  --endpoints=localhost:12379 get "$file" --write-out=json)
        VALUE=$(echo $OBJECT | jq -r '.kvs[0].value' | base64 --decode | jq )
        DIR=$(dirname "$file")
        FILE=$(basename "$file")
        mkdir -p "$BACKUP_OUTPUT_DIR/$DIR"
        echo "$VALUE" > "$BACKUP_OUTPUT_DIR/$DIR/$FILE.json"
        echo $BACKUP_OUTPUT_DIR/$DIR/$FILE.json
      done
      ```

   - По завершении в каталоге `$BACKUP_OUTPUT_DIR` появятся выгруженные ресурсы в формате JSON.

1. Удалите вспомогательный под с etcd командой:

   ```shell
   kubectl delete po etcd-restore --force
   ```

   Под будет остановлен, а ресурсы, выгруженные в JSON-файлы, останутся доступными для дальнейшего восстановления в основном кластере.

### Восстановление объектов Kubernetes из выгруженных JSON-файлов

Для восстановления объектов выполните следующие шаги:

1. Подготовьте JSON-файлы к восстановлению. Перед тем как загружать объекты обратно в кластер, удалите из их описаний технические поля, которые могли устареть или нарушить процесс восстановления:

   - `creationTimestamp`;
   - `UID`;
   - `status`.

   Редактирование можно выполнить вручную или с помощью утилит для обработки YAML/JSON (например, `yq` или `jq`).

1. Создайте объекты в кластере. Для восстановления ресурсов выполните команду:

   ```shell
   kubectl create -f <ПУТЬ_К_ФАЙЛУ>.json
   ```

   При необходимости можно указать путь к конкретному файлу или каталогу.

1. Если нужно массово восстановить сразу несколько объектов, воспользуйтесь утилитой `find`:

   ```shell
   find $BACKUP_OUTPUT_DIR -type f -name "*.json" -exec kubectl create -f {} \;
   ```

   Эта команда найдёт все .json-файлы в заданном каталоге `$BACKUP_OUTPUT_DIR` и поочерёдно применит к ним `kubectl create`.

После выполнения этих шагов выбранные объекты будут воссозданы в кластере согласно описаниям из JSON-файлов.

## Восстановление при смене IP-адреса master-узла

{% alert level="warning" %}
Этот раздел описывает ситуацию, когда меняется только IP-адрес master-узла, а все остальные объекты в резервной копии etcd (например, CA-сертификаты) остаются валидными. Предполагается, что восстановление выполняется в кластере с одним master-узлом.
{% endalert %}

Для восстановления объектов из резервной копии etcd при смене IP-адреса выполните шаги:

1. Восстановите etcd из резервной копии. Следуйте стандартной процедуре восстановления etcd с использованием снапшота. Убедитесь, что на этапе восстановления вы не изменяете никаких других параметров, кроме данных etcd.
1. Обновите IP-адрес в статичных конфигурационных файлах^

   - Проверьте файлы манифестов компонентов Kubernetes, расположенные в `/etc/kubernetes/manifests/`.
   - Проверьте системные настройки kubelet (файлы в `/etc/systemd/system/kubelet.service.d/` или аналогичные директории).
   - При необходимости измените IP-адрес и в других конфигурациях, если они ссылаются на старый адрес.
1. Перевыпустите сертификаты, выданные для старого IP. Удалите или переместите старые сертификаты, связанные с apiserver и, при необходимости, с etcd. Сгенерируйте новые сертификаты, указав в качестве SAN (Subject Alternative Name) новый IP-адрес master-узла.
1. Перезапустите все сервисы, использующие обновлённые конфигурации и сертификаты. Заставьте kubelet перезапустить манифесты control-plane (API-сервер, etcd и т.д.). Перезагрузите системные службы (например, `systemctl restart kubelet`) или убедитесь, что все нужные процессы перезапущены автоматически.
1. Дождитесь, пока kubelet обновит собственный сертификат.

Данные действия можно произвести как [автоматизировано](автоматизированная-выгрузка-объектов-при-смене-ip-адреса) — с помощью скрипта, так и [вручную](ручное-восстановление) — путем выполнения одиночных команд.

### Автоматизированная выгрузка объектов при смене IP-адреса

Чтобы упростить процесс восстановления кластера при смене IP-адреса master-узла, воспользуйтесь готовым скриптом, представленным ниже. Перед запуском скрипта:  

1. Укажите корректные пути и IP-адреса:
   - `ETCD_SNAPSHOT_PATH` — путь до резервного снапшота etcd.
   - `OLD_IP` — старый IP-адрес master-узла, под которым создавалась резервная копия.
   - `NEW_IP` — новый IP-адрес master-узла.
1. Убедитесь, что версия Kubernetes (`KUBERNETES_VERSION`) совпадает с установленной в кластере. Это необходимо для корректной загрузки соответствующей версии kubeadm.

   {% offtopic title="Скрипт для выгрузки объектов" %}
   ```shell
   ETCD_SNAPSHOT_PATH="./etcd-backup.snapshot" # Путь до резервного снимка (snapshot) etcd.
   OLD_IP=10.242.32.34                         # IP-адрес старого master-узла.
   NEW_IP=10.242.32.21                         # IP-адрес нового master-узла.
   KUBERNETES_VERSION=1.28.0                   # Версия Kubernetes.

   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml 
   mkdir ./etcd_old
   mv /var/lib/etcd ~/etcd_old
   ETCDCTL_PATH=$(find /var/lib/containerd/ -name etcdctl)

   ETCDCTL_API=3 $ETCDCTL_PATH snapshot restore etcd-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd 

   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml

   find /etc/kubernetes/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
   find /etc/systemd/system/kubelet.service.d -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
   find  /var/lib/bashible/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'

   mkdir -p ./old_certs/etcd
   mv /etc/kubernetes/pki/apiserver.* ./old_certs/
   mv /etc/kubernetes/pki/etcd/server.* ./old_certs/etcd/
   mv /etc/kubernetes/pki/etcd/peer.* ./old_certs/etcd/

   curl -LO https://dl.k8s.io/v$KUBERNETES_VERSION/bin/linux/amd64/kubeadm
   chmod +x kubeadm
   ./kubeadm init phase certs all --config /etc/kubernetes/deckhouse/kubeadm/config.yaml

   crictl ps --name 'kube-apiserver' -o json | jq -r '.containers[0].id' | xargs crictl stop
   crictl ps --name 'kubernetes-api-proxy' -o json | jq -r '.containers[0].id' | xargs crictl stop
   crictl ps --name 'etcd' -o json | jq -r '.containers[].id' | xargs crictl stop

   systemctl daemon-reload
   systemctl restart kubelet.service
   ```
   {% endofftopic %}

1. После выполнения скрипта необходимо дождаться, пока kubelet обновит свой сертификат, учитывающий новый IP-адрес. Проверить это можно в директории `/var/lib/kubelet/pki/`, где должен появиться новый сертификат.

### Ручное восстановление объектов при смене IP-адреса

Если вы хотите вручную внести изменения при восстановлении кластера с обновлённым IP-адресом master-узла, выполните следующие действия:

1. Восстановите etcd из резервной копии:

   - Переместите манифест etcd так, чтобы kubelet остановил соответствующий под:

     ```shell
     mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
     ```

   - Создайте директорию для резервного хранения прежних данных:

     ```shell
     mkdir ./etcd_old
     mv /var/lib/etcd ./etcd_old
     ```

   - Найдите или скачайте утилиту `etcdctl`, если она не установлена, и выполните восстановление из снапшота:

     ```shell
     ETCD_SNAPSHOT_PATH="./etcd-backup.snapshot" # Путь до резервного снимка etcd.
     ETCDCTL_PATH=$(find /var/lib/containerd/ -name etcdctl)

     ETCDCTL_API=3 $ETCDCTL_PATH snapshot restore \
       etcd-backup.snapshot \
       --cacert /etc/kubernetes/pki/etcd/ca.crt \
       --cert /etc/kubernetes/pki/etcd/ca.crt \
       --key /etc/kubernetes/pki/etcd/ca.key \
       --endpoints https://127.0.0.1:2379/ \
       --data-dir=/var/lib/etcd
     ```

   - Верните манифест etcd на место, чтобы kubelet снова запустил pod:

     ```shell
     mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
     ```

   - Убедитесь, что etcd успешно запустился, проверив список подов с помощью `crictl ps | grep etcd` или просмотрев логи kubelet.

1. Обновите IP-адреса в статичных конфигурационных файлах.  Если в манифестах или системных сервисах kubelet прописан старый IP-адрес, замените его на новый:

    ```shell
    OLD_IP=10.242.32.34                         # Старый IP-адрес master-узла
    NEW_IP=10.242.32.21                         # # Новый IP-адрес master-узла.

    find /etc/kubernetes/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    find /etc/systemd/system/kubelet.service.d -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    find  /var/lib/bashible/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    ```

1. Перевыпустите сертификаты, выпущенные для старого IP-адреса:

   - Подготовьте каталог для временного хранения старых сертификатов:

      ```shell
      mkdir -p ./old_certs/etcd
      mv /etc/kubernetes/pki/apiserver.* ./old_certs/
      mv /etc/kubernetes/pki/etcd/server.* ./old_certs/etcd/
      mv /etc/kubernetes/pki/etcd/peer.* ./old_certs/etcd/
      ```

   - Установите или скачайте kubeadm в соответствии с текущей версией Kubernetes:

     ```shell
     KUBERNETES_VERSION=1.28.0 # Версия Kubernetes.
     curl -LO https://dl.k8s.io/v$KUBERNETES_VERSION/bin/linux/amd64/kubeadm
     chmod +x kubeadm
     ```

   - Сгенерируйте новые сертификаты:

     ```shell
     ./kubeadm init phase certs all --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
     ```

     В созданных сертификатах будет учтён новый IP-адрес.

1. Перезапустите сервисы, использующие обновлённые конфигурации и сертификаты. Для немедленного прекращения работы запущенных контейнеров выполните:

    ```shell
    crictl ps --name 'kube-apiserver' -o json | jq -r '.containers[0].id' | xargs crictl stop
    crictl ps --name 'kubernetes-api-proxy' -o json | jq -r '.containers[0].id' | xargs crictl stop
    crictl ps --name 'etcd' -o json | jq -r '.containers[].id' | xargs crictl stop

    systemctl daemon-reload
    systemctl restart kubelet.service
    ```

    kubelet перезапустит соответствующие поды, а компоненты Kubernetes загрузят новые сертификаты.

1. Дождитесь, пока kubelet обновит собственный сертификат. kubelet автоматически генерирует и обновляет свой сертификат, в котором будет прописан новый IP-адрес:
  
   - Проверьте директорию `/var/lib/kubelet/pki/`.
   - Убедитесь, что новый сертификат присутствует и валиден.

   После выполнения всех вышеперечисленных шагов кластер будет успешно восстановлен и продолжит работу с новым IP-адресом master-узла.

## Создание резервных копий с помощью Deckhouse CLI

Deckhouse CLI (`d8`) предоставляет команду `backup` для создания резервных копий различных компонентов кластера:

- `etcd` — снимок ключевого хранилища данных Deckhouse;
- `cluster-config` — архив с ключевыми конфигурационными объектами кластера;
- `loki` — выгрузка логов из встроенного API Loki.

### Резервное копирование etcd

Снимок etcd позволяет сохранить текущее состояние кластера на уровне key-value хранилища. Это полный дамп, который можно использовать для восстановления.

Для создания резервной копии выполните команду:

```console
d8 backup etcd <путь-до-снапшота> [флаги]
```

Флаги:

- `-p`, `--etcd-pod string` — имя пода etcd, с которого необходимо снять снимок;
- `-h`, `--help` — показать справку по команде etcd;
- `--verbose` — подробный (расширенный) вывод логов.

Пример:

```console
d8 backup etcd mybackup.snapshot
```

Пример вывода команды:

```console
2025/04/22 08:38:58 Trying to snapshot etcd-sandbox-master-0
2025/04/22 08:39:01 Snapshot successfully taken from etcd-sandbox-master-0
```

#### Автоматическое резервное копирование etcd

Deckhouse выполняет ежедневную резервную копию etcd автоматически с помощью CronJob. Задание запускается в поде `d8-etcd-backup` в пространстве имён `kube-system`.

Внутри пода выполняются команды:

```console
etcdctl snapshot save etcd-backup.snapshot
tar -czvf etcd-backup.tar.gz etcd-backup.snapshot
```

Итоговый архив `etcd-backup.tar.gz` сохраняется локально на узле в директории `/var/lib/etcd/`:

```console
mv etcd-backup.tar.gz /var/lib/etcd/etcd-backup.tar.gz
```

### Резервное копирование конфигурации кластера

Команда `d8 backup cluster-config` создаёт архив с набором ключевых ресурсов, относящихся к конфигурации кластера. Это не полная резервная копия всех объектов, а определённый whitelist.

Для создания резервной копии выполните команду:

```console
d8 backup cluster-config <путь-до-резервной-копии>
```

Пример:

```console
d8 backup cluster-config /backup/cluster-config-2025-04-21.tar
```

В архив включаются только те объекты, которые соответствуют следующим критериям:

- Объекты CustomResource, чьи CRD помечены аннотацией:

  ```console
  backup.deckhouse.io/cluster-config=true
  ```

- StorageClass'ы, имеющие лейбл:

  ```console
  heritage=deckhouse
  ```

- Секреты и ConfigMap'ы, из пространств имён, начинающихся на `d8-` или `kube-`, если они явно перечислены в файле whitelist.

- Роли и биндинги уровня кластера (ClusterRole и ClusterRoleBinding), если они не помечены меткой:

  ```console
  heritage=deckhouse
  ```

> Резервная копия включает только объекты CR, но не сами определения CRD. Для полного восстановления кластера CRD должны быть заранее установлены (например, из манифестов модулей Deckhouse).

Пример содержимого whitelist:

| Пространство имён   | Объект     | Название                                           |
|---------------------|------------|----------------------------------------------------|
| `d8-system`         | Secret     | `d8-cluster-terraform-state`                      |
|                     |            | <span title="Строка интерпретируется как регулярное выражение и охватывает все секреты с именем, начинающимся на d8-node-terraform-state-."><code style="color:#d63384">$regexp:^d8-node-terraform-state-(.*)$</code></span> |
|                     |            | `deckhouse-registry`                              |
|                     | ConfigMap  | `d8-deckhouse-version-info`                       |
| `kube-system`       | ConfigMap  | `d8-cluster-is-bootstraped`                       |
|                     |            | `d8-cluster-uuid`                                 |
|                     |            | `extension-apiserver-authentication`              |
|                     | Secret     | `d8-cloud-provider-discovery-data`                |
|                     |            | `d8-cluster-configuration`                        |
|                     |            | `d8-cni-configuration`                            |
|                     |            | `d8-control-plane-manager-config`                 |
|                     |            | `d8-node-manager-cloud-provider`                  |
|                     |            | `d8-pki`                                          |
|                     |            | `d8-provider-cluster-configuration`               |
|                     |            | `d8-static-cluster-configuration`                 |
|                     |            | `d8-secret-encryption-key`                        |
| `d8-cert-manager`   | Secret     | `cert-manager-letsencrypt-private-key`            |
|                     |            | `selfsigned-ca-key-pair`                          |

### Выгрузка логов из Loki

Команда `d8 backup loki` предназначена для выгрузки логов из встроенного Loki. Это не полноценныая резервная копия, а лишь диагностическая выгрузка: полученные данные нельзя восстановить обратно в Loki.

Для создания резервной копии выполните команду:

```console
d8 backup loki [флаги]
```

Флаги:

- `--start`, `--end` — временные метки в формате "YYYY-MM-DD HH:MM:SS";
- `--days` — ширина временного окна выгрузки (по умолчанию 5 дней);
- `--limit` — максимум строк в одном запросе (по умолчанию 5000).

Список доступных флагов можно получить через команду `d8 backup loki --help`.
