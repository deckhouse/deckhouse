---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---


## Как вручную переключить keepalived

1. зайти в нужный под `kubectl -n d8-keepalived exec -it keepalived-<name> -- sh`
1. отредактировать `vi /etc/keepalived/keepalived.conf` и в строке с `priority` заменить значение на число подов keepalived + 1
1. отправить сигнал на перечитывание конфигурации `kill -HUP 1`
