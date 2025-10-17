# Enable color support
export TERM=xterm-256color
export CLICOLOR=1

# History configuration
export HISTSIZE=10000
export HISTFILESIZE=20000
export HISTCONTROL=ignoredups:erasedups
export HISTTIMEFORMAT='%F %T '
shopt -s histappend
shopt -s cmdhist
shopt -s histreedit
shopt -s histverify

# Shell options for better usability
shopt -s checkwinsize
shopt -s cdspell
shopt -s dirspell
shopt -s globstar 2>/dev/null

# Environment variables
export EDITOR=vim
export PAGER=less
export LESS='-R -i -M -S -x4'
export GREP_OPTIONS='--color=auto'
export LS_COLORS='di=34:ln=35:so=32:pi=33:ex=31:bd=46;34:cd=43;34:su=41;30:sg=46;30:tw=42;30:ow=43;30'

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
