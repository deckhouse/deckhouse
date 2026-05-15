<script type="text/javascript" src='{% javascript_asset_tag getting-started-config-highlight %}[_assets/js/getting-started-config-highlight.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-shared %}[_assets/js/dvp/getting-started-dvp-shared.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-getting-started-access %}[_assets/js/dvp/getting-started-dvp-access.js]{% endjavascript_asset_tag %}'></script>
<script type="text/javascript" src='{% javascript_asset_tag dvp-prepare-placeholders %}[_assets/js/dvp/getting-started-prepare-placeholders.js]{% endjavascript_asset_tag %}'></script>

## Installing the OS on master and worker nodes

Prepare the nodes and install the required packages.

## SSH keys for master and worker access

### On the master node

1. Generate an SSH key with an empty passphrase by running on the master:

   ```bash
   ssh-keygen -t rsa -f /dev/shm/caps-id -C "" -N ""
   ```

1. Print the public part of the key (you will need it on the next step):

   ```bash
   cat /dev/shm/caps-id.pub
   ```

### On the worker node

On the prepared worker node, create the `caps` user. Run the following (replace `<SSH_KEY>` with the public key from the master; use `sudo` if your user lacks privileges):

```bash
export KEY='<SSH_KEY>' # Paste the public SSH key here.
useradd -m -s /bin/bash caps
usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
mkdir /home/caps/.ssh
echo $KEY | tee -a /home/caps/.ssh/authorized_keys
chown -R caps:caps /home/caps
chmod 700 /home/caps/.ssh
chmod 600 /home/caps/.ssh/authorized_keys
```

{% alert level="info" %}
On Astra Linux with the Parsec mandatory integrity module enabled, set the maximum integrity level for the `caps` user:

```bash
pdpl-user -i 63 caps
```
{% endalert %}

## NFS server setup

{% alert level="warning" %}
Below is an example of configuring storage on an external NFS server running Debian/Ubuntu. For other storage types, see [Storage configuration](../../documentation/admin/install/steps/storage.html).
{% endalert %}

Configure storage for cluster component metrics and VM disks.

1. Install NFS server packages if missing:

   ```bash
   sudo apt update && sudo apt install nfs-kernel-server
   ```

1. Create the data directory:

   ```bash
   sudo mkdir -p <NFS_SHARE>
   ```

1. Set ownership:

   ```bash
   sudo chown -R nobody:nogroup <NFS_SHARE>
   ```

1. Export the directory with root access from clients (`no_root_squash` on Linux). Append to `/etc/exports`:

   ```bash
   echo "<NFS_SHARE> <SUBNET_CIDR>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   ```

1. Apply exports:

   ```bash
   sudo exportfs -ra
   ```

1. Restart NFS:

   ```bash
   sudo systemctl restart nfs-kernel-server
   ```

1. On master and worker nodes, verify mount works:

   ```bash
   sudo apt update && sudo apt install -y nfs-common
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt
   sudo umount /mnt
   ```
