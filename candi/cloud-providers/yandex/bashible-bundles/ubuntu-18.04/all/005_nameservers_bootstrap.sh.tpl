if bb-flag? is-bootstrapped; then exit 0; fi

echo "Not realized yet!"
exit 1

{{- /* if .Values.cloudInstanceManager.internal.cloudProvider.yandex.nameservers */ -}}

ip_addr_show_output=$(ip -json addr show)
primary_mac="$(grep -Po '(?<=macaddress: ).+' /etc/netplan/50-cloud-init.yaml)"
primary_ifname="$(echo "$ip_addr_show_output" | jq -re --arg mac "$primary_mac" '.[] | select(.address == $mac) | .ifname')"

cat > /etc/netplan/51-nameservers.yaml <<END
network:
    version: 2
    ethernets:
        ${primary_ifname}:
            nameservers:
                addresses: [{{- /* .Values.cloudInstanceManager.internal.cloudProvider.yandex.nameservers | join ", " */ -}}]
            dhcp4-overrides:
              use-dns: false
END

netplan generate
netplan apply
{{- /* end */ -}}
