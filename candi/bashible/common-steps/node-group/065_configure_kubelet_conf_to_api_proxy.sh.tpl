{{- if ne .runType "ClusterBootstrap" }}

kubelet_kubeconfig_path="/etc/kubernetes/kubelet.conf"

if [ ! -f $kubelet_kubeconfig_path ]; then
  exit 0
fi

kubelet_certificate_path="/var/lib/kubelet/pki/kubelet-client-current.pem"
kubelet_kubeconfig_user=$(kubectl --kubeconfig ${kubelet_kubeconfig_path} config view -o json | jq '.users[].name' -r)

# Reconfigure kubelet if it doesn't use kubernetes-api-proxy
if ! kubectl --kubeconfig ${kubelet_kubeconfig_path} config view -o json | jq '.clusters[].cluster.server' -r | grep 'https://kubernetes:6445' -q ; then
  kubectl --kubeconfig ${kubelet_kubeconfig_path} config set clusters.kubernetes.server https://kubernetes:6445
  bb-flag-set kubelet-need-restart
fi

# If kubelet use incorrect certs, reconfigure to use certs that are auto-renewed
if ! kubectl --kubeconfig ${kubelet_kubeconfig_path} config view -o json | jq --arg user ${kubelet_kubeconfig_user} '.users[] | select(.name == $user) | .user."client-certificate"' -r | grep ${kubelet_certificate_path} -q ; then
  kubectl --kubeconfig ${kubelet_kubeconfig_path} config set   users.${kubelet_kubeconfig_user}.client-certificate ${kubelet_certificate_path}
  kubectl --kubeconfig ${kubelet_kubeconfig_path} config unset users.${kubelet_kubeconfig_user}.client-certificate-data
  bb-flag-set kubelet-need-restart
fi
if ! kubectl --kubeconfig ${kubelet_kubeconfig_path} config view -o json | jq --arg user ${kubelet_kubeconfig_user} '.users[] | select(.name == $user) | .user."client-key"' -r | grep ${kubelet_certificate_path} -q ; then
  kubectl --kubeconfig ${kubelet_kubeconfig_path} config set   users.${kubelet_kubeconfig_user}.client-key ${kubelet_certificate_path}
  kubectl --kubeconfig ${kubelet_kubeconfig_path} config unset users.${kubelet_kubeconfig_user}.client-key-data
  bb-flag-set kubelet-need-restart
fi
{{- end }}
