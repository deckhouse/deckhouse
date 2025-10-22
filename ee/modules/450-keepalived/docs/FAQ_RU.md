---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---

## Как вручную переключить keepalived?

1. Зайдите в нужный под: `d8 k -n d8-keepalived exec -it keepalived-<name> -- sh`.
1. Отредактируйте файл `vi /etc/keepalived/keepalived.conf`, где в строке с параметром `priority` замените значение на число подов keepalived + 1.
1. Отправьте сигнал на перечитывание конфигурации: `kill -HUP 1`.
