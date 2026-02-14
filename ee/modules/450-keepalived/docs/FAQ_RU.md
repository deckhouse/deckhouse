---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---

## Как вручную переключить keepalived?

1. Зайдите в нужный под, используя debug-контейнер с общим пространством процессов:
  `d8 k debug -n d8-keepalived -it keepalived-<name> --profile=general --target keepalived`.
2. Отредактируйте файл конфигурации `vim /proc/1/root/etc/keepalived/keepalived.conf`, где в строке с параметром `priority` замените значение на <число подов keepalived + 1> или установите значение выше, чем у текущего VRRP-мастера (например, `255`).
3. Примените настройки – отправьте сигнал на перечитывание конфигурации: `kill -HUP 1`.
