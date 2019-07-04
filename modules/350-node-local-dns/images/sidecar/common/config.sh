#!/usr/bin/env bash

kube_dns_endpoints=$(kubectl -n kube-system get ep kube-dns -o json | jq -re '[.subsets[].addresses[].ip] | join(" ")')

cat << EOF
$KUBE_CLUSTER_DOMAIN:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints {
            force_tcp
    }
    prometheus 127.0.0.1:9254
    health $KUBE_DNS_SVC_IP:8080
}
in-addr.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints {
            force_tcp
    }
    prometheus 127.0.0.1:9254
}
ip6.arpa:53 {
    errors
    cache 30
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . $kube_dns_endpoints {
            force_tcp
    }
    prometheus 127.0.0.1:9254
}
.:53 {
    errors
    cache 300
    reload
    loop
    bind $KUBE_DNS_SVC_IP 169.254.20.10
    forward . /etc/resolv.conf
    prometheus 127.0.0.1:9254
}
EOF
