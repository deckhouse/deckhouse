#!/bin/sh

iptables_once() {
  sh -c "iptables -C $1 2>/dev/null || iptables -A $1"
}

proto="$1"
[ -z "$proto" ] && proto="tcp"
route_table="10" # tcp
mgmtport="8989"  # tcp
[ "${proto}" = "udp" ] && route_table="11" && mgmtport="9090"

iptables_once "POSTROUTING -t nat -s {{ include "get_network_with_bitmask" (list . .Values.openvpn.tunnelNetwork) }} ! -d {{ include "get_network_with_bitmask" (list . .Values.openvpn.tunnelNetwork) }} -j MASQUERADE"
iptables_once "PREROUTING -t mangle -i tun-${proto} -j CONNMARK --set-mark ${route_table}"
iptables_once "PREROUTING -t mangle ! -i tun+ -j CONNMARK --restore-mark"
iptables_once "OUTPUT -t mangle -j CONNMARK --restore-mark"
ip rule add fwmark ${route_table} lookup ${route_table} pref ${route_table} || true

sh -c "while [ ! -d  /sys/class/net/tun-${proto} ] ; do echo \"Wait for tun-${proto} init\"; sleep 1; done; ip route add {{ include "get_network_with_bitmask" (list . .Values.openvpn.tunnelNetwork) }} dev tun-${proto} table ${route_table}" &

mkdir -p /dev/net
if [ ! -c /dev/net/tun ]; then
    mknod /dev/net/tun c 10 200
fi

wait_file() {
  file_path="$1"
  while true; do
    if [ -f $file_path ]; then
      break
    fi
    echo "wait $file_path"
    sleep 2
  done
}

easyrsa_path="/etc/openvpn/certs"

wait_file "$easyrsa_path/pki/ca.crt"
wait_file "$easyrsa_path/pki/private/server.key"
wait_file "$easyrsa_path/pki/issued/server.crt"
wait_file "$easyrsa_path/pki/ta.key"
wait_file "$easyrsa_path/pki/dh.pem"
wait_file "$easyrsa_path/pki/crl.pem"

exec openvpn --config /etc/openvpn/openvpn.conf --proto "${proto}" --management 127.0.0.1 "${mgmtport}" --dev "tun-${proto}"
