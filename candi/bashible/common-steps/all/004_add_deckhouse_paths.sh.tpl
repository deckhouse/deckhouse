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


_chmod_dh_bin_path() {
for f in /etc/bashrc.d/02-deckhouse-path.sh /etc/profile.d/02-deckhouse-path.sh; do
  [ -f "$f" ] && chmod +x "$f"
done
}

bb-event-on 'bb-sync-file-changed' '_chmod_dh_bin_path'

bb-sync-file /etc/profile.d/02-deckhouse-path.sh - << "EOF"
export PATH="/opt/deckhouse/bin:$PATH"
EOF

if [[ $(bb-is-bundle) == "altlinux" ]]; then
bb-sync-file /etc/bashrc.d/02-deckhouse-path.sh - << "EOF"
PROMPT_COMMAND='
  if [ -z "$__deckhouse_path" ]; then
    case ":$PATH:" in
      *:/opt/deckhouse/bin:*) ;;
      *) PATH="/opt/deckhouse/bin:$PATH" ;;
    esac
    __deckhouse_path=1
  fi
'"$PROMPT_COMMAND"
EOF
fi
