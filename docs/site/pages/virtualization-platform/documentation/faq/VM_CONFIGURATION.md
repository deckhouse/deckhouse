---
title: "FAQ: VM configuration"
permalink: en/virtualization-platform/documentation/faq/vm-configuration.html
---

## How to use cloud-init to configure virtual machines?

[Cloud-init](https://cloudinit.readthedocs.io/) is used for initial guest OS configuration on first boot. The configuration is written in YAML and starts with the `#cloud-config` directive.

{% alert level="warning" %}
When using cloud images (for example, official distribution images), you must provide a cloud-init configuration. Without it, some distributions do not configure network connectivity, and the virtual machine becomes unreachable on the network, even if the main network (Main) is attached.

In addition, cloud images do not allow login by default — you must either add SSH keys for the default user or create a new user with SSH access. Otherwise, you will not be able to access the virtual machine.
{% endalert %}

### Updating and installing packages

Example `cloud-config` for updating the system and installing packages from a list:

```yaml
#cloud-config
# Update package lists
package_update: true
# Upgrade installed packages to latest versions
package_upgrade: true
# List of packages to install
packages:
  - nginx
  - curl
  - htop
# Commands to run after package installation
runcmd:
  - systemctl enable --now nginx.service
```

### Creating a user

Example `cloud-config` for creating a local user with a password and SSH key:

```yaml
#cloud-config
# List of users to create
users:
  - name: cloud                    # Username
    passwd: "$6$rounds=4096$saltsalt$..."  # Password hash (SHA-512)
    lock_passwd: false            # Do not lock the account
    sudo: ALL=(ALL) NOPASSWD:ALL  # Sudo privileges without password prompt
    shell: /bin/bash              # Default shell
    ssh-authorized-keys:          # SSH keys for access
      - ssh-ed25519 AAAAC3NzaC... your-public-key ...
# Allow password authentication via SSH
ssh_pwauth: true
```

To generate a password hash for the `passwd` field, run:

```shell
mkpasswd --method=SHA-512 --rounds=4096
```

### Creating a file with required permissions

Example `cloud-config` for creating a file with specified access permissions:

```yaml
#cloud-config
# List of files to create
write_files:
  - path: /opt/scripts/start.sh    # File path
    content: |                     # File content
      #!/bin/bash
      echo "Starting application"
    owner: cloud:cloud            # File owner (user:group)
    permissions: '0755'           # Access permissions (octal format)
```

### Configuring disk and filesystem

Example `cloud-config` for disk partitioning, filesystem creation, and mounting:

```yaml
#cloud-config
# Disk partitioning setup
disk_setup:
  /dev/sdb:                        # Disk device
    table_type: gpt                # Partition table type (gpt or mbr)
    layout: true                   # Automatically create partitions
    overwrite: false               # Do not overwrite existing partitions

# Filesystem setup
fs_setup:
  - label: data                    # Filesystem label
    filesystem: ext4               # Filesystem type
    device: /dev/sdb1              # Partition device
    partition: auto                # Automatically detect partition

# Filesystem mounting
mounts:
  # [device, mount_point, fs_type, options, dump, pass]
  - ["/dev/sdb1", "/mnt/data", "ext4", "defaults", "0", "2"]
```

### Configuring network interfaces for additional networks

{% alert level="warning" %}
The settings described in this section apply only to additional networks. The main network (Main) is configured automatically via cloud-init and does not require manual configuration.
{% endalert %}

If additional networks are connected to a virtual machine, configure them manually via cloud-init: create configuration files in `write_files` and apply the settings in `runcmd`.

For more information on connecting additional networks to a virtual machine, see [Additional network interfaces](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#additional-network-interfaces).

#### For systemd-networkd

Example `cloud-config` for distributions that use `systemd-networkd` (Debian, CoreOS, and others):

```yaml
#cloud-config
write_files:
  - path: /etc/systemd/network/10-eth1.network
    content: |
      [Match]
      Name=eth1

      [Network]
      Address=192.168.1.10/24
      Gateway=192.168.1.1
      DNS=8.8.8.8

runcmd:
  - systemctl restart systemd-networkd
```

#### For Netplan (Ubuntu)

Example `cloud-config` for Ubuntu and other systems that use `Netplan`:

```yaml
#cloud-config
write_files:
  - path: /etc/netplan/99-custom.yaml
    content: |
      network:
        version: 2
        ethernets:
          eth1:
            addresses:
              - 10.0.0.5/24
            gateway4: 10.0.0.1
            nameservers:
              addresses: [8.8.8.8]
          eth2:
            dhcp4: true

runcmd:
  - netplan apply
```

#### For ifcfg (RHEL/CentOS)

Example `cloud-config` for RHEL-compatible distributions that use the `ifcfg` scheme and `NetworkManager`:

```yaml
#cloud-config
write_files:
  - path: /etc/sysconfig/network-scripts/ifcfg-eth1
    content: |
      DEVICE=eth1
      BOOTPROTO=none
      ONBOOT=yes
      IPADDR=192.168.1.10
      PREFIX=24
      GATEWAY=192.168.1.1
      DNS1=8.8.8.8

runcmd:
  - nmcli connection reload
  - nmcli connection up eth1
```

#### For Alpine Linux

Example `cloud-config` for distributions that use the traditional `/etc/network/interfaces` format (Alpine and similar):

```yaml
#cloud-config
write_files:
  - path: /etc/network/interfaces
    append: true
    content: |
      auto eth1
      iface eth1 inet static
          address 192.168.1.10
          netmask 255.255.255.0
          gateway 192.168.1.1

runcmd:
  - /etc/init.d/networking restart
```

## How to use Ansible to provision virtual machines?

[Ansible](https://docs.ansible.com/ansible/latest/index.html) is an automation tool for running tasks on remote servers over SSH. This example shows how to use Ansible with virtual machines in the `demo-app` project.

The example assumes that:

- `demo-app` namespace contains a VM named `frontend`.
- VM has a `cloud` user with SSH access.
- Private SSH key on the machine where Ansible runs is stored in `/home/user/.ssh/id_rsa`.

1. Create an `inventory.yaml` file:

   ```yaml
   ---
   all:
     vars:
       ansible_ssh_common_args: '-o ProxyCommand="d8 v port-forward --stdio=true %h %p"'
       # Default user for SSH access.
       ansible_user: cloud
       # Path to private key.
       ansible_ssh_private_key_file: /home/user/.ssh/id_rsa
     hosts:
       # Host name in the format <VM name>.<namespace>.
       frontend.demo-app:

   ```

1. Check the virtual machine `uptime`:

   ```bash
   ansible -m shell -a "uptime" -i inventory.yaml all

   # frontend.demo-app | CHANGED | rc=0 >>
   # 12:01:20 up 2 days,  4:59,  0 users,  load average: 0.00, 0.00, 0.00
   ```

If you do not want to use an inventory file, pass all parameters on the command line:

```bash
ansible -m shell -a "uptime" \
  -i "frontend.demo-app," \
  -e "ansible_ssh_common_args='-o ProxyCommand=\"d8 v port-forward --stdio=true %h %p\"'" \
  -e "ansible_user=cloud" \
  -e "ansible_ssh_private_key_file=/home/user/.ssh/id_rsa" \
  all
```

## How to automatically generate inventory for Ansible?

{% alert level="warning" %}
The `d8 v ansible-inventory` command requires `d8` v0.27.0 or higher.

The command works only for virtual machines that have the main cluster network (Main) connected.
{% endalert %}

Instead of manually creating an inventory file, you can use the `d8 v ansible-inventory` command, which automatically generates an Ansible inventory from virtual machines in the specified namespace. The command is compatible with the [ansible inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_inventory.html#inventory-scripts) interface.

The command includes only virtual machines with assigned IP addresses in the `Running` state. Host names are formatted as `<vmname>.<namespace>` (for example, `frontend.demo-app`).

1. Optionally set host variables via annotations (for example, the SSH user):

   ```bash
   d8 k -n demo-app annotate vm frontend provisioning.virtualization.deckhouse.io/ansible_user="cloud"
   ```

1. Run Ansible with a dynamically generated inventory:

   ```bash
   ANSIBLE_INVENTORY_ENABLED=yaml ansible -m shell -a "uptime" all -i <(d8 v ansible-inventory -n demo-app -o yaml)
   ```

{% alert level="info" %}
The `<(...)` construct is necessary because Ansible expects a file or script as the source of the host list. Simply specifying the command in quotes will not work — Ansible will try to execute the string as a script. The `<(...)` construct passes the command output as a file that Ansible can read.
{% endalert %}

1. Or save the inventory to a file and run the check:

   ```bash
   d8 v ansible-inventory --list -o yaml -n demo-app > inventory.yaml
   ansible -m shell -a "uptime" -i inventory.yaml all
   ```

### How to redirect traffic to a virtual machine?

The virtual machine runs in a Kubernetes cluster, so directing network traffic to it works like routing traffic to pods. To route traffic to a virtual machine, use the standard Kubernetes mechanism — the Service resource, which selects targets using a label selector.

1. Create a service with the required settings.

   For example, consider a virtual machine with the label `vm: frontend-0`, an HTTP service exposed on ports 80 and 443, and SSH access on port 22:

   ```yaml
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: frontend-0
     namespace: dev
     labels:
       vm: frontend-0
   spec: ...
   ```

1. To route network traffic to the virtual machine's ports, create the following Service:

1. To route network traffic to the virtual machine's ports, create the following service:

This service listens on ports 80 and 443 and forwards traffic to the target virtual machine’s ports 80 and 443. SSH access from outside the cluster is provided on port 2211.

   ```yaml
   apiVersion: v1
   kind: Service
   metadata:
     name: frontend-0-svc
     namespace: dev
   spec:
     type: LoadBalancer
     ports:
     - name: ssh
       port: 2211
       protocol: TCP
       targetPort: 22
     - name: http
       port: 80
       protocol: TCP
       targetPort: 80
     - name: https
       port: 443
       protocol: TCP
       targetPort: 443
     selector:
       vm: frontend-0
   ```
