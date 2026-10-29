---
title: Виртуальная машина (ВМ)
description: Машина, которую Deckhouse Platform создаёт и которой управляет декларативно через модуль virtualization. Она работает в поде на узле кластера под управлением QEMU/KVM, которым управляет KubeVirt.
lang: ru
---

Машину описывает [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) в [проекте](glossary.html#project) или неймспейсе. Её [диски](glossary.html#vm-disk) и [образы](glossary.html#vm-image) подключаются как [блочные устройства](glossary.html#vm-block-device), процессор задаётся числом ядер и [долей ядра](glossary.html#vm-core-fraction) в пределах класса ВМ, а состояние поддерживает [политика запуска](glossary.html#vm-run-policy). Гостевая система настраивается при первом запуске [начальной инициализацией](glossary.html#vm-provisioning), после чего Deckhouse Platform общается с ней через [агента гостевой ОС](glossary.html#vm-guest-agent).

Ядро модуля основано на KubeVirt: процесс QEMU, который libvirtd запускает в контейнере `compute` пода машины, и есть работающая машина. Машины переезжают между узлами [живой миграцией](glossary.html#live-migration), объединяются в [пулы](glossary.html#vm-pool) и проходят фазы [жизненного цикла виртуальной машины](glossary.html#vm-lifecycle).

Подробнее в разделе [«Подсистема Virtualization»](architecture/virtualization/).
