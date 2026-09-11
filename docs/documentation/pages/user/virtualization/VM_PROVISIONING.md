---
title: "Initial provisioning and the guest OS agent"
permalink: en/user/virtualization/vm-provisioning.html
description: "Initial guest system configuration through cloud-init and Sysprep, plus the guest OS agent and what it provides to a virtual machine."
search: cloud-init, Sysprep, provisioning, guest OS agent, qemu-guest-agent
---

On the first boot, the guest system is configured by an initialization script, and after that DP communicates with it through the guest OS agent.

## VM initialization scripts

Initialization scripts are designed for the initial configuration of a virtual machine when it starts.

The following initialization scripts are supported:

- [Cloud-Init](https://cloudinit.readthedocs.io).
- [Sysprep](https://learn.microsoft.com/en-us/windows-hardware/manufacture/desktop/sysprep--system-preparation--overview).

### Cloud-Init

Cloud-Init is a tool for automatically configuring virtual machines at first boot. It performs a wide range of configuration tasks without manual intervention.

{% alert level="warning" %}
The Cloud-Init configuration is written in YAML and has to start with the `#cloud-config` header at the beginning of the configuration block. For other possible headers and their purpose, see the [official cloud-init documentation](https://cloudinit.readthedocs.io/en/latest/explanation/format.html#headers-and-content-types).
{% endalert %}

Key Cloud-Init capabilities:

- creating users, setting passwords, adding SSH keys for access;
- automatically installing the required software at first boot;
- running arbitrary commands and scripts to configure the system;
- automatically starting and enabling system services (for example, [`qemu-guest-agent`](#guest-os-agent)).

Here are the typical scenarios.

1. Adding an SSH key for a [preinstalled user](../../admin/configuration/virtualization/cluster-images.html#image-resources-table) that may already be present in a cloud image (for example, the `ubuntu` user in official Ubuntu images). The name of such a user depends on the image. Check it in the documentation for your distribution.

   ```yaml
   #cloud-config
   ssh_authorized_keys:
     - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD... your-public-key ...
   ```

1. Creating a user with a password and an SSH key:

   ```yaml
   #cloud-config
   users:
     - name: cloud
       passwd: <PASSWORD_HASH>
       lock_passwd: false
       sudo: ALL=(ALL) NOPASSWD:ALL
       shell: /bin/bash
       ssh-authorized-keys:
         - ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQD... your-public-key ...
   ssh_pwauth: True
   ```

   Where `<PASSWORD_HASH>` is the password hash in quotes, generated with the `mkpasswd --method=SHA-512 --rounds=4096` command.

1. Installing packages and services:

   ```yaml
   #cloud-config
   package_update: true
   packages:
     - nginx
     - qemu-guest-agent
   runcmd:
     - systemctl daemon-reload
     - systemctl enable --now nginx.service
     - systemctl enable --now qemu-guest-agent.service
   ```

The following example shows how to pass the script to a virtual machine:

{% tabs cloud-init-usage %}

{% tab "Using the CLI" %}

You can embed a Cloud-Init script directly into the virtual machine specification, but such a script is limited to 2048 bytes:

```yaml
spec:
  provisioning:
    type: UserData
    userData: |
      #cloud-config
      package_update: true
      ...
```

For longer scripts or scripts with private data, create the virtual machine initialization script in a Secret resource. Here is an example of a Secret resource with a Cloud-Init script:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cloud-init-example
data:
  userData: <base64 data>
type: provisioning.virtualization.deckhouse.io/cloud-init
```

A fragment of the virtual machine configuration that uses a Cloud-Init initialization script stored in a Secret resource:

```yaml
spec:
  provisioning:
    type: UserDataRef
    userDataRef:
      kind: Secret
      name: cloud-init-example
```

> The value of the `.data.userData` field has to be Base64-encoded. To encode it, use the `base64 -w 0` or `echo -n "content" | base64` command.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Create a virtual machine or select an existing one and click its name.
1. On the **Configuration** tab, scroll down to the **Cloud-init** toggle and enable it.
1. Select the input mode:
   - **Basic setup**: Fill in the **Username**, **Password**, and **Public SSH key** fields, and enable the **Unrestricted sudo access** toggle if required. DP builds the cloud-init configuration itself.
   - **Editing**: Enter the cloud-init configuration manually in the **Parameters** field. The used volume is shown below the field (no more than 2048 bytes). In the **Linked secret** field, you can select an existing initialization script, and its contents load into the field. If no secret is linked, the configuration is stored in the VM specification.
1. Click the **Save** button that appears (or **Create** when creating the VM).

You can store a script as a separate resource and reuse it for several VMs. To create such a resource:

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Initialization scripts**.
1. Click **Create**.
1. In the **Name** field, enter the script name, and in the **Type** field, select `cloud-init` or `sysprep`.
1. In the **Files** block, set the key in the **File name** field (`userData` by default), and enter the contents manually, drag a file to the field, or click it to upload a file.
1. Click **Create**.

The **Initialization scripts** section shows secrets of the `provisioning.virtualization.deckhouse.io/*` type, each with its name, type (`cloud-init` or `sysprep`), the list of keys, and the resource age. To make a virtual machine use such a script, reference it in the [`.spec.provisioning.userDataRef`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-provisioning-userdataref) parameter.

{% endtab %}

{% endtabs %}

### Sysprep

To configure virtual machines running Windows with Sysprep, only the Secret resource option is supported.

Here is an example of a Secret resource with a Sysprep script:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sysprep-example
data:
  unattend.xml: <base64 data>
type: provisioning.virtualization.deckhouse.io/sysprep
```

{% alert level="info" %}
The value of the `.data.unattend.xml` field has to be Base64-encoded. To encode it, use the `base64 -w 0` or `echo -n "content" | base64` command.
{% endalert %}

A fragment of the virtual machine configuration that uses a Sysprep initialization script in a Secret resource:

```yaml
spec:
  provisioning:
    type: SysprepRef
    sysprepRef:
      kind: Secret
      name: sysprep-example
```

## Guest OS agent

Install QEMU Guest Agent in the guest system so that DP can interact with the operating system inside the VM. The agent is needed for three things:

- it makes consistent disk and VM snapshots possible;
- it reports information about the running system, and that information lands in the [`.status.guestOSInfo`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-guestosinfo) block;
- it shows that the operating system has actually booted, rather than just the virtual machine having started.

DP works with `qemu-guest-agent` version 5.2.0 and later. To check the installed version, run the following command:

```bash
qemu-guest-agent --version
```

Guest system information looks like this:

```yaml
status:
  guestOSInfo:
    id: fedora
    kernelRelease: 6.11.4-301.fc41.x86_64
    kernelVersion: "#1 SMP PREEMPT_DYNAMIC Sun Oct 20 15:02:33 UTC 2024"
    machine: x86_64
    name: Fedora Linux
    prettyName: Fedora Linux 41 (Cloud Edition)
    version: 41 (Cloud Edition)
    versionId: "41"
```

The `AGENT` column shows whether the agent is running:

```bash
d8 k get vm -o wide
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME     PHASE     UPTIME   CORES   COREFRACTION   MEMORY   NEED RESTART   AGENT   MIGRATABLE   NODE           IPADDRESS    AGE
fedora   Running   5d21h    6       5%             8000Mi   False          True    True         virtlab-pt-1   10.66.10.1   5d21h
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

Install the agent with the command for your distribution and start the service:

```bash
# Debian and derivatives.
sudo apt install qemu-guest-agent

# CentOS and derivatives.
sudo yum install qemu-guest-agent

sudo systemctl enable --now qemu-guest-agent
```

For Linux, it's convenient to automate the installation with an initialization script:

```yaml
#cloud-config
package_update: true
packages:
  - qemu-guest-agent
runcmd:
  - systemctl enable --now qemu-guest-agent.service
```

The agent doesn't need any configuration after installation. If your snapshots need application data consistency, put the preparation scripts in the `/etc/qemu-ga/hooks.d/` directory on Debian and Ubuntu, or `/etc/qemu/fsfreeze-hook.d/` on RHEL, CentOS, and Fedora. The scripts have to be executable, and the agent runs them before the file system freeze and after the thaw, so you don't have to stop the application services.
