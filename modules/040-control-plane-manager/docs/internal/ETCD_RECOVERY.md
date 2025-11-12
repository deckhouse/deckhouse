# Restoring ETCD functionality

**Caution!** Back up your etcd files on the node before any recovery attempts.
Before doing this, make sure that etcd is not running. To stop etcd, remove the etcd static Pod manifest from the manifest directory.

### High‑level notes (read first)

- Use the right binary for your etcd version: from etcd v3.6 the restore/defrag utilities moved to `etcdutl`; older guides using `etcdctl snapshot restore` apply to ≤v3.5. ([etcd][6])
- Deckhouse exposes a concise “Backup and restore” entry point (and CPM config) you can reference before diving into low‑level steps. ([Deckhouse][1])
- When in doubt, the upstream etcd “Disaster recovery” page is the canonical source on safe snapshot restore and membership semantics. ([etcd][4])

## Single-master

### Restoring from a backup

Follow these steps to restore from a backup:

1. If necessary restore etcd-server access keys and certificates into `/etc/kubernetes` directory.

1. Upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Delete the data directory.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

1. Copy backup file to `~/etc-backup.snapshot`.

1. Restore the etcd database.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etc-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

1. Start etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

**Prefer this first:** Deckhouse “Backup and restore” (single control‑plane) — stop the etcd static Pod, wipe the data dir, and restore the snapshot with the matching etcd tool (`etcdutl` for v3.6+, `etcdctl` for ≤v3.5), then bring the manifest back. ([Deckhouse][1])

**If that doesn’t work:** proceed with the detailed steps in this section.

## Multi-master

### Complete data loss or recovery to previous state from a backup

**Prefer this first:** Deckhouse control‑plane‑manager FAQ recipe — temporarily run as a single master, restore etcd from the snapshot on that node, clean up deleted masters, then return to HA. ([Deckhouse][2])

**If that doesn’t work:** use your advanced recovery (including `--force-new-cluster`) and relabeling workflow below. (The `--force-new-cluster` flag recreates a one‑member cluster to bootstrap recovery.) ([etcd][3])

If there is a complete loss of data, perform the following steps on all nodes of the etcd cluster:
1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Delete the data directory.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

Select any node as first recovered.

On another (two) nodes do the following:
1. Stop kubelet.

   ```shell
   systemctl stop kubelet.service
   ```

1. Remove all containers.

   ```shell
   systemctl is-active -q docker && systemctl restart docker
   kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
   ```

1. Clear a node.

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   ```

On the selected node do the following:
1. If necessary restore etcd-server access keys and certificates into `/etc/kubernetes` directory.

1. Upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Copy backup file to `~/etc-backup.snapshot`.
1. Restore the etcd database.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore ~/etc-backup.snapshot --cacert /etc/kubernetes/pki/etcd/ca.crt \
   --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd
   ```

1. Add the `--force-new-cluster` flag to the `~/etcd.yaml` manifest.
1. Try to run etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. Remove the `--force-new-cluster` flag from the `/etc/kubernetes/manifests/etcd.yaml` manifest after successful up etcd.
1. Set [HA-mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability) for prevent removing HA-mode (for example we can lose one prometheus replica and data for lost replica).
1. Remove control-plane role label from nodes objects expect selected (recover in current time).

   ```shell
   kubectl label no NOT_SELECTED_NODE_1 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   kubectl label no NOT_SELECTED_NODE_2 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   ```

Start kubelet on another nodes:

```shell
systemctl start kubelet.service
```

