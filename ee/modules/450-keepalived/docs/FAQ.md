---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---


## How to manually switch keepalived?

1. Go to the desired pods: `d8 k -n d8-keepalived exec -it keepalived-<name> -- sh`
1. Edit the `/etc/keepalived/keepalived.conf` file and in the line with the `priority` parameter, replace the value with the number of keepalived pods + 1.
1. Send a signal to reread the configuration: `kill -HUP 1`.
