rm -rf /etc/kubernetes/pki/
mkdir -m 0700 /etc/kubernetes/pki/
kubeadm init phase certs ca --config /var/lib/bashible/kubeadm/config.yaml
