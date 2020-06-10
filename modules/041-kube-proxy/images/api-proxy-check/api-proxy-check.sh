#!/bin/bash
set -Eeuo pipefail

api_proxy_address="127.0.0.1:6445"

sa="/var/run/secrets/kubernetes.io/serviceaccount"
token=$(cat $sa/token)

cp /var/lib/kube-proxy-cm/config.conf /var/lib/kube-proxy/config.conf
cp /var/lib/kube-proxy-cm/kubeconfig.conf /var/lib/kube-proxy/kubeconfig.conf

if curl -svk -o /dev/null --header "Authorization: Bearer $token" "https://$api_proxy_address/api"; then
  sed 's#server:.*#server: https://kubernetes:6445#' /var/lib/kube-proxy-cm/kubeconfig.conf > /var/lib/kube-proxy/kubeconfig.conf
fi
