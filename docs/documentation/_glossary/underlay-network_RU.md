---
title: Underlay-сеть
description: Прямое подключение физических функций (PF) и виртуальных функций (VF) сетевых интерфейсов узла к подам через Kubernetes Dynamic Resource Allocation. Нужна нагрузкам с прямым доступом к оборудованию, например DPDK-приложениям.
lang: ru
---

Физические сетевые интерфейсы подключаются к подам в одном из двух режимов: `Shared`, когда из Physical Function с помощью SR-IOV создаются Virtual Functions и несколько подов делят оборудование, или `Dedicated`, когда под получает Physical Function целиком. Администратор создаёт объект [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork) модуля [`sdn`](/modules/sdn/) и помечает [неймспейс](glossary.html#namespace), которому разрешено её использовать. Под запрашивает устройство аннотацией.

DPDK, Data Plane Development Kit, — набор библиотек для обработки сетевых пакетов в пространстве пользователя в обход сетевого стека ядра.

Подробнее в разделе [«Подключение физических сетевых интерфейсов к подам DPDK-приложений»](user/network/sdn/underlay.html).
