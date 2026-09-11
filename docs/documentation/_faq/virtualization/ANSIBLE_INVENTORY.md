---
title: How to automatically generate inventory for Ansible?
subsystems:
- virtualization
lang: en
---

{% alert level="warning" %}
The `d8 v ansible-inventory` command requires `d8` v0.27.0 or higher.

The command works only for virtual machines that have the main cluster network (Main) connected.
{% endalert %}

Instead of manually creating an inventory file, you can use the `d8 v ansible-inventory` command, which automatically generates an Ansible inventory from virtual machines in the specified namespace. The command is compatible with the [ansible inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_inventory.html#inventory-scripts) interface.

Only machines in the `Running` phase that have an assigned IP address get into the inventory. Host names are formatted as `<VM_NAME>.<NAMESPACE>` (for example, `frontend.demo-app`).

1. Optionally set host variables via annotations (for example, the SSH user):

   ```bash
   d8 k -n demo-app annotate vm frontend vars.ansible.deckhouse.io/ansible_user="cloud"
   ```

1. Run Ansible with a dynamically generated inventory:

   ```bash
   ANSIBLE_INVENTORY_ENABLED=yaml ansible -m shell -a "uptime" all -i <(d8 v ansible-inventory -n demo-app -o yaml)
   ```

{% alert level="info" %}
The `<(...)` construct is necessary because Ansible expects a file or script as the source of the host list. Simply specifying the command in quotes won't work, because Ansible tries to execute the string as a script. The `<(...)` construct passes the command output as a file that Ansible can read.
{% endalert %}

1. Or save the inventory to a file and run the check:

   ```bash
   d8 v ansible-inventory --list -o yaml -n demo-app > inventory.yaml
   ansible -m shell -a "uptime" -i inventory.yaml all
   ```
