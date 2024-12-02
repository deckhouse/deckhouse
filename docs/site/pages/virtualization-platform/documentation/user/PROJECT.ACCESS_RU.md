---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/documentation/user/project-access.html
lang: ru
---

Для подключения к проекту выполните следующие действия:

1. Запросите у Администратора платформы ссылку для получения файла конфигурации (`kubeconfig.<domain>`).
2. Введите почтовый адрес и пароль для доступа к проекту.
3. Скопируйте конфигурацию в домашний каталог `~/.kube/config`.
4. Установите утилиту [d8](../reference/console-utilities/d8.html).
5. Далее используйте следующий формат команды для управления ресурсами проекта: `d8 k -n <project_name>` или `d8 v -n <project_name>`.
