PS1='\[\033[01;30m\][deckhouse]\[\033[00m\] \[\033[01;33m\]\u@\h\[\033[01;34m\] \w \$\[\033[00m\] '

source /etc/bashrc.d/bash_completion.sh


if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 kubectl_version="{{ index .kubectlForBaseComponents 0 }}"
fi

case "$kubernetes_version" in
{{ $lens := len .k8sVersions }}
  {{ index .k8sVersions 0 }}.* {{ if gt $lens 1 }}| {{ index .k8sVersions 1 }}.* {{ end }}{{ if gt $lens 2 }}| {{ index .k8sVersions 2 }}.* {{ end }})
    kubectl_version="{{ index .kubectlForBaseComponents 0 }}"
    ;;
{{- if gt $lens 3 }}
  {{ index .k8sVersions 3 }}.* {{ if gt $lens 4 }}| {{ index .k8sVersions 4 }}.* {{ end }}{{ if gt $lens 5 }}| {{ index .k8sVersions 5 }}.* {{ end }})
    kubectl_version="{{ index .kubectlForBaseComponents 1 }}"
    ;;
{{- end }}
esac

eval "$(kubectl-${kubectl_version} completion bash)"
eval "$(deckhouse-controller --completion-script-bash | sed -e s/deckhouse/deckhouse-controller/g)"
