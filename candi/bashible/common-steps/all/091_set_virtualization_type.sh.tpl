{{- if eq .runType "Normal" }}
# If there is no kubelet.conf than node is not bootstrapped and there is nothing to do
kubeconfig="/etc/kubernetes/kubelet.conf"
if [ ! -f "$kubeconfig" ]; then
  exit 0
fi

virtualization="$(virt-what | head -n 1)"
if [[ "$virtualization" == "" ]]; then
  virtualization="unknown"
fi
max_attempts=5
node=$(hostname -s)

until kubectl --kubeconfig $kubeconfig annotate --overwrite=true node "$node" node.deckhouse.io/virtualization="$virtualization"; do
  attempt=$(( attempt + 1 ))
  if [ "$attempt" -gt "$max_attempts" ]; then
    bb-log-error "failed to annotate node $node after $max_attempts attempts"
    exit 1
  fi
  echo "Waiting for annotate node $node (attempt $attempt of $max_attempts)..."
  sleep 5
done
{{- end  }}
