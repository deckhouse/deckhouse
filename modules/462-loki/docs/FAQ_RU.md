---
title: "Loki: FAQ"
type:
  - instruction
search: Loki
---

{% raw %}

## Алерт `LokiDiscardedSamplesWarning`

Алерт **`LokiDiscardedSamplesWarning`** срабатывает, когда **Loki** отбрасывает логи во время приёма данных, потому что они **не проходят валидацию** или **превышают лимиты ingestion-а**.

Это значит, что **log-shipper** пытается отправить данные, которые Loki не принимает.

---

### Возможные причины

1. **Превышен лимит на размер или количество стримов**
   - Loki ограничивает:
     - количество уникальных label’ов в стриме;
     - длину label’ов и их значений;
     - количество стримов от одного клиента;
     - размер batch-а при ingestion-е.

Подробное описание каждого случая можно найти в [официальной документации Loki](https://grafana.com/docs/loki/lateoperations/request-validation-rate-limits/#validation-errors).

---

### Что делать

1. **Посмотреть причину в метрике**
   - В алерте указано:
     ```
     Samples are discarded because of "{{ $labels.reason }}"
     ```
     Это поле подскажет, на какой лимит наткнулся Loki (`reason`).

2. **Найти соответствующий лимит в ModuleConfig**
   - Перейдите в **MC** и отредактируйте лимит по имени (`reason`) в конфигурации Loki.
Описание доступных для тюнинга полей можно найти в [документации к модулю Loki](configuration.html#parameters-lokiconfig).
   - Например:
     - `rate_limited` → увеличить `ingestionRateMB` или же `ingestionBurstSizeMB` если возможны "всплески" количества логов. Также как один из вариантов - это ограничить количество отсылаемых логов на строне приложения;



---

{% endraw %}
