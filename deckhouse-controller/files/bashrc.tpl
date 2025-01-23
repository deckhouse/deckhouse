PS1='\[\033[01;30m\][deckhouse]\[\033[00m\] \[\033[01;33m\]\u@\h\[\033[01;34m\] \w \$\[\033[00m\] '

source /etc/bashrc.d/bash_completion.sh


if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 kubectl_version="{{ index .kubectlForBaseComponents 0 }}"
fi

case "$kubernetes_version" in
  {{ index .k8sVersions 0 }}.* | {{ index .k8sVersions 1 }}.* | {{ index .k8sVersions 2 }}.* )
    kubectl_version="{{ index .kubectlForBaseComponents 0 }}"
    ;;
  {{ index .k8sVersions 3 }}.* | {{ index .k8sVersions 4 }}.* {{ if gt (len .k8sVersions) 5 }}| {{ index .k8sVersions 5 }}.* {{ end }})
    kubectl_version="{{ index .kubectlForBaseComponents 1 }}"
    ;;
esac

eval "$(kubectl-${kubectl_version} completion bash)"
eval "$(deckhouse-controller --completion-script-bash | sed -e s/deckhouse/deckhouse-controller/g)"
