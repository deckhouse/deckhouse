---
title: How to use Ansible to provision virtual machines?
sections:
- vm_configuration
lang: en
---

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
