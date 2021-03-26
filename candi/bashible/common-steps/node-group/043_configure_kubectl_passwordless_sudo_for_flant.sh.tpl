{{- if eq .nodeGroup.name "master" }}
bb-sync-file /etc/sudoers.d/sudoers_flant_kubectl - << "EOF"
%flant ALL=(root) NOPASSWD:/usr/bin/kubectl,/usr/local/bin/kubectl
EOF
{{- end  }}
