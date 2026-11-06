---
title: Canary-развёртывание
description: Стратегия развёртывания приложений, при которой новая версия сначала получает небольшую часть трафика, а по мере уверенности в её стабильности трафик переключается на неё, с быстрым откатом на старую версию при проблемах.
lang: ru
---

В Deckhouse Platform canary-развёртывание реализуется модулем [`ingress-nginx`](/modules/ingress-nginx/) через аннотации Ingress или модулем [`istio`](/modules/istio/) с помощью объектов VirtualService и DestinationRule (рекомендуемый способ). Новая версия работает как отдельные Deployment и Service рядом со стабильной.

Подробнее в разделе [«Настройка Canary deployment»](user/network/canary-deployment.html).
