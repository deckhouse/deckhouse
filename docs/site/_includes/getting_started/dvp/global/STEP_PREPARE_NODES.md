{%- include getting_started/dvp/global/partials/gs_scripts.liquid step='prepare' -%}

Prepare cluster nodes: configure the NFS server and worker node before platform installation.

## Configure the NFS server

To configure NFS, complete the following steps:

1. Set up the NFS server for VM disk storage. Run the following commands on the **NFS server**:

   {% tabs dvp-nfs-server %}
   {% tab "For Ubuntu-based OS" %}
   ```bash
   sudo apt update
   sudo apt install nfs-kernel-server
   sudo mkdir -p <NFS_SHARE>
   sudo chown -R nobody:nogroup <NFS_SHARE>
   echo "<NFS_SHARE> <INTERNAL_NETWORK_CIDRS>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   sudo exportfs -ra
   sudo systemctl restart nfs-kernel-server
   ```
   {% endtab %}
   {% tab "For CentOS, Rocky Linux, ALT Linux, ROSA Server, RED OS, MOS OS" %}
   ```bash
   sudo dnf install -y nfs-utils
   sudo mkdir -p <NFS_SHARE>
   sudo chown -R nobody:nobody <NFS_SHARE>
   echo "<NFS_SHARE> <INTERNAL_NETWORK_CIDRS>(rw,sync,no_subtree_check,no_root_squash)" | sudo tee -a /etc/exports
   sudo exportfs -ra
   sudo systemctl enable --now nfs-server
   sudo systemctl restart nfs-server
   ```
   {% endtab %}
   {% endtabs %}

1. Verify NFS access from the **master node**. Run the following commands on the **master node**:

   {% tabs dvp-nfs-master %}
   {% tab "For Ubuntu-based OS" %}
   ```bash
   sudo apt update
   sudo apt install nfs-common
   sudo mkdir -p /mnt/dvp-nfs-test
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt/dvp-nfs-test
   ls /mnt/dvp-nfs-test
   sudo umount /mnt/dvp-nfs-test
   sudo rmdir /mnt/dvp-nfs-test
   ```
   {% endtab %}
   {% tab "For CentOS, Rocky Linux, ALT Linux, ROSA Server, RED OS, MOS OS" %}
   ```bash
   sudo dnf install -y nfs-utils
   sudo mkdir -p /mnt/dvp-nfs-test
   sudo mount -t nfs4 <NFS_HOST>:<NFS_SHARE> /mnt/dvp-nfs-test
   ls /mnt/dvp-nfs-test
   sudo umount /mnt/dvp-nfs-test
   sudo rmdir /mnt/dvp-nfs-test
   ```
   {% endtab %}
   {% endtabs %}

   If mount fails, check that the NFS server is reachable from the master node, its IP differs from the master and worker IPs, and that the export in `/etc/exports` allows access from the cluster nodes subnet.

## Prepare the worker node

{% alert level="info" %}
An SSH key pair for the `caps` user, required for further cluster setup, was generated automatically in your browser. Save the keys: you may need them if you want to add more worker nodes.

Public key:

```text
<CAPS_SSH_PUBLIC_KEY>
```

Private key:

```text
<CAPS_SSH_PRIVATE_KEY>
```
{% endalert %}

{% alert level="warning" %}
If you deploy the lab on virtual machines, enable nested virtualization on the hypervisor for the **worker node**. See [installation requirements](/products/virtualization-platform/gs/bm/step1.html#hardware-and-software-requirements).
{% endalert %}

To continue setup, create the `caps` user by running the following commands on the **worker node**:

{% tabs dvp-caps-worker %}
{% tab "For Ubuntu-based OS" %}
```bash
export KEY='<CAPS_SSH_PUBLIC_KEY>'
sudo useradd -m -s /bin/bash caps
sudo usermod -aG sudo caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
sudo mkdir -p /home/caps/.ssh
echo "$KEY" | sudo tee -a /home/caps/.ssh/authorized_keys
sudo chown -R caps:caps /home/caps
sudo chmod 700 /home/caps/.ssh
sudo chmod 600 /home/caps/.ssh/authorized_keys
```
{% endtab %}
{% tab "For CentOS, Rocky Linux, ALT Linux, ROSA Server, RED OS, MOS OS" %}
```bash
export KEY='<CAPS_SSH_PUBLIC_KEY>'
sudo useradd -m -s /bin/bash caps
sudo usermod -aG wheel caps
echo 'caps ALL=(ALL) NOPASSWD: ALL' | sudo EDITOR='tee -a' visudo
sudo mkdir -p /home/caps/.ssh
echo "$KEY" | sudo tee -a /home/caps/.ssh/authorized_keys
sudo chown -R caps:caps /home/caps
sudo chmod 700 /home/caps/.ssh
sudo chmod 600 /home/caps/.ssh/authorized_keys
```
{% endtab %}
{% endtabs %}
On **Astra Linux** with Parsec enabled, set the maximum integrity level for `caps`:
```bash
sudo pdpl-user -i 63 caps
```

The cluster nodes are ready for Deckhouse Virtualization Platform installation.
