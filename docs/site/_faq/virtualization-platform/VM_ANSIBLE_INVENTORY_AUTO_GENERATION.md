---
title: How to automatically generate inventory for Ansible?
section: vm_configuration
lang: en
---

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
