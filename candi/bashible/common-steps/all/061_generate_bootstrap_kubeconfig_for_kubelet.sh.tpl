{{- if eq .runType "Normal" }}
# Do nothing, if kubelet is already bootstraped
if [ -f /etc/kubernetes/kubelet.conf ] ; then exit 0 ; fi

# Generate bootstrap kubeconfig for kubelet
cat > /etc/kubernetes/bootstrap-kubelet.conf << EOF
apiVersion: v1
kind: Config
current-context: kubelet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: $(cat /var/lib/bashible/ca.crt | base64 -w0)
    server: https://kubernetes:6445/
  name: default
contexts:
- context:
    cluster: default
    user: kubelet-bootstrap
  name: kubelet-bootstrap@default
users:
- name: kubelet-bootstrap
  user:
    as-user-extra: {}
    token: $(</var/lib/bashible/bootstrap-token)
EOF
chmod 0600 /etc/kubernetes/bootstrap-kubelet.conf
{{- end }}
