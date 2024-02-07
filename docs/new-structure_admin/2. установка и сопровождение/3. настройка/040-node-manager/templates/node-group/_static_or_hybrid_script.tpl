{{- define "node_group_static_or_hybrid_script" -}}
  {{- $context := index . 0 -}}
  {{- $ng := index . 1 -}}
  {{- $bootstrap_token := index . 2 -}}
#!/bin/bash

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

checkBashible=$(systemctl is-active bashible.timer)
if [[ "$checkBashible" != "active" ]]; then
  /var/lib/bashible/bootstrap.sh
else
  echo "The node already exists in the cluster and under bashible."
fi
{{ end }}
