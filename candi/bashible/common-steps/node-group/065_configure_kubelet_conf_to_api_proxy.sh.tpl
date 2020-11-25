{{- if ne .runType "ClusterBootstrap" }}

# Do nothing, if kubelet wasn't bootstraped yet
if [ ! -f /etc/kubernetes/kubelet.conf ] ; then exit 0 ; fi
if [ ! -f /var/lib/kubelet/pki/kubelet-client-current.pem ] ; then exit 0 ; fi

bb-event-on 'bb-sync-file-changed' 'bb-flag-set kubelet-need-restart'

bb-sync-file /etc/kubernetes/kubelet.conf - << EOF
apiVersion: v1
kind: Config

clusters:
- cluster:
    certificate-authority-data: $(cat /etc/kubernetes/pki/ca.crt | base64 -w0)
    server: https://127.0.0.1:6445
  name: d8-cluster

users:
- name: d8-user
  user:
    client-certificate: /var/lib/kubelet/pki/kubelet-client-current.pem
    client-key: /var/lib/kubelet/pki/kubelet-client-current.pem

contexts:
- context:
    cluster: d8-cluster
    namespace: default
    user: d8-user
  name: d8-context

current-context: d8-context
preferences: {}
EOF
{{- end }}
