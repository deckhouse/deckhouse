{{- if ne .runType "ClusterBootstrap" }}
# If kubelet use apiserver address direct or haproxy, reconfigure to use kubernetes-api-proxy service
if grep -E 'server: https://([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}:6443)|(apiserver:6444)' /etc/kubernetes/kubelet.conf > /dev/null; then
  sed -r -e 's/server: https:\/\/(\b[0-9]{1,3}\.){3}[0-9]{1,3}:6443/server: https:\/\/kubernetes:6445/' -e 's/server: https:\/\/apiserver:6444/server: https:\/\/kubernetes:6445/' -i /etc/kubernetes/kubelet.conf
  bb-flag-set kubelet-need-restart
fi

# IF kubelet use incorrect certs, reconfigure to use auto renew certs
if grep -E '(client-certificate-data:\s+[a-zA-Z0-9=]+)|(client-certificate:\s+.+kubelet-client.crt)' /etc/kubernetes/kubelet.conf > /dev/null; then
  sed -i 's/    client-certificate.*$/    client-certificate: \/var\/lib\/kubelet\/pki\/kubelet-client-current.pem/' /etc/kubernetes/kubelet.conf
  bb-flag-set kubelet-need-restart
fi
if grep -E '(client-key-data:\s+[a-zA-Z0-9=]+)|(client-key:\s+.+kubelet-client.key)' /etc/kubernetes/kubelet.conf > /dev/null ; then
  sed -i 's/    client-key.*$/    client-key: \/var\/lib\/kubelet\/pki\/kubelet-client-current.pem/' /etc/kubernetes/kubelet.conf
  bb-flag-set kubelet-need-restart
fi
{{- end }}
