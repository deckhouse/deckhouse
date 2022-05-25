# Восстановление работоспособности ETCD
Перед любой попыткой восстановления сделайте бекап файлов etcd на ноде.
Перед этим убедитесь, что etcd не запущен. Чтобы остановить etcd, переместите манифест статик-пода
etcd из директории с манифестами.

Остановка etcd:
```bash
mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
```

Пример бекапа файлов:
```bash
cp -R /var/lib/etcd/ /var/lib/etcd-backup
```

Старт etcd после попытки восстановления:
```bash
mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
```

## Single-master

### Ситуация 1
Etcd выработал лимит `quota-backend-bytes` и после перезапуска в логах видим ошибку:
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
Исходя из [issue](https://github.com/etcd-io/etcd/issues/11949) такая ошибка может появиться и после некорректного завершения работы etcd.

#### Решение 1
[Источник](https://github.com/etcd-io/etcd/issues/11949#issuecomment-1029906679)
- останавливаем etcd (перемещаем манифест пода)
- бекапим файлы
- увеличиваем `quota-backend-bytes` в манифесте
- удаляем snap файлы `rm /var/lib/etcd/member/snap/*.snap`
- пытаемся запустить etcd
- если ошибка выше ушла, проверяем статус:
```bash
kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table'
```
- если имеем ошибку `alarm:NOSPACE`, выполняем:
```bash
kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ alarm disarm'
```
- дефрагментируем etcd если это необходимо:
```bash
kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s'
```

#### Решение 2
Если решение 1 не помогло, то:
- загружаем на сервер [etcdctl](https://github.com/etcd-io/etcd/releases), желательно той же версии, что и на сервере.
- останавливаем etcd (перемещаем манифест пода)
- увеличиваем `quota-backend-bytes` в манифесте, если это необходимо
- бекапим файлы, например, в `/var/lib/etcd-backup`
- удаляем папку c данными `rm -rf /var/lib/etcd/`
- восстанавливаем базу `ETCDCTL_API=3 etcdctl snapshot restore /var/lib/etcd-backup/member/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check`.
- пытаемся запустить etcd
- если ошибка выше ушла, проверяем статус
```bash
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table
```
- если имеем ошибку `alarm:NOSPACE`, выполняем:
```bash
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ alarm disarm
```
- дефрагментируем etcd если это необходимо:
```bash
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
```

### Восстанавливаемся из бекапа
- загружаем на сервер [etcdctl](https://github.com/etcd-io/etcd/releases), желательно той же версии, что и на сервере
- останавливаем etcd (перемещаем манифест пода)
- удаляем папку c данными `rm -rf /var/lib/etcd/`
- восстанавливаем базу `ETCDCTL_API=3 etcdctl snapshot restore /var/lib/etcd-backup/member/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check`
- пытаемся запустить etcd

## Multi-master

### Потеря кворума
- добавляем в манифест аргумент `--force-new-cluster` на оставшейся ноде
- ждем поднятия etcd
- удаляем аргумент `--force-new-cluster`
Далее, если ноды были потеряны безвозвратно, то добавлям новые через `dhctl converge` или вручную, если кластер статический.

Если ноды возвратились, то они уже не являются членами кластера.
На узлах, которые не являются членами кластера выполняем следующие операции:
- останавливаем etcd (перемещаем манифест пода)
- бекапим файлы (на всякий случай)
- удаляем папку c данными `rm -rf /var/lib/etcd/`

Возвращаемся на рабочий кластер, и перезапускаем DaemonsSet `d8-control-plane-manager`:
```bash
kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
```
Ожидаем переката всех Pod'ов control-plane и переход их в состояние `Ready` и смотрим что все etcd инстансы стали участниками кластера.
```
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```
### Полная потеря данных и восстановление из бекапа

На всех нодах:
- останавливаем etcd (перемещаем манифест пода)
- выполняем резервное копирование файлов (на всякий случай)
- удаляем папку c данными `rm -rf /var/lib/etcd/`
- восстанавливаем на одной ноде как для [single-мастера](#восстанавливаемся-из-бекапа)
- перезапускаем DaemonSet `d8-control-plane-manager`:
```bash
kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
```
Ожидаем переката всех Pod'ов control-plane и переход их в состояние `Ready` и смотрим что все etcd-инстансы стали участниками кластера:
```
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```
