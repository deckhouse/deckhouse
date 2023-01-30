---
title: "Модуль virtualization"
---

Модуль позволяет управлять виртуальными машинами с помощью Kubernetes. Модуль использует проект [kubevirt](https://github.com/kubevirt/kubevirt). 

Для работы виртуальных машин используется стэк QEMU (KVM) + libvirtd и CNI Cilium (необходим включенный модуль [cni-cilium](../021-cni-cilium/)). В качестве хранилища гарантируется работа с [LINSTOR](../041-linstor) или [CEPH](../099-ceph-csi/), но также возможны и другие варианты хранилища. 

Основные преимущества модуля:
- Простой интерфейс работы с виртуальными машинами как [примитивами Kubernetes](cr.html) (работа с ВМ аналогична работе с Pod'ами), включая:
  - создание/удаление, пуск/остановку виртуальных машин;
  - live migration (скоро...). 
- Высокая производительность сетевого взаимодействия за счет использования CNI cilium с поддержкой [MacVTap](https://github.com/kvaps/community/blob/macvtap-mode-for-pod-networking/design-proposals/macvtap-mode-for-pod-networking/macvtap-mode-for-pod-networking.md) (исключает накладные расходы на трансляцию адресов).

