---
title: How to use cloud-init to configure virtual machines?
section: vm_configuration
lang: en
---

[Cloud-init](https://cloudinit.readthedocs.io/) is used for initial guest OS configuration on first boot. The configuration is written in YAML and starts with the `#cloud-config` directive.

{% alert level="warning" %}
When using cloud images (for example, official distribution images), you must provide a cloud-init configuration. Without it, some distributions do not configure network connectivity, and the virtual machine becomes unreachable on the network, even if the main network (Main) is attached.

In addition, cloud images do not allow login by default — you must either add SSH keys for the default user or create a new user with SSH access. Otherwise, you will not be able to access the virtual machine.
{% endalert %}

#### Updating and installing packages

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

#### Creating a user

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

#### Configuring disk and filesystem

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

#### Configuring network interfaces for additional networks

{% alert level="warning" %}
The settings described in this section apply only to additional networks. The main network (Main) is configured automatically via cloud-init and does not require manual configuration.
{% endalert %}

If additional networks are connected to a virtual machine, configure them manually via cloud-init: create configuration files in `write_files` and apply the settings in `runcmd`.

For more information on connecting additional networks to a virtual machine, see [Additional network interfaces](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#additional-network-interfaces).

##### For systemd-networkd

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

##### For Netplan (Ubuntu)

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

##### For ifcfg (RHEL/CentOS)

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

##### For Alpine Linux

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
