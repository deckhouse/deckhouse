{{- if ne .runType "ClusterBootstrap" }}
# If kubelet use apiserver address direct or haproxy, reconfigure to use kubernetes-api-proxy service
if grep -E 'server: https://([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}:6443)|(apiserver:6444)' /etc/kubernetes/kubelet.conf > /dev/null; then
  sed -r -e 's/server: https:\/\/(\b[0-9]{1,3}\.){3}[0-9]{1,3}:6443/server: https:\/\/kubernetes:6445/' -e 's/server: https:\/\/apiserver:6444/server: https:\/\/kubernetes:6445/' -i /etc/kubernetes/kubelet.conf
  bb-flag-set kubelet-need-restart
fi
{{- end }}
