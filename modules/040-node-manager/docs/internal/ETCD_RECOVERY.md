# Restoring ETCD functionality

Caution! Back up your etcd files on the node before any recovery attempts.
Before doing this, make sure that etcd is not running. To stop etcd, remove the etcd static Pod manifest from the manifest directory.

## Single-master

### "Failed to find database snapshot file (snap: snapshot file doesn't exist)"

This error may occur after restarting etcd if etcd has reached the `quota-backend-bytes` limit.

Below is an example of the corresponding etcd logs:

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

This [issue](https://github.com/etcd-io/etcd/issues/11949) suggests that such an error can also occur after etcd has been terminated incorrectly.

#### Solving the problem - First method

The solution is based on this [issue](https://github.com/etcd-io/etcd/issues/11949#issuecomment-1029906679) and involves the following steps:

- stop etcd: 
  
  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- back up your files:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- increase the `quota-backend-bytes` parameter in the `~/etcd.yaml` manifest, if necessary;
- delete the .snap files:

  ```shell
  rm /var/lib/etcd/member/snap/*.snap
  ```

- try to run etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- if no error message appears, check the status:

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \ 
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table'
  ```

- if there is an `alarm:NOSPACE`, run the following command:

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ alarm disarm'
  ```

- defragment etcd (if necessary):

  ```shell
  kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
    --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s'
  ```

#### Second method

This method can be used if the first one has failed.

Do the following:

- upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server);
- stop etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- increase the `quota-backend-bytes` parameter in the `~/etcd.yaml` manifest, if necessary;
- back up your files:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- delete the data directory:

  ```shell
  rm -rf /var/lib/etcd/
  ```

- restore the etcd database:

  ```shell
  ETCDCTL_API=3 etcdctl snapshot restore /var/lib/etcd-backup/member/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
    --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
  ```

- try to run etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- if no error message appears, check the status:

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ endpoint status -w table
  ```

- if there is an `alarm:NOSPACE` error, run the following:

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ alarm disarm
  ```

- defragment etcd (if necessary):

  ```shell
  ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
    --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
  ```

### Restoring from a backup

Follow these steps to restore from a backup:
- upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server);
- stop etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- back up your files:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- delete the data directory:

  ```shell
  rm -rf /var/lib/etcd/
  ```

- restore the etcd database:

  ```shell
  ETCDCTL_API=3 etcdctl snapshot restore BACKUP_FILE --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
    --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
  ```

- start etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

## Multi-master

### etcd quorum loss

Perform the following steps to restore the quorum in the etcd cluster:
- add the `--force-new-cluster` flag to the `/etc/kubernetes/manifests/etcd.yaml` manifest on the running node; 
- wait for etcd to start;
- remove the `--force-new-cluster` flag from the `/etc/kubernetes/manifests/etcd.yaml` manifest. 

If nodes have been lost permanently, add new ones using the `dhctl converge` command (or manually if the cluster is static).

If the nodes have been lost temporarily, they are no longer members of the cluster.

To turn them into cluster members, do the following on those nodes:

- stop etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- back up your files:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- delete the data directory:

  ```shell
  rm -rf /var/lib/etcd/
  ```

Switch to the running cluster and restart the `d8-control-plane-manager` DaemonSet:

```shell
kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
```

Wait for all control plane Pods rolling over and becoming `Ready`.

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

### Complete data loss and recovery from a backup

If there is a complete loss of data, perform the following steps on all nodes of the etcd cluster:
- stop etcd:

  ```shell
  mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
  ```

- back up your files:

  ```shell
  cp -r /var/lib/etcd/ /var/lib/deckhouse-etcd-backup
  ```

- delete the data directory:

  ```shell
  rm -rf /var/lib/etcd/
  ```

Select any node and do the following:
- upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server);
- restore the etcd database: `ETCDCTL_API=3 etcdctl snapshot restore BACKUP_FILE --cacert /etc/kubernetes/pki/etcd/ca.crt \
  --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check`
- add the `--force-new-cluster` flag to the `~/etcd.yaml` manifest;
- try to run etcd:

  ```shell
  mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
  ```

- remove the `--force-new-cluster` flag from the `/etc/kubernetes/manifests/etcd.yaml` manifest; 
- restart the `d8-control-plane-manager` DaemonSet:

  ```shell
  kubectl -n kube-system rollout restart daemonset d8-control-plane-manager
  ```

Wait for all control plane Pods rolling over and becoming `Ready`.

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```
