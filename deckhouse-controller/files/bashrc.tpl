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

# Enhanced prompt with git branch support
parse_git_branch() {
    git branch 2> /dev/null | sed -e '/^[^*]/d' -e 's/* \(.*\)/(\1)/'
}

PS1='\[\033[01;30m\][deckhouse]\[\033[00m\] \[\033[01;33m\]\u@\h\[\033[01;34m\] \w \[\033[01;32m\]$(parse_git_branch)\[\033[00m\]\$ '

# Useful aliases
alias ll='ls -alF --color=auto'
alias la='ls -A --color=auto'
alias l='ls -CF --color=auto'
alias ls='ls --color=auto'
alias grep='grep --color=auto'
alias fgrep='fgrep --color=auto'
alias egrep='egrep --color=auto'
alias dir='dir --color=auto'
alias vdir='vdir --color=auto'

# Navigation aliases
alias ..='cd ..'
alias ...='cd ../..'
alias ....='cd ../../..'
alias .....='cd ../../../..'

# Safety aliases
alias rm='rm -i'
alias cp='cp -i'
alias mv='mv -i'

# System information aliases
alias df='df -h'
alias du='du -h'
alias free='free -h'
alias ps='ps aux'
alias psg='ps aux | grep'
alias top='top -c'
alias htop='htop -C'

# Network aliases
alias ports='netstat -tulanp'
alias listening='netstat -tlnp'

# Kubernetes aliases
alias k='kubectl'
alias kgp='kubectl get pods'
alias kgs='kubectl get services'
alias kgd='kubectl get deployments'
alias kgn='kubectl get nodes'
alias kd='kubectl describe'
alias kl='kubectl logs'
alias kex='kubectl exec -it'

# Deckhouse specific aliases
alias dh='deckhouse-controller'
alias dhc='deckhouse-controller'

# Functions
mkcd() {
    mkdir -p "$1" && cd "$1"
}

extract() {
    if [ -f "$1" ] ; then
        case "$1" in
            *.tar.bz2)   tar xjf "$1"     ;;
            *.tar.gz)    tar xzf "$1"     ;;
            *.bz2)       bunzip2 "$1"     ;;
            *.rar)       unrar e "$1"     ;;
            *.gz)        gunzip "$1"      ;;
            *.tar)       tar xf "$1"      ;;
            *.tbz2)      tar xjf "$1"     ;;
            *.tgz)       tar xzf "$1"     ;;
            *.zip)       unzip "$1"       ;;
            *.Z)         uncompress "$1"  ;;
            *.7z)        7z x "$1"        ;;
            *)           echo "'$1' cannot be extracted via extract()" ;;
        esac
    else
        echo "'$1' is not a valid file"
    fi
}

# kubectl version detection and completion
if [ -s /tmp/kubectl_version ]; then
 kubernetes_version="$(cat /tmp/kubectl_version)"
else
 kubectl_version="{{ index (index . 0) "kubectl" }}"
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

# Enable completions
eval "$(kubectl-${kubectl_version} completion bash)"
eval "$(deckhouse-controller --completion-script-bash | sed -e s/deckhouse/deckhouse-controller/g)"

# Enable kubectl completion for 'k' alias
complete -F __start_kubectl k
