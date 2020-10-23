bb-event-on 'd8-br-netfilter-changed' '_load_module_br_netfilter'
_load_module_br_netfilter() {
  modprobe br_netfilter
}

bb-sync-file /etc/modules-load.d/d8_br_netfilter.conf - d8-br-netfilter-changed <<< "br_netfilter"
