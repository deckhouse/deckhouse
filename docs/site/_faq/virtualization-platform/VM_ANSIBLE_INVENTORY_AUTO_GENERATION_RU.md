---
title: Как автоматически сгенерировать inventory для Ansible?
section: vm_configuration
lang: ru
---

{% alert level="warning" %}
Для использования команды `d8 v ansible-inventory` требуется версия `d8` v0.27.0 или выше.

Команда работает только для виртуальных машин, у которых подключена основная сеть кластера (Main).
{% endalert %}

Вместо ручного создания inventory-файла можно использовать команду `d8 v ansible-inventory`, которая автоматически генерирует инвентарь Ansible из виртуальных машин в указанном неймспейсе. Команда совместима с интерфейсом [ansible inventory script](https://docs.ansible.com/ansible/latest/user_guide/intro_inventory.html#inventory-scripts).

Команда включает в инвентарь только виртуальные машины с назначенными IP-адресами в состоянии `Running`. Имена хостов формируются в формате `<vmname>.<namespace>` (например, `frontend.demo-app`).

1. При необходимости задайте переменные хоста через аннотации (например, пользователя для SSH):

   ```bash
   d8 k -n demo-app annotate vm frontend provisioning.virtualization.deckhouse.io/ansible_user="cloud"
   ```

1. Запустите Ansible с динамически сформированным инвентарём:

   ```bash
   ANSIBLE_INVENTORY_ENABLED=yaml ansible -m shell -a "uptime" all -i <(d8 v ansible-inventory -n demo-app -o yaml)
   ```

   {% alert level="info" %}
   Конструкция `<(...)` необходима, потому что Ansible ожидает файл или скрипт в качестве источника списка хостов. Простое указание команды в кавычках не сработает — Ansible попытается выполнить строку как скрипт. Конструкция `<(...)` передаёт вывод команды как файл, который Ansible может прочитать.
   {% endalert %}

1. Либо сохраните инвентарь в файл и выполните проверку:

   ```bash
   d8 v ansible-inventory --list -o yaml -n demo-app > inventory.yaml
   ansible -m shell -a "uptime" -i inventory.yaml all
   ```
