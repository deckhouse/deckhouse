---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---


## How to manually switch keepalived

1. go to the desired pods `kubectl -n d8-keepalived exec -it keepalived-<name> -- sh`
1. edit `vi /etc/keepalived/keepalived.conf` and in the line with `priority` replace the value with the number of keepalived pods + 1
1. send a signal to reread the configuration `kill -HUP 1`
