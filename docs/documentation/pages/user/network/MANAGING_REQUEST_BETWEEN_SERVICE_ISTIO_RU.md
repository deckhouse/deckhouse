---
title: "Управление балансировкой запросов между endpoint’ами сервиса с Istio"
permalink: ru/user/network/managing_request_between_service_istio.html
lang: ru
---

Для управления балансировкой запросов между endpoint’ами сервиса можно использовать [istio](../reference/mc/istio/).
Перед настройкой балансировки убедитесь, что модуль включен в кластере.

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/#%D1%83%D0%BF%D1%80%D0%B0%D0%B2%D0%BB%D0%B5%D0%BD%D0%B8%D0%B5-%D0%B1%D0%B0%D0%BB%D0%B0%D0%BD%D1%81%D0%B8%D1%80%D0%BE%D0%B2%D0%BA%D0%BE%D0%B9-%D0%B7%D0%B0%D0%BF%D1%80%D0%BE%D1%81%D0%BE%D0%B2-%D0%BC%D0%B5%D0%B6%D0%B4%D1%83-endpoint%D0%B0%D0%BC%D0%B8-%D1%81%D0%B5%D1%80%D0%B2%D0%B8%D1%81%D0%B0 -->

Основной ресурс для управления балансировкой запросов — [DestinationRule](#ресурс-destinationrule) от istio.io, он позволяет настроить нюансы исходящих из подов запросов:

* лимиты/таймауты для TCP;
* алгоритмы балансировки между endpoint'ами;
* правила определения проблем на стороне endpoint'а для выведения его из балансировки;
* нюансы шифрования.

> **Важно!** Все настраиваемые лимиты работают для каждого пода клиента по отдельности! Если настроить для сервиса ограничение на одно TCP-соединение, а клиентских подов — три, то сервис получит три входящих соединения.

## Ресурс DestinationRule

<!-- перенесено из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/istio/istio-cr.html#destinationrule -->

[Reference](https://istio.io/v1.19/docs/reference/config/networking/destination-rule/)

Позволяет:
* Определить стратегию балансировки трафика между эндпоинтами сервиса:
  * алгоритм балансировки (LEAST_CONN, ROUND_ROBIN, ...);
  * признаки смерти эндпоинта и правила его выведения из балансировки;
  * лимиты TCP-соединений и реквестов для эндпоинтов;
  * Sticky Sessions;
  * Circuit Breaker.
* Определить альтернативные группы эндпоинтов для обработки трафика (применимо для Canary Deployments). При этом у каждой группы можно настроить свои стратегии балансировки.
* Настройка TLS для исходящих запросов.
