---
title: "Первичная настройка доступа"
permalink: ru/virtualization-platform/documentation/admin/install/steps/access.html
lang: ru
---

После завершения установки, подключиться к платформе можно следующими способами:

- С master-узла, подключившись к нему по SSH;
- Удаленно, настроив подключение на любом персональном компьютере.

## Подключение к платформе с master-узла

Подключитесь к master-узлу по SSH (IP-адрес master-узла выводится инсталлятором по завершении установки):

```bash
ssh <USER_NAME>@<MASTER_IP>
```

Проверьте, что ресурсы платформы доступны, выведя список узлов кластера:

```bash
sudo -i d8 k get nodes
```

## Удаленное подключение к платформе

Для настройки удаленного подключения к кластеру, выполните действия согласно [инструкции](../../platform-management/access-control/user-management.html) и установите утилиту [d8](/products/virtualization-platform/reference/console-utilities/d8.html) (Deckhouse CLI).
