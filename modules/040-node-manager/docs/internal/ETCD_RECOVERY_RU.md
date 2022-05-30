# Восстановление работоспособности ETCD

Перед любой попыткой восстановления выполните резервное копирование файлов etcd на узле.
Перед этим убедитесь, что etcd не запущен. Чтобы остановить etcd, переместите манифест static-Pod'а etcd из директории с манифестами.

## Single-master

### Ошибка "Failed to find database snapshot file (snap: snapshot file doesn't exist)"

Ошибка может появиться после перезапуска etcd, когда etcd выработал лимит `quota-backend-bytes`.

Пример логов:

```
{"level":"warn","ts":"2022-05-24T08:38:01.886Z","caller":"snap/db.go:88","msg":"failed to find [SNAPSHOT-INDEX].snap.db","snapshot-index":40004,"snapshot-file-path":"/var/lib/etcd/member/snap/0000000000009c44.snap.db","error":"snap: snapshot file doesn't exist"}
{"level":"panic","ts":"2022-05-24T08:38:01.886Z","caller":"etcdserver/server.go:515","msg":"failed to recover v3 backend from snapshot","error":"failed to find database snapshot file (snap: snapshot file doesn't exist)","stacktrace":"go.etcd.io/etcd/server/v3/etcdserver.NewServer\n\t/go/src/go.etcd.io/etcd/release/etcd/server/etcdserver/server.go:515\ngo.etcd.io/etcd/server/v3/embed.StartEtcd\n\t/go/src/go.etcd.io/etcd/release/etcd/server/embed/etcd.go:245\ngo.etcd.io/etcd/server/v3/etcdmain.startEtcd\n\t/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/etcd.go:228\ngo.etcd.io/etcd/server/v3/etcdmain.startEtcdOrProxyV2\n\t/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/etcd.go:123\ngo.etcd.io/etcd/server/v3/etcdmain.Main\n\t/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/main.go:40\nmain.main\n\t/go/src/go.etcd.io/etcd/release/etcd/server/main.go:32\nruntime.main\n\t/go/gos/go1.16.15/src/runtime/proc.go:225"}
panic: failed to recover v3 backend from snapshot

goroutine 1 [running]:
go.uber.org/zap/zapcore.(*CheckedEntry).Write(0xc0000d40c0, 0xc0001427c0, 0x1, 0x1)
	/go/pkg/mod/go.uber.org/zap@v1.17.0/zapcore/entry.go:234 +0x58d
go.uber.org/zap.(*Logger).Panic(0xc000100230, 0x1234726, 0x2a, 0xc0001427c0, 0x1, 0x1)
	/go/pkg/mod/go.uber.org/zap@v1.17.0/logger.go:227 +0x85
go.etcd.io/etcd/server/v3/etcdserver.NewServer(0x7ffe9e0e4e4d, 0xf, 0x0, 0x0, 0x0, 0x0, 0xc00014e900, 0x1, 0x1, 0xc00014eb40, ...)
	/go/src/go.etcd.io/etcd/release/etcd/server/etcdserver/server.go:515 +0x1656
go.etcd.io/etcd/server/v3/embed.StartEtcd(0xc0000da000, 0xc0000da600, 0x0, 0x0)
	/go/src/go.etcd.io/etcd/release/etcd/server/embed/etcd.go:245 +0xef8
go.etcd.io/etcd/server/v3/etcdmain.startEtcd(0xc0000da000, 0x12089be, 0x6, 0xc000126201, 0x2)
	/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/etcd.go:228 +0x32
go.etcd.io/etcd/server/v3/etcdmain.startEtcdOrProxyV2(0xc00003a160, 0x15, 0x16)
	/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/etcd.go:123 +0x257a
go.etcd.io/etcd/server/v3/etcdmain.Main(0xc00003a160, 0x15, 0x16)
	/go/src/go.etcd.io/etcd/release/etcd/server/etcdmain/main.go:40 +0x13f
main.main()
	/go/src/go.etcd.io/etcd/release/etcd/server/main.go:32 +0x45
```

