bb-event-on 'bb-package-installed' 'post-install'
post-install() {
  if bb-flag? there-was-docker-installed; then
    bb-flag-set reboot
    bb-flag-unset there-was-docker-installed
  fi

  systemctl enable docker.service
{{ if ne .runType "ImageBuilding" -}}
  systemctl restart docker.service
{{- end }}
}

{{- $nvidia_docker := false }}
{{- if hasKey .nodeGroup "docker" }}
  {{- if .nodeGroup.docker.nvidia }}
    {{- $nvidia_docker = true }}
  {{- end }}
{{- end }}

{{- if $nvidia_docker }}
package="docker-ce=5:18.09.7~3-0~ubuntu-$(lsb_release -sc) docker-ce-cli=5:18.09.7~3-0~ubuntu-$(lsb_release -sc) nvidia-container-runtime=2.0.0+docker18.09.7-3 nvidia-docker2=2.0.3+docker18.09.7-3"

if bb-apt-package? docker.io; then
  bb-apt-remove docker.io
fi
{{- else }}

if bb-is-ubuntu-version? 18.04 ; then
  package="docker.io=18.09.7-0ubuntu1~18.04.4"
elif bb-is-ubuntu-version? 16.04 ; then
  package="docker.io=18.09.7-0ubuntu1~16.04.5"
else
  bb-log-error "Unsupported Ubuntu version"
  exit 1
fi

if bb-apt-package? nvidia-docker2; then
  bb-apt-remove nvidia-docker2
fi

if bb-apt-package? nvidia-container-runtime; then
  bb-apt-remove nvidia-container-runtime
fi

if bb-apt-package? docker-ce; then
  bb-apt-remove docker-ce
fi

if bb-apt-package? docker-ce-cli; then
  bb-apt-remove docker-ce-cli
fi

if bb-apt-package? containerd.io; then
  bb-apt-remove containerd.io
fi
{{- end }}

if bb-apt-package? $(echo $pacakge | cut -f1 -d"="); then
  bb-flag-set there-was-docker-installed
fi

if ! bb-apt-package? $package; then
  bb-deckhouse-get-disruptive-update-approval
fi

bb-apt-install $package
