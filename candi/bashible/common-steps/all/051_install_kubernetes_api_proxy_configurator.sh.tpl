bb-event-on 'bb-sync-file-changed' '_on_kubernetes_api_proxy_configurator_changed'
_on_kubernetes_api_proxy_configurator_changed() {
  if systemctl is-enabled --quiet kubernetes-api-proxy 2>/dev/null ; then
    systemctl restart kubernetes-api-proxy-configurator
  fi
}

bb-sync-file /var/lib/bashible/kubernetes-api-proxy-configurator.sh - << "EOF"
#!/bin/bash

# Read from command args
apiserver_endpoints=$@
apiserver_backup_endpoints=""

if [ -z "$apiserver_endpoints" ] ; then
  self_node_addresses=""
  if self_node=$(kubectl --kubeconfig=/etc/kubernetes/kubelet.conf get node $HOSTNAME -o json); then
    self_node_addresses="$(echo "$self_node" | jq '.status.addresses[] | .address' -r)"
  fi

  if eps=$(kubectl --kubeconfig=/etc/kubernetes/kubelet.conf -n default get endpoints kubernetes -o json) ; then
    for ep in $(echo "$eps" | jq '.subsets[] | (.ports[0].port | tostring) as $port | .addresses[] | .ip + ":" +  $port' -r) ; do
      ip_regex=$(echo $ep | cut -d: -f1 | sed 's/\./\\./g')

      if echo "$self_node_addresses" | grep $ip_regex > /dev/null ; then
        apiserver_endpoints="$apiserver_endpoints $ep"
      else
        # If endpoint is not local treat it as a backup
        apiserver_backup_endpoints="$apiserver_backup_endpoints $ep"
      fi
    done

    # If there are no local enpoints use remote normally
    if [ -z "$apiserver_endpoints" ] ; then
      apiserver_endpoints="$apiserver_backup_endpoints"
      apiserver_backup_endpoints=""
    fi
  fi
fi

# Fail, if there are no endpoints
if [ -z "$apiserver_endpoints" ] ; then
  exit 1
fi

# Generate nginx config (to the temporary location)
cat > /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf << END
{{- if or (eq .bundle "ubuntu-18.04") (eq .bundle "ubuntu-lts") }}
user www-data;
{{- else if eq .bundle "centos-7" }}
user nginx;
{{- end }}

pid /var/run/kubernetes-api-proxy.pid;
error_log stderr notice;

worker_processes 2;
worker_rlimit_nofile 130048;
worker_shutdown_timeout 10s;

events {
  multi_accept on;
  use epoll;
  worker_connections 16384;
}

stream {
  upstream kubernetes {
    least_conn;
$(
for ep in $apiserver_endpoints ; do
  echo -e "    server $ep;"
done
for ep in $apiserver_backup_endpoints ; do
  echo -e "    server $ep backup;"
done
)
  }
  server {
    listen 127.0.0.1:6445;
    proxy_pass kubernetes;
    proxy_timeout 10m;
    proxy_connect_timeout 1s;
  }
}
END

if [ ! -f /etc/kubernetes/kubernetes-api-proxy/nginx.conf ] ; then
  cp /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf /etc/kubernetes/kubernetes-api-proxy/nginx.conf
  systemctl restart kubernetes-api-proxy
else
  old_config=$(sha256sum /etc/kubernetes/kubernetes-api-proxy/nginx.conf | awk '{print $1}')
  new_config=$(sha256sum /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf | awk '{print $1}')
  if [ "$old_config" != "$new_config" ] ; then
    mv /etc/kubernetes/kubernetes-api-proxy/nginx_new.conf /etc/kubernetes/kubernetes-api-proxy/nginx.conf
    systemctl reload kubernetes-api-proxy
  fi
fi
EOF

chmod +x /var/lib/bashible/kubernetes-api-proxy-configurator.sh
