---
title: "Настройка доступа к проекту"
permalink: ru/virtualization-platform/documentation/user/project-access.html
lang: ru
---

Для подключения к проекту выполните следующие действия:

1. Запросите у Администратора платформы ссылку для получения файла конфигурации (`kubeconfig.<domain>`).
1. Введите email и пароль для доступа к проекту.
1. Скопируйте конфигурацию в домашний каталог `~/.kube/config`.
1. Установите утилиту [d8](/products/kubernetes-platform/documentation/v1/cli/d8/).
1. Для управления ресурсами проекта используйте команду: `d8 k -n <project_name>` или `d8 v -n <project_name>`.
