---
title: Как использовать Ansible для конфигурирования виртуальных машин?
section: vm_configuration
lang: ru
---

[Ansible](https://docs.ansible.com/ansible/latest/index.html) — это инструмент автоматизации, который позволяет выполнять задачи на удаленных серверах с использованием протокола SSH. В данном примере мы рассмотрим, как использовать Ansible для управления виртуальными машинами расположенными в проекте `demo-app`.

В рамках примера предполагается, что:

- в неймспейсе `demo-app` есть ВМ `frontend`;
- в ВМ есть пользователь `cloud` с доступом по SSH;
- на машине, где запускается Ansible, приватный SSH-ключ хранится в файле `/home/user/.ssh/id_rsa`.

1. Создайте файл `inventory.yaml`:

   ```yaml
   ---
   all:
     vars:
       ansible_ssh_common_args: '-o ProxyCommand="d8 v port-forward --stdio=true %h %p"'
       # Пользователь по умолчанию, для доступа по SSH.
       ansible_user: cloud
       # Путь к приватному ключу.
       ansible_ssh_private_key_file: /home/user/.ssh/id_rsa
     hosts:
       # Название узла в формате <название ВМ>.<название проекта>.
       frontend.demo-app:

   ```

1. Проверьте значение `uptime` виртуальной машины:

   ```bash
   ansible -m shell -a "uptime" -i inventory.yaml all

   # frontend.demo-app | CHANGED | rc=0 >>
   # 12:01:20 up 2 days,  4:59,  0 users,  load average: 0.00, 0.00, 0.00
   ```

Если вы не хотите использовать файл inventory, передайте все параметры прямо в командной строке:

```bash
ansible -m shell -a "uptime" \
  -i "frontend.demo-app," \
  -e "ansible_ssh_common_args='-o ProxyCommand=\"d8 v port-forward --stdio=true %h %p\"'" \
  -e "ansible_user=cloud" \
  -e "ansible_ssh_private_key_file=/home/user/.ssh/id_rsa" \
  all
```
