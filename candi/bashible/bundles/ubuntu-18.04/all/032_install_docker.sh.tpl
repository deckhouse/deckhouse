bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  systemctl enable docker.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart docker.service
{{- end }}
}

bb-apt-install "docker.io=18.09.7-0ubuntu1~18.04.4"
