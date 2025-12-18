---
title: "Модуль loki: FAQ"
type:
  - instruction
search: Loki, LokiDiscardedSamplesWarning
description: "FAQ по модулю Loki: устранение проблем с алертом LokiDiscardedSamplesWarning, решение проблем с приёмом логов и настройка лимитов Loki"
---

{% raw %}

## Почему срабатывает алерт LokiDiscardedSamplesWarning?

[Алерт `LokiDiscardedSamplesWarning`](/products/kubernetes-platform/documentation/v1/reference/alerts.html#loki-lokidiscardedsampleswarning) срабатывает, когда Loki отказывается принимать логи во время приёма данных, потому что они не проходят валидацию или превышают установленные лимиты.

Иными словами, `log-shipper` пытается отправить данные, которые Loki не принимает.

### Возможные причины

Вероятнее всего превышен лимит на размер или количество стримов.

Loki ограничивает:

- количество уникальных лейблов в стриме;
- длину лейблов и их значений;
- количество стримов от одного клиента;
- размер пакета данных (batch) при приёме.

Подробное описание каждого ограничения можно найти в [официальной документации Loki](https://grafana.com/docs/loki/latest/operations/request-validation-rate-limits/#validation-errors).

### Устранение проблемы

1. Уточните причину в описании алерта.

   ```text
   Samples are being discarded because of "{{ $labels.reason }}"...
   ```

   Поле `{{ $labels.reason }}` указывает на сработавший лимит и причину, по которой Loki не принимает логи.

1. Отредактируйте соответствующий лимит в конфигурации модуля `loki`.

   Описание доступных для настройки полей можно найти в [документации модуля](configuration.html#parameters-lokiconfig).

   Например, если в качестве причины указан `rate_limited`, необходимо увеличить `ingestionRateMB` или `ingestionBurstSizeMB`, если возможны временные повышения объёма логов. В качестве альтернативы можно ограничить количество отсылаемых логов на стороне приложения.

{% endraw %}
