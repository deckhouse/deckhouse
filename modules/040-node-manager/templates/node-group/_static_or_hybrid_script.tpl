{{- define "node_group_static_or_hybrid_script" -}}
  {{- $context := index . 0 -}}
  {{- $ng := index . 1 -}}
  {{- $bootstrap_token := index . 2 -}}
#!/bin/bash

if [[ -f /var/lib/bashible/bootstrap-token ]]; then
  echo "The node already have bootstrap-token and under bashible."
  exit 1
fi

checkBashible=$(systemctl is-active bashible.timer)
if [[ "$checkBashible" == "active" ]]; then
  echo "The node already exists in the cluster and under bashible."
  exit 2
fi

mkdir -p /var/lib/bashible

cat > /var/lib/bashible/bootstrap.sh <<"END"
{{- include "bootstrap_script" (list $context $ng) }}
END
chmod +x /var/lib/bashible/bootstrap.sh

cat > /var/lib/bashible/ca.crt <<"EOF"
{{ $context.Values.nodeManager.internal.kubernetesCA }}
EOF

cat > /var/lib/bashible/bootstrap-token <<"EOF"
{{ $bootstrap_token }}
EOF
chmod 0600 /var/lib/bashible/bootstrap-token

touch /var/lib/bashible/first_run

/var/lib/bashible/bootstrap.sh
{{ end }}
