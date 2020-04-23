{{- if hasKey . "cloudProviderClusterConfiguration" }}
  {{- if hasKey .cloudProviderClusterConfiguration "nameservers" }}

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
            nameservers:
                addresses: [{{- .cloudProviderClusterConfiguration.nameservers | join ", " -}}]
            dhcp4-overrides:
              use-dns: false
END
  {{- end -}}
{{- end -}}
