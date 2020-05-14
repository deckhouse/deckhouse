#!/bin/bash

{{- if hasKey . "cloudProvider" }}
  {{- if hasKey .cloudProvider.yandex "dns" }}

bb-event-on 'bb-sync-file-changed' '_on_netplan_config_changed'
_on_netplan_config_changed() {
  netplan generate
  netplan apply
}

ip_addr_show_output=$(ip -json addr show)
primary_mac="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
primary_ifname="$(echo "$ip_addr_show_output" | jq -re --arg mac "$primary_mac" '.[] | select(.address == $mac) | .ifname')"

bb-sync-file /etc/netplan/51-nameservers.yaml - <<END
network:
    version: 2
    ethernets:
        ${primary_ifname}:
  {{- if hasKey .cloudProvider.yandex.dns "nameservers" }}
    {{- if .cloudProvider.yandex.dns.nameservers }}
            nameservers:
                addresses: [{{- .cloudProvider.yandex.dns.nameservers | join ", " -}}]
    {{- end }}
  {{- end }}
  {{- if hasKey .cloudProvider.yandex.dns "search" }}
    {{- if .cloudProvider.yandex.dns.search }}
            nameservers:
                search: [{{- .cloudProvider.yandex.dns.search | join ", " -}}]
    {{- end }}
  {{- end }}
            dhcp4-overrides:
              use-dns: false
  {{- if hasKey .cloudProvider.yandex.dns "search" }}
              use-domains: false
  {{- end }}
END
  {{- end -}}
{{- end -}}
