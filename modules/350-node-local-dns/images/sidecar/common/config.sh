#!/usr/bin/env bash

set -Eeuo pipefail

upstream_nameservers="/etc/resolv.conf"

if upstreams_config=$(kubectl -n kube-system get cm kube-dns -o json | jq '.data.upstreamNameservers' -r | yq r -j - | jq -r '. | join(" ")'); then
    upstream_nameservers="$upstreams_config"
fi


kube_dns_endpoints=$(kubectl -n kube-system get ep kube-dns -o json | jq -re '[.subsets[].addresses[].ip] | join(" ")')

cat << EOF
$KUBE_CLUSTER_DOMAIN:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints
    prometheus 127.0.0.1:9254
    health 127.0.0.1:9225
}
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints
    prometheus 127.0.0.1:9254
}
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints
    prometheus 127.0.0.1:9254
}
.:53 {
    errors
    cache 300
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $upstream_nameservers
    prometheus 127.0.0.1:9254
}
EOF
