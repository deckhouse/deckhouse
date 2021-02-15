modprobe br_netfilter
modprobe overlay

bb-sync-file /etc/modules-load.d/d8_br_netfilter.conf - <<< "br_netfilter"
bb-sync-file /etc/modules-load.d/d8_overlay.conf - <<< "overlay"
