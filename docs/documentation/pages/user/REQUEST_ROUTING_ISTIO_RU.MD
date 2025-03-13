---
title: "Маршрутизация запросов с Istio"
permalink: ru/user/request_routing_istio.html
lang: ru
---

Для маршрутизации HTTP- и TCP-запросов можно использовать [istio](#).
Перед настройкой маршрутизации убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D0%BC%D0%B0%D1%80%D1%88%D1%80%D1%83%D1%82%D0%B8%D0%B7%D0%B0%D1%86%D0%B8%D1%8F-%D0%B7%D0%B0%D0%BF%D1%80%D0%BE%D1%81%D0%BE%D0%B2 -->

Основной ресурс для управления маршрутизацией — [VirtualService](#), он позволяет переопределять судьбу HTTP- или TCP-запроса. Доступные аргументы для принятия решения о маршрутизации:

* Host и любые другие заголовки;
* URI;
* метод (GET, POST и пр.);
* лейблы пода или namespace источника запросов;
* dst-IP или dst-порт для не-HTTP-запросов.
