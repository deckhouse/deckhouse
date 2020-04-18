# Remove bootstrap token if kubelet is succesfully bootstraped
if test -f /var/lib/bashible/bootstrap-token && test -f /etc/kubernetes/kubelet.conf
then
  rm /var/lib/bashible/bootstrap-token
fi
