{{- if eq .runType "Normal" }}
if [ ! -f /etc/kubernetes/pki/ca.crt ] ; then
  cp /var/lib/bashible/ca.crt /etc/kubernetes/pki/
fi
{{- end }}
