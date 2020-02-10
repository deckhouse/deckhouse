{{- if .instanceGroup.instanceClass.additionalSubnets }}
  {{- fail "CentOS support is not implemented yet" }}
{{- end }}
#!/bin/bash

# Overriding hostname received from metadata server
hostnamectl set-hostname "$(hostname | cut -d "." -f 1)"