On the first recovered node do the following steps:
1. Restart and wait for Deckhouse to be ready.

   ```shell
   kubectl -n d8-system rollout restart deployment deckhouse
   ```

   If Deckhouse Pod is stuck in a Terminating state, forcibly delete the Pod:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse --force
   ```

   If you got the error `lock the main queue: waiting for all control-plane-manager Pods to become Ready`, forcibly remove control plane Pods for other nodes.

1. Wait for the control plane Pod to roll over and become `Ready`.

   ```shell
   watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
   ```

1. Check that node etcd member has peer and client host as internal node IP.

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

Add control-plane role for each other control-plane nodes:

```shell
kubectl label no NOT_SELECTED_NODE_I node.deckhouse.io/group= node-role.kubernetes.io/control-plane=
```

Wait for all control plane Pods rolling over and becoming `Ready`:

```shell
watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
```

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

[1]: https://deckhouse.io/products/kubernetes-platform/documentation/v1/admin/configuration/backup/backup-and-restore.html?utm_source=chatgpt.com "Backup and restore | Deckhouse Kubernetes Platform"
[2]: https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/control-plane-manager/faq.html?utm_source=chatgpt.com "Managing control plane: FAQ | Deckhouse Kubernetes Platform"
[3]: https://etcd.io/docs/v3.6/op-guide/configuration/?utm_source=chatgpt.com "Configuration options"
[4]: https://etcd.io/docs/v3.5/op-guide/recovery/?utm_source=chatgpt.com "Disaster recovery"
[5]: https://etcd.io/docs/v3.5/op-guide/maintenance/?utm_source=chatgpt.com "Maintenance"
[6]: https://etcd.io/blog/2025/announcing-etcd-3.6/?utm_source=chatgpt.com "Announcing etcd v3.6.0"

### etcd quorum loss

**Prefer this first:** etcd “Disaster recovery” — restore the cluster from a known‑good snapshot (v3.6+: `etcdutl snapshot restore`) and re‑form membership cleanly; use this instead of ad‑hoc edits when possible. ([etcd][4])

**If that doesn’t work:** follow your section’s `--force-new-cluster` bootstrap on the surviving node, then re‑add control‑plane nodes as described below. (Flag semantics documented here.) ([etcd][3])

Perform the following steps to restore the quorum in the etcd cluster:
1. Add the `--force-new-cluster` flag to the `/etc/kubernetes/manifests/etcd.yaml` manifest on the running node.
1. Wait for etcd to start.
1. Remove the `--force-new-cluster` flag from the `/etc/kubernetes/manifests/etcd.yaml` manifest.
1. Set [HA-mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability) for prevent removing HA-mode (for example we can lose one prometheus replica and data for lost replica).
1. Remove control-plane role label from nodes objects expect selected (recover in current time).

   ```shell
   kubectl label no LOST_NODE_1 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   kubectl label no LOST_NODE_2 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   ```

If nodes have been lost permanently, add new ones using the `dhctl converge` command (or manually if the cluster is static).
Don't forget to delete objects of lost nodes.

If the nodes have been lost temporarily, they are no longer members of the cluster.

To turn them into cluster members, do the following on those nodes:

1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Delete the data directory.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

1. Clear a node.

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   ```

1. Restart and wait for Deckhouse to be ready.

   ```shell
   kubectl -n d8-system rollout restart deployment deckhouse
   ```

1. Wait for control plane Pods to roll over and become `Ready`.

   ```shell
   watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
   ```

1. Check that node etcd member has peer and client host as internal node IP.

   ```shell
   kubectl -n kube-system exec -ti ETCD_POD -- /bin/sh -c 'ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table'
   ```

Add a control-plane role for each lost nodes:

```shell
kubectl label no LOST_NODE_I node.deckhouse.io/group= node-role.kubernetes.io/control-plane=
```

Wait for all control plane Pods to roll over and become `Ready`:

```shell
watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
```

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

## "Failed to find database snapshot file (snap: snapshot file doesn't exist)"

**Prefer this first:** etcd maintenance guidance — handle NOSPACE/compaction issues by freeing space, defragmenting, and clearing alarms; many “missing snapshot” panics are preceded by quota exhaustion. ([etcd][5])

**If that doesn’t work:** apply your targeted fix paths below that remove broken `.snap` files or re‑inflate from a snapshot.

This error may occur after restarting etcd if etcd has reached the `quota-backend-bytes` limit.

Below is an example of the corresponding etcd logs:

