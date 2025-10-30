---
title: Disk layout and size
permalink: en/guides/fs-requirements.html
description: A guide on how to select the disk size and layout of the file system before installing the Deckhouse Kubernetes Platform.
layout: sidebar-guides
---

In the [guide to choosing the minimum required disk space](hardware-requirements.html#deciding-on-the-amount-of-resources-needed-for-nodes) for various types of Deckhouse Kubernetes Platform (DKP) nodes, the disk volumes that must be allocated for successful installation and operation of DKP are specified. But you should also pay attention to the correct configuration of the file system so that the disk space does not suddenly "run out" despite the correctly selected volume at the installation stage.

{% alert level="info" %}
Problems may arise not through the fault of the administrator, but due to the peculiarities of the installer of the selected Linux distribution. For example, during installation, Astra Linux may allocate 15 GB for the root file system (`/`), 15 GB for the user's home directory (`/home`), and leave the rest unallocated, despite the connected disk having a total capacity of 60 GB, as recommended in the guide. With this configuration, the DKP installation will not fail with an error about insufficient disk space.
{% endalert %}

To avoid problems in the future, before installing, it is better to make sure that the file system partitions of the disk allocated for the machine meet the DKP volume requirements.

## Where and what does DKP store

DKP stores various types of data in specific file system directories. Let's look at the main ones in more detail:

* `/etc/kubernetes/`, `/etc/containerd`, etc. — directories with Kubernetes component configuration;
* `/var/lib/containerd` — layers of images of DKP components and other containers running on the node. The more DKP components are installed on a node or user load containers are launched on it, the more free space in this directory will be required.
* `/var/lib/kubelet` — two types of information are stored in this directory:
  * data about the pods running in the cluster;
  * ephemeral-storage data — for example, if 7 GB of ephemeral-storage is requested on the master node, and there is not enough space in this directory, pods will not be scheduled for this node.
* `/var/lib/etcd` — the etcd database, which stores the information necessary for the operation of the Kubernetes cluster;
* `/var/lib/deckhouse/downloaded/` — repository of release configurations for Deckhouse DKP modules ([ModuleRelease](../documentation/v1/reference/api/cr.html#modulerelease));
* `/var/lib/deckhouse/stronghold/` — data storage for [Stronghold](../../stronghold/) (if the corresponding module is enabled);
* `/var/log/pods/` — storage of pod logs;
* `/opt/deckhouse/` — DKP service components such as kubelet, containerd, static utilities (e.g. lsblk), etc.;
* `/opt/local-path-provider/` — directory for storing data when using [local storage Local Path Provisioner](../documentation/v1/admin/configuration/storage/sds/local-path-provisioner.html) (may be redefined [in configuration](../documentation/v1/admin/configuration/storage/sds/local-path-provisioner.html#example-localpathprovisioner-resources)).

## Disk Space Recommendations

The sections below show the disk volumes occupied by the various cluster components.

{% alert level="info" %}
The total amount of space specified in the tables may exceed the minimum recommended disk size for a cluster node specified in the "Quick Start" or ["Bare metal Cluster Requirements"](./hardware-requirements.html ). This is due to the fact that the table shows the maximum required values for the components, and the minimum requirements for the node indicate the average value.
{% endalert %}

### Master nodes

The table below shows the recommended amounts of space for the directories used by DKP on the cluster's master nodes.

<table>
  <thead>
    <tr>
      <th>Directory</th>
      <th>Disk size, GB</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>/mnt/vector-data</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/tmp</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/var/lib</code> <button type="button" onclick="toggleDetails('varlib-details')" style="background: none; border: none; cursor: pointer; font-size: 0.9em; color: #666;">[{{ site.data.i18n.common["show_details"][page.lang] }}]</button></td>
      <td>75</td>
    </tr>
    <tbody id="varlib-details" style="display: none;">
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/containerd</code></td>
        <td>30</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/deckhouse</code></td>
        <td>5</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/etcd</code></td>
        <td>10</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/kubelet</code></td>
        <td>25</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/upmeter</code></td>
        <td>5</td>
      </tr>
    </tbody>
    <tr>
      <td><code>/var/log/kube-audit</code></td>
      <td>2</td>
    </tr>
    <tr>
      <td><code>/var/log/pods</code></td>
      <td>5 (<a href="#pod-log-storage">details...</a>)</td>
    </tr>
  </tbody>
</table>

### Worker nodes

The table below shows the recommended amounts of space for the directories used by DKP on the worker nodes of the cluster.

<table>
  <thead>
    <tr>
      <th>Directory</th>
      <th>Disk size, GB</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>/mnt/vector-data</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/opt/local-path-provisioner</code>
      <p style="font-size: 0.9em; color: #666;">Depends on the storage settings set by the user. It is recommended to put it on a separate section.</p>
      </td>
      <td>100</td>
    </tr>
    <tr>
      <td><code>/tmp</code></td>
      <td>1</td>
    </tr>
    <tr>
      <td><code>/var/lib</code> <button type="button" onclick="toggleDetails('varlib-worker-details')" style="background: none; border: none; cursor: pointer; font-size: 0.9em; color: #666;">[{{ site.data.i18n.common["show_details"][page.lang] }}]</button></td>
      <td>55</td>
    </tr>
    <tbody id="varlib-worker-details" style="display: none;">
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/bashible</code></td>
        <td>1</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/containerd</code></td>
        <td>30</td>
      </tr>
      <tr>
        <td style="padding-left: 3em;"><code>/var/lib/kubelet</code></td>
        <td>24</td>
      </tr>
    </tbody>
    <tr>
      <td><code>/var/log/pods</code></td>
      <td>5 (<a href="#pod-log-storage">details...</a>)</td>
    </tr>
  </tbody>
</table>

### System nodes

The system nodes are the nodes on which the DKP components are running. When adding such nodes to the cluster, keep in mind that they host the monitoring load, including:

- [Prometheus](../../../modules/prometheus/);
- [loki](../../../modules/loki/);
- [upmeter](../../../modules/upmeter/) and other DKP components.

If monitoring data is stored locally on the nodes, it is recommended to additionally allocate ≥ 100 GB of free disk space for each system node.

{% alert level="info" %}
If you use a cluster configuration without dedicated system nodes, the above load will be distributed to other nodes, and you need to take into account the recommended amount of disk storage when choosing their configuration.
{% endalert %}

### Pod log storage

Pod logs are stored in the `/var/log/pods/` directory. The amount of logs used depends on the number of containers and DKP settings. On average, about 90 containers are running on the master node when using the [Default module set](../documentation/v1/admin/configuration/#module-bundles), with about 50 MB of space allocated to the logs of each of them by default. That is, there should be a minimum of `90 * 50 MB = 4.5 GB` space available in the directory `/var/log/pods/`.

The log storage parameters can also be redefined in the `containerLogMaxSize` parameter of [node groups](../documentation/v1/admin/configuration/platform-scaling/node/node-customization.html):

```yaml
containerLogMaxSize: 50Mi
containerLogMaxFiles: 4
```

### Trivy Vulnerability Database Repository

DKP has a built-in [vulnerability image scanning system](../documentation/v1/admin/configuration/security/scanning.html) based on [Trivy](https://github.com/aquasecurity/trivy), which scans all container images used in the cluster's files. Both public vulnerability databases and enriched data from Astra Linux, ALT Linux and RED OS are used for scanning. The total amount of disk space occupied by databases is 5 GB, so it must be taken into account when choosing the disk partition configuration.

Databases are stored on the cluster's system nodes, and if there are no such nodes in the cluster, the databases will be located on the worker node.

## If resource limits are configured

If resource limits in terms of disk space are configured for cluster entities, then the necessary free disk space on the node must be available in any case, otherwise the load will be displaced from these nodes.

## LVM-based local storage

In a DKP cluster, you can configure [local storage on nodes](../documentation/v1/admin/configuration/storage/sds/lvm-local.html), using LVM.

Requirements and placement procedure:

- Free block devices (disk partitions) should be available on the node.
- These devices will be used by the [sds-local-volume](../../../modules/sds-local-volume/) module to create a StorageClass.
- The amount of free space on the block device must correspond to the amount that is planned to be provided through the created StorageClass.

{% include table-toggle-details.js %}
