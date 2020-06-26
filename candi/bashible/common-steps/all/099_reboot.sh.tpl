{{- if ne .runType "ClusterBootstrap" }}
if bb-flag? reboot; then
  bb-deckhouse-get-disruptive-update-approval
  bb-log-info "Rebooting machine after bootstrap process completed"
  bb-flag-unset reboot

  {{- if eq .runType "Normal" }}
  systemctl stop kubelet

  # Wait till kubelet stopped
  attempt=0
  until ! pidof kubelet > /dev/null; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "20" ]; then
      bb-log-error "Can't stop kubelet. Will try to set NotReady status while kubelet is running."
      break
    fi
    bb-log-info "Waiting till kubelet stopped (20sec)..."
    sleep 1
  done

  # Our task is to force setting Node status to NotReady to prevent unwanted schedulings during reboot.
  # We could update .status.conditions directly, but:
  # * kubectl can't edit status subresource by design (related discussion https://github.com/kubernetes/kubectl/issues/564).
  # * curl in CentOS can't read kubelet client certificate key from /var/lib/kubelet/pki/kubelet-client-current.pem due to libnss bug.
  # * wget in CentOS has no --method argument, so we cant use PATCH HTTP request.
  # The solution — to delete Lease object for our node and handle this event with Deckhouse hook modules/040-node-manager/hooks/node_lease_handler.
  bb-log-info "Deleting node Lease resource..."
  attempt=0
  until kubectl --kubeconfig=/etc/kubernetes/kubelet.conf -n kube-node-lease delete lease "${HOSTNAME}"; do
    attempt=$(( attempt + 1 ))
    if [ "$attempt" -gt "2" ]; then
      bb-log-warning "Can't delete node Lease resource. Node status won't be set to NotReady."
      break
    fi
    bb-log-info "Retrying delete node Lease resource..."
    sleep 1
  done
  {{- end }}

  shutdown -r now
fi
{{- else }}
# to prevent extra reboot during first "Normal" run.
bb-flag-unset reboot
{{- end }}
