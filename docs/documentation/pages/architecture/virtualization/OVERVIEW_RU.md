---
title: Подсистема Virtualization
permalink: ru/architecture/virtualization/
lang: ru
search: virtualization, virtualization subsystem, подсистема виртуализации, dvp
description: Архитектура подсистемы Virtualization в Deckhouse Kubernetes Platform.
---

В данном подразделе описана архитектура подсистемы Virtualization (подсистемы виртуализации) Deckhouse Kubernetes Platform (DKP).

Подсистема Virtualization представлена модулем [`virtualization`](/modules/virtualization/), который позволяет декларативно создавать, запускать и управлять виртуальными машинами (ВМ) и их ресурсами.

Модуль [`virtualization`](/modules/virtualization/) включает в себя следующие компоненты:

- Virtualization API (API) — контроллер, реализующий API пользователя для создания и управления ресурсами виртуальных машин;
- [ядро модуля](core.html) — основано на проекте KubeVirt и использует QEMU/KVM + libvirtd для запуска ВМ. [KubeVirt](https://github.com/kubevirt/kubevirt) — это Open Source-проект, который позволяет запускать, развёртывать и управлять ВМ с использованием Kubernetes в качестве платформы оркестрации. Он обеспечивает совместную работу традиционных ВМ и контейнерных рабочих нагрузок в одном кластере Kubernetes, предоставляя единую плоскость управления;
- [Deckhouse Virtualization Container Registry (DVCR)](dvcr.html) — хранилище образов контейнеров для хранения и кеширования образов ВМ;
- [Containerized Data Importer (CDI)](cdi.html) — дополнение для управления постоянным хранилищем в Kubernetes. Его основная цель — предоставить декларативный способ создания дисков ВМ на основе ресурсов PersistentVolumeClaim (PVC). CDI предоставляет возможность импортировать образы и диски ВМ в PVC-тома для использования в ВМ, управляемых KubeVirt;
- вспомогательные компоненты — компоненты, реализующие следующие вспомогательные функции:

  - аудит событий безопасности;
  - проброс USB-устройств в ВМ;
  - обновление сетевых маршрутов;
  - удаление ресурсов перед деактивацией модуля [`virtualization`](/modules/virtualization/).

Подробнее с описанием модуля можно ознакомиться [в разделе документации модуля](/modules/virtualization/).
