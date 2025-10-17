PS1='\[\033[01;30m\][deckhouse]\[\033[00m\] \[\033[01;33m\]\u@\h\[\033[01;34m\] \w \$\[\033[00m\] '

# Load bash completion
if [ -f /usr/share/bash-completion/bash_completion ] && ! shopt -oq posix; then
  . /usr/share/bash-completion/bash_completion
elif [ -f /etc/bash_completion ] && ! shopt -oq posix; then
  . /etc/bash_completion
fi

if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 kubernetes_version="{{ index (index . 0) "kubectl" }}"
fi

case "$kubernetes_version" in
{{- range . }}
  {{- $versions := list }}
  {{- range .version }}
    {{- $versions = append $versions (printf "%s.*" .) }}
  {{- end }}
  {{ join " | " $versions }} )
    kubectl_version="{{ .kubectl }}"
    ;;
{{- end }}
esac

eval "$(kubectl-${kubectl_version} completion bash)"
eval "$(deckhouse-controller --completion-script-bash | sed -e s/deckhouse/deckhouse-controller/g)"