```text
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

### Solving the problem - First method

**Prefer this first:** etcd maintenance flow — defragment and clear `NOSPACE` alarms (on‑line via `etcdctl defrag` for ≤v3.5; note v3.6 moves certain offline ops to `etcdutl`). ([etcd][5])

**If that doesn’t work:** use the deeper steps you listed (remove stale `.snap`, bump `quota-backend-bytes`, restart, then verify).

First method works on the single and multi-master environments both.

The solution is based on this [issue](https://github.com/etcd-io/etcd/issues/11949#issuecomment-1029906679) and involves the following steps on affected nodes:

1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Increase the `quota-backend-bytes` parameter in the `~/etcd.yaml` manifest, if necessary.
1. Delete the .snap files.

   ```shell
   rm /var/lib/etcd/member/snap/*.snap
   ```

1. Try to run etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. If no error message appears, check the status:

   ```shell
   kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \ 
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ endpoint status -w table
   ```

1. If there is an `alarm:NOSPACE`, run the following command:

   ```shell
   kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Defragment etcd (if necessary):

   ```shell
   kubectl -n kube-system exec -ti ETCD_POD_ON_AFFECTED_HOST -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
   ```

### Second method

#### Single-master

**Prefer this first:** Deckhouse single‑node restore — stop etcd, delete data dir, restore from snapshot with the correct tool, start etcd. Keep tool/version parity (`etcdutl` for 3.6+). ([Deckhouse][1])

**If that doesn’t work:** use your forced re‑creation approach (`--skip-hash-check` only when you accept the risk) as described below.

This method can be used if the first one has failed.

Do the following:

1. Upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Increase the `quota-backend-bytes` parameter in the `~/etcd.yaml` manifest, if necessary.
1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Delete the data directory.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

1. Restore the etcd database.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore /var/lib/deckhouse-etcd-backup/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
   ```

1. Try to run etcd.

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. If no error message appears, check the status:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ endpoint status -w table
   ```

1. If there is an `alarm:NOSPACE` error, run the following command:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Defragment etcd (if necessary).

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
   ```
  
#### Multi-master

##### If this error affect one node

**Prefer this first:** etcd maintenance + standard rejoin — defragment healthy members, take/verify a snapshot, then fully re‑provision the bad node and let it rejoin from a clean state rather than hand‑editing its data. ([etcd][4])

**If that doesn’t work:** follow your “clear the node → relabel control‑plane → wait for pods → verify membership” sequence.

1. Defragment etcd on another (two) nodes (if necessary).

   ```shell
   kubectl -n kube-system exec -ti ETCD_POD_NOT_AFFECTED_HOST -- etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt \
     --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
   ```

1. Remove control-plane role label from affected node.

   ```shell
   kubectl label no NOT_SELECTED_NODE_1 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   ```

On the affected node:

1. Clear the node.

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   rm -rf /var/lib/etcd/member/
   ```

1. Add control-plane role to affected node.

   ```shell
   kubectl label no AFFECTED_NODE node.deckhouse.io/group= node-role.kubernetes.io/control-plane=
   ```

Wait for all control plane Pods rolling over and becoming `Ready`:

```shell
watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
```

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```

##### If this error affect > 1 nodes

**Prefer this first:** Deckhouse control‑plane‑manager FAQ “restore whole cluster” — pivot to a single master, restore from the snapshot there, and rebuild the HA control plane. This avoids partial, inconsistent restores across members. ([Deckhouse][2])

**If that doesn’t work:** use your advanced flow: restore from the backup on one chosen node, start with `--force-new-cluster`, increase backend quota if needed, then reintroduce the remaining control‑plane nodes and verify etcd membership. (Flag behavior reference.) ([etcd][3])

1. Upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Stop etcd.

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Back up your files.

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

1. Delete the data directory.

   ```shell
   rm -rf /var/lib/etcd/member/
   ```

Select any node as first recovered.

On another (two) nodes do the following:
1. Stop kubelet.

   ```shell
   systemctl stop kubelet.service
   ```

1. Remove all containers.

   ```shell
   systemctl list-units --full --all | grep -q docker.service && systemctl restart docker
   kill $(ps ax | grep containerd-shim | grep -v grep |awk '{print $1}')
   ```

1. Clear the node:

   ```shell
   rm -f /etc/kubernetes/manifests/{etcd,kube-apiserver,kube-scheduler,kube-controller-manager}.yaml
   rm -f /etc/kubernetes/{scheduler,controller-manager}.conf
   rm -f /etc/kubernetes/authorization-webhook-config.yaml
   rm -f /etc/kubernetes/admin.conf /root/.kube/config
   rm -rf /etc/kubernetes/deckhouse
   rm -rf /etc/kubernetes/pki/{ca.key,apiserver*,etcd/,front-proxy*,sa.*}
   ```

On the selected node do the following:

1. Upload [etcdctl](https://github.com/etcd-io/etcd/releases) to the server (best if it has the same version as the etcd version on the server).

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.5.4/etcd-v3.5.4-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.5.4-linux-amd64.tar.gz && mv etcd-v3.5.4-linux-amd64/etcdctl /usr/local/bin/etcdctl
   ```

1. Restore the etcd database.

   ```shell
   ETCDCTL_API=3 etcdctl snapshot restore /var/lib/deckhouse-etcd-backup/snap/db --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
     --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/  --data-dir=/var/lib/etcd --skip-hash-check
   ```

1. Add the `--force-new-cluster` flag to the `~/etcd.yaml` manifest.
1. Increase the `quota-backend-bytes` parameter in the `~/etcd.yaml` manifest, if necessary.
1. Try to run etcd:

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. Remove the `--force-new-cluster` flag from the `/etc/kubernetes/manifests/etcd.yaml` manifest after successful up etcd.
1. If no error message appears, check the status:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ endpoint status -w table
   ```

1. If there is an `alarm:NOSPACE` error, run the following command:

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ alarm disarm
   ```

1. Defragment etcd (if necessary).

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt --key /etc/kubernetes/pki/etcd/ca.key \
     --endpoints https://127.0.0.1:2379/ defrag --command-timeout=60s
   ```

1. Set [HA-mode](https://deckhouse.io/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters-highavailability) for prevent removing HA-mode (for example we can lose one prometheus replica and data for lost replica).
1. Remove control-plane role label from nodes objects expect selected (recover in current time).

   ```shell
   kubectl label no NOT_SELECTED_NODE_1 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   kubectl label no NOT_SELECTED_NODE_2 node.deckhouse.io/group- node-role.kubernetes.io/control-plane-
   ```

On another nodes, start kubelet:

```shell
systemctl start kubelet.service
```

On the first recovered node do the following:
1. Restart and wait readiness Deckhouse.

   ```shell
   kubectl -n d8-system rollout restart deployment deckhouse
   ```

   If Deckhouse Pod stuck in Terminating state, force delete Pod:

   ```shell
   kubectl -n d8-system delete po -l app=deckhouse --force
   ```

   If you got error `lock the main queue: waiting for all control-plane-manager Pods to become Ready`, force remove control plane Pods for another nodes.

1. Wait for control plane Pod rolling over and becoming `Ready`.

   ```shell
   watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
   ```

1. Check node etcd member has peer and client host as internal node IP.

   ```shell
   ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt   --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
   ```

For each another control-plane nodes:

1. Add control-plane role.

   ```shell
   kubectl label no NOT_SELECTED_NODE_I node.deckhouse.io/group= node-role.kubernetes.io/control-plane=
   ```

Wait for all control plane Pods rolling over and becoming `Ready`.

```shell
watch "kubectl -n kube-system get po -o wide | grep d8-control-plane-manager"
```

Make sure that all etcd instances are now cluster members:

```shell
ETCDCTL_API=3 etcdctl --cacert /etc/kubernetes/pki/etcd/ca.crt --cert /etc/kubernetes/pki/etcd/ca.crt \
  --key /etc/kubernetes/pki/etcd/ca.key --endpoints https://127.0.0.1:2379/ member list -w table
```
