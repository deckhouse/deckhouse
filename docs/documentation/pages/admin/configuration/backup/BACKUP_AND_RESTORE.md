---
title: Backup and restore
permalink: en/admin/configuration/backup/backup-and-restore.html
description: "Setting up backup and recovery in the Deckhouse Kubernetes Platform. Manual cluster recovery, disaster recovery. Data protection strategies."
---

## Manual cluster restore

### Restoring a cluster with a single control plane node

To properly restore the cluster, follow these steps on the master node:

1. Prepare the `etcdutl` utility. Locate and copy the executable on the node:

   ```shell
   cp $(find /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/ \
   -name etcdutl -print | tail -n 1) /usr/local/bin/etcdutl
   ```

   Check the version of `etcdutl`:

   ```shell
   etcdutl version
   ```

   Make sure the output of `etcdutl version` is displayed without errors.

   If `etcdutl` is not found, download the binary from [the official etcd repository]((https://github.com/etcd-io/etcd/releases)), choosing a version that matches your cluster's etcd version:

   ```shell
   wget "https://github.com/etcd-io/etcd/releases/download/v3.6.1/etcd-v3.6.1-linux-amd64.tar.gz"
   tar -xzvf etcd-v3.6.1-linux-amd64.tar.gz && mv etcd-v3.6.1-linux-amd64/etcdutl /usr/local/bin/etcdutl
   ```

1. Check the etcd version in the cluster (if the Kubernetes API is accessible):

   ```shell
   d8 k -n kube-system exec -ti etcd-$(hostname) -- etcdutl version
   ```

   If the command executes successfully, it will display the current etcd version.

1. Stop etcd. Move the etcd manifest to prevent kubelet from launching the etcd pod:

   ```shell
   mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
   ```

1. Make sure the etcd pod is stopped:

   ```shell
   crictl ps | grep etcd
   ```

   If the command returns nothing, the etcd pod has been successfully stopped.

1. Backup current etcd data. Create a backup copy of the `member` directory:

   ```shell
   cp -r /var/lib/etcd/member/ /var/lib/deckhouse-etcd-backup
   ```

   This backup will allow you to roll back in case of issues.

1. Clean the etcd directory. Remove old data to prepare for restore:

   ```shell
   rm -rf /var/lib/etcd
   ```

   Verify that `/var/lib/etcd` is now empty or does not exist:

   ```shell
   ls -la /var/lib/etcd
   ```

1. Place the etcd snapshot file. Copy or move the `etcd-backup.snapshot` file to the current user's (root) home directory:

   ```shell
   cp /path/to/backup/etcd-backup.snapshot ~/etcd-backup.snapshot
   ```

   Ensure the file is readable:

   ```shell
   ls -la ~/etcd-backup.snapshot
   ```

1. Restore the etcd database from the snapshot using `etcdutl`:

   ```shell
   ETCDCTL_API=3 etcdutl snapshot restore ~/etcd-backup.snapshot  --data-dir=/var/lib/etcd
   ```

   After the command completes, check that files have appeared in `/var/lib/etcd/`, reflecting the restored state.

1. Start etcd. Move the manifest back so that kubelet relaunches the etcd pod:

   ```shell
   mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
   ```

1. Wait for the pod to be created and reach `Running` state. Make sure it is up and running:

   ```shell
   crictl ps --label io.kubernetes.pod.name=etcd-$HOSTNAME
   ```

   Pod startup may take some time. Once etcd is running, the cluster will be restored from the snapshot.

   Output example:

   ```console
   CONTAINER        IMAGE            CREATED              STATE     NAME      ATTEMPT     POD ID          POD
   4b11d6ea0338f    16d0a07aa1e26    About a minute ago   Running   etcd      0           ee3c8c7d7bba6   etcd-gs-test
   ```

1. Restart the master node.

### Restoring a multi-master cluster

To properly restore a multi-master cluster, follow these steps:

1. Enable High Availability (HA) mode. This is necessary to preserve at least one Prometheus replica and its PVC, since HA is disabled by default in single-master clusters.

1. Switch the cluster to single master mode:

   - In a cloud cluster, follow the [instructions](../platform-scaling/control-plane/scaling-and-changing-master-nodes.html#common-scaling-scenarios).
   - In a static cluster, manually remove the additional master nodes.

1. Restore etcd from the backup on the only remaining master node. Follow the [instructions](#restoring-a-cluster-with-a-single-control-plane-node) for restoring a cluster with a single control-plane node.

1. Once etcd is restored, remove the records of the previously deleted master nodes from the cluster using the following command (replace with the actual node name):

   ```shell
   d8 k delete node <MASTER_NODE_NAME>
   ```

1. Reboot all cluster nodes. Ensure that after the reboot all nodes are available and functioning correctly.

1. Wait for Deckhouse to process all tasks in the queue:

   ```shell
   d8 platform queue main
   ```

1. Switch the cluster back to multi-master mode. For cloud clusters, follow the [instructions](../platform-scaling/control-plane/scaling-and-changing-master-nodes.html#common-scaling-scenarios).

Once you go through these steps, the cluster will be successfully restored in the multi-master configuration.

## Restoring individual objects

### Restoring Kubernetes objects from an etcd backup

To restore individual cluster objects (e.g., specific Deployments, Secrets, or ConfigMaps) from an etcd snapshot, follow these steps:

1. Launch a temporary etcd instance. Create a separate etcd instance that runs independently from the main cluster.
1. Load data from [backup copy](#backing-up-etcd) into temporary etcd instance. Use the existing etcd snapshot file to populate the temporary instance with the necessary data.
1. Unload the manifests of the necessary objects in YAML format.
1. Restore cluster objects from uploaded YAML files.

#### Example of steps to restore objects from an etcd backup

In the example below, `etcd-backup.snapshot` is a [etcd shapshot](#backing-up-etcd), `infra-production` is the namespace in which objects need to be restored.

- To decode objects from `etcd` you would need [auger](https://github.com/etcd-io/auger/tree/main). It can be built from source on any machine that has Docker installed (it cannot be done on cluster nodes).

  ```shell
  git clone -b v1.0.1 --depth 1 https://github.com/etcd-io/auger
  cd auger
  make release
  build/auger -h
  ```
  
- Resulting executable `build/auger`, and also the `snapshot` from the backup copy of etcd must be uploaded on master-node, on which following actions would be performed.

Following actions are performed on a master node, to which `etcd snapshot` file and `auger` tool were copied:

1. Set the correct access permissions for the backup file:

   ```shell
   chmod 644 etcd-backup.snapshot
   ```

1. Set full path for snapshot file and for the tool into environmental variables:

   ```shell
   SNAPSHOT=/root/etcd-restore/etcd-backup.snapshot
   AUGER_BIN=/root/auger 
   chmod +x $AUGER_BIN
   ```

1. Run a Pod with temporary instance of `etcd`.
   - Create Pod manifest. It should schedule on current master node by `$HOSTNAME` variable, and mounts snapshot file by `$SNAPSHOT` variable, which it then restores in temporary `etcd` instance:

     ```shell
     cat <<EOF >etcd.pod.yaml 
     apiVersion: v1
     kind: Pod
     metadata:
       name: etcdrestore
       namespace: default
     spec:
       nodeName: $HOSTNAME
       tolerations:
       - operator: Exists
       initContainers:
       - command:
         - etcdutl
         - snapshot
         - restore
         - "/tmp/etcd-snapshot"
         - --data-dir=/default.etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-snapshot-restore
         # Uncomment the fragment below to set limits for the container if the node does not have enough resources to run it.
         # resources:
         #   requests:
         #     ephemeral-storage: "200Mi"
         #   limits:
         #     ephemeral-storage: "500Mi"
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
         - name: etcd-snapshot
           mountPath: /tmp/etcd-snapshot
           readOnly: true
       containers:
       - command:
         - etcd
         image: $(kubectl -n kube-system get pod -l component=etcd -o jsonpath="{.items[*].spec.containers[*].image}" | cut -f 1 -d ' ')
         imagePullPolicy: IfNotPresent
         name: etcd-temp
         volumeMounts:
         - name: etcddir
           mountPath: /default.etcd
       volumes:
       - name: etcddir
         emptyDir: {}
         # Use the snippet below instead of emptyDir: {} to set limits for the container if the node's resources are insufficient to run it.
         # emptyDir:
         #  sizeLimit: 500Mi
       - name: etcd-snapshot
         hostPath:
           path: $SNAPSHOT
           type: File
     EOF
     ```

   - Create Pod from the resulting manifest:

     ```shell
     d8 k create -f etcd.pod.yaml
     ```

1. Set environment variables. In this example:

   - `infra-production` — namespace which we will search resources in.

   - `/root/etcd-restore/output` — path for outputting recovered resource manifests.

   - `/root/auger` — path to `auger` executable.

     ```shell
     FILTER=infra-production
     BACKUP_OUTPUT_DIR=/root/etcd-restore/output
     mkdir -p $BACKUP_OUTPUT_DIR && cd $BACKUP_OUTPUT_DIR
     ```

1. Commands below will filter needed resources by `$FILTER` and output them into `$BACKUP_OUTPUT_DIR` directory:

   ```shell
   files=($(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get / --prefix --keys-only | grep "$FILTER"))
   for file in "${files[@]}"
   do
     OBJECT=$(kubectl -n default exec etcdrestore -c etcd-temp -- etcdctl  --endpoints=localhost:2379 get "$file" --print-value-only | $AUGER_BIN decode)
     FILENAME=$(echo $file | sed -e "s#/registry/##g;s#/#_#g")
     echo "$OBJECT" > "$BACKUP_OUTPUT_DIR/$FILENAME.yaml"
     echo $BACKUP_OUTPUT_DIR/$FILENAME.yaml
   done
   ```

1. [Restore objects](#restoring-cluster-objects-from-exported-yaml-files) from exported YAML files.

1. Delete the Pod with a temporary instance of etcd:

   ```bash
   d8 k -n default delete pod etcdrestore
   ```

### Restoring cluster objects from exported YAML files

To restore objects from exported YAML files, follow these steps:

1. Prepare the YAML files for restoration. Before applying them to the cluster, remove technical fields that may be outdated or interfere with the recovery process:

   - `creationTimestamp`
   - `UID`
   - `status`

   You can edit these fields manually or use YAML/JSON processing tools such as `yq` or `jq`.

1. Create the objects in the cluster. To restore individual resources, run:

   ```shell
   d8 k create -f <PATH_TO_FILE>.json
   ```

   You can specify either a single file or a directory path.

1. To restore multiple objects at once, use the `find` command:

   ```shell
   find $BACKUP_OUTPUT_DIR -type f -name "*.yaml" -exec d8 k create -f {} \;
   ```

   This will locate all `.yaml` files within the specified `$BACKUP_OUTPUT_DIR` and apply them sequentially using `d8 k create`.

After completing these steps, the selected objects will be recreated in the cluster based on the definitions in the YAML files.

## Restoring objects after changing the master node IP address

{% alert level="warning" %}
This section describes a scenario where only the IP address of the master node has changed, and all other objects in the etcd backup (such as CA certificates) remain valid. It assumes the restoration is performed in a single-master-node cluster.
{% endalert %}

To restore etcd objects after changing the master node's IP address, follow these steps:

1. Restore etcd from the backup. Use the standard etcd restore procedure with a snapshot. Make sure not to change any parameters during restoration other than the etcd data itself.

1. Update the IP address in static configuration files:

   - Check the Kubernetes component manifest files located in `/etc/kubernetes/manifests/`.
   - Review kubelet's system configuration files (typically found in `/etc/systemd/system/kubelet.service.d/` or similar directories).
   - Update the IP address in any other configurations that reference the old address, if necessary.

1. Regenerate certificates that were issued for the old IP. Delete or move old certificates related to the API server and etcd (if applicable). Then generate new certificates, specifying the new master node IP address as a SAN (Subject Alternative Name).

1. Restart all services that use the updated configurations and certificates. Force kubelet to restart control-plane manifests (API server, etcd, etc.). Either restart the system services manually (e.g., `systemctl restart kubelet`) or ensure they restart automatically.

1. Wait for kubelet to regenerate its own certificate.

These actions can be performed either [automatically](#automated-object-extraction-when-changing-ip-address) using a script, or [manually](#manual-object-restore-after-changing-the-ip-address) by running the required commands step-by-step.

### Automated object extraction when changing IP address

To simplify cluster recovery after the master node's IP address changes, use the script provided below. Before running the script:

1. Specify the correct paths and IP addresses:
   - `ETCD_SNAPSHOT_PATH`: The path to the etcd snapshot backup.
   - `OLD_IP`: The old master node IP address used when the backup was created.
   - `NEW_IP`: The new IP address of the master node.

1. Make sure the Kubernetes version (`KUBERNETES_VERSION`) matches the one used in the cluster. This is necessary for downloading the correct version of kubeadm.

1. [Download](#restoring-a-cluster-with-a-single-control-plane-node) `etcdutl` if it is not installed.

1. After running the script, wait for the kubelet to regenerate its certificate with the new IP address. You can verify this in the `/var/lib/kubelet/pki/` directory, where a new certificate should appear.

{% offtopic title="Object extraction script" %}

```shell
ETCD_SNAPSHOT_PATH="./etcd-backup.snapshot" # Path to the etcd snapshot.
OLD_IP=10.242.32.34                         # Old master node IP address.
NEW_IP=10.242.32.21                         # New master node IP address.
KUBERNETES_VERSION=1.28.0                   # Kubernetes version.

mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml 
mkdir ./etcd_old
mv /var/lib/etcd ~/etcd_old
ETCDUTL_PATH=$(find /var/lib/containerd/ -name etcdutl)

ETCDCTL_API=3 $ETCDUTL_PATH snapshot restore etcd-backup.snapshot --data-dir=/var/lib/etcd 

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

### Manual object restore after changing the IP address

If you prefer to manually make changes during cluster recovery after the master node’s IP address has changed, follow these steps:

1. Restore etcd from the backup:

   - Move the etcd manifest so that kubelet stops the corresponding pod:

     ```shell
     mv /etc/kubernetes/manifests/etcd.yaml ~/etcd.yaml
     ```

   - Create a directory to temporarily store the previous etcd data:

     ```shell
     mkdir ./etcd_old
     mv /var/lib/etcd ./etcd_old
     ```

   - Find or download the `etcdutl` utility if it’s not available, and perform the snapshot restore:

     ```shell
     ETCD_SNAPSHOT_PATH="./etcd-backup.snapshot" # Path to the etcd snapshot.
     ETCDUTL_PATH=$(find /var/lib/containerd/ -name etcdutl)

     ETCDCTL_API=3 $ETCDUTL_PATH snapshot restore \
       etcd-backup.snapshot \
       --data-dir=/var/lib/etcd
     ```

   - Restore the etcd manifest so kubelet starts the etcd pod again:

     ```shell
     mv ~/etcd.yaml /etc/kubernetes/manifests/etcd.yaml
     ```

   - Verify etcd is running by checking the pod list using `crictl ps | grep etcd` or reviewing the kubelet logs.

1. Update the IP address in static configuration files. If the old IP address is used in manifests or kubelet services, replace it with the new one:

    ```shell
    OLD_IP=10.242.32.34                         # Old master node IP address.
    NEW_IP=10.242.32.21                         # New master node IP address.

    find /etc/kubernetes/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    find /etc/systemd/system/kubelet.service.d -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    find  /var/lib/bashible/ -type f -exec sed -i "s/$OLD_IP/$NEW_IP/g" {} ';'
    ```

1. Regenerate certificates issued for the old IP address:

   - Prepare a directory to store the old certificates:

     ```shell
     mkdir -p ./old_certs/etcd
     mv /etc/kubernetes/pki/apiserver.* ./old_certs/
     mv /etc/kubernetes/pki/etcd/server.* ./old_certs/etcd/
     mv /etc/kubernetes/pki/etcd/peer.* ./old_certs/etcd/
     ```

   - Install or download kubeadm to match the current Kubernetes version:

     ```shell
     KUBERNETES_VERSION=1.28.0 # Kubernetes version.
     curl -LO https://dl.k8s.io/v$KUBERNETES_VERSION/bin/linux/amd64/kubeadm
     chmod +x kubeadm
     ```

   - Generate new certificates with the updated IP:

     ```shell
     ./kubeadm init phase certs all --config /etc/kubernetes/deckhouse/kubeadm/config.yaml
     ```

     The new IP address will be included in the generated certificates.

1. Restart all services that use the updated configuration and certificates. To immediately stop active containers, run:

    ```shell
    crictl ps --name 'kube-apiserver' -o json | jq -r '.containers[0].id' | xargs crictl stop
    crictl ps --name 'kubernetes-api-proxy' -o json | jq -r '.containers[0].id' | xargs crictl stop
    crictl ps --name 'etcd' -o json | jq -r '.containers[].id' | xargs crictl stop

    systemctl daemon-reload
    systemctl restart kubelet.service
    ```

    Kubelet will restart the necessary pods, and Kubernetes components will load the new certificates.

1. Wait for kubelet to regenerate its own certificate. Kubelet will automatically generate a new certificate with the updated IP address:

   - Check the `/var/lib/kubelet/pki/` directory.
   - Ensure the new certificate is present and valid.

Once these steps are completed, the cluster will be successfully restored and fully functional with the new master node IP address.

## Creating backups with Deckhouse CLI

Deckhouse CLI (`d8`) provides the `backup` command for creating backups of various cluster components:

- `etcd`: Snapshot of the Deckhouse key-value data store.
- `cluster-config`: Archive containing key configuration objects of the cluster.
- `loki`: Export of logs from the built-in Loki API.

### Backing up etcd

An etcd snapshot allows you to preserve the current state of the cluster at the key-value storage level. This is a full dump that can be used for recovery.

To create a snapshot, run the following command:

```shell
d8 backup etcd <path-to-snapshot> [flags]
```

Flags:

- `-p`, `--etcd-pod string`: Name of the etcd pod to snapshot.
- `-h`, `--help`: Show help for the etcd command.
- `--verbose`: Enable verbose output for detailed logging.

Example:

```shell
d8 backup etcd etcd-backup.snapshot
```

Example output:

```console
2025/04/22 08:38:58 Trying to snapshot etcd-sandbox-master-0
2025/04/22 08:39:01 Snapshot successfully taken from etcd-sandbox-master-0
```

#### Automatic etcd backup

Deckhouse automatically performs a daily etcd backup using a CronJob that runs inside the `d8-etcd-backup` pod in the `kube-system` namespace. The job creates a snapshot of the database, compresses it, and saves the archive locally on the node at `/var/lib/etcd/`:

```shell
etcdctl snapshot save etcd-backup.snapshot
tar -czvf etcd-backup.tar.gz etcd-backup.snapshot
mv etcd-backup.tar.gz /var/lib/etcd/etcd-backup.tar.gz
```

To configure automatic etcd backups, use the [`control-plane-manager`](/modules/control-plane-manager/) module. The required parameters are specified in its configuration:

| Parameter                  | Description                                                                 |
|---------------------------|-----------------------------------------------------------------------------|
| `etcd.backup.enabled`     | Enables daily etcd backup.                                                  |
| `etcd.backup.cronSchedule`| Cron-formatted schedule for running the backup. Local time of `kube-controller-manager` is used. |
| `etcd.backup.hostPath`    | Path on master nodes where etcd backup archives will be stored.             |

Example configuration fragment:

```yaml
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
spec:
  etcd:
    backup:
      enabled: true
      cronSchedule: "0 1 * * *"
      hostPath: "/var/lib/etcd"
```

### Cluster configuration backup

The `d8 backup cluster-config` command creates an archive containing a set of key resources related to the cluster configuration. This is not a full backup of all objects, but a specific whitelist.

To create the backup, run the following command:

```shell
d8 backup cluster-config <path-to-backup-file>
```

Example:

```shell
d8 backup cluster-config /backup/cluster-config-2025-04-21.tar
```

The archive includes only those objects that meet the following criteria:

- CustomResource objects whose CRDs are annotated with:

  ```console
  backup.deckhouse.io/cluster-config=true
  ```

- StorageClasses with the label:

  ```console
  heritage=deckhouse
  ```

- Secrets and ConfigMaps from namespaces starting with `d8-` or `kube-`, if they are explicitly listed in the whitelist file.

- Cluster-level Roles and Bindings (ClusterRole and ClusterRoleBinding), if they are not labeled with:

  ```console
  heritage=deckhouse
  ```

> The backup includes only CR objects, but not the CRD definitions themselves. To fully restore the cluster, the corresponding CRDs must already be present (e.g., installed by Deckhouse modules).

Example whitelist content:

| Namespace           | Object     | Name                                               |
|---------------------|------------|----------------------------------------------------|
| `d8-system`         | Secret     | `d8-cluster-terraform-state`                      |
|                     |            | <span title="The string is interpreted as a regular expression and covers all secrets whose names start with d8-node-terraform-state-."><code style="color:#d63384">$regexp:^d8-node-terraform-state-(.*)$</code></span> |
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

### Exporting logs from Loki

The `d8 backup loki` command is intended for exporting logs from the built-in Loki. This is not a full backup, but rather a diagnostic export: the resulting data cannot be restored back into Loki.

To perform the export, `d8` accesses the Loki API using the `loki` ServiceAccount in the `d8-monitoring` namespace, authenticated via a token stored in a Kubernetes secret.

The `loki` ServiceAccount is automatically created starting from Deckhouse v1.69.0. However, to use the `d8 backup loki` command, you must manually create the token secret and assign the necessary Role and RoleBinding if they are not already present.

Apply the manifests below before running `d8 backup loki` to ensure the command can properly authenticate and access the Loki API.

Example manifests:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: loki-api-token
  namespace: d8-monitoring
  annotations:
    kubernetes.io/service-account.name: loki
type: kubernetes.io/service-account-token
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: access-to-loki-from-d8
  namespace: d8-monitoring
rules:
  - apiGroups: ["apps"]
    resources:
      - "statefulsets/http"
    resourceNames: ["loki"]
    verbs: ["create", "get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: access-to-loki-from-d8
  namespace: d8-monitoring
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: access-to-loki-from-d8
subjects:
  - kind: ServiceAccount
    name: loki
    namespace: d8-monitoring
```

To create a log backup, run the following command:

```shell
d8 backup loki [flags]
```

Example:

```shell
d8 backup loki --days 1 > ./loki.log
```

Flags:

- `--start`, `--end`: Time range boundaries in the format "YYYY-MM-DD HH:MM:SS".
- `--days`: The time window size for log export (default is 5 days).
- `--limit`: The maximum number of log lines per request (default is 5000).

You can list all available flags using the following command:

```shell
d8 backup loki --help
```
