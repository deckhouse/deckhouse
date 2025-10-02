---
title: "Extended monitoring модуль: FAQ"
type:
  - instruction
search: extended monitoring, image-availability-exporter
---

{% raw %}

## Как переключиться на HTTP вместо HTTPS для проверки образов из собственного registry?

Чтобы изменить протокол проверки вашего registry с HTTPS на HTTP, измените параметр `settings.imageAvailability.registry.scheme` в конфигурации модуля.

Подробные инструкции смотрите в [документации по настройке модуля](./configuration.html#parameters-imageavailability-registry-scheme).

{% endraw %}
