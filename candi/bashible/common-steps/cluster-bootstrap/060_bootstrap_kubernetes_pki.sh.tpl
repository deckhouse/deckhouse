{{- if eq .runType "ClusterBootstrap" }}
# Read previously discovered IP
export MY_IP="$(</var/lib/bashible/discovered-node-ip)"

function subst_config() {
    tmpfile=$(mktemp /tmp/kubeadm-config.XXXXXX)
    envsubst < "$1" > "$tmpfile"
    mv "$tmpfile" "$1"
}

subst_config /var/lib/bashible/kubeadm/config.yaml
for file in $(find /var/lib/bashible/kubeadm/kustomize/*.yaml); do
  subst_config "$file"
done
{{- end }}

kubeadm init phase certs ca --config /var/lib/bashible/kubeadm/config.yaml
