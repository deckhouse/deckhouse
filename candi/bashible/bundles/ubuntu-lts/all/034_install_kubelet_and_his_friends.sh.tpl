extra_packages=""

{{ if eq .kubernetesVersion "1.14" }}
kubernetes_version="1.14.10-00"
extra_packages="kubernetes-cni=0.7.5-00" # kubernetes-cni package is obsoleted by package kubelet since versions 1.16.11-0, 1.17.7-0, 1.18.4-0

{{ else if eq .kubernetesVersion "1.15" }}
kubernetes_version="1.15.12-00"
extra_packages="kubernetes-cni=0.7.5-00" # kubernetes-cni package is obsoleted by package kubelet since versions 1.16.11-0, 1.17.7-0, 1.18.4-0

{{ else if eq .kubernetesVersion "1.16" }}
kubernetes_version="1.16.11-00"

{{ else if eq .kubernetesVersion "1.17" }}
kubernetes_version="1.17.7-00"

{{ else if eq .kubernetesVersion "1.18" }}
kubernetes_version="1.18.4-00"

{{ else }}
  {{ fail (printf "Unsupported kubernetes version: %s" .kubernetesVersion) }}
{{ end }}

bb-apt-install "kubelet=${kubernetes_version}" "kubectl=${kubernetes_version}" $extra_packages

if [[ "$FIRST_BASHIBLE_RUN" == "yes" && ! -f /etc/systemd/system/kubelet.service.d/10-deckhouse.conf ]]; then
  # stop kubelet immediately after the first install to prevent joining to the cluster with wrong configurations
  systemctl stop kubelet
fi

if kubelet_pid="$(pidof kubelet)"; then
  kubelet_start_date="$(ps -o lstart= -q "$kubelet_pid")"
  kubelet_start_unixtime="$(date --date="$kubelet_start_date" +%s)"
  kubelet_bin_change_unixtime="$(stat -c %Z /usr/bin/kubelet)"

  if [ "$kubelet_bin_change_unixtime" -gt "$kubelet_start_unixtime" ]; then
    bb-flag-set kubelet-need-restart
  fi
fi

mkdir -p /etc/kubernetes/manifests
mkdir -p /etc/systemd/system/kubelet.service.d
mkdir -p /etc/kubernetes/pki
