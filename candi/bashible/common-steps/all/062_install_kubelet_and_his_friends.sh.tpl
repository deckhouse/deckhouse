# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

{{- $kubernetesVersion := printf "%s%s" (.kubernetesVersion | toString) (index .k8s .kubernetesVersion "patch" | toString) | replace "." "" }}
{{- $kubernetesCniVersion := "1.6.2" | replace "." "" }}
bb-package-install "kubernetes-cni:{{ index .images.registrypackages (printf "kubernetesCni%s" $kubernetesCniVersion) | toString }}"

old_kubelet_hash=""
if [ -f "${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest" ]; then
  old_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest")
fi

bb-package-install "kubelet:{{ index .images.registrypackages (printf "kubelet%s" $kubernetesVersion) | toString }}"

new_kubelet_hash=$(<"${BB_RP_INSTALLED_PACKAGES_STORE}/kubelet/digest")
if [[ "${old_kubelet_hash}" != "${new_kubelet_hash}" ]]; then
  bb-flag-set kubelet-need-restart
fi

if grep -qF '# "\e[5~": history-search-backward' /etc/inputrc; then
  sed -i 's/\# \"\\e\[5~\": history-search-backward/\"\\e\[5~\": history-search-backward/' /etc/inputrc
fi
if grep -qF '# "\e[6~": history-search-forward' /etc/inputrc; then
  sed -i 's/^\# \"\\e\[6~\": history-search-forward/\"\\e\[6~\": history-search-forward/' /etc/inputrc
fi

if grep -qF '#force_color_prompt=yes' /root/.bashrc; then
  sed -i 's/\#force_color_prompt=yes/force_color_prompt=yes/' /root/.bashrc
fi
if grep -qF '01;32m' /root/.bashrc; then
  sed -i 's/01;32m/01;31m/' /root/.bashrc
fi

completion="if [ -f /etc/bash_completion ] && ! shopt -oq posix; then . /etc/bash_completion ; fi"
if ! grep -qF -- "$completion"  /root/.bashrc; then
  echo "$completion" >> /root/.bashrc
fi

# Install d8 with completion
bb-package-install "d8:{{ .images.registrypackages.d8 }}"

if [ ! -f "/etc/bash_completion.d/d8" ]; then
  mkdir -p /etc/bash_completion.d
  d8 completion bash > /etc/bash_completion.d/d8
fi

# Install kubectl as alias for d8 k

# This need for correct Tab-completion in kubectl alias
# Bash does not expand aliases during completion, so we
# rewrite "kubectl" to "d8 k" and call d8 __complete directly
cat <<'EOF' > /etc/bash_completion.d/kubectl_d8_completion
__start_kubectl() {
    local cur prev words cword
    _init_completion -n =: || return
    local args=("k" "${words[@]:1}")
    local requestComp="/opt/deckhouse/bin/d8 __complete ${args[*]}"
    local lastParam="${words[$((${#words[@]}-1))]}"
    local lastChar="${lastParam:$((${#lastParam}-1)):1}"
    if [[ -z "$cur" && "$lastChar" != "=" ]]; then
        requestComp="${requestComp} \"\""
    fi
    local out
    out=$(eval "${requestComp}" 2>/dev/null)
    local completions=()
    while IFS='' read -r line; do
        [[ "$line" =~ ^:[0-9]+$ ]] && continue
        [[ "$line" =~ ^Completion ]] && continue
        [[ -z "$line" ]] && continue
        completions+=("${line%%$'\t'*}")
    done <<< "$out"
    COMPREPLY=()
    if [[ ${#completions[@]} -gt 0 ]]; then
        local IFS=$'\n'
        COMPREPLY=($(compgen -W "${completions[*]}" -- "$cur"))
    fi
}
complete -o default -F __start_kubectl kubectl
EOF

if [ -f /etc/bash_completion.d/kubectl ]; then
  rm -f /etc/bash_completion.d/kubectl
fi

if ! type kubectl >/dev/null 2>&1; then
  cat <<'EOF' > /opt/deckhouse/bin/kubectl
#!/bin/bash
exec /opt/deckhouse/bin/d8 k "$@"
EOF
  chmod +x /opt/deckhouse/bin/kubectl
fi

if command -v d8 >/dev/null 2>&1; then
  alias_line='alias kubectl="/opt/deckhouse/bin/d8 k"'
  if ! grep -qF -- "$alias_line" /root/.bashrc; then
    echo "$alias_line" >> /root/.bashrc
  fi
fi
