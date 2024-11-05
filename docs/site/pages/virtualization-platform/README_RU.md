---
title: "Deckhouse Virtualization Platform"
permalink: ru/virtualization-platform/readme.html
lang: ru
---

Deckhouse Virtualization Platform позволяет декларативно создавать, запускать и управлять виртуальными машинами и их ресурсами.

## Сценарии использования

- Запуск виртуальных машин с x86_64 совместимой ОС.
- Запуска виртуальных машин и контейнеризованных приложений в одном окружении.

  ![](/images/virtualization-platform/cases-vms.ru.png)

  ![](/images/virtualization-platform/cases-pods-and-vms.ru.png)

{% alert level="warning" %}
Если вы планируете использовать Deckhouse Virtualization Platform в production-среде, рекомендуется разворачивать его на физических серверах. Развертывание Deckhouse Virtualization Platform на виртуальных машинах также возможно, но в этом случае необходимо включить nested-виртуализацию.
{% endalert %}

Для работы виртуализации требуется кластер Deckhouse Kubernetes Platform. Пользователям редакции Enterprise Edition доступна возможность управления ресурсами через графический интерфейс (UI).

Для подключения к виртуальным машинам с использованием последовательного порта, VNC или по протоколу ssh используется утилита командной строки [d8](https://deckhouse.ru/documentation/v1/deckhouse-cli/).

## Архитектура

Платформа включает в себя следующие компоненты:

- Ядро платформы (CORE), основанное на проекте KubeVirt и использующее QEMU/KVM + libvirtd для запуска виртуальных машин.
- Deckhouse Virtualization Container Registry (DVCR) — репозиторий для хранения и кэширования образов виртуальных машин.
- Virtualization-API (API) — контроллер, реализующий API пользователя для создания и управления ресурсами виртуальных машин.
- Контроллер маршрутизации (ROUTER) - контроллер, управляющий маршрутами для обеспечения сетевой связности виртуальных машин.

API предоставляет возможности для декларативного создания, модификации и удаления следующих ресурсов:

- образы виртуальных машин и загрузочные образы;
- диски виртуальных машин;
- классы виртуальных машин;
- виртуальные машины;
- операции над виртуальными машинами.

## Ролевая модель

Для управления ресурсами предусмотрены следующие роли пользователей:

- Пользователь (User)
- Привилегированный пользователь (PrivilegedUser)
- Редактор (Editor)
- Администратор (Admin)
- Редактор кластера (ClusterEditor)
- Администратор кластера (ClusterAdmin)

Далее таблице представлены матрица доступа для данных ролей

| Resource                             | User | PrivilegedUser | Editor | Admin | ClusterEditor | ClusterAdmin |
| ------------------------------------ | ---- | -------------- | ------ | ----- | ------------- | ------------ |
| virtualmachines                      | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualdisks                         | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualimages                        | R    | R              | R      | CRUD  | CRUD          | CRUD         |
| clustervirtualimages                 | R    | R              | R      | R     | CRUD          | CRUD         |
| virtualmachineblockdeviceattachments | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineoperations             | R    | CR             | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineipaddresses            | R    | R              | CRUD   | CRUD  | CRUD          | CRUD         |
| virtualmachineipaddressleases        | -    | -              | -      | R     | R             | CRUD         |
| virtualmachineclasses                | R    | R              | R      | R     | CRUD          | CRUD         |

Команды доступные для операций с утилитой командной строки d8

| d8 cli                        | User | PrivilegedUser | Editor | Admin | ClusterEditor | ClusterAdmin |
| ----------------------------- | ---- | -------------- | ------ | ----- | ------------- | ------------ |
| d8 v console                  | N    | Y              | Y      | Y     | Y             | Y            |
| d8 v ssh / scp / port-forward | N    | Y              | Y      | Y     | Y             | Y            |
| d8 v vnc                      | N    | Y              | Y      | Y     | Y             | Y            |

Перечень сокращений

| Сокращение | Операция | Соответствующая операция Kubernetes |
| ---------- | -------- | ----------------------------------- |
| C          | создать  | create                              |
| R          | читать   | get,list,watch                      |
| U          | изменить | patch, update                       |
| D          | удалить  | delete, deletecollection            |
