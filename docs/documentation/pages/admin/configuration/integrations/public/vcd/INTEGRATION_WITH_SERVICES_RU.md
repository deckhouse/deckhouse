---
title: Интеграция с облаком VMware Cloud Director
permalink: ru/admin/integrations/public/vcd/vcd-services.html
lang: ru
---

Deckhouse Kubernetes Platform интегрируется с инфраструктурой VMware Cloud Director и использует ресурсы VCDInstanceClass для описания характеристик виртуальных машин, разворачиваемых в кластере.

## Основные возможности

- Заказ и удаление виртуальных машин в составе кластера Kubernetes;
- Назначение шаблона, sizing policy и storage profile для каждой группы узлов;
- Работа с внутренними сетями и пробросом трафика через Edge Gateway (DNAT, firewall);
- Использование хранилища VMware Cloud Director с возможностью задания типов дисков;
- Поддержка статических и динамических IP-адресов, включая L2-режим с MetalLB.