Согласно [issue](https://github.com/etcd-io/etcd/issues/11949), подобная ошибка может появиться также и после некорректного завершения работы etcd.

#### Решение 1

Решение основано на [issue](https://github.com/etcd-io/etcd/issues/11949#issuecomment-1029906679) и состоит из следующих шагов:

- остановите etcd: 
  
  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- выполните резервное копирование файлов:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- увеличьте параметр `quota-backend-bytes` в манифесте `~/etcd.yaml`, если это необходимо;
- удалите snap-файлы:

  ```shell
  rm /var/lib/etcd/member/snap/*.snap
  ```

- попытайтесь запустить etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- если ошибка не появляется, проверьте статус:

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \ 
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table'
  ```

- если получили ошибку `alarm:NOSPACE`, то выполните:

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ alarm disarm'
  ```

- дефрагментируйте etcd, если это необходимо:

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s'
  ```

#### Решение 2

Следующим вариантом решения можно воспользоваться, если первый вариант решения не помог.

Выполните следующие шаги:

- загрузите на сервер [etcdctl](https://github.com/etcd-io/etcd/releases) (желательно той же версии, что и на версия etc на сервере);
- остановите etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- увеличьте `quota-backend-bytes` в манифесте `~/etcd.yaml`, если это необходимо;
- выполните резервное копирование файлов:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- удалите папку c данными:

  ```shell
  rm -rf /var/lib/etcd/
  ```

- восстановите базу etcd:

  ```shell
  ETCDCTL_API=3 etcdctl snapshot restore /var/lib/etcd-backup/member/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
    --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
  ```

- попытайтесь запустить etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- если ошибка не появляется, то проверьте статус:

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ endpoint status -w table
  ```

- если получили ошибку `alarm:NOSPACE`, то выполните:

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ alarm disarm
  ```

- дефрагментируйте etcd, если это необходимо:

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
  ```

### Восстановление из резервной копии

Для восстановления из резервной копии выполните следующие шаги:
- загрузите на сервер [etcdctl](https://github.com/etcd-io/etcd/releases) (желательно той же версии, что и на версия etc на сервере);
- остановите etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- выполните резервное копирование файлов:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- удалите папку c данными:

  ```shell
  rm -rf /var/lib/etcd/
  ```

- восстановите базу etcd:

  ```shell
  ETCDCTL_API=3 etcdctl snapshot restore BACKUP_FILE --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
    --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
  ```

- запустите etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

## Multi-master

### Потеря кворума

При потере кворума в etcd-кластере, выполните следующие шаги:
- добавьте в манифест `/etc/kubernetes/manifests/etcd.yaml` аргумент `--force-new-cluster` на оставшемся узле;
- подождите запуска etcd;
- удалите аргумент `--force-new-cluster` из манифеста `/etc/kubernetes/manifests/etcd.yaml`. 

Если узлы были потеряны безвозвратно, то добавьте новые узлы в облаке с помощью команды `dhctl converge` или вручную, если кластер статический.

Если узлы возвратились, то они уже не являются членами кластера.

На узлах, которые не являются членами кластера выполните следующие операции:

- остановите etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- выполните резервное копирование файлов:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- удалите папку c данными:

  ```shell
  rm -rf /var/lib/etcd/
  ```

Переключитесь на рабочий кластер и перезапустите DaemonSet `d8-control-plane-manager`:

```shell
kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
```

Ожидайте переката всех Pod'ов control-plane и переход их в состояние `Ready`.

Убедитесь, что все инстансы etcd стали участниками кластера:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

### Полная потеря данных и восстановление из резервной копии

При полной потере данных, выполните на всех узлах etcd-кластера следующие шаги:
- остановите etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- выполните резервное копирование файлов:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- удалите папку c данными:

  ```shell
  rm -rf /var/lib/etcd/
  ```

Выберите произвольный узел и выполните на нем следующие шаги:
- загрузите на сервер [etcdctl](https://github.com/etcd-io/etcd/releases) (желательно той же версии, что и на версия etc на сервере);
- восстановите базу `ETCDCTL_API=3 etcdctl snapshot restore BACKUP_FILE --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check`
- добавьте в манифест `~/etcd.yaml` аргумент `--force-new-cluster`;
- попытайтесь запустить etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- удалите аргумент `--force-new-cluster` из манифеста `/etc/kubernetes/manifests/etcd.yaml`; 
- перезапустите DaemonSet `d8-control-plane-manager`:

  ```shell
  kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
  ```

Ожидайте переката всех Pod'ов control-plane и переход их в состояние `Ready`.

Убедитесь, что все инстансы etcd стали участниками кластера:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```
