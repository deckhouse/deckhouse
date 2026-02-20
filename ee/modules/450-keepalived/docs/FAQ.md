---
title: "keepalived: FAQ"
type:
  - instruction
search: keepalived, manual, switch
---

## How to manually switch keepalived?

1. Enter the desired pod using a debug container with a shared process namespace:
   `d8 k debug -n d8-keepalived -it keepalived-<name> --profile=general --target keepalived`.
1. Edit the configuration file `vim /proc/1/root/etc/keepalived/keepalived.conf`, replace the value in the `priority` line with <number of keepalived pods + 1> or set a value higher than the current VRRP master (e.g., `255`).
1. Apply settings – send a signal to reload the configuration: `kill -HUP 1`.
